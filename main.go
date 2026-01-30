package main

import (
	"fmt"
	"io"
	"strings"
	"time"
)

func main() {
	// Shared encryption key for all servers
	encryptionKey := NewEncryptionKey()

	// Server 1
	server1 := NewFileServer(FileServerOpts{
		ListenAddr:        ":3000",
		StorageRoot:       "3000_data",
		PathTransformFunc: CASPathTransformFunc,
		EncryptionKey:     encryptionKey,
		BootstrapNodes:    []string{}, // First node, no bootstrap
	})
	go server1.Start()

	data := strings.NewReader("Black monkye mother fucker!")
	key, err := server1.StoreFile("myfile", data)
	if err != nil {
		fmt.Println("Error storing file:", err)
	}

	time.Sleep(time.Second)

	// Server 2 connects to Server 1
	server2 := NewFileServer(FileServerOpts{
		ListenAddr:        ":4000",
		StorageRoot:       "4000_data",
		PathTransformFunc: CASPathTransformFunc,
		EncryptionKey:     encryptionKey,     // Same key as server1
		BootstrapNodes:    []string{":3000"}, // Connect to server1
	})
	go server2.Start()

	time.Sleep(2 * time.Second)

	fmt.Println("Server 2 requesting file from network...")
	reader, err := server2.GetFile(key)
	if err != nil {
		fmt.Println("Getfile error:", err)
		return
	}
	defer reader.Close()

	content, _ := io.ReadAll(reader)
	fmt.Printf("Server2 received: %s\n", string(content))

	fmt.Println("Done!")
	select {}
}
