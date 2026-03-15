package node

import (
	"context"
	"fmt"
	"path/filepath"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	flatfs "github.com/ipfs/go-ds-flatfs"

	"github.com/ipfs/boxo/blockstore"
)

// SplitBlockstore routes blocks to either a pinned store or a cache store.
// Blocks added via PinPut go to the pinned store. All other writes (e.g. from
// bitswap) go to the cache store. Reads check both stores, pinned first.
type SplitBlockstore struct {
	pinned blockstore.Blockstore
	cache  blockstore.Blockstore
}

// OpenSplitBlockstore creates two flatfs-backed blockstores under dataDir.
func OpenSplitBlockstore(dataDir string) (*SplitBlockstore, ds.Batching, ds.Batching, error) {
	pinnedPath := filepath.Join(dataDir, "pinned")
	cachePath := filepath.Join(dataDir, "cache")

	pinnedDS, err := flatfs.CreateOrOpen(pinnedPath, flatfs.NextToLast(2), false)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("opening pinned datastore: %w", err)
	}

	cacheDS, err := flatfs.CreateOrOpen(cachePath, flatfs.NextToLast(2), false)
	if err != nil {
		pinnedDS.Close()
		return nil, nil, nil, fmt.Errorf("opening cache datastore: %w", err)
	}

	// NoPrefix() avoids adding a /blocks/ prefix to keys — flatfs doesn't
	// support path separators in keys, and each store is already dedicated
	// to a single purpose (pinned blocks or cached blocks).
	pinnedBS := blockstore.NewBlockstore(dssync.MutexWrap(pinnedDS), blockstore.NoPrefix())
	cacheBS := blockstore.NewBlockstore(dssync.MutexWrap(cacheDS), blockstore.NoPrefix())

	return &SplitBlockstore{
		pinned: pinnedBS,
		cache:  cacheBS,
	}, pinnedDS, cacheDS, nil
}

// PinPut stores a block in the pinned store. Used when explicitly adding content.
func (s *SplitBlockstore) PinPut(ctx context.Context, blk blocks.Block) error {
	return s.pinned.Put(ctx, blk)
}

// PinPutMany stores multiple blocks in the pinned store.
func (s *SplitBlockstore) PinPutMany(ctx context.Context, blks []blocks.Block) error {
	return s.pinned.PutMany(ctx, blks)
}

// Put stores a block in the cache store (default for bitswap/network content).
func (s *SplitBlockstore) Put(ctx context.Context, blk blocks.Block) error {
	// If already pinned, skip cache write.
	has, err := s.pinned.Has(ctx, blk.Cid())
	if err == nil && has {
		return nil
	}
	return s.cache.Put(ctx, blk)
}

// PutMany stores multiple blocks in the cache store.
func (s *SplitBlockstore) PutMany(ctx context.Context, blks []blocks.Block) error {
	return s.cache.PutMany(ctx, blks)
}

// Get retrieves a block, checking pinned store first.
func (s *SplitBlockstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	blk, err := s.pinned.Get(ctx, c)
	if err == nil {
		return blk, nil
	}
	return s.cache.Get(ctx, c)
}

// GetSize returns block size, checking pinned store first.
func (s *SplitBlockstore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	size, err := s.pinned.GetSize(ctx, c)
	if err == nil {
		return size, nil
	}
	return s.cache.GetSize(ctx, c)
}

// Has checks if a block exists in either store.
func (s *SplitBlockstore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	has, err := s.pinned.Has(ctx, c)
	if err == nil && has {
		return true, nil
	}
	return s.cache.Has(ctx, c)
}

// DeleteBlock removes a block from both stores.
func (s *SplitBlockstore) DeleteBlock(ctx context.Context, c cid.Cid) error {
	_ = s.pinned.DeleteBlock(ctx, c)
	return s.cache.DeleteBlock(ctx, c)
}

// AllKeysChan returns CIDs from both stores.
func (s *SplitBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	pinnedCh, err := s.pinned.AllKeysChan(ctx)
	if err != nil {
		return nil, err
	}
	cacheCh, err := s.cache.AllKeysChan(ctx)
	if err != nil {
		return nil, err
	}

	merged := make(chan cid.Cid)
	go func() {
		defer close(merged)
		seen := make(map[cid.Cid]struct{})
		for c := range pinnedCh {
			seen[c] = struct{}{}
			select {
			case merged <- c:
			case <-ctx.Done():
				return
			}
		}
		for c := range cacheCh {
			if _, ok := seen[c]; ok {
				continue
			}
			select {
			case merged <- c:
			case <-ctx.Done():
				return
			}
		}
	}()
	return merged, nil
}

// Verify interface compliance.
var _ blockstore.Blockstore = (*SplitBlockstore)(nil)
