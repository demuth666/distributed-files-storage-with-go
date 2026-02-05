package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <key> <output-path>",
	Short: "Retrieve a file from the network",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		outputPath := args[1]
		url := fmt.Sprintf("http://localhost%s/get/%s", apiAddr, key)

		resp, _ := http.Get(url)

		outFile, _ := os.Create(outputPath)
		defer outFile.Close()
		io.Copy(outFile, resp.Body)

		fmt.Printf("File saved to %s\n", outputPath)
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}
