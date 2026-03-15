package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"
)

var generateSwarmKeyCmd = &cobra.Command{
	Use:   "generate-swarm-key",
	Short: "Generate a new IPFS swarm key",
	Long:  "Generates a random 32-byte swarm key and prints it as a hex string. Share this key with all nodes in your private IPFS network.",
	RunE: func(cmd *cobra.Command, args []string) error {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return fmt.Errorf("generating random key: %w", err)
		}
		fmt.Println(hex.EncodeToString(key))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateSwarmKeyCmd)
}
