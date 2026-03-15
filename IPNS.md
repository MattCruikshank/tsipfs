# IPNS Support Plan

## What IPNS solves

CIDs are content-addressed — when the content changes, the CID changes. IPNS provides stable, updatable names that point to CIDs. This is useful for:

- **Asset collections** — a stable address for "latest texture pack" that updates as new assets are added
- **Indexes** — a stable address for a browsable catalog of all assets on the network
- **Channels** — each publisher gets a stable address others can follow

## How IPNS works

1. A **keypair** generates a stable IPNS name (the public key hash)
2. The owner signs an **IPNS record** pointing the name at a CID
3. The record is published to the DHT so other nodes can resolve it
4. Records have a **TTL** and **sequence number** for freshness

```
/ipns/k51qzi5uqu5d...  →  QmXYZ...  (signed, seq=3, ttl=1h)
```

The node's peer ID is itself a key, so every node gets one IPNS name for free. Additional named keys can be generated for separate channels.

## What we already have

- Boxo gateway already supports `/ipns/` resolution
- DHT is running with `ValueStore` (used for IPNS record storage/retrieval)
- libp2p host identity provides the default keypair

## What we need to add

### 1. Key management

Store named keypairs in `data/keys/`. Each key gets an IPNS name.

```
tsipfs key gen <name>         # generate a new keypair, print its IPNS name
tsipfs key list               # list all keys and their IPNS names
tsipfs key rm <name>          # delete a keypair
```

The node's own identity key is always available as `self`.

### 2. Publishing

Update an IPNS name to point at a CID.

```
tsipfs name publish <cid>               # publish using the 'self' key
tsipfs name publish --key=<name> <cid>  # publish using a named key
```

Options:
- `--ttl` — how long resolvers should cache (default: 1h)
- `--lifetime` — how long the record is valid (default: 24h)

### 3. Resolving

```
tsipfs name resolve <ipns-name>    # resolve an IPNS name to a CID
```

The gateway already does this for `/ipns/` URLs. This CLI command is for scripting.

### 4. REST API

```
POST   /api/v1/keys              Create a keypair (body: {"name": "my-collection"})
GET    /api/v1/keys              List keys and their IPNS names
DELETE /api/v1/keys/:name        Delete a keypair

POST   /api/v1/names/publish     Publish (body: {"cid": "Qm...", "key": "self"})
GET    /api/v1/names/:name       Resolve an IPNS name to a CID
GET    /api/v1/names             List this node's published names
```

### 5. Admin UI

Add a **Names** section:
- List published IPNS names with their current CID, key name, and last updated time
- Publish form: select a key, enter/pick a CID, publish
- Copyable IPNS gateway URL (e.g. `https://host/ipns/k51qzi5...`)

## Implementation details

### Boxo packages needed

- `github.com/ipfs/boxo/namesys` — IPNS publisher and resolver
- `github.com/ipfs/boxo/ipns` — IPNS record creation, validation, signing
- `github.com/libp2p/go-libp2p/core/crypto` — already imported, used for key generation

### Key storage

Keys stored as marshaled private keys in `data/keys/<name>.key` (same format as `identity.key`). A metadata file `data/keys/keys.json` maps names to IPNS addresses for fast listing.

### Publishing flow

1. Load the named key (or `self` for the node identity)
2. Create an IPNS record: sign `(name → CID, seq, validity, ttl)` with the private key
3. Put the record into the DHT via `routing.ValueStore.PutValue()`
4. Save the current mapping locally for the admin UI

### Resolution flow

1. Gateway request comes in for `/ipns/<name>`
2. Boxo gateway backend calls `namesys.Resolve()`
3. `namesys` fetches the record from the DHT via `routing.ValueStore.GetValue()`
4. Validates signature and freshness
5. Returns the CID, gateway serves the content

### Republishing

IPNS records expire. A background goroutine should republish all active names every 4 hours (configurable) to keep them alive in the DHT.

## Example workflow

```sh
# Create a key for your texture collection
tsipfs key gen textures
# → /ipns/k51qzi5uqu5dg8...

# Upload and pin v1 of the collection
tsipfs add textures-v1.zip
# → QmABC...

# Publish the stable name
tsipfs name publish --key=textures QmABC...
# → published /ipns/k51qzi5uqu5dg8... → QmABC...

# Anyone can access it at a stable URL:
# https://your-node/ipns/k51qzi5uqu5dg8...

# Later, upload v2 and update the pointer
tsipfs add textures-v2.zip
# → QmXYZ...

tsipfs name publish --key=textures QmXYZ...
# → published /ipns/k51qzi5uqu5dg8... → QmXYZ...

# Same URL, new content
```

## Phases

### Phase A — Core
- [ ] Key generation and storage
- [ ] `tsipfs key gen/list/rm` CLI commands
- [ ] IPNS publish and resolve using Boxo namesys
- [ ] `tsipfs name publish/resolve` CLI commands
- [ ] Background republisher

### Phase B — API & UI
- [ ] REST API endpoints for keys and names
- [ ] Admin UI names section
- [ ] Gateway `/ipns/` resolution (verify it works with current setup)
