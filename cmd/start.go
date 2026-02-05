package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/demuth666/distributedfilestoragego/server"
	"github.com/demuth666/distributedfilestoragego/store"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the file server",
	Long:  `Start the distributed file storage server and begin accepting connections.`,
	Run: func(cmd *cobra.Command, args []string) {
		encKey := loadOrCreateEncryptionKey()

		opts := server.FileServerOpts{
			ListenAddr:        listenAddr,
			StorageRoot:       storageRoot,
			PathTransformFunc: store.CASPathTransformFunc,
			EncryptionKey:     encKey,
			BootstrapNodes:    bootstrap,
			ReplicationFactor: replication,
		}

		fs := server.NewFileServer(opts)

		// Handle graceful shutdown
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigCh
			fmt.Println("\nShutting down...")
			fs.Stop()
			os.Exit(0)
		}()

		go func() {
			api := server.NewAPI(fs, apiAddr)
			if err := api.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "API error: %v\n", err)
			}
		}()

		fmt.Printf("Starting server on %s\n", listenAddr)
		fmt.Printf("Storage root: %s\n", storageRoot)
		fmt.Printf("Replication factor: %d\n", replication)
		fmt.Printf("API server on %s\n", apiAddr)
		if len(bootstrap) > 0 {
			fmt.Printf("Bootstrap nodes: %v\n", bootstrap)
		}

		if err := fs.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
			os.Exit(1)
		}
	},
}

func loadOrCreateEncryptionKey() []byte {
	if encKeyFile == "" {
		// Generate a new key if no file specified
		fmt.Println("No encryption key file specified, generating new key...")
		return store.NewEncryptionKey()
	}

	// Try to read existing key
	data, err := os.ReadFile(encKeyFile)
	if err == nil && len(data) == 32 {
		fmt.Printf("Loaded encryption key from %s\n", encKeyFile)
		return data
	}

	// Generate and save new key
	key := store.NewEncryptionKey()
	if err := os.WriteFile(encKeyFile, key, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save encryption key: %v\n", err)
	} else {
		fmt.Printf("Generated and saved new encryption key to %s\n", encKeyFile)
	}
	return key
}

func init() {
	rootCmd.AddCommand(startCmd)
}
