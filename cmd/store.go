package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var storeCmd = &cobra.Command{
	Use:   "store <filepath>",
	Short: "Store a file in the network",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filepath := args[0]
		url := fmt.Sprintf("http://localhost%s/store", apiAddr)

		file, err := os.Open(filepath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", filepath)
		io.Copy(part, file)
		writer.Close()

		resp, _ := http.Post(
			url,
			writer.FormDataContentType(),
			body,
		)

		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		fmt.Printf("Stored with key: %s\n", result["key"])
	},
}

func init() {
	rootCmd.AddCommand(storeCmd)
}
