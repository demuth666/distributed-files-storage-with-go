package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show server status",
	Run: func(cmd *cobra.Command, args []string) {
		url := fmt.Sprintf("http://localhost%s/status", apiAddr)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to server: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)

		fmt.Println("Server Status:")
		fmt.Printf("  Listen Address:  %v\n", result["listen_addr"])
		fmt.Printf("  Storage Root:    %v\n", result["storage_root"])
		fmt.Printf("  Replication:     %v\n", result["replication"])
		fmt.Printf("  Connected Peers: %v\n", result["peer_count"])
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
