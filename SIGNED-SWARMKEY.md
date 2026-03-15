# Signed Swarm Key Plan

## The problem

The swarm key is a shared secret. When it's rotated, every node needs the new key. But how do they know the new key is legitimate? An attacker who obtains the old key could send a fake "new key" and fragment the network, or lure nodes into a malicious swarm.

## The solution

A **network identity keypair** that signs every swarm key. The public key is the stable, unforgeable identity of the network. Nodes verify the signature before accepting any swarm key.

```
Network founder:
  1. Generates a network keypair (once, forever)
  2. Signs each swarm key with the private key
  3. Distributes: public key + signed swarm key bundle

Node operator:
  1. Knows the network public key (configured once)
  2. Receives a signed swarm key bundle
  3. Verifies signature before using the swarm key
```

## What gets signed

A **signed swarm key bundle** contains:

```json
{
  "swarm_key": "a3f8b2c1...64 hex chars",
  "sequence": 3,
  "created_at": "2026-03-15T04:00:00Z",
  "note": "Q1 2026 rotation",
  "signature": "base64-encoded Ed25519 signature"
}
```

- `swarm_key` — the actual 32-byte swarm key (hex)
- `sequence` — monotonically increasing, prevents replay of old keys
- `created_at` — when this key was issued
- `note` — optional human-readable reason for rotation
- `signature` — Ed25519 signature over `swarm_key + sequence + created_at`

The bundle is stored as a single file: `swarm_key_bundle.json`.

## Network identity

The **network public key** is the trust anchor. It never changes. It's a short string that can be shared out-of-band (website, README, chat):

```
network_pubkey: MCowBQYDK2VwAyEA...  (base64 Ed25519 public key)
```

The private key is held only by the network founder (or a small set of trusted admins). It is NOT stored on regular nodes.

## Changes to existing commands

### `tsipfs init`

Currently:
```sh
tsipfs init <hostname> <swarm-key> <bootstrap-peers...>
```

New:
```sh
tsipfs init <hostname> <network-pubkey> <signed-bundle> <bootstrap-peers...>
```

Or more practically, the bundle includes everything. The `init` input becomes:

```sh
tsipfs init <hostname> <signed-bundle-base64> <bootstrap-peers...>
```

Where the signed bundle is a single base64 string encoding the JSON bundle + network public key. `init` decodes it, verifies the signature, extracts the swarm key, and writes the `.env` with:

```env
TSIPFS_SWARM_KEY=a3f8b2c1...
TSIPFS_NETWORK_PUBKEY=MCowBQYDK2VwAyEA...
TSIPFS_SWARM_KEY_SEQ=3
```

### `tsipfs fetch`

Same change — the first argument becomes the signed bundle instead of a raw swarm key. Verification happens before joining the swarm.

### Admin UI — "Share known nodes"

The share output includes the signed bundle instead of the raw swarm key. When pasted into "Add known nodes", the signature is verified.

### Admin UI — Key rotation notification

When a new signed bundle is received (via bootstrap peers or the API) with a higher sequence number:

1. Verify the signature against the stored network public key
2. Show a notification in the admin UI: "Swarm key rotation available (seq 4). Restart to apply."
3. Store the new bundle in `data/pending_swarm_key.json`
4. On next restart, the node picks up the new key

## New commands

### For the network founder

```sh
# Generate the network identity (once, ever)
tsipfs network init
# → Network private key saved to network.key (KEEP THIS SAFE)
# → Network public key: MCowBQYDK2VwAyEA...
# → Share the public key with your community

# Sign a new swarm key
tsipfs network sign <swarm-key-hex> --seq=1 --note="Initial key"
# → Signed bundle: eyJzd2FybV9rZXkiOi...  (base64)

# Rotate: generate a new swarm key and sign it
tsipfs network rotate --seq=2 --note="Q1 2026 rotation"
# → New swarm key: b4c9d3e2...
# → Signed bundle: eyJzd2FybV9rZXkiOi...
```

### For node operators

```sh
# Verify a bundle without applying it
tsipfs network verify <signed-bundle-base64>
# → Valid. Swarm key: a3f8...f0a1, sequence: 3, signed by MCowBQ...
```

## Key rotation flow

### Founder

1. `tsipfs network rotate --seq=4 --note="March 2026"`
2. Shares the new signed bundle (via chat, website, or the admin UI)
3. Restarts their own node with the new key

### Node operator (manual)

1. Receives the signed bundle
2. Pastes into admin UI "Add known nodes" or runs `tsipfs network apply <bundle>`
3. Node verifies signature and sequence number
4. Node stores the pending key and prompts for restart
5. On restart, the new swarm key takes effect

### Node operator (automatic, future)

1. A running node receives a key rotation announcement via a DHT topic or libp2p pubsub
2. Verifies the signature
3. Schedules a graceful restart (or applies it live if the swarm protocol supports rekeying)

This is a future enhancement — manual rotation is the starting point.

## Configuration changes

| Variable | Description |
|----------|-------------|
| `TSIPFS_SWARM_KEY` | The current swarm key (unchanged) |
| `TSIPFS_NETWORK_PUBKEY` | Network identity public key (set once, never changes) |
| `TSIPFS_SWARM_KEY_SEQ` | Sequence number of the current key (prevents rollback) |

## Security properties

- **Authenticity** — only the holder of the network private key can issue valid swarm keys
- **Replay protection** — sequence numbers prevent reuse of old keys
- **No single point of failure** — the network private key can be held by multiple trusted admins (future: threshold signatures)
- **Backward compatible** — nodes without a network pubkey configured still accept raw swarm keys (no verification)

## File layout

```
# Network founder's machine only:
network.key                    # Ed25519 private key (NEVER share)

# Every node:
.env                           # includes TSIPFS_NETWORK_PUBKEY, TSIPFS_SWARM_KEY_SEQ
data/swarm_key_bundle.json     # current signed bundle (for re-sharing)
data/pending_swarm_key.json    # pending rotation (applied on restart)
```

## Idea: Scheduled rotation via IPNS

The network founder could use an IPNS name to pre-publish the **next** swarm key with an activation timestamp. Nodes check it periodically and stage the rotation automatically — no manual distribution needed.

### How it would work

1. Founder creates an IPNS key for rotation announcements (e.g. `tsipfs key gen rotation`)
2. Publishes a signed bundle with `activate_at` in the future:

```json
{
  "swarm_key": "b4c9d3e2...",
  "sequence": 4,
  "activate_at": "2026-04-01T00:00:00Z",
  "note": "Q2 2026 rotation",
  "signature": "..."
}
```

3. Nodes periodically resolve the IPNS name, verify the signature, and stage the key
4. At `activate_at`, nodes restart with the new key (or apply it live)

### Tradeoffs

**Good:**
- Fully automatic — publish once, all nodes rotate on schedule
- Founder doesn't need to be online at rotation time
- Reduces coordination overhead for large networks

**Risky:**
- If the pre-published key leaks, attackers know the upcoming key early
- IPNS resolution requires DHT access, which requires a valid swarm key — a node already locked out can't resolve the new key (chicken-and-egg)
- Pre-committing removes the ability to react to a compromise by skipping to an unplanned key

**Mitigation:**
- Only publish the *next* key, not multiple future keys — limits exposure window
- Emergency override: an out-of-band bundle with a higher sequence number always takes priority over a scheduled one
- The `TSIPFS_NETWORK_PUBKEY` trust anchor means even a leaked swarm key can't produce a valid signed bundle

### Configuration

The IPNS name for rotation announcements would be a well-known configuration:

```env
TSIPFS_ROTATION_IPNS=k51qzi5uqu5dg8...
```

Nodes that have this set will poll for updates. Nodes without it rely on manual rotation.

This is a future enhancement — manual rotation (Phases A–C) should come first.

## Phases

### Phase A — Signing infrastructure
- [ ] `tsipfs network init` — generate network keypair
- [ ] `tsipfs network sign` — sign a swarm key
- [ ] `tsipfs network rotate` — generate + sign a new key
- [ ] `tsipfs network verify` — verify a signed bundle
- [ ] Bundle format (JSON + base64 encoding)

### Phase B — Node verification
- [ ] `tsipfs init` accepts signed bundles
- [ ] `tsipfs fetch` accepts signed bundles
- [ ] Verify signature + sequence on startup
- [ ] Reject swarm keys with lower sequence numbers

### Phase C — Rotation UX
- [ ] `tsipfs network apply` — stage a pending key rotation
- [ ] Admin UI notification for pending rotation
- [ ] Admin UI share output uses signed bundles
- [ ] "Add known nodes" verifies bundle signatures

### Phase D — Automatic rotation (future)
- [ ] Key rotation announcements via libp2p pubsub
- [ ] Automatic verification and staging
- [ ] Graceful restart or live rekeying
