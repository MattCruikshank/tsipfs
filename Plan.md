# tsipfs — Plan

## Overview

A single Go binary that runs a private IPFS node (using a swarm key) with public content serving via Tailscale Funnel and a private admin UI/API authenticated via Tailscale identity. Built on Boxo + go-libp2p for a lean, customizable implementation.

**Purpose:** Share Creative Commons, Public Domain, MIT-licensed, and other open game development assets across a distributed network of thousands of independently operated nodes.

---

## Architecture

```
┌───────────────────────────────────────────────────────┐
│                    tsipfs binary                      │
│                                                       │
│  ┌────────────────┐    ┌────────────────────────────┐ │
│  │   tsnet        │    │   IPFS Node (Boxo/libp2p)  │ │
│  │                │    │                            │ │
│  │  Funnel:443 ───┼──> │  HTTP Gateway (read-only)  │ │
│  │  (public)      │    │  Bitswap (content exchange)│ │
│  │                │    │  Swarm port via Funnel     │ │
│  │  Tailnet:8080 ─┼──> │                            │ │
│  │  (private)     │    │  ┌──────────┐ ┌─────────┐  │ │
│  │                │    │  │ Pinned   │ │ Cache   │  │ │
│  │                │    │  │ Store    │ │ Store   │  │ │
│  │                │    │  │ (keep)   │ │ (wipe)  │  │ │
│  │                │    │  └──────────┘ └─────────┘  │ │
│  └────────────────┘    └────────────────────────────┘ │
│                                                       │
│  ┌────────────────┐    ┌────────────────────────────┐ │
│  │ Admin Web UI   │    │  REST API (tailnet-only)   │ │
│  │ (tailnet-only) │    │  - Upload / Pin / Unpin    │ │
│  │ - Upload       │    │  - List pins               │ │
│  │ - Pin/Unpin    │    │  - Node status             │ │
│  │ - Node status  │    │  - Cache management        │ │
│  └────────────────┘    └────────────────────────────┘ │
│                                                       │
│  ┌─────────────────────────────────────────────────┐  │
│  │ CLI commands (via the binary itself)            │  │
│  │ - tsipfs generate-swarm-key                     │  │
│  │ - tsipfs add <path>                             │  │
│  │ - tsipfs get <cid>                              │  │
│  │ - tsipfs flush-cache                            │  │
│  │ - tsipfs run (default)                          │  │
│  └─────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────┘
```

---

## Networking

### Public (via Tailscale Funnel)
- **IPFS Gateway** — HTTP gateway for reading/downloading content by CID. Exposed on Funnel port 443.
- **IPFS Swarm port** — libp2p swarm listener exposed via Funnel so nodes on separate tailnets can connect to each other over the public internet.

### Private (tailnet-only)
- **Admin Web UI** — Upload assets, manage pins, view node status. Authenticated by Tailscale identity (WhoIs).
- **REST API** — Programmatic access for the same operations. Same Tailscale identity auth.

### Bootstrap
- Any tsipfs node can act as a bootstrap node. Bootstrap peer addresses are configured via environment/config (their Funnel URLs). New nodes connect to one or more bootstrap peers to join the private swarm.

---

## Storage

Two separate datastores on disk:

| Store | Path (default) | Purpose | Configurable limit |
|-------|---------------|---------|-------------------|
| **Pinned** | `./data/pinned/` | Content the owner explicitly uploaded and pinned. Persists until manually unpinned. | `TSIPFS_PIN_MAX_SIZE` (default: unlimited) |
| **Cache** | `./data/cache/` | Content fetched from the swarm on behalf of requests. Evicted LRU when limit is reached. | `TSIPFS_CACHE_MAX_SIZE` (default: 10GB) |

The `flush-cache` CLI command wipes the cache store without affecting pinned content.

---

## Configuration

All configuration via environment variables (or `.env` file):

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TSIPFS_SWARM_KEY` | Yes | — | Hex-encoded IPFS swarm key for the private network |
| `TSIPFS_DATA_DIR` | No | `./data` | Root directory for pinned and cache stores |
| `TSIPFS_PIN_MAX_SIZE` | No | `0` (unlimited) | Max disk usage for pinned content (bytes, or human: `500GB`) |
| `TSIPFS_CACHE_MAX_SIZE` | No | `10GB` | Max disk usage for cached content |
| `TSIPFS_BOOTSTRAP_PEERS` | No | `""` | Comma-separated multiaddrs of bootstrap peers |
| `TSIPFS_FUNNEL_HOSTNAME` | No | (auto from tsnet) | Hostname for Funnel |
| `TSIPFS_ADMIN_PORT` | No | `8080` | Port for the tailnet-only admin UI/API |
| `TS_AUTHKEY` | No | — | Tailscale auth key (for headless/container startup) |

---

## CLI Commands

```
tsipfs run                  # Start the node (default command)
tsipfs generate-swarm-key   # Generate a new swarm key, print to stdout
tsipfs add <path>           # Add a file (and pin it), return its CID
tsipfs get <cid>            # Download IPFS content by CID to current directory
tsipfs flush-cache          # Delete all cached (non-pinned) content
tsipfs version              # Print version info
```

---

## REST API (tailnet-only)

All endpoints require Tailscale identity (verified via tsnet WhoIs).

```
POST   /api/v1/pins          Upload and pin content (multipart file upload)
GET    /api/v1/pins          List all pinned CIDs
GET    /api/v1/pins/:cid     Get pin details
DELETE /api/v1/pins/:cid     Unpin content

GET    /api/v1/status        Node status (peer count, storage usage, uptime)
GET    /api/v1/peers         Connected peers

POST   /api/v1/cache/flush   Flush the cache (same as CLI flush-cache)
GET    /api/v1/cache/status  Cache usage stats
```

---

## Technology Choices

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Language | Go (latest stable) | Single binary, good libp2p ecosystem |
| IPFS | Boxo + go-libp2p | Modular, lean binary, custom datastore control for split storage. More work than Kubo but avoids fighting its storage opinions. |
| Networking | tsnet | Embedded Tailscale — Funnel for public, tailnet for private, identity for auth |
| Datastore | Flatfs or Badger | Flatfs is simple and proven for IPFS block storage; Badger if we need better performance at TB scale |
| Admin UI | Embedded static files (Go embed) | Single binary, no external dependencies |
| CLI | cobra | Standard Go CLI framework |
| Config | envconfig + godotenv | Idiomatic Go env-based config with .env support |

---

## Implementation Phases

### Phase 1 — Foundation
- [ ] Go module setup, project structure
- [ ] Configuration loading (env + .env file)
- [ ] `generate-swarm-key` command
- [ ] Basic tsnet setup (start, authenticate, get Funnel hostname)

### Phase 2 — IPFS Node
- [ ] libp2p host with swarm key (private network)
- [ ] Split datastore (pinned + cache with size limits)
- [ ] Bitswap for content exchange
- [ ] Bootstrap peer connection
- [ ] Expose swarm port via Funnel
- [ ] IPFS HTTP Gateway (read-only, public via Funnel)

### Phase 3 — Admin API, Pinning & CLI Content Commands
- [ ] `tsipfs add <path>` — add file to IPFS, pin it, print CID
- [ ] `tsipfs get <cid>` — download content by CID to current directory
- [ ] REST API server on tailnet-only listener
- [ ] Tailscale WhoIs identity verification middleware
- [ ] Upload + pin endpoint
- [ ] List / get / unpin endpoints
- [ ] Cache flush endpoint + CLI command
- [ ] Status endpoints (peers, storage)

### Phase 4 — Admin Web UI
- [ ] Embedded SPA (Go embed, vanilla JS or lightweight framework)
- [ ] Upload with progress
- [ ] Pin management (list, unpin)
- [ ] Node status dashboard (peers, storage, uptime)

### Phase 5 — Docker & Operations
- [ ] Dockerfile (multi-stage build, minimal final image)
- [ ] docker-compose.yml with .env example
- [ ] Persistent volume mounts for data + tsnet state
- [ ] Health check endpoint

### Phase 6 — Hardening
- [ ] LRU cache eviction when cache limit is hit
- [ ] Graceful shutdown
- [ ] Structured logging
- [ ] Metrics (optional, Prometheus)
- [ ] Rate limiting on public gateway

---

## Project Structure

```
tsipfs/
├── main.go
├── cmd/
│   ├── root.go              # cobra root command
│   ├── run.go               # start the node
│   ├── generate_swarm_key.go
│   ├── add.go               # add + pin a file
│   ├── get.go               # download content by CID
│   ├── flush_cache.go
│   └── version.go
├── config/
│   └── config.go            # env loading, validation
├── node/
│   ├── node.go              # IPFS node lifecycle
│   ├── datastore.go         # split pinned/cache datastore
│   ├── gateway.go           # HTTP gateway handler
│   └── swarm.go             # swarm key, bootstrap
├── api/
│   ├── router.go            # REST API routes
│   ├── middleware.go         # Tailscale auth middleware
│   ├── pins.go              # pin CRUD handlers
│   ├── cache.go             # cache management handlers
│   └── status.go            # status/peer handlers
├── ui/
│   ├── embed.go             # go:embed for static files
│   └── static/              # HTML/CSS/JS for admin UI
├── network/
│   └── tsnet.go             # tsnet setup, Funnel, listeners
├── .env.example
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
├── Plan.md
└── README.md
```

---

## Open Decisions

1. **Datastore engine** — Flatfs (simple, filesystem-based) vs Badger (LSM tree, better at scale). Can start with Flatfs and migrate later if needed.
2. **Admin UI framework** — Could be vanilla JS, or a lightweight lib like Preact/Alpine.js. Keeping it simple since it's an admin tool.
3. **Gateway path convention** — Standard IPFS gateway uses `/ipfs/:cid/path`. We'll follow this convention.
