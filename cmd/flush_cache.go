package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MattCruikshank/tsipfs/config"
	"github.com/spf13/cobra"
)

var flushCacheCmd = &cobra.Command{
	Use:   "flush-cache",
	Short: "Delete all cached (non-pinned) content",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		cacheDir := filepath.Join(cfg.DataDir, "cache")
		if err := os.RemoveAll(cacheDir); err != nil {
			return fmt.Errorf("removing cache directory: %w", err)
		}
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			return fmt.Errorf("recreating cache directory: %w", err)
		}

		fmt.Println("Cache flushed successfully.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(flushCacheCmd)
}
