# Tankoids (mmorms)

A multiplayer real-time artillery tank game inspired by Worms. Play in your browser with tanks, shooting, jumping, shields, destructible terrain, and bots.

Play now: http://tankoids.vbo.name

## Architecture

- **mmorms** — Game server (Go). Physics, WebSocket game logic, terrain destruction, bots, leaderboard. Serves the web client and game WebSocket.
- **overlord** — Matchmaking server. Keeps a list of game servers and player counts. Clients connect to overlord first to pick the best server, then join the game.

### Runtime (multi-server)

```mermaid
sequenceDiagram
    participant U as User Browser
    participant S as Site (static)
    participant O as Overlord
    participant G1 as Game Server 1
    participant G2 as Game Server 2
    participant GN as Game Server N

    U->>S: 1. Load HTML/JS/CSS
    U->>O: 2. WebSocket: request server list
    O->>U: 3. Server list (URL, player count)
    U->>U: 4. Pick server (fewest players)
    U->>G2: 5. Connect WebSocket
    G2->>U: 6. Game state, play
    G1->>O: /update (register)
    G2->>O: /update (register)
    GN->>O: /update (register)
```

### Build & Deploy

```mermaid
flowchart LR
    subgraph Col1[" "]
        direction TB
        PUSH["Push to main/master"]
        subgraph CI["GitHub Actions (deploy.yml)"]
            direction TB
            C1["actions/checkout"]
            C2["Setup flyctl"]
            C3["Compute version"]
            C4["fly deploy (FLY_API_TOKEN)"]
            C1 --> C2 --> C3 --> C4
        end
        PUSH --> C1
    end

    subgraph Col2[" "]
        direction TB
        subgraph Build["Remote build (Depot)"]
            direction TB
            B1["Docker build"]
            B2["Push to registry"]
            B1 --> B2
        end
    end

    subgraph Col3[" "]
        subgraph Fly["Fly.io"]
            subgraph Mmorms["mmorms app"]
                H1["instance 1"]
                H2["instance 2"]
                HN["instance N"]
            end
            subgraph Overlord["mmorms-overlord app"]
                O["Overlord"]
            end
        end
        H1 & H2 & HN -.->|"/update"| O
    end

    C4 --> B1
    B2 --> H1
```

## Requirements

- [Go](https://go.dev/) 1.22+ (or version in `go.mod`)
- Python 3 (for build/run scripts)

## Quick Start

```bash
python build_run.py
```

Then open **http://localhost:8080** in your browser.

This will:

1. Build `mmorms` (game server) and `overlord` (matchmaking server)
2. Start overlord on port 7070
3. Start mmorms on port 8080

## Manual Run

```bash
# Build both binaries
go build -o mmorms .
go build -o overlord ./overlord

# Start both servers
python singlebox.py
```

## Scripts

| Script | Description |
|--------|-------------|
| `build_run.py` | Build both binaries and start servers (Windows & Linux) |
| `singlebox.py` | Start overlord + mmorms only (expects binaries already built) |
| `cleanup.py` | Kill overlord and mmorms processes |

Press **Ctrl+C** in the terminal running `singlebox.py` to stop both servers.

## Controls

| Key | Action |
|-----|--------|
| ← → | Move |
| ↑ ↓ | Aim gun |
| C | Shoot (hold for more power) |
| X | Jump (hold for higher jump) |
| Z | Shield |

## Single-Server Mode

If the overlord is unavailable, the client falls back to connecting directly to the game server on the same host (e.g. `ws://localhost:8080/ws`), so you can run mmorms alone for local play.

## Project Layout

```
.
├── main.go        # Game loop, physics, maps
├── network.go     # WebSocket server, HTTP, overlord registration
├── bot.go         # AI bots
├── overlord/      # Matchmaking server
└── public/        # Static assets (index.html, game.js, render.js, maps)
```
