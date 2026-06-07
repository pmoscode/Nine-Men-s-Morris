# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Browser-based Nine Men's Morris (Mühle) game — a fully self-contained Go binary with an embedded frontend. Supports multiplayer (WebSocket), single-player vs. AI, spectator mode, a persistent SQLite highscore backend, and rematch functionality.

## Commands

All tasks are managed via [Task](https://taskfile.dev) (`task`):

```bash
task tw:install   # Download TailwindCSS standalone binary to bin/ (auto-detected OS/arch)
task tw:build     # Generate static/css/app.css (minified, one-shot)
task tw:watch     # Regenerate CSS on file changes (development)
task build        # Build Go binary to bin/muehle (runs tw:build first)
task run          # Run the app locally on :8080 (runs tw:build first)
task test         # Run all tests
task lint         # Run golangci-lint
task tidy         # go mod tidy
```

Run a single test package:
```bash
go test ./internal/game/...
go test ./internal/ai/...
```

Run a single test by name:
```bash
go test -run TestFormsMill ./internal/game/...
```

**CSS must be built before running** — `static/css/app.css` is embedded into the binary at compile time via `embed.FS`. The `bin/tailwindcss` binary is not committed to git; run `task tw:install` first.

## Architecture

### Self-Contained Embedding

All templates, CSS, and JS are embedded into the Go binary at build time:
```go
//go:embed templates/* static/*
var embeddedFiles embed.FS
```
This means `static/css/app.css` must exist before `go build`.

### Request Lifecycle

```
HTTP/WebSocket → Gin router (main.go) → Handler (internal/handler/) → Hub or Repository
```

- **`internal/hub/`** — In-memory room registry (`Hub`, mutex-protected map). A `Room` holds two `Client` pointers, game state, and spectator list. All concurrent access is guarded by `hub.mu` (RWMutex) for the map and `room.mu` (Mutex) for per-room state.
- **`internal/game/`** — Pure game logic, no I/O. `board.go` defines the 24-position layout and adjacency. `rules.go` implements `ApplyPlace`/`ApplyMove`/`ApplyRemove` and mill detection. `state.go` holds `GameState` (only value types — struct copy = deep copy).
- **`internal/ai/`** — Minimax with alpha-beta pruning. `BestMove(gs, aiPlayer, depth, easy)` returns the best `Move`. Easy mode randomises 40% of choices; depth 2/4/6 maps to easy/medium/hard.
- **`internal/handler/`** — Gin handlers. `ws.go` upgrades connections and delegates to `hub.Connect`. `ai_ws.go` runs a self-contained AI session (no `Hub` involved). `lobby.go` handles room create/join/list.
- **`internal/repository/`** — SQLite via `modernc.org/sqlite` (pure Go, no CGO). `RecordResult` does upsert-based win/loss tracking and ELO updates.

### WebSocket Protocol

**Player connection flow:** `POST /api/room/create` → get `{roomID, playerToken}` → `GET /ws/:roomID` with `X-Player-Token` header.

**Client → Server message types:** `place` `{pos}`, `move` `{from, to}`, `remove` `{pos}`, `rematch`

**Server → Client message types:** `waiting`, `game_start`, `state_update`, `game_over`, `error`, `opponent_left`, `opponent_disconnected`, `opponent_reconnected`, `rematch_offer`

### Board Coordinates

24 positions encoded as integers 0–23:
- Outer ring: 0–7
- Middle ring: 8–15
- Inner ring: 16–23

16 possible mills are defined as a static array in `internal/game/board.go`.

### Player Identity

Players register a name (stored in a `player_name` cookie). The server upserts on first contact — no passwords. The `playerToken` (UUID) in the cookie authorises WebSocket connections and reconnections.

### Reconnect Handling

If a player disconnects mid-game, the hub starts a 60-second grace timer. Reconnecting with the same token cancels the timer and restores game state. After the grace period, the opponent receives `opponent_left` and the room is deleted.

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.23, Gin, gorilla/websocket |
| Database | SQLite via `modernc.org/sqlite` (no CGO) |
| Templating | `html/template` + `embed.FS` |
| CSS | TailwindCSS 4 (local standalone binary) |
| Frontend | AlpineJS + native JS (`static/js/game.js`) |
| Container | Docker multi-stage build (downloads TailwindCSS in builder stage) |
