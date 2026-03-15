package node

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
)

const bootstrapFile = "bootstrap_peers.txt"

// LoadSavedBootstrapPeers reads multiaddrs from bootstrap_peers.txt in dataDir.
func LoadSavedBootstrapPeers(dataDir string) []string {
	path := filepath.Join(dataDir, bootstrapFile)
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var peers []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			peers = append(peers, line)
		}
	}
	return peers
}

// SaveBootstrapPeer appends a multiaddr to bootstrap_peers.txt.
func SaveBootstrapPeer(dataDir, multiaddr string) error {
	path := filepath.Join(dataDir, bootstrapFile)

	// Check for duplicates
	existing := LoadSavedBootstrapPeers(dataDir)
	for _, e := range existing {
		if e == multiaddr {
			return nil // already saved
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, multiaddr)
	return err
}

// ConnectPeer connects to a peer by multiaddr string and optionally saves it.
func (n *Node) ConnectPeer(ctx context.Context, multiaddr string, save bool) error {
	ai, err := peer.AddrInfoFromString(multiaddr)
	if err != nil {
		return fmt.Errorf("invalid multiaddr: %w", err)
	}

	if err := n.Host.Connect(ctx, *ai); err != nil {
		return fmt.Errorf("connecting to peer: %w", err)
	}
	log.Printf("connected to peer %s", ai.ID)

	if save {
		if err := SaveBootstrapPeer(n.DataDir, multiaddr); err != nil {
			log.Printf("warning: failed to save bootstrap peer: %v", err)
		}
	}

	return nil
}

// BootstrapMultiaddr returns this node's multiaddr for sharing with others.
// Uses the Funnel hostname so it's reachable from outside the tailnet.
func (n *Node) BootstrapMultiaddr(funnelHostname string) string {
	if funnelHostname == "" {
		return ""
	}
	return fmt.Sprintf("/dns4/%s/tcp/443/p2p/%s", funnelHostname, n.Host.ID())
}
