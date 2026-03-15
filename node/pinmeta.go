package node

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PinMeta stores supplementary metadata (like timestamps) for pins.
// The pinner itself doesn't track when pins were created.
type PinMeta struct {
	mu   sync.RWMutex
	path string
	data map[string]PinMetaEntry
}

type PinMetaEntry struct {
	PinnedAt time.Time `json:"pinned_at"`
}

func NewPinMeta(dataDir string) (*PinMeta, error) {
	p := filepath.Join(dataDir, "pin_meta.json")
	pm := &PinMeta{
		path: p,
		data: make(map[string]PinMetaEntry),
	}

	raw, err := os.ReadFile(p)
	if err == nil {
		json.Unmarshal(raw, &pm.data)
	}
	return pm, nil
}

func (pm *PinMeta) Set(cid string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.data[cid] = PinMetaEntry{PinnedAt: time.Now()}
	pm.save()
}

func (pm *PinMeta) Get(cid string) (PinMetaEntry, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	e, ok := pm.data[cid]
	return e, ok
}

func (pm *PinMeta) Delete(cid string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.data, cid)
	pm.save()
}

func (pm *PinMeta) save() {
	raw, err := json.Marshal(pm.data)
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(pm.path), 0o755)
	os.WriteFile(pm.path, raw, 0o644)
}
