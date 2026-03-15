package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/MattCruikshank/tsipfs/node"
)

type StatusHandler struct {
	Node       *node.Node
	StartTime  time.Time
	GatewayURL string // public Funnel URL, e.g. "https://tsipfs.tail1234.ts.net"
}

type nodeStatus struct {
	Status             string `json:"status"`
	PeerCount          int    `json:"peer_count"`
	PeerID             string `json:"peer_id"`
	Uptime             string `json:"uptime"`
	PinnedSize         string `json:"pinned_size"`
	CacheSize          string `json:"cache_size"`
	GatewayURL         string `json:"gateway_url"`
	BootstrapMultiaddr string `json:"bootstrap_multiaddr"`
}

type peerInfo struct {
	ID    string   `json:"id"`
	Addrs []string `json:"addrs"`
}

// NodeStatus handles GET /api/v1/status.
func (h *StatusHandler) NodeStatus(w http.ResponseWriter, r *http.Request) {
	peers := h.Node.Host.Network().Peers()

	pinnedSize, _ := dirSize(filepath.Join(h.Node.DataDir, "pinned"))
	cacheSize, _ := dirSize(filepath.Join(h.Node.DataDir, "cache"))

	uptime := "unknown"
	if !h.StartTime.IsZero() {
		uptime = time.Since(h.StartTime).Truncate(time.Second).String()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodeStatus{
		Status:     "ok",
		PeerCount:  len(peers),
		PeerID:     h.Node.Host.ID().String(),
		Uptime:     uptime,
		PinnedSize: humanSize(pinnedSize),
		CacheSize:  humanSize(cacheSize),
		GatewayURL:         h.GatewayURL,
		BootstrapMultiaddr: h.Node.BootstrapMultiaddr(h.funnelHost()),
	})
}

func (h *StatusHandler) funnelHost() string {
	if h.GatewayURL == "" {
		return ""
	}
	// Strip "https://" prefix
	host := h.GatewayURL
	if len(host) > 8 && host[:8] == "https://" {
		host = host[8:]
	}
	return host
}

// ConnectPeer handles POST /api/v1/peers/connect — connect to a bootstrap peer and save it.
func (h *StatusHandler) ConnectPeer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Multiaddr string `json:"multiaddr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Multiaddr == "" {
		http.Error(w, "multiaddr is required", http.StatusBadRequest)
		return
	}

	if err := h.Node.ConnectPeer(r.Context(), req.Multiaddr, true); err != nil {
		http.Error(w, fmt.Sprintf("connect failed: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "connected"})
}

// BootstrapList handles GET /api/v1/bootstrap — returns all known bootstrap multiaddrs.
func (h *StatusHandler) BootstrapList(w http.ResponseWriter, r *http.Request) {
	var addrs []string

	// This node's own address
	if own := h.Node.BootstrapMultiaddr(h.funnelHost()); own != "" {
		addrs = append(addrs, own)
	}

	// Saved bootstrap peers
	addrs = append(addrs, node.LoadSavedBootstrapPeers(h.Node.DataDir)...)

	if addrs == nil {
		addrs = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(addrs)
}

// Peers handles GET /api/v1/peers.
func (h *StatusHandler) Peers(w http.ResponseWriter, r *http.Request) {
	peers := h.Node.Host.Network().Peers()

	result := make([]peerInfo, 0, len(peers))
	for _, p := range peers {
		conns := h.Node.Host.Network().ConnsToPeer(p)
		addrs := make([]string, 0, len(conns))
		for _, c := range conns {
			addrs = append(addrs, fmt.Sprintf("%s", c.RemoteMultiaddr()))
		}
		result = append(result, peerInfo{
			ID:    p.String(),
			Addrs: addrs,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
