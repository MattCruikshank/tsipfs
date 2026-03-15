# Suggestions

Prioritized list of improvements for tsipfs.

## High value — do soon

### Tests
Unit tests for config parsing, swarm key generation, split blockstore, and rate limiter. These are all testable without a running network.

### LICENSE file
The README references MIT but there's no LICENSE file yet.

## Medium value

### Structured logging
Replace `log.Printf` with `log/slog`. Current logging gets noisy at scale; structured logging makes it filterable and machine-readable.

### Health endpoint
`GET /healthz` on Funnel for uptime monitoring by external tools.

### `tsipfs pin <cid>` command
Pin content that's already in the swarm without uploading a local file. Useful for intentionally replicating others' content to increase availability.

### Config validation on startup
Verify swarm key format, check data dir is writable, warn if no bootstrap peers are configured.

## Nice to have later

### Prometheus metrics
Request counts, bitswap exchange stats, storage gauges, peer counts. The dependencies are already pulled in transitively.

### Dockerfile
Multi-stage build, minimal final image, persistent volume mounts for data + tsnet state. Skipped during initial build but users running on servers will want it.
