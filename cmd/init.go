package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init <hostname> <swarm-key> [bootstrap-peers...]",
	Short: "Create a .env file for this node",
	Long: `Creates a .env file with your node configuration. Run this once, then use 'tsipfs run' from now on.

Example:
  tsipfs init my-node a3f8b2c1... /dns4/peer1/tcp/443/p2p/12D3KooW...

The swarm key and bootstrap peers can be pasted from another node's admin UI (Copy all).`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		hostname := args[0]
		swarmKey := args[1]
		bootstrapPeers := args[2:]

		// Check if .env already exists
		if _, err := os.Stat(".env"); err == nil {
			return fmt.Errorf(".env already exists. Remove it first if you want to reinitialize")
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("TSIPFS_SWARM_KEY=%s\n", swarmKey))
		sb.WriteString(fmt.Sprintf("TSIPFS_FUNNEL_HOSTNAME=%s\n", hostname))

		if len(bootstrapPeers) > 0 {
			sb.WriteString(fmt.Sprintf("TSIPFS_BOOTSTRAP_PEERS=%s\n", strings.Join(bootstrapPeers, ",")))
		}

		if err := os.WriteFile(".env", []byte(sb.String()), 0o600); err != nil {
			return fmt.Errorf("writing .env: %w", err)
		}

		fmt.Println("Created .env:")
		fmt.Printf("  Hostname:        %s\n", hostname)
		fmt.Printf("  Swarm key:       %s...%s\n", swarmKey[:8], swarmKey[len(swarmKey)-8:])
		if len(bootstrapPeers) > 0 {
			fmt.Printf("  Bootstrap peers: %d\n", len(bootstrapPeers))
		}
		fmt.Println()
		fmt.Println("Run 'tsipfs run' to start your node.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
