package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/MattCruikshank/tsipfs/config"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <cid>",
	Short: "Download IPFS content by CID",
	Long:  "Downloads content from the IPFS network by CID and saves it to the current directory.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cidStr := args[0]

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		outPath := cidStr

		baseURL := fmt.Sprintf("http://localhost:%d", cfg.AdminPort)
		resp, err := http.Get(fmt.Sprintf("%s/ipfs/%s", baseURL, cidStr))
		if err != nil {
			return fmt.Errorf("connecting to tsipfs node (is it running?): %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
		}

		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer f.Close()

		n, err := io.Copy(f, resp.Body)
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}

		fmt.Printf("saved %s (%d bytes)\n", outPath, n)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}
