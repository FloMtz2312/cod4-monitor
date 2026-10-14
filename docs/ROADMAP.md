# Roadmap cod4-monitor — implementation

🇬🇧 **English** · 🇫🇷 [Français](ROADMAP.fr.md)

## P0 — Security and integrity

- [x] **Trusted proxies/IPs**: `TRUSTED_PROXIES`, X-Forwarded-For chain validated by Gin, HTTPS passed on only by an allowed peer. Production Docker keeps a dedicated network for Caddy/monitor and trusts only Caddy (`172.30.94.2`). When running natively, only loopback addresses are allowed by default.
- [x] **Sessions/secrets**: 7-day sessions, HttpOnly and SameSite Strict cookies, revocation on password changes, revocation of the admin sessions after the credentials in the environment change, limit of 10 sessions per account. PBKDF2 with 600,000 iterations for new passwords; old hashes are still accepted. HTTP body limited to 1 MiB, HTTP read timeouts, loopback-only listening enforced without authentication. FTP/RCON encrypted with AES-256-GCM in the database, with automatic migration. Explicit FTPS available in the form. Local log paths confined by `os.Root`, symbolic links included.
- [x] **Log resumption**: events, clock, pending KE and position saved in the same transaction; external effects wait for the commit. Fingerprint of the 128 bytes before the position to detect replaced files, even longer ones. Reads limited to 256 KiB, partial lines kept until the line break. Tests for rotation, truncation, restart, replay and SQLite errors.
- [x] **Timestamps**: the game's relative clock is anchored in UTC and persisted; gaps between events are kept, including after the monitor was interrupted. Fractions of a second are accepted when parsing; the event schema keeps Unix seconds. After the server clock is reset, or on a first observation without an anchor, the absolute time remains an estimate. `enterleave.log` keeps its absolute dates and its configured time zone.

## P1 — Reliability and maintenance

- [x] **CI/tests**: unit tests and HTTP/SQLite/log integration tests, `go vet`, `go mod verify`, `go test -race -count=1 ./...`, build without CGO. The deployment workflow runs these checks before the existing SSH call; the Docker build keeps its own check.
- [x] **Observability**: `/api/observability` restricted to the admin, a table in **More → Server status**, Prometheus metrics for bytes behind, last successful read, lines, rejections, errors, rotations, missing files and last verified restore. A quiet file that is read correctly is not considered behind; use the backlog and the read date together.
- [x] **SQLite**: indexes by round, player/period and last observation; daily combat aggregates maintained transactionally by a trigger and rebuilt once during the migration. A player's recent form combines the aggregated days with the exact time boundaries. `PRAGMA optimize` at startup and with the daily maintenance. `EXPLAIN QUERY PLAN` test for the rounds index.
- [x] **Backups**: every new backup is copied/restored into a temporary folder and checked with `integrity_check`, by reading the tables and by decrypting the secrets. Atomic publication with 0600 permissions; retention only after success. At startup, an existing backup of the day is also checked if it hasn't been validated yet. A corrupted backup is never deleted silently.

## P2 — Player experience

- [x] **Richer profile**: last 7 days / previous 7 days, K/D difference with a warning for small samples, accuracy, comparison with another player, last 10 rounds clickable.
- [x] **Timeline**: timestamped events per round, kills/headshots/teamkills, plants/defuses and objective/clutch events. Rounds are opened on demand. Explicit limit of 5,000 events on display.
- [x] **Shareable cards**: 1200 × 630 PNG for a player (7 days) or a match, with no IP or GUID in the image. Button on both pages; native file sharing when available, otherwise a download to attach in Discord. Nothing is sent to Discord automatically and no bot is required.
- [x] **Community challenges**: week from Monday 00:00 UTC to the next Monday, kills/headshots/objectives, collective progress, top contributors and daily records. Default goals 1,000 / 200 / 50; the admin sets those of the following weeks. The current week's targets stay fixed.

## Operations and restore

Automatic deployment from GitHub remains optional (README, "Automatic deployment from GitHub"). No new required variable is needed for an installation with an admin account and the provided production compose file.

1. At the first start, the monitor creates `DB_PATH.key` next to the database if `MONITOR_SECRET_KEY` is not set, then encrypts the existing settings. With Docker, the file is kept in the `/data` volume.
2. Keep a copy of this key in a secrets manager or a protected backup **separate** from the `.db` files. SQLite backups alone can no longer recover the FTP/RCON passwords.
3. To restore: stop the monitor, restore the database and its matching key (or the same `MONITOR_SECRET_KEY` value), remove the old `-wal`/`-shm` files only after stopping and replacing the database, set the permissions to 0600 and restart. Never restore over an open database.
4. Changing the main password (`DASHBOARD_PASSWORD`) logs out the admin sessions at the next restart of the monitor. Changing a secondary password immediately logs out all of its sessions.
5. Database encryption does not encrypt the game's UDP RCON protocol. FTPS is optional because some hosts don't offer it; it verifies the server's TLS certificate.
6. If the automatic restore check fails, keep the reported backup for diagnosis, move it away from the day's file name, then let the monitor try again. Older backups are kept as long as a new backup hasn't been validated.
7. Old copies of the database made before this migration may still contain secrets in clear text. Treat them like the old `.env` file.

### Local check

```sh
go mod verify
go vet ./...
go test -race -count=1 ./...
CGO_ENABLED=0 go build -trimpath -o monitor ./cmd/monitor
```

The tests use temporary folders and synthetic data, with no access to the real COD4 server.
