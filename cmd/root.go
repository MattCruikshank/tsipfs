package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tsipfs",
	Short: "A private IPFS node with Tailscale Funnel gateway",
	Long:  "tsipfs runs a private IPFS node using a swarm key, exposes a public gateway via Tailscale Funnel, and provides a tailnet-only admin UI and REST API.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
