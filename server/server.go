package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/demuth666/distributedfilestoragego/p2p"
	"github.com/demuth666/distributedfilestoragego/store"
)

type FileServerOpts struct {
	ListenAddr        string
	StorageRoot       string
	PathTransformFunc store.PathTransformFunc
	EncryptionKey     []byte
	BootstrapNodes    []string
	ReplicationFactor int
}

type FileServer struct {
	opts      FileServerOpts
	store     *store.Store
	transport *p2p.TCPTransport

	mu    sync.RWMutex
	peers map[net.Addr]*p2p.TCPPeer

	fileLocations map[string]map[net.Addr]bool

	quitCh chan struct{}
}

func NewFileServer(opts FileServerOpts) *FileServer {
	st := store.NewStore(store.StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
		EncryptionKey:     opts.EncryptionKey,
	})

	transport := p2p.NewTCPTransport(opts.ListenAddr)
	fs := &FileServer{
		opts:          opts,
		store:         st,
		transport:     transport,
		peers:         make(map[net.Addr]*p2p.TCPPeer),
		fileLocations: make(map[string]map[net.Addr]bool),
		quitCh:        make(chan struct{}),
	}

	transport.OnPeer = fs.OnPeer

	return fs
}

func (s *FileServer) Start() error {
	if err := s.transport.ListenAndAccept(); err != nil {
		return err
	}

	s.bootstrapNetwork()
	go s.reconnectLoop()
	s.loop()

	return nil
}

func (s *FileServer) Stop() {
	close(s.quitCh)
}

func (s *FileServer) OnPeer(peer *p2p.TCPPeer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers[peer.RemoteAddr()] = peer
	fmt.Printf("[%s] Peer connected: %s\n", s.opts.ListenAddr, peer.RemoteAddr())
	return nil
}

func (s *FileServer) RemovePeer(peer *p2p.TCPPeer) {
	addr := peer.RemoteAddr()
	files := s.getFilesOnPeer(addr)

	s.mu.Lock()
	delete(s.peers, peer.RemoteAddr())

	for key := range s.fileLocations {
		delete(s.fileLocations[key], addr)
	}
	s.mu.Unlock()

	peer.Close()
	fmt.Printf("[%s] Peer disconnected: %s\n", s.opts.ListenAddr, peer.RemoteAddr())

	for _, key := range files {
		s.checkAndReplicate(key)
	}
}

func (s *FileServer) StoreFile(key string, r io.Reader) (string, error) {
	buf := new(bytes.Buffer)
	tee := io.TeeReader(r, buf)

	casKey, err := s.store.Write(tee)
	if err != nil {
		return "", err
	}

	fmt.Printf("[%s] Stored file: %s\n", s.opts.ListenAddr, casKey.Hash)

	peersNeeded := s.opts.ReplicationFactor - 1
	peers := s.selectPeersReplication(peersNeeded, nil)

	msg := &p2p.Message{
		Type: p2p.MessageTypeStoreFile,
		Payload: p2p.MessageStoreFile{
			Key:  casKey.Hash,
			Data: buf.Bytes(),
		},
	}

	for _, peer := range peers {
		if err := msg.Encode(peer); err != nil {
			fmt.Printf("[%s] Failed to send to %s: %v\n", s.opts.ListenAddr, peer.RemoteAddr(), err)
		}
	}

	return casKey.Hash, nil
}

func (s *FileServer) GetFile(key string) (io.ReadCloser, error) {
	if s.store.Has(key) {
		return s.store.Read(key)
	}

	msg := &p2p.Message{
		Type: p2p.MessageTypeGetFile,
		Payload: p2p.MessageGetFile{
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

func (s *FileServer) DeleteFile(key string) error {
	if err := s.store.Delete(key); err != nil {
		return err
	}

	msg := &p2p.Message{
		Type: p2p.MessageTypeDeleteFile,
		Payload: p2p.MessageDeleteFile{
			Key: key,
		},
	}

	if err := s.broadcast(msg); err != nil {
		return err
	}

	return nil
}

func (s *FileServer) ListPeers() []net.Addr {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addrs := make([]net.Addr, 0, len(s.peers))
	for addr := range s.peers {
		addrs = append(addrs, addr)
	}
	return addrs
}

func (s *FileServer) broadcast(msg *p2p.Message) error {
	s.mu.RLock()

	var failedPeers []*p2p.TCPPeer

	for addr, peer := range s.peers {
		if err := msg.Encode(peer); err != nil {
			fmt.Printf("[%s] Failed to send to %s: %v\n", s.opts.ListenAddr, addr, err)
			failedPeers = append(failedPeers, peer)
			continue
		}
	}

	s.mu.RUnlock()

	for _, peer := range failedPeers {
		s.RemovePeer(peer)
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

func (s *FileServer) reconnectLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.reconnectToBootstrapNodes()
		case <-s.quitCh:
			return
		}
	}
}

func (s *FileServer) reconnectToBootstrapNodes() {
	for _, addr := range s.opts.BootstrapNodes {
		if !s.isConnectedTo(addr) {
			fmt.Printf("[%s] Reconnecting to %s\n", s.opts.ListenAddr, addr)
			if err := s.transport.Dial(addr); err != nil {
				fmt.Printf("[%s] Failed to reconnect to %s: %v\n", s.opts.ListenAddr, addr, err)
			}
		}
	}
}

func (s *FileServer) isConnectedTo(addr string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for peerAddr := range s.peers {
		if peerAddr.String() == addr || peerAddr.String() == "127.0.0.1"+addr {
			return true
		}
	}
	return false
}

func (s *FileServer) handleMessage(msg *p2p.Message, peer *p2p.TCPPeer) error {
	switch v := msg.Payload.(type) {
	case p2p.MessageStoreFile:
		fmt.Printf("[%s] Received file: %s (%d bytes)\n", s.opts.ListenAddr, v.Key, len(v.Data))
		if err := s.store.WriteWithKey(v.Key, bytes.NewReader(v.Data)); err != nil {
			return err
		}

		ack := &p2p.Message{
			Type:    p2p.MessageTypeAck,
			Payload: p2p.MessageAck{Key: v.Key},
		}
		return ack.Encode(peer)
	case p2p.MessageGetFile:
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

		response := &p2p.Message{
			Type: p2p.MessageTypeStoreFile,
			Payload: p2p.MessageStoreFile{
				Key:  v.Key,
				Data: data,
			},
		}

		return response.Encode(peer)
	case p2p.MessageDeleteFile:
		fmt.Printf("[%s] Delete file: %s\n", s.opts.ListenAddr, v.Key)
		s.store.Delete(v.Key)
		return nil
	case p2p.MessageAck:
		fmt.Printf("[%s] Received ACK for file: %s from %s\n", s.opts.ListenAddr, v.Key, peer.RemoteAddr())

		s.addFileLocation(v.Key, peer.RemoteAddr())
		return nil
	}

	return nil
}

func (s *FileServer) handlePeerMessages(peer *p2p.TCPPeer) {
	defer func() {
		s.RemovePeer(peer)
	}()

	for {
		msg := &p2p.Message{}
		if err := msg.Decode(peer); err != nil {
			return
		}
		if err := s.handleMessage(msg, peer); err != nil {
			fmt.Printf("[%s] Error handling message: %v\n", s.opts.ListenAddr, err)
		}
	}
}

func (s *FileServer) addFileLocation(key string, addr net.Addr) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fileLocations[key] == nil {
		s.fileLocations[key] = make(map[net.Addr]bool)
	}
	s.fileLocations[key][addr] = true
}

func (s *FileServer) selectPeersReplication(count int, exclude map[net.Addr]bool) []*p2p.TCPPeer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var selected []*p2p.TCPPeer
	for addr, peer := range s.peers {
		if exclude != nil && exclude[addr] {
			continue
		}
		selected = append(selected, peer)
		if len(selected) >= count {
			break
		}
	}
	return selected
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

func (s *FileServer) getFilesOnPeer(addr net.Addr) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var files []string
	for key, locations := range s.fileLocations {
		if locations[addr] {
			files = append(files, key)
		}
	}
	return files
}

func (s *FileServer) getReplicationCount(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0

	if s.store.Has(key) {
		count = 1
	}

	if locations, ok := s.fileLocations[key]; ok {
		count += len(locations)
	}
	return count
}

func (s *FileServer) checkAndReplicate(key string) {
	count := s.getReplicationCount(key)

	if count < s.opts.ReplicationFactor {
		fmt.Printf("[%s] File %s has %d replicas (need %d), re-replicating...\n",
			s.opts.ListenAddr, key, count, s.opts.ReplicationFactor)

		// Only re-replicate if WE have the file
		if !s.store.Has(key) {
			return
		}

		// Get existing locations to exclude
		s.mu.RLock()
		exclude := s.fileLocations[key]
		s.mu.RUnlock()

		// Select new peers
		needed := s.opts.ReplicationFactor - count
		peers := s.selectPeersReplication(needed, exclude)

		if len(peers) == 0 {
			fmt.Printf("[%s] No available peers for re-replication\n", s.opts.ListenAddr)
			return
		}

		// Read file and send to new peers
		reader, err := s.store.Read(key)
		if err != nil {
			return
		}
		defer reader.Close()

		data, _ := io.ReadAll(reader)
		msg := &p2p.Message{
			Type:    p2p.MessageTypeStoreFile,
			Payload: p2p.MessageStoreFile{Key: key, Data: data},
		}

		for _, peer := range peers {
			msg.Encode(peer)
		}
	}
}
