package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a file from the network",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]

		url := fmt.Sprintf("http://localhost%s/delete/%s", apiAddr, key)
		req, err := http.NewRequest(http.MethodDelete, url, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "Error: server returned %s\n", resp.Status)
			os.Exit(1)
		}

		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		fmt.Printf("Deleted: %s\n", key)
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
