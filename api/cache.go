package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/MattCruikshank/tsipfs/node"
)

type CacheHandler struct {
	Node *node.Node
}

type cacheStatus struct {
	SizeBytes int64  `json:"size_bytes"`
	SizeHuman string `json:"size_human"`
}

// Flush handles POST /api/v1/cache/flush — wipe the cache directory.
func (h *CacheHandler) Flush(w http.ResponseWriter, r *http.Request) {
	cacheDir := filepath.Join(h.Node.DataDir, "cache")
	if err := os.RemoveAll(cacheDir); err != nil {
		http.Error(w, fmt.Sprintf("removing cache: %v", err), http.StatusInternalServerError)
		return
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		http.Error(w, fmt.Sprintf("recreating cache dir: %v", err), http.StatusInternalServerError)
		return
	}
	log.Println("cache flushed via API")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "flushed"})
}

// Status handles GET /api/v1/cache/status — cache usage stats.
func (h *CacheHandler) Status(w http.ResponseWriter, r *http.Request) {
	cacheDir := filepath.Join(h.Node.DataDir, "cache")
	size, err := dirSize(cacheDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("calculating cache size: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cacheStatus{
		SizeBytes: size,
		SizeHuman: humanSize(size),
	})
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return size, err
}

func humanSize(bytes int64) string {
	const (
		kb = 1 << 10
		mb = 1 << 20
		gb = 1 << 30
		tb = 1 << 40
	)
	switch {
	case bytes >= tb:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(tb))
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
