package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type FileServerOpts struct {
	ListenAddr        string
	StorageRoot       string
	PathTransformFunc PathTransformFunc
	EncryptionKey     []byte
	BootstrapNodes    []string
}

type FileServer struct {
	opts      FileServerOpts
	store     *Store
	transport *TCPTransport

	mu    sync.RWMutex
	peers map[net.Addr]*TCPPeer

	quitCh chan struct{}
}

func NewFileServer(opts FileServerOpts) *FileServer {
	store := NewStore(StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
		EncryptionKey:     opts.EncryptionKey,
	})

	transport := NewTCPTransport(opts.ListenAddr)
	fs := &FileServer{
		opts:      opts,
		store:     store,
		transport: transport,
		peers:     make(map[net.Addr]*TCPPeer),
		quitCh:    make(chan struct{}),
	}

	transport.OnPeer = fs.OnPeer

	return fs
}

func (s *FileServer) Start() error {
	if err := s.transport.ListenAndAccept(); err != nil {
		return err
	}

	s.bootstrapNetwork()
	s.loop()

	return nil
}

func (s *FileServer) Stop() {
	close(s.quitCh)
}

func (s *FileServer) OnPeer(peer *TCPPeer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers[peer.RemoteAddr()] = peer
	fmt.Printf("[%s] New peer connected: %s\n", s.opts.ListenAddr, peer.RemoteAddr())
	return nil
}

func (s *FileServer) StoreFile(key string, r io.Reader) (string, error) {
	buf := new(bytes.Buffer)
	tee := io.TeeReader(r, buf)

	casKey, err := s.store.Write(tee)
	if err != nil {
		return "", err
	}

	fmt.Printf("[%s] Stored file: %s\n", s.opts.ListenAddr, casKey.Hash)

	msg := &Message{
		Type: MessageTypeStoreFile,
		Payload: MessageStoreFile{
			Key:  casKey.Hash,
			Data: buf.Bytes(),
		},
	}

	if err := s.broadcast(msg); err != nil {
		return "", err
	}

	return casKey.Hash, nil
}

func (s *FileServer) GetFile(key string) (io.ReadCloser, error) {
	if s.store.Has(key) {
		return s.store.Read(key)
	}

	msg := &Message{
		Type: MessageTypeGetFile,
		Payload: MessageGetFile{
			Key: key,
		},
	}

	s.broadcast(msg)

	time.Sleep(500 * time.Millisecond)

	if !s.store.Has(key) {
		return nil, errors.New("file not found")
	}

	return s.store.Read(key)
}

func (s *FileServer) broadcast(msg *Message) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for addr, peer := range s.peers {
		if err := msg.Encode(peer); err != nil {
			fmt.Printf("[%s] Failed to send to %s: %v\n", s.opts.ListenAddr, addr, err)
			continue
		}
	}

	return nil
}

func (s *FileServer) loop() {
	for {
		select {
		case rpc := <-s.transport.Consume():
			go s.handlePeerMessages(rpc.Peer)
		case <-s.quitCh:
			return
		}
	}
}

func (s *FileServer) handleMessage(msg *Message, peer *TCPPeer) error {
	switch v := msg.Payload.(type) {
	case MessageStoreFile:
		fmt.Printf("[%s] Received file: %s (%d bytes)\n", s.opts.ListenAddr, v.Key, len(v.Data))
		// Use WriteWithKey to preserve the original key from the sender
		return s.store.WriteWithKey(v.Key, bytes.NewReader(v.Data))
	case MessageGetFile:
		fmt.Printf("[%s] Peer requested file: %s\n", s.opts.ListenAddr, v.Key)

		if !s.store.Has(v.Key) {
			return nil
		}

		reader, err := s.store.Read(v.Key)
		if err != nil {
			return err
		}
		defer reader.Close()

		data, err := io.ReadAll(reader)
		if err != nil {
			return err
		}

		response := &Message{
			Type: MessageTypeStoreFile,
			Payload: MessageStoreFile{
				Key:  v.Key,
				Data: data,
			},
		}

		return response.Encode(peer)
	}

	return nil
}

func (s *FileServer) handlePeerMessages(peer *TCPPeer) {
	for {
		msg := &Message{}
		if err := msg.Decode(peer); err != nil {
			return
		}

		if err := s.handleMessage(msg, peer); err != nil {
			fmt.Printf("[%s] Error handling message: %v\n", s.opts.ListenAddr, err)
		}
	}
}

func (s *FileServer) bootstrapNetwork() {
	for _, addr := range s.opts.BootstrapNodes {
		go func(addr string) {
			if err := s.transport.Dial(addr); err != nil {
				fmt.Printf("[%s] Failed to connect to %s: %v\n", s.opts.ListenAddr, addr, err)
			}
		}(addr)
	}
}
