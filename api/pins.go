package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	chunk "github.com/ipfs/boxo/chunker"
	merkledag "github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/ipld/unixfs/importer/balanced"
	"github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	"github.com/ipfs/go-cid"

	"github.com/MattCruikshank/tsipfs/node"
)

type PinHandler struct {
	Node *node.Node
}

type pinInfo struct {
	CID      string `json:"cid"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type"`
	PinnedAt string `json:"pinned_at,omitempty"`
}

// Upload handles POST /api/v1/pins — multipart file upload, adds and pins content.
func (h *PinHandler) Upload(w http.ResponseWriter, r *http.Request) {
	// Limit upload size to 4GB
	r.Body = http.MaxBytesReader(w, r.Body, 4<<30)

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("reading upload: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	name := r.FormValue("name")
	if name == "" {
		name = header.Filename
	}

	// Build UnixFS DAG using the pinned blockstore
	pinnedDAG := merkledag.NewDAGService(
		node.NewPinnedBlockService(h.Node.Blockstore, h.Node.Bitswap),
	)

	params := helpers.DagBuilderParams{
		Maxlinks: helpers.DefaultLinksPerBlock,
		Dagserv:  pinnedDAG,
	}
	splitter := chunk.DefaultSplitter(file)

	dbh, err := params.New(splitter)
	if err != nil {
		http.Error(w, fmt.Sprintf("creating DAG builder: %v", err), http.StatusInternalServerError)
		return
	}

	rootNode, err := balanced.Layout(dbh)
	if err != nil {
		http.Error(w, fmt.Sprintf("building DAG: %v", err), http.StatusInternalServerError)
		return
	}

	// Pin the root CID
	if err := h.Node.Pinner.Pin(r.Context(), rootNode, true, name); err != nil {
		http.Error(w, fmt.Sprintf("pinning: %v", err), http.StatusInternalServerError)
		return
	}
	if err := h.Node.Pinner.Flush(r.Context()); err != nil {
		log.Printf("warning: pinner flush error: %v", err)
	}

	rootCID := rootNode.Cid()
	if h.Node.PinMeta != nil {
		h.Node.PinMeta.Set(rootCID.String())
	}
	log.Printf("pinned %s (%s)", rootCID, name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pinInfo{
		CID:  rootCID.String(),
		Name: name,
		Type: "recursive",
	})
}

// List handles GET /api/v1/pins — list all pinned CIDs.
func (h *PinHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var pins []pinInfo

	// List recursive pins (channel-based API)
	for sp := range h.Node.Pinner.RecursiveKeys(ctx, true) {
		if sp.Err != nil {
			http.Error(w, fmt.Sprintf("listing recursive pins: %v", sp.Err), http.StatusInternalServerError)
			return
		}
		pi := pinInfo{CID: sp.Pin.Key.String(), Name: sp.Pin.Name, Type: "recursive"}
		if h.Node.PinMeta != nil {
			if m, ok := h.Node.PinMeta.Get(pi.CID); ok {
				pi.PinnedAt = m.PinnedAt.UTC().Format("2006-01-02T15:04:05Z")
			}
		}
		pins = append(pins, pi)
	}

	// List direct pins
	for sp := range h.Node.Pinner.DirectKeys(ctx, true) {
		if sp.Err != nil {
			http.Error(w, fmt.Sprintf("listing direct pins: %v", sp.Err), http.StatusInternalServerError)
			return
		}
		pi := pinInfo{CID: sp.Pin.Key.String(), Name: sp.Pin.Name, Type: "direct"}
		if h.Node.PinMeta != nil {
			if m, ok := h.Node.PinMeta.Get(pi.CID); ok {
				pi.PinnedAt = m.PinnedAt.UTC().Format("2006-01-02T15:04:05Z")
			}
		}
		pins = append(pins, pi)
	}

	if pins == nil {
		pins = []pinInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pins)
}

// Get handles GET /api/v1/pins/:cid — get pin details.
func (h *PinHandler) Get(w http.ResponseWriter, r *http.Request) {
	cidStr := r.PathValue("cid")
	c, err := cid.Decode(cidStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid CID: %v", err), http.StatusBadRequest)
		return
	}

	reason, pinned, err := h.Node.Pinner.IsPinned(r.Context(), c)
	if err != nil {
		http.Error(w, fmt.Sprintf("checking pin: %v", err), http.StatusInternalServerError)
		return
	}

	if !pinned {
		http.Error(w, "not pinned", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pinInfo{
		CID:  c.String(),
		Type: reason,
	})
}

// Delete handles DELETE /api/v1/pins/:cid — unpin content.
func (h *PinHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cidStr := r.PathValue("cid")
	c, err := cid.Decode(cidStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid CID: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.Node.Pinner.Unpin(r.Context(), c, true); err != nil {
		http.Error(w, fmt.Sprintf("unpinning: %v", err), http.StatusInternalServerError)
		return
	}
	if err := h.Node.Pinner.Flush(r.Context()); err != nil {
		log.Printf("warning: pinner flush error: %v", err)
	}

	if h.Node.PinMeta != nil {
		h.Node.PinMeta.Delete(c.String())
	}
	log.Printf("unpinned %s", c)
	w.WriteHeader(http.StatusNoContent)
}
