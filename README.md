# tsipfs

A single Go binary that runs a private IPFS node with a public gateway via [Tailscale Funnel](https://tailscale.com/kb/1223/funnel). Built for sharing open-licensed game development assets (Creative Commons, Public Domain, MIT, etc.) across a distributed network of independently operated nodes.

## How it works

Each tsipfs node joins a **private IPFS swarm** using a shared swarm key. Nodes discover each other and exchange content via [Bitswap](https://docs.ipfs.tech/concepts/bitswap/). When you pin an asset, it becomes available to every other node in the swarm.

- **Public gateway** — Anyone on the internet can download assets by CID via your node's Funnel URL
- **Private admin** — Only you (via your Tailscale identity) can upload, pin, and manage content
- **CLI** — Add and retrieve files from the command line

```
Internet ──Funnel──▶ IPFS Gateway (read-only)
                          │
Tailnet  ──────────▶ Admin UI + REST API
                          │
                    ┌─────┴─────┐
                    │  Pinned   │  ← your uploads (persistent)
                    │  Cache    │  ← others' content (auto-evicted)
                    └───────────┘
```

## Quick start

### 1. Build

```sh
go build -o tsipfs .
```

### 2. Generate a swarm key

One person generates the key and shares it with all node operators:

```sh
./tsipfs generate-swarm-key
# outputs: a3f8b2c1...  (64 hex chars)
```

### 3. Configure

Copy the example and fill in your swarm key:

```sh
cp .env.example .env
```

```env
TSIPFS_SWARM_KEY=a3f8b2c1...
```

That's the only required setting. See [Configuration](#configuration) for the full list.

### 4. Run

```sh
./tsipfs run
```

On first run, tsipfs will prompt you to authenticate with Tailscale by opening a URL in your browser. After that, it starts:

- A public IPFS gateway on your Funnel URL (e.g. `https://tsipfs.tail1234.ts.net`)
- An admin UI on your tailnet at port 8080
- A local unix socket for CLI commands

### 5. Add content

```sh
./tsipfs add my-asset-pack.zip
# added QmXyz... my-asset-pack.zip
```

The file is chunked, stored in your pinned datastore, and announced to the swarm. Other nodes can now fetch it by CID.

### 6. Retrieve content

```sh
./tsipfs get QmXyz...
# saved QmXyz... (4218903 bytes)
```

Or use the public gateway in a browser:

```
https://tsipfs.tail1234.ts.net/ipfs/QmXyz...
```

## CLI reference

| Command | Description |
|---------|-------------|
| `tsipfs run` | Start the node |
| `tsipfs add <path>` | Add a file, pin it, print its CID |
| `tsipfs get <cid>` | Download content by CID to the current directory |
| `tsipfs generate-swarm-key` | Generate a new 32-byte swarm key (hex) |
| `tsipfs flush-cache` | Delete all cached (non-pinned) content |
| `tsipfs version` | Print version info |

## Admin UI

The admin UI is served on your tailnet at port 8080 (configurable). It provides:

- **Node status** — peer ID, connected peers, uptime, storage usage
- **Upload** — drag-and-drop file upload with progress
- **Pin management** — list all pins, unpin content
- **Cache** — view usage, flush cache

Access it at `http://tsipfs:8080` from any device on your tailnet. Authentication is handled automatically by Tailscale identity.

## REST API

All endpoints are tailnet-only, authenticated via Tailscale identity.

```
POST   /api/v1/pins           Upload and pin a file (multipart form, field: "file")
GET    /api/v1/pins           List all pinned CIDs
GET    /api/v1/pins/:cid      Get pin details
DELETE /api/v1/pins/:cid      Unpin content

GET    /api/v1/status         Node status (peer count, storage, uptime)
GET    /api/v1/peers          Connected peers

POST   /api/v1/cache/flush    Flush the cache
GET    /api/v1/cache/status   Cache disk usage
```

Example:

```sh
# Upload from another machine on your tailnet
curl -X POST -F "file=@textures.zip" http://tsipfs:8080/api/v1/pins

# List pins
curl http://tsipfs:8080/api/v1/pins
```

## Configuration

All settings are via environment variables or a `.env` file in the working directory.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TSIPFS_SWARM_KEY` | **Yes** | — | Hex-encoded 32-byte swarm key |
| `TSIPFS_DATA_DIR` | No | `./data` | Root directory for all persistent data |
| `TSIPFS_PIN_MAX_SIZE` | No | `0` (unlimited) | Max storage for pinned content (`500GB`, `1TB`, etc.) |
| `TSIPFS_CACHE_MAX_SIZE` | No | `10GB` | Max storage for cached content (LRU-evicted) |
| `TSIPFS_BOOTSTRAP_PEERS` | No | — | Comma-separated bootstrap peer multiaddrs |
| `TSIPFS_FUNNEL_HOSTNAME` | No | `tsipfs` | Tailscale hostname |
| `TSIPFS_ADMIN_PORT` | No | `8080` | Tailnet-only admin UI/API port |
| `TS_AUTHKEY` | No | — | Tailscale auth key (for headless startup) |

## Storage

tsipfs maintains two separate stores under `TSIPFS_DATA_DIR`:

| Directory | Contents | Lifecycle |
|-----------|----------|-----------|
| `data/pinned/` | Content you explicitly added and pinned | Persists until you unpin it |
| `data/cache/` | Content fetched from the swarm for gateway requests | Auto-evicted (LRU) when `TSIPFS_CACHE_MAX_SIZE` is exceeded |

Run `tsipfs flush-cache` to wipe all cached content without affecting your pins.

Other directories:
- `data/tsnet/` — Tailscale state (node identity, keys)
- `data/dht/` — DHT routing table
- `data/pins/` — Pin metadata
- `data/identity.key` — libp2p node identity

## Bootstrapping a network

1. **First node** — Run `tsipfs run` with just a swarm key. Note your Funnel URL and peer ID from the logs.

2. **Additional nodes** — Set `TSIPFS_BOOTSTRAP_PEERS` to the first node's multiaddr:

   ```env
   TSIPFS_BOOTSTRAP_PEERS=/dns4/tsipfs.tail1234.ts.net/tcp/443/p2p/12D3KooW...
   ```

3. **Any node can be a bootstrap node.** As the network grows, add multiple bootstrap peers for resilience.

## Architecture

Built on:
- [Boxo](https://github.com/ipfs/boxo) + [go-libp2p](https://github.com/libp2p/go-libp2p) — modular IPFS implementation
- [tsnet](https://pkg.go.dev/tailscale.com/tsnet) — embedded Tailscale for Funnel (public) and tailnet (private) networking
- [Kademlia DHT](https://github.com/libp2p/go-libp2p-kad-dht) — content routing within the private swarm
- [Flatfs](https://github.com/ipfs/go-ds-flatfs) — filesystem-based block storage

## License

[MIT](LICENSE)
