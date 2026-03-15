package node

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// CacheEvictor monitors cache disk usage and evicts the oldest blocks
// when the cache exceeds its size limit. It uses file modification time
// as a proxy for LRU ordering.
type CacheEvictor struct {
	cacheDir string
	maxBytes int64
	interval time.Duration

	mu sync.Mutex
}

// NewCacheEvictor creates a new evictor. If maxBytes is 0, eviction is disabled.
func NewCacheEvictor(cacheDir string, maxBytes int64) *CacheEvictor {
	return &CacheEvictor{
		cacheDir: cacheDir,
		maxBytes: maxBytes,
		interval: 60 * time.Second,
	}
}

// Run starts the eviction loop. It blocks until ctx is cancelled.
func (e *CacheEvictor) Run(ctx context.Context) {
	if e.maxBytes <= 0 {
		log.Println("cache eviction disabled (no size limit)")
		return
	}

	log.Printf("cache evictor started (limit: %s, check interval: %s)", humanBytes(e.maxBytes), e.interval)

	// Run immediately on start
	e.evictIfNeeded()

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("cache evictor stopped")
			return
		case <-ticker.C:
			e.evictIfNeeded()
		}
	}
}

type fileEntry struct {
	path    string
	size    int64
	modTime time.Time
}

func (e *CacheEvictor) evictIfNeeded() {
	if !e.mu.TryLock() {
		return // already running
	}
	defer e.mu.Unlock()

	files, totalSize, err := e.scanCache()
	if err != nil {
		log.Printf("cache evictor: scan error: %v", err)
		return
	}

	if totalSize <= e.maxBytes {
		return
	}

	// Sort by modification time, oldest first
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	evicted := 0
	evictedBytes := int64(0)
	target := totalSize - e.maxBytes

	for _, f := range files {
		if evictedBytes >= target {
			break
		}
		if err := os.Remove(f.path); err != nil {
			continue
		}
		evictedBytes += f.size
		evicted++
	}

	if evicted > 0 {
		log.Printf("cache evictor: removed %d blocks (%s), cache was %s, limit %s",
			evicted, humanBytes(evictedBytes), humanBytes(totalSize), humanBytes(e.maxBytes))
	}
}

func (e *CacheEvictor) scanCache() ([]fileEntry, int64, error) {
	var files []fileEntry
	var totalSize int64

	err := filepath.Walk(e.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, fileEntry{
			path:    path,
			size:    info.Size(),
			modTime: info.ModTime(),
		})
		totalSize += info.Size()
		return nil
	})

	if os.IsNotExist(err) {
		return nil, 0, nil
	}
	return files, totalSize, err
}

func humanBytes(b int64) string {
	const (
		kb = 1 << 10
		mb = 1 << 20
		gb = 1 << 30
		tb = 1 << 40
	)
	switch {
	case b >= tb:
		return formatFloat(float64(b)/float64(tb), "TB")
	case b >= gb:
		return formatFloat(float64(b)/float64(gb), "GB")
	case b >= mb:
		return formatFloat(float64(b)/float64(mb), "MB")
	case b >= kb:
		return formatFloat(float64(b)/float64(kb), "KB")
	default:
		return formatFloat(float64(b), "B")
	}
}

func formatFloat(v float64, unit string) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d %s", int64(v), unit)
	}
	return fmt.Sprintf("%.1f %s", v, unit)
}
