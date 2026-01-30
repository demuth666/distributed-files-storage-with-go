package main

import (
	"io"
	"net"
)

type TCPTransport struct {
	listenAddr string
	listener   net.Listener
	rpcch      chan RPC
	OnPeer     func(*TCPPeer) error
}

type TCPPeer struct {
	net.Conn
	outbound bool
}

type RPC struct {
	From   net.Addr
	Stream io.Reader
	Peer   *TCPPeer
}

func NewTCPTransport(listenAddr string) *TCPTransport {
	return &TCPTransport{
		listenAddr: listenAddr,
		rpcch:      make(chan RPC, 1024),
	}
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		Conn:     conn,
		outbound: outbound,
	}
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listener, err = net.Listen("tcp", t.listenAddr)
	if err != nil {
		return err
	}

	go t.acceptLoop()
	return nil
}

func (t *TCPTransport) acceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			continue
		}
		go t.handleConn(conn, false)
	}
}

func (p *TCPPeer) Send(data []byte) error {
	_, err := p.Write(data)
	return err
}

func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	go t.handleConn(conn, true)
	return nil
}

func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcch
}

func (t *TCPTransport) Close() error {
	return t.listener.Close()
}

func (t *TCPTransport) handleConn(conn net.Conn, outbound bool) {
	peer := NewTCPPeer(conn, outbound)

	if t.OnPeer != nil {
		if err := t.OnPeer(peer); err != nil {
			peer.Close()
			return
		}
	}

	t.rpcch <- RPC{
		From:   peer.RemoteAddr(),
		Stream: peer,
		Peer:   peer,
	}
}
