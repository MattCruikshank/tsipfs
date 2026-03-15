# Ephemeral Fetch Plan

## What it solves

A new user should be able to download content from the tsipfs network with a single command, without initializing a node or creating a `.env` file. This is the "curl equivalent" for tsipfs.

## The command

```sh
go run github.com/MattCruikshank/tsipfs@latest fetch \
  <swarm-key> \
  <bootstrap-peer> \
  <content-ref>
```

Where `<content-ref>` is either:
- A CID: `QmXYZ...`
- An IPNS name: `/ipns/k51qzi5...`

Multiple bootstrap peers can be provided:

```sh
go run github.com/MattCruikshank/tsipfs@latest fetch \
  a3f8b2c1... \
  /dns4/peer1/tcp/443/p2p/12D3KooW... \
  /dns4/peer2/tcp/443/p2p/12D3KooW... \
  /ipns/k51qzi5uqu5dg8...
```

The last argument is always the content ref (starts with `Qm`, `bafy`, or `/ipns/`). Everything between the swarm key and the content ref is a bootstrap peer.

## How it works

1. **No persistence** — uses a temp directory for all state, cleaned up on exit
2. **No Tailscale** — doesn't need tsnet, Funnel, or a tailnet. Connects directly to bootstrap peers over TCP
3. **Ephemeral libp2p host** — joins the private swarm with the swarm key, connects to bootstrap peers
4. **Minimal subsystems** — only what's needed: libp2p host, DHT (client mode), bitswap, block service
5. **If IPNS** — resolves the name via DHT, then fetches the CID
6. **Downloads to current directory** — saves as the CID or IPNS name
7. **Exits** — tears everything down, deletes temp state

## What it doesn't do

- No pinning, no cache, no gateway, no admin UI
- No tsnet/Tailscale dependency
- No `.env` needed
- Doesn't persist the node identity between runs

## Implementation

### Argument parsing

```
fetch <swarm-key> <bootstrap-peers...> <content-ref>
```

- First arg: 64-char hex string → swarm key
- Middle args: strings starting with `/` → bootstrap peers
- Last arg: CID or `/ipns/...` → content to fetch

### Ephemeral node

```go
// 1. Temp directory
tmpDir, _ := os.MkdirTemp("", "tsipfs-fetch-*")
defer os.RemoveAll(tmpDir)

// 2. Generate throwaway identity
privKey, _, _ := crypto.GenerateEd25519Key(rand.Reader)

// 3. Minimal libp2p host (no Tailscale)
host, _ := libp2p.New(
    libp2p.Identity(privKey),
    libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),  // random port
    libp2p.PrivateNetwork(psk),
)

// 4. DHT in client mode (don't serve queries, just ask)
dht, _ := dht.New(ctx, host,
    dht.Mode(dht.ModeClient),
    dht.ProtocolPrefix("/tsipfs"),
)

// 5. Connect to bootstrap peers
// 6. Bitswap + block service (in-memory datastore)
// 7. If IPNS: resolve via namesys
// 8. Fetch content via block service
// 9. Write to file
```

### DHT client mode

The ephemeral node uses `dht.ModeClient` instead of `dht.ModeServer`. It can query the DHT but doesn't need to serve records — it's just passing through.

### In-memory datastore

No need for flatfs or leveldb. Use `datastore.NewMapDatastore()` wrapped in `sync.MutexWrap()`. The content is written to disk as a file, not kept in the IPFS datastore.

### Output

```sh
$ go run github.com/MattCruikshank/tsipfs@latest fetch a3f8... /dns4/peer1/.../p2p/12D3... /ipns/k51qzi5...
Joining swarm...
Connected to 1 peer(s)
Resolving /ipns/k51qzi5... → QmXYZ...
Downloading QmXYZ...
Saved QmXYZ... (4218903 bytes)
```

### Error cases

- No bootstrap peers reachable → "Could not connect to any bootstrap peers"
- IPNS name not found → "Could not resolve IPNS name (is the publisher online?)"
- CID not found → "Content not available (no peers have it)"
- Swarm key wrong → connections will fail silently (PSK mismatch), surface as "Could not connect"

## Shareable one-liner

A node operator can generate a fetch command for any pinned content. This could be surfaced in the admin UI next to each pin:

```
go run github.com/MattCruikshank/tsipfs@latest fetch a3f8b2c1... /dns4/my-node/tcp/443/p2p/12D3KooW... QmXYZ...
```

Or for an IPNS name:

```
go run github.com/MattCruikshank/tsipfs@latest fetch a3f8b2c1... /dns4/my-node/tcp/443/p2p/12D3KooW... /ipns/k51qzi5...
```

### Admin UI integration

- Each pin row gets a "Share" button that copies a `fetch` command
- Each IPNS name gets a "Share" button too
- The command includes the swarm key and all known bootstrap peers

## Phase

- [ ] `tsipfs fetch` command with ephemeral node
- [ ] IPNS resolution support in fetch
- [ ] Admin UI "Share" buttons generating fetch commands
