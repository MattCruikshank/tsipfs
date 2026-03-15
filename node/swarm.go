package node

import (
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/libp2p/go-libp2p/core/pnet"
)

// FormatSwarmKey takes a hex-encoded 32-byte key and returns a reader
// in the standard IPFS swarm key file format.
func FormatSwarmKey(hexKey string) (io.Reader, error) {
	hexKey = strings.TrimSpace(hexKey)
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("decoding hex swarm key: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("swarm key must be 32 bytes, got %d", len(keyBytes))
	}
	formatted := fmt.Sprintf("/key/swarm/psk/1.0.0/\n/base16/\n%s\n", hexKey)
	return strings.NewReader(formatted), nil
}

// ParseSwarmKey takes a hex-encoded swarm key string and returns a PSK.
func ParseSwarmKey(hexKey string) (pnet.PSK, error) {
	reader, err := FormatSwarmKey(hexKey)
	if err != nil {
		return nil, err
	}
	psk, err := pnet.DecodeV1PSK(reader)
	if err != nil {
		return nil, fmt.Errorf("decoding PSK: %w", err)
	}
	return psk, nil
}
