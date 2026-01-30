package main

import (
	"encoding/gob"
	"io"
)

type MessageType byte

const (
	MessageTypeStoreFile MessageType = iota + 1
	MessageTypeGetFile
)

type Message struct {
	Type    MessageType
	Payload any
}

type MessageStoreFile struct {
	Key  string
	Data []byte
}

type MessageGetFile struct {
	Key string
}

func init() {
	gob.Register(MessageStoreFile{})
	gob.Register(MessageGetFile{})
}

func (m *Message) Encode(w io.Writer) error {
	return gob.NewEncoder(w).Encode(m)
}

func (m *Message) Decode(r io.Reader) error {
	return gob.NewDecoder(r).Decode(m)
}
