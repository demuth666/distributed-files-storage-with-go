package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	listenAddr  string
	storageRoot string
	bootstrap   []string
	replication int
	encKeyFile  string
)

var apiAddr string

var rootCmd = &cobra.Command{
	Use:   "dfs",
	Short: "Distributed File Storage - A peer-to-peer file storage system",
	Long: `A distributed file storage system that provides:
- Content-addressed storage using SHA-256 hashing
- AES-256 encryption at rest
- Peer-to-peer file replication
- Automatic re-replication when peers disconnect`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&listenAddr, "addr", "a", ":3000", "Listen address for the server")
	rootCmd.PersistentFlags().StringVarP(&storageRoot, "storage", "s", "./dfs_data", "Storage root directory")
	rootCmd.PersistentFlags().StringSliceVarP(&bootstrap, "bootstrap", "b", []string{}, "Bootstrap node addresses (comma-separated)")
	rootCmd.PersistentFlags().IntVarP(&replication, "replication", "r", 3, "Replication factor")
	rootCmd.PersistentFlags().StringVarP(&encKeyFile, "key", "k", "", "Encryption key file path")
	rootCmd.PersistentFlags().StringVar(&apiAddr, "api", ":3001", "HTTP API address for CLI commands")
}
