package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
)

type StoreOpts struct {
	Root              string
	PathTransformFunc PathTransformFunc
	EncryptionKey     []byte
}

type Store struct {
	opts StoreOpts
}

func NewStore(opts StoreOpts) *Store {
	if opts.Root == "" {
		opts.Root = "storage"
	}
	if opts.PathTransformFunc == nil {
		opts.PathTransformFunc = DefaultPathTransformFunc
	}
	return &Store{
		opts: opts,
	}
}

func (s *Store) Write(r io.Reader) (CASKey, error) {
	buf := new(bytes.Buffer)

	if s.opts.EncryptionKey != nil {
		_, err := copyEncrypt(s.opts.EncryptionKey, buf, r)
		if err != nil {
			return CASKey{}, err
		}
	} else {
		io.Copy(buf, r)
	}

	data := buf.Bytes()
	key, err := GenerateKey(bytes.NewReader(data))
	if err != nil {
		return CASKey{}, err
	}

	pathKey := s.opts.PathTransformFunc(key.Hash)
	fullPath := filepath.Join(s.opts.Root, pathKey.FullPath())

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return CASKey{}, err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return CASKey{}, err
	}
	defer file.Close()

	if _, err := io.Copy(file, buf); err != nil {
		return CASKey{}, err
	}

	return key, nil
}

// WriteWithKey stores data with a specific key (used when receiving from network)
func (s *Store) WriteWithKey(key string, r io.Reader) error {
	buf := new(bytes.Buffer)

	if s.opts.EncryptionKey != nil {
		_, err := copyEncrypt(s.opts.EncryptionKey, buf, r)
		if err != nil {
			return err
		}
	} else {
		io.Copy(buf, r)
	}

	pathKey := s.opts.PathTransformFunc(key)
	fullPath := filepath.Join(s.opts.Root, pathKey.FullPath())

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, buf)
	return err
}

func (s *Store) Read(key string) (io.ReadCloser, error) {
	pathKey := s.opts.PathTransformFunc(key)
	fullPath := filepath.Join(s.opts.Root, pathKey.FullPath())

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}

	if s.opts.EncryptionKey == nil {
		return file, nil
	}

	var decrypted bytes.Buffer
	copyDecrypt(s.opts.EncryptionKey, &decrypted, file)
	file.Close()

	return io.NopCloser(bytes.NewReader(decrypted.Bytes())), nil
}

func (s *Store) Has(key string) bool {
	pathKey := s.opts.PathTransformFunc(key)
	fullPath := filepath.Join(s.opts.Root, pathKey.FullPath())

	_, err := os.Stat(fullPath)
	return err == nil
}

func (s *Store) Delete(key string) error {
	pathKey := s.opts.PathTransformFunc(key)
	firstCompnent := filepath.Join(s.opts.Root, pathKey.FirstPathComponent())

	return os.RemoveAll(firstCompnent)
}

func (s *Store) Clear() error {
	return os.RemoveAll(s.opts.Root)
}
