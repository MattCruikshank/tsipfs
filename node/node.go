package node

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/ipfs/boxo/bitswap"
	bsnet "github.com/ipfs/boxo/bitswap/network/bsnet"
	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/gateway"
	merkledag "github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/pinning/pinner/dspinner"
	pinner "github.com/ipfs/boxo/pinning/pinner"
	ds "github.com/ipfs/go-datastore"
	ipld "github.com/ipfs/go-ipld-format"
	leveldb "github.com/ipfs/go-ds-leveldb"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/MattCruikshank/tsipfs/config"
)

// Node is the core IPFS node, owning all subsystems.
type Node struct {
	Host         host.Host
	DHT          *dht.IpfsDHT
	Bitswap      *bitswap.Bitswap
	BlockService blockservice.BlockService
	DAGService   ipld.DAGService
	Pinner       pinner.Pinner
	Blockstore   *SplitBlockstore
	PinMeta      *PinMeta
	DataDir      string
	SwarmKey     string

	pinnedDS ds.Batching
	cacheDS  ds.Batching
	pinnerDS ds.Batching
	dhtDS    ds.Batching
}

// Start creates and wires all IPFS subsystems.
func Start(ctx context.Context, cfg *config.Config) (*Node, error) {
	// 1. Parse swarm key
	psk, err := ParseSwarmKey(cfg.SwarmKey)
	if err != nil {
		return nil, fmt.Errorf("parsing swarm key: %w", err)
	}
	log.Println("swarm key loaded")

	// 2. Open split blockstore (flatfs — good for single-segment CID keys)
	splitBS, pinnedDS, cacheDS, err := OpenSplitBlockstore(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("opening blockstore: %w", err)
	}

	// 3. Generate or load node identity
	privKey, err := loadOrCreateIdentity(cfg.DataDir)
	if err != nil {
		pinnedDS.Close()
		cacheDS.Close()
		return nil, fmt.Errorf("loading identity: %w", err)
	}

	// 4. Create libp2p host with private network
	h, err := libp2p.New(
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/4001"),
		libp2p.PrivateNetwork(psk),
	)
	if err != nil {
		pinnedDS.Close()
		cacheDS.Close()
		return nil, fmt.Errorf("creating libp2p host: %w", err)
	}
	log.Printf("libp2p host started: %s", h.ID())
	for _, addr := range h.Addrs() {
		log.Printf("  listening on %s/p2p/%s", addr, h.ID())
	}

	// 5. Create DHT for content routing (leveldb — supports hierarchical keys)
	dhtDS, err := leveldb.NewDatastore(filepath.Join(cfg.DataDir, "dht"), nil)
	if err != nil {
		h.Close()
		pinnedDS.Close()
		cacheDS.Close()
		return nil, fmt.Errorf("opening DHT datastore: %w", err)
	}

	ipfsDHT, err := dht.New(ctx, h,
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix("/tsipfs"),
		dht.Datastore(dhtDS),
	)
	if err != nil {
		h.Close()
		pinnedDS.Close()
		cacheDS.Close()
		dhtDS.Close()
		return nil, fmt.Errorf("creating DHT: %w", err)
	}
	log.Println("DHT created in server mode")

	// Bootstrap DHT
	if err := ipfsDHT.Bootstrap(ctx); err != nil {
		log.Printf("warning: DHT bootstrap failed: %v", err)
	}

	// Connect to bootstrap peers (from env + saved file)
	allPeers := append(cfg.BootstrapPeers, LoadSavedBootstrapPeers(cfg.DataDir)...)
	if len(allPeers) > 0 {
		connectBootstrapPeers(ctx, h, allPeers)
	}

	// 6. Create bitswap
	bsNetwork := bsnet.NewFromIpfsHost(h)
	bswap := bitswap.New(ctx, bsNetwork, ipfsDHT, splitBS)
	log.Println("bitswap started")

	// 7. Block service and DAG service
	bserv := blockservice.New(splitBS, bswap)
	dagServ := merkledag.NewDAGService(bserv)

	// 8. Pinner (leveldb — supports hierarchical keys like /pins/state/dirty)
	pinnerDSPath := filepath.Join(cfg.DataDir, "pins")
	pinnerLDB, err := leveldb.NewDatastore(pinnerDSPath, nil)
	if err != nil {
		bswap.Close()
		ipfsDHT.Close()
		h.Close()
		pinnedDS.Close()
		cacheDS.Close()
		dhtDS.Close()
		return nil, fmt.Errorf("opening pinner datastore: %w", err)
	}

	p, err := dspinner.New(ctx, pinnerLDB, dagServ)
	if err != nil {
		pinnerLDB.Close()
		bswap.Close()
		ipfsDHT.Close()
		h.Close()
		pinnedDS.Close()
		cacheDS.Close()
		dhtDS.Close()
		return nil, fmt.Errorf("creating pinner: %w", err)
	}
	log.Println("pinner initialized")

	// 9. Pin metadata (timestamps)
	pinMeta, err := NewPinMeta(cfg.DataDir)
	if err != nil {
		log.Printf("warning: pin metadata unavailable: %v", err)
	}

	return &Node{
		Host:         h,
		DHT:          ipfsDHT,
		Bitswap:      bswap,
		BlockService: bserv,
		DAGService:   dagServ,
		Pinner:       p,
		Blockstore:   splitBS,
		PinMeta:      pinMeta,
		DataDir:      cfg.DataDir,
		SwarmKey:     cfg.SwarmKey,
		pinnedDS:     pinnedDS,
		cacheDS:      cacheDS,
		pinnerDS:     pinnerLDB,
		dhtDS:        dhtDS,
	}, nil
}

// GatewayHandler returns an http.Handler for the public IPFS gateway.
func (n *Node) GatewayHandler() (http.Handler, error) {
	backend, err := gateway.NewBlocksBackend(n.BlockService,
		gateway.WithValueStore(n.DHT),
	)
	if err != nil {
		return nil, fmt.Errorf("creating gateway backend: %w", err)
	}

	gwHandler := gateway.NewHandler(gateway.Config{
		DeserializedResponses: true,
	}, backend)

	mux := http.NewServeMux()
	mux.Handle("/ipfs/", gwHandler)
	return mux, nil
}

// Close shuts down all subsystems in reverse order.
func (n *Node) Close() error {
	log.Println("shutting down IPFS node...")
	n.BlockService.Close()
	n.Bitswap.Close()
	if err := n.DHT.Close(); err != nil {
		log.Printf("warning: DHT close error: %v", err)
	}
	if err := n.Host.Close(); err != nil {
		log.Printf("warning: host close error: %v", err)
	}
	if closer, ok := n.pinnedDS.(interface{ Close() error }); ok {
		closer.Close()
	}
	if closer, ok := n.cacheDS.(interface{ Close() error }); ok {
		closer.Close()
	}
	if closer, ok := n.pinnerDS.(interface{ Close() error }); ok {
		closer.Close()
	}
	if closer, ok := n.dhtDS.(interface{ Close() error }); ok {
		closer.Close()
	}
	log.Println("IPFS node stopped")
	return nil
}

// loadOrCreateIdentity loads a libp2p private key from disk, or generates one.
func loadOrCreateIdentity(dataDir string) (crypto.PrivKey, error) {
	keyPath := filepath.Join(dataDir, "identity.key")

	keyBytes, err := readFile(keyPath)
	if err == nil {
		priv, err := crypto.UnmarshalPrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("unmarshaling identity key: %w", err)
		}
		return priv, nil
	}

	// Generate new Ed25519 key
	priv, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		return nil, fmt.Errorf("generating identity key: %w", err)
	}

	raw, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshaling identity key: %w", err)
	}

	if err := writeFile(keyPath, raw); err != nil {
		return nil, fmt.Errorf("saving identity key: %w", err)
	}

	log.Println("generated new node identity")
	return priv, nil
}

// connectBootstrapPeers attempts to connect to the configured bootstrap peers.
func connectBootstrapPeers(ctx context.Context, h host.Host, addrs []string) {
	for _, addr := range addrs {
		if addr == "" {
			continue
		}
		ai, err := peer.AddrInfoFromString(addr)
		if err != nil {
			log.Printf("warning: invalid bootstrap peer %q: %v", addr, err)
			continue
		}
		go func(pi peer.AddrInfo) {
			if err := h.Connect(ctx, pi); err != nil {
				log.Printf("warning: failed to connect to bootstrap peer %s: %v", pi.ID, err)
			} else {
				log.Printf("connected to bootstrap peer %s", pi.ID)
			}
		}(*ai)
	}
}
