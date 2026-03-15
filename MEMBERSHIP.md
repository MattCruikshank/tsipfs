# Membership Plan

## The problem

When a swarm key is rotated, everyone with the old key needs the new one. But what if you want to **remove** someone? They have the old key, they know the bootstrap peers, and there's no way to stop them from joining unless you change the key and only give the new one to the people you still trust.

Distributing the new key manually works, but doesn't scale and is error-prone. You need a system where:

- Each member has their own identity
- The network admin encrypts the new swarm key for each member individually
- Removed members simply don't receive the new key
- Distribution is automatic via IPNS

## Design

### Each member gets a keypair

When a member joins, they generate an Ed25519 keypair. They share their **public key** with the network admin. The admin doesn't need their private key — only the public key, to encrypt swarm keys for them.

### Per-member IPNS channels

The network admin creates an IPNS key for each member. Each IPNS name is a private channel: it contains the latest swarm key, encrypted with that member's public key.

```
/ipns/k51abc...  →  { encrypted swarm key for Alice }
/ipns/k51def...  →  { encrypted swarm key for Bob }
/ipns/k51ghi...  →  { encrypted swarm key for Carol }
```

Each member only knows their own IPNS name. They can't see other members' channels or determine who else is in the network.

### Encrypted swarm key record

```json
{
  "encrypted_key": "base64...",
  "sequence": 4,
  "activate_at": "2026-04-01T00:00:00Z",
  "note": "Q2 2026 rotation",
  "nonce": "base64...",
  "network_signature": "base64..."
}
```

- `encrypted_key` — swarm key encrypted with the member's public key (X25519 ECDH + XChaCha20-Poly1305, using Ed25519→X25519 key conversion)
- `sequence` — must be higher than the member's current sequence (prevents rollback)
- `activate_at` — when to switch to this key
- `nonce` — encryption nonce
- `network_signature` — signed by the network identity key (from SIGNED-SWARMKEY.md) over the plaintext swarm key + sequence + activate_at, so members can verify after decryption

### Why per-member IPNS (not one big record)

- **Privacy** — members can't see who else is in the network
- **Isolation** — adding/removing a member doesn't touch other members' records
- **Simplicity** — each member resolves exactly one IPNS name
- **Stealth removal** — a removed member's IPNS channel simply stops updating. They can't tell if they were removed or if there's no rotation happening.

## Flows

### Network admin: adding a member

```sh
# Member generates their keypair and shares the public key
tsipfs member keygen
# → Member public key: MCowBQ...
# → Member IPNS channel: (assigned by admin)

# Admin registers the member
tsipfs member add alice --pubkey=MCowBQ...
# → Created IPNS channel for alice: k51abc...
# → Share this with alice: k51abc...
```

The admin gives Alice two things (once, out-of-band):
1. Her IPNS channel name (`k51abc...`)
2. The initial swarm key bundle (so she can join immediately)

### Network admin: rotating the swarm key

```sh
tsipfs member rotate --seq=4 --note="Q2 2026"
# → Generated new swarm key: b4c9d3e2...
# → Encrypted for 12 members
# → Published to 12 IPNS channels
```

This:
1. Generates a new swarm key
2. Signs it with the network identity key
3. Encrypts it individually for each active member
4. Publishes each encrypted record to the member's IPNS channel

### Network admin: removing a member

```sh
tsipfs member rm bob
# → Removed bob. Their IPNS channel will not receive future keys.
# → Run 'tsipfs member rotate' to issue a new swarm key.
```

Removing a member just deletes their record from the admin's member list. The next rotation won't encrypt a key for them. The admin should rotate immediately after removal.

### Node operator: receiving a key rotation

Nodes poll their IPNS channel periodically (e.g. every hour):

1. Resolve the IPNS name
2. Decrypt the record with the member's private key
3. Verify the network signature on the decrypted swarm key
4. Check the sequence number is higher than the current one
5. Stage the new key for activation at `activate_at`
6. Restart (or apply live) when the activation time arrives

### Node operator: initial setup

```sh
# First time: admin gives you the IPNS channel and initial bundle
tsipfs init my-node <initial-signed-bundle> <bootstrap-peers...> \
  --member-key=<path-to-private-key> \
  --channel=k51abc...
```

The `.env` gets:
```env
TSIPFS_MEMBER_KEY=path/to/member.key
TSIPFS_MEMBER_CHANNEL=k51abc...
```

## Admin data model

The admin maintains a member list in `data/members.json`:

```json
{
  "alice": {
    "pubkey": "MCowBQ...",
    "ipns_key_name": "member-alice",
    "added_at": "2026-03-15T04:00:00Z"
  },
  "bob": {
    "pubkey": "MCowBQ...",
    "ipns_key_name": "member-bob",
    "added_at": "2026-03-10T12:00:00Z",
    "removed_at": "2026-03-20T09:00:00Z"
  }
}
```

Each member has a corresponding IPNS keypair stored in `data/keys/member-<name>.key`.

## Scale considerations

This design is for **small to medium networks** (tens to low hundreds of members):

- Each member = 1 IPNS key to manage + 1 IPNS record to republish every 4 hours
- 50 members = 50 republishes every 4 hours (fine)
- 500 members = still feasible but getting noisy
- 5000+ members = consider a group key scheme (MLS, sender keys, or a key server)

## Relationship to other plans

- **SIGNED-SWARMKEY.md** — provides the network identity keypair and signed bundle format. Membership builds on top of this.
- **IPNS.md** — provides the IPNS key management and publishing infrastructure. Membership uses per-member IPNS channels.
- **FETCH.md** — ephemeral fetch still takes a raw/signed swarm key. Members would extract their current key and pass it to fetch.

## Phases

### Phase A — Admin tooling
- [ ] `tsipfs member keygen` — generate a member keypair
- [ ] `tsipfs member add <name> --pubkey=...` — register a member, create IPNS channel
- [ ] `tsipfs member rm <name>` — remove a member
- [ ] `tsipfs member list` — list members and status
- [ ] `tsipfs member rotate` — generate new swarm key, encrypt for all active members, publish

### Phase B — Node auto-rotation
- [ ] Periodic IPNS channel polling
- [ ] Decryption and verification
- [ ] Staged key rotation with activation timestamps
- [ ] Admin UI showing membership status and pending rotations

### Phase C — Admin UI
- [ ] Member management in admin UI (add, remove, list)
- [ ] One-click rotation
- [ ] Rotation history
