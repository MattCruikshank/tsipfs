package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	SwarmKey       string
	DataDir        string
	PinMaxSize     int64 // bytes, 0 = unlimited
	CacheMaxSize   int64 // bytes
	BootstrapPeers []string
	FunnelHostname string
	AdminPort      int
	TSAuthKey      string
}

func Load() (*Config, error) {
	// Load .env file if it exists (don't error if missing)
	_ = godotenv.Load()

	cfg := &Config{
		SwarmKey:       os.Getenv("TSIPFS_SWARM_KEY"),
		DataDir:        envOr("TSIPFS_DATA_DIR", "./data"),
		FunnelHostname: os.Getenv("TSIPFS_FUNNEL_HOSTNAME"),
		TSAuthKey:      os.Getenv("TS_AUTHKEY"),
	}

	var err error
	cfg.PinMaxSize, err = parseSize(envOr("TSIPFS_PIN_MAX_SIZE", "0"))
	if err != nil {
		return nil, fmt.Errorf("invalid TSIPFS_PIN_MAX_SIZE: %w", err)
	}

	cfg.CacheMaxSize, err = parseSize(envOr("TSIPFS_CACHE_MAX_SIZE", "10GB"))
	if err != nil {
		return nil, fmt.Errorf("invalid TSIPFS_CACHE_MAX_SIZE: %w", err)
	}

	cfg.AdminPort, err = strconv.Atoi(envOr("TSIPFS_ADMIN_PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid TSIPFS_ADMIN_PORT: %w", err)
	}

	if peers := os.Getenv("TSIPFS_BOOTSTRAP_PEERS"); peers != "" {
		cfg.BootstrapPeers = strings.Split(peers, ",")
		for i, p := range cfg.BootstrapPeers {
			cfg.BootstrapPeers[i] = strings.TrimSpace(p)
		}
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parseSize parses a human-readable size string like "10GB", "500MB", "1TB" into bytes.
// Also accepts plain integers as bytes.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}

	s = strings.ToUpper(s)

	multipliers := []struct {
		suffix string
		mult   int64
	}{
		{"TB", 1 << 40},
		{"GB", 1 << 30},
		{"MB", 1 << 20},
		{"KB", 1 << 10},
	}

	for _, m := range multipliers {
		if strings.HasSuffix(s, m.suffix) {
			numStr := strings.TrimSuffix(s, m.suffix)
			numStr = strings.TrimSpace(numStr)
			n, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid number %q in size %q", numStr, s)
			}
			return n * m.mult, nil
		}
	}

	// Plain number = bytes
	return strconv.ParseInt(s, 10, 64)
}
