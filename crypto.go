package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
)

type CASKey struct {
	Hash string
}

func GenerateKey(r io.Reader) (CASKey, error) {
	hash := sha256.New()

	if _, err := io.Copy(hash, r); err != nil {
		return CASKey{}, err
	}

	sum := hash.Sum(nil)
	result := hex.EncodeToString(sum)

	return CASKey{result}, nil
}

func NewEncryptionKey() []byte {
	key := make([]byte, 32)

	io.ReadFull(rand.Reader, key)

	return key
}

func copyEncrypt(key []byte, dst io.Writer, src io.Reader) (int, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, err
	}

	iv := make([]byte, block.BlockSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return 0, err
	}

	dst.Write(iv)

	stream := cipher.NewCTR(block, iv)
	streamWriter := cipher.StreamWriter{S: stream, W: dst}
	n, err := io.Copy(streamWriter, src)
	return int(n), err
}

func copyDecrypt(key []byte, dst io.Writer, src io.Reader) (int, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, err
	}

	iv := make([]byte, block.BlockSize())
	if _, err := io.ReadFull(src, iv); err != nil {
		return 0, err
	}

	stream := cipher.NewCTR(block, iv)
	streamReader := cipher.StreamReader{S: stream, R: src}

	n, err := io.Copy(dst, streamReader)
	return int(n), err
}
