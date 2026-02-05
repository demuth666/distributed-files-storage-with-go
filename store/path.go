package store

import (
	"path/filepath"
	"strings"
)

type PathKey struct {
	Pathname string
	Filename string
}

type PathTransformFunc func(key string) PathKey

func DefaultPathTransformFunc(key string) PathKey {
	return PathKey{
		Pathname: "",
		Filename: key,
	}
}

func CASPathTransformFunc(key string) PathKey {
	var paths []string

	chunkSize := 5

	for i := 0; i < len(key); i += chunkSize {
		end := i + chunkSize
		if end > len(key) {
			end = len(key)
		}

		chunk := key[i:end]
		paths = append(paths, chunk)
	}

	pathName := filepath.Join(paths[:len(paths)-1]...)
	fileName := paths[len(paths)-1]

	return PathKey{
		Pathname: pathName,
		Filename: fileName,
	}
}

func (p PathKey) FullPath() string {
	return filepath.Join(p.Pathname, p.Filename)
}

func (p PathKey) FirstPathComponent() string {
	parts := strings.Split(p.Pathname, string(filepath.Separator))
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
