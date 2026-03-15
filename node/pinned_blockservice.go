package node

import (
	"context"

	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/blockservice"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

// pinnedBlockstore wraps a SplitBlockstore to route all writes to the pinned store.
type pinnedBlockstore struct {
	split *SplitBlockstore
}

func (p *pinnedBlockstore) DeleteBlock(ctx context.Context, c cid.Cid) error {
	return p.split.DeleteBlock(ctx, c)
}

func (p *pinnedBlockstore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	return p.split.Has(ctx, c)
}

func (p *pinnedBlockstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	return p.split.Get(ctx, c)
}

func (p *pinnedBlockstore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	return p.split.GetSize(ctx, c)
}

func (p *pinnedBlockstore) Put(ctx context.Context, blk blocks.Block) error {
	return p.split.PinPut(ctx, blk)
}

func (p *pinnedBlockstore) PutMany(ctx context.Context, blks []blocks.Block) error {
	return p.split.PinPutMany(ctx, blks)
}

func (p *pinnedBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return p.split.AllKeysChan(ctx)
}

// NewPinnedBlockService creates a blockservice that writes to the pinned store.
func NewPinnedBlockService(split *SplitBlockstore, exchange *bitswap.Bitswap) blockservice.BlockService {
	return blockservice.New(&pinnedBlockstore{split: split}, exchange)
}
