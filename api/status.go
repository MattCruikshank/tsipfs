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
	Node      *node.Node
	StartTime time.Time
}

type nodeStatus struct {
	Status     string `json:"status"`
	PeerCount  int    `json:"peer_count"`
	PeerID     string `json:"peer_id"`
	Uptime     string `json:"uptime"`
	PinnedSize string `json:"pinned_size"`
	CacheSize  string `json:"cache_size"`
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
	})
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
