package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var peersCmd = &cobra.Command{
	Use:   "peers",
	Short: "List connected peers",
	Run: func(cmd *cobra.Command, args []string) {
		url := fmt.Sprintf("http://localhost%s/peers", apiAddr)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		var result map[string][]string
		json.NewDecoder(resp.Body).Decode(&result)

		peers := result["peers"]
		if len(peers) == 0 {
			fmt.Println("No peers connected")
			return
		}

		fmt.Printf("Connected peers (%d):\n", len(peers))
		for _, p := range peers {
			fmt.Printf("  - %s\n", p)
		}
	},
}

func init() {
	rootCmd.AddCommand(peersCmd)
}
