# Nine Men's Morris (Mühle)

Eine browserbasierte Implementierung des klassischen Mühle-Spiels als vollständig eigenständiges Go-Binary mit
eingebettetem Frontend. Unterstützt Mehrspieler über WebSocket, Einzelspieler gegen eine KI, Zuschauermodus, eine
persistente SQLite-Highscore-Datenbank und Revanche-Funktion.

## Features

- **Mehrspieler** – Raum erstellen, Link teilen, per WebSocket in Echtzeit spielen
- **Einzelspieler vs. KI** – Minimax mit Alpha-Beta-Pruning, drei Schwierigkeitsgrade (Leicht/Mittel/Schwer)
- **Zuschauermodus** – laufenden Partien live zusehen
- **Wiederverbindung** – bei Verbindungsabbruch 60 Sekunden Kulanzzeit zum Reconnect
- **Revanche** – nach Spielende direkt eine neue Partie gegen denselben Gegner starten
- **Highscores** – persistente Sieg-/Niederlage- und ELO-Statistik pro Spielername (SQLite)
- **Self-contained** – Templates, CSS und JS sind zur Build-Zeit ins Go-Binary eingebettet, kein externes Dateisystem
  nötig

## Spielregeln

- 2 Spieler, je 9 Steine
- Spielbrett: 3 konzentrische Quadrate mit 24 Feldern, verbunden durch Linien
- **Phase 1 – Setzen:** Spieler setzen abwechselnd je einen Stein auf freie Felder
- **Phase 2 – Ziehen:** Steine werden auf benachbarte, freie Felder verschoben
- **Phase 3 – Springen:** Ein Spieler mit nur noch 3 Steinen darf auf jedes freie Feld springen
- **Mühle:** 3 eigene Steine in einer Reihe → ein gegnerischer Stein darf entfernt werden (Steine aus einer
  geschlossenen gegnerischen Mühle nur, wenn keine anderen Steine mehr verfügbar sind)
- **Spielende:** Der Gegner hat nur noch 2 Steine, oder alle seine Züge sind blockiert

Eine ausführliche, interaktive Spielanleitung ist unter `/rules` in der App verfügbar.

## Tech-Stack

| Bereich    | Technologie                                                                                            |
|------------|--------------------------------------------------------------------------------------------------------|
| Backend    | Go, [Gin](https://github.com/gin-gonic/gin), [gorilla/websocket](https://github.com/gorilla/websocket) |
| Datenbank  | SQLite via [`modernc.org/sqlite`](https://gitlab.com/cznic/sqlite) (reines Go, kein CGO)               |
| Templating | `html/template` + `embed.FS`                                                                           |
| Frontend   | [AlpineJS](https://alpinejs.dev/) + natives JavaScript                                                 |
| CSS        | [TailwindCSS 4](https://tailwindcss.com/) (lokales Standalone-Binary)                                  |
| Container  | Docker Multi-Stage-Build                                                                               |

## Schnellstart

Voraussetzung: [Go](https://go.dev/) und [Task](https://taskfile.dev/) sind installiert.

```bash
task tw:install   # TailwindCSS-Binary einmalig herunterladen (bin/)
task run          # baut CSS und startet die App auf :8080
```

Anschließend im Browser: [http://localhost:8080](http://localhost:8080)

> **Wichtig:** `static/css/app.css` wird zur Build-Zeit via `embed.FS` ins Binary eingebettet und muss daher vor
> `go build`/`go run` existieren. `task run` und `task build` erledigen das automatisch (`tw:build`-Dependency).

## Verfügbare Tasks

```bash
task tw:install   # TailwindCSS-Standalone-Binary herunterladen (OS/Arch werden automatisch erkannt)
task tw:build      # static/css/app.css einmalig generieren (minifiziert)
task tw:watch      # CSS bei Dateiänderungen neu generieren (Entwicklung)
task build         # Go-Binary nach bin/muehle bauen (führt vorher tw:build aus)
task run           # App lokal auf :8080 starten (führt vorher tw:build aus)
task test          # alle Tests ausführen
task lint          # golangci-lint ausführen
task tidy          # go mod tidy
```

Einzelnes Test-Package ausführen:

```bash
go test ./internal/game/...
go test ./internal/ai/...
```

Einzelnen Test per Name ausführen:

```bash
go test -run TestFormsMill ./internal/game/...
```

## Konfiguration

Die Anwendung wird über Umgebungsvariablen konfiguriert:

| Variable          | Default                 | Beschreibung                                              |
|-------------------|-------------------------|-----------------------------------------------------------|
| `PORT`            | `8080`                  | HTTP-Port                                                 |
| `DB_PATH`         | `./database/muehle.db`  | Pfad zur SQLite-Datenbankdatei                            |
| `TRUSTED_PROXIES` | *(leer, keine Proxies)* | Kommagetrennte Liste vertrauenswürdiger Proxy-IPs für Gin |

## Docker

```bash
docker compose up --build
```

Das Compose-Setup baut das Image, mappt `PORT` (Default `8080`) und persistiert die Datenbank in einem benannten Volume
(`muehle-data`, gemountet unter `/app/data`).

Alternativ manuell:

```bash
docker build -t muehle .
docker run -p 8080:8080 -v muehle-data:/app/data -e DB_PATH=/app/data/muehle.db muehle
```

## Architektur

```
HTTP/WebSocket → Gin-Router (main.go) → Handler (internal/handler/) → Hub bzw. Repository
```

- **`internal/hub/`** – In-Memory-Raum-Registry (`Hub`, mutex-geschützte Map). Ein `Room` hält zwei `Client`-Pointer,
  den Spielzustand und eine Zuschauerliste. Der gesamte nebenläufige Zugriff ist über `hub.mu` (RWMutex) für die Map und
  `room.mu` (Mutex) pro Raum abgesichert.
- **`internal/game/`** – Reine Spiellogik, keine I/O. `board.go` definiert das 24-Felder-Layout und die
  Nachbarschaftsbeziehungen. `rules.go` implementiert `ApplyPlace`/`ApplyMove`/`ApplyRemove` sowie die Mühlenerkennung.
  `state.go` enthält `GameState` (nur Value-Types – eine Struct-Kopie ist damit eine Deep-Copy).
- **`internal/ai/`** – Minimax mit Alpha-Beta-Pruning. `BestMove(gs, aiPlayer, depth, easy)` liefert den besten `Move`.
  Der Easy-Modus würfelt 40 % der Züge zufällig aus; Tiefe 2/4/6 entspricht Leicht/Mittel/Schwer.
- **`internal/handler/`** – Gin-Handler. `ws.go` führt das WebSocket-Upgrade durch und delegiert an `hub.Connect`.
  `ai_ws.go` betreibt eine eigenständige KI-Session (ohne `Hub`). `lobby.go` behandelt Raum-Erstellung/-Beitritt/-Liste.
- **`internal/repository/`** – SQLite via `modernc.org/sqlite`. `RecordResult` führt Upsert-basiertes
  Sieg-/Niederlage-Tracking und ELO-Updates durch (Standard-ELO mit K=32, Startwert 1200, Untergrenze 100).

Datenbank-Schema (`players`-Tabelle):

```sql
CREATE TABLE players
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL UNIQUE,
    wins       INTEGER NOT NULL DEFAULT 0,
    losses     INTEGER NOT NULL DEFAULT 0,
    elo        INTEGER NOT NULL DEFAULT 1200,
    created_at DATETIME         DEFAULT CURRENT_TIMESTAMP
);
```

Die Highscore-Tabelle ist nach ELO absteigend sortiert, bei Gleichstand nach Wins absteigend.

### Request-Routen

| Methode | Pfad                     | Beschreibung                             |
|---------|--------------------------|------------------------------------------|
| GET     | `/`                      | Name-Eingabe bzw. Lobby (je nach Cookie) |
| GET     | `/lobby`                 | Lobby: Raum erstellen/beitreten          |
| GET     | `/game/:roomID`          | Spielfeld                                |
| GET     | `/spectate/:roomID`      | Zuschauermodus für laufende Partie       |
| GET     | `/ai`                    | Einzelspieler gegen KI                   |
| GET     | `/highscores`            | Highscore-Tabelle                        |
| GET     | `/rules`                 | Spielanleitung                           |
| POST    | `/api/player/register`   | Namen registrieren / Cookie setzen       |
| POST    | `/api/room/create`       | Raum erstellen → `{roomID, playerToken}` |
| POST    | `/api/room/join/:roomID` | Raum beitreten → `{playerToken}`         |
| GET     | `/api/rooms`             | Offene Räume auflisten                   |
| GET     | `/api/highscores`        | Highscores abrufen (JSON)                |
| GET     | `/ws/:roomID`            | WebSocket-Verbindung für Spieler         |
| GET     | `/ws/:roomID/spectate`   | WebSocket-Verbindung für Zuschauer       |
| GET     | `/ai/ws`                 | WebSocket-Verbindung für KI-Partie       |

### WebSocket-Protokoll

Verbindungsablauf: `POST /api/room/create` → `{roomID, playerToken}` erhalten → `GET /ws/:roomID` mit Header
`X-Player-Token`.

**Client → Server:** `place {pos}`, `move {from, to}`, `remove {pos}`, `rematch`

**Server → Client:** `waiting`, `game_start`, `state_update`, `game_over`, `error`, `opponent_left`,
`opponent_disconnected`, `opponent_reconnected`, `rematch_offer`

### Spielfeld-Koordinaten

Die 24 Positionen werden intern als Integer 0–23 kodiert:

- Außenring: 0–7
- Mittelring: 8–15
- Innenring: 16–23

Visuelle Anordnung:

```
 0 -------- 1 -------- 2
 |          |          |
 |  8 ----- 9 ----- 10 |
 |  |       |       |  |
 |  |  16--17--18   |  |
 |  |   |       |   |  |
 7  15  23      19  11  3
 |  |   |       |   |  |
 |  |  22--21--20   |  |
 |  |       |       |  |
 |  14----13-----12 |
 |          |          |
 6 -------- 5 -------- 4
```

Die 16 möglichen Mühlen sind als statisches Array in `internal/game/board.go` hinterlegt.

### Spieler-Identität

Spieler registrieren einen Namen (gespeichert im Cookie `player_name`). Der Server legt den Spieler beim ersten Kontakt
per Upsert an – ohne Passwort. Der `playerToken` (UUID) im Cookie autorisiert WebSocket-Verbindungen und Reconnects.

### Reconnect-Handling

Trennt sich ein Spieler mitten im Spiel, startet der Hub einen 60-Sekunden-Kulanz-Timer. Ein Reconnect mit demselben
Token bricht den Timer ab und stellt den Spielzustand wieder her. Nach Ablauf der Kulanzzeit erhält der Gegner
`opponent_left` und der Raum wird gelöscht.

## Lizenz

[MIT](LICENSE) © 2026 Peter Motzko
