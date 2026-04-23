# Mühle – Konzept

## Überblick

Eine browserbasierte Implementierung des klassischen Mühle-Spiels (Nine Men's Morris) als vollständig eingebettete
Go-Anwendung mit Highscore-Backend.

---

## Spielregeln (Zusammenfassung)

- 2 Spieler, je 9 Steine
- Spielbrett: 3 konzentrische Quadrate mit 24 Feldern, verbunden durch Linien
- **Phase 1 – Setzen:** Spieler setzen abwechselnd je einen Stein auf freie Felder
- **Phase 2 – Ziehen:** Steine werden auf benachbarte, freie Felder verschoben
- **Phase 3 – Springen:** Spieler mit nur noch 3 Steinen dürfen auf jedes freie Feld springen
- **Mühle:** 3 eigene Steine in einer Reihe → gegnerischen Stein entfernen (nicht aus einer geschlossenen Mühle, solange
  andere Steine verfügbar)
- **Spielende:** Gegner hat nur noch 2 Steine, oder alle Züge des Gegners sind blockiert

---

## Architektur

```
muehle/
├── main.go                  # Einstiegspunkt, Server-Start
├── go.mod
├── go.sum
├── Dockerfile
├── Taskfile.yml             # Dev-Tasks inkl. CSS-Generierung (taskfile.dev)
├── internal/
│   ├── game/
│   │   ├── board.go         # Spielfeld, Positionen, Adjazenz
│   │   ├── rules.go         # Spiellogik (Mühle, Züge, Phasen)
│   │   └── state.go         # Spielzustand + Room-Structs
│   ├── hub/
│   │   ├── hub.go           # Room-Registry (alle aktiven Spiele im Memory)
│   │   └── client.go        # WebSocket-Client (eine Verbindung = ein Spieler)
│   ├── handler/
│   │   ├── lobby.go         # HTTP-Handler: Raum erstellen / beitreten
│   │   ├── ws.go            # WebSocket-Upgrade und Message-Loop
│   │   ├── highscore.go     # HTTP-Handler für Highscores
│   │   └── page.go          # Template-Rendering
│   ├── repository/
│   │   └── highscore.go     # SQLite-Datenbankzugriff
│   └── model/
│       ├── player.go        # Daten-Modell Player (Name, Wins, Losses)
│       └── message.go       # WebSocket-Nachrichten-Typen (JSON)
├── templates/
│   ├── layout.html          # Basis-Layout
│   ├── index.html           # Name-Eingabe (erster Besuch)
│   ├── lobby.html           # Lobby: Spiel erstellen / beitreten
│   ├── game.html            # Spielfeld
│   ├── highscores.html      # Highscore-Tabelle (Rang, Name, Wins, Losses, Win-Rate)
│   └── rules.html           # Spielanleitung (statisch)
├── static/
│   ├── css/
│   │   ├── input.css        # TailwindCSS 4 Eingabe-Datei (@import "tailwindcss")
│   │   └── app.css          # Generiertes CSS (via tailwindcss-Binary) – eingebettet
│   └── js/
│       └── game.js          # AlpineJS + WebSocket-Spielsteuerung
└── bin/
    └── tailwindcss          # Lokales TailwindCSS 4 Standalone-Binary (nicht ins Git)
```

---

## Backend

### Technologie-Stack

| Komponente | Technologie                                             |
|------------|---------------------------------------------------------|
| Sprache    | Go 1.26                                                 |
| Framework  | Gin                                                     |
| WebSockets | `gorilla/websocket`                                     |
| Datenbank  | SQLite (via `modernc.org/sqlite` – reines Go, kein CGO) |
| Templating | `html/template`                                         |
| Embedding  | `embed.FS`                                              |

### Einbettung (Embedding)

Alle statischen Dateien (Templates, CSS, JS) werden via `//go:embed` direkt in das Binary eingebettet. Das Binary ist
damit selbst-enthalten und benötigt keine externe Dateisystemstruktur.

```go
//go:embed templates/* static/*
var embeddedFiles embed.FS
```

### Multiplayer-Ablauf

```
Spieler 1                        Server                        Spieler 2
    |                               |                               |
    |-- POST /api/room/create ------>|                               |
    |<-- { roomID, playerToken } ---|                               |
    |                               |                               |
    |-- GET /game/{roomID} -------->|                               |
    |-- WS  /ws/{roomID} ---------->|  (wartet auf Spieler 2)       |
    |                               |                               |
    |                               |<-- POST /api/room/join -------|
    |                               |--- { playerToken } ---------->|
    |                               |<-- WS  /ws/{roomID} ----------|
    |                               |                               |
    |<-- WS: game_start ------------|-- WS: game_start ------------>|
    |                               |                               |
    |-- WS: move {pos} ------------>|                               |
    |<-- WS: state_update ----------|-- WS: state_update ---------->|
```

### Spieler-Identifikation

Beim ersten Besuch gibt der Spieler einmalig seinen **Namen** ein (kein Passwort). Der Name wird im Browser als Cookie
`player_name` gespeichert und bei jedem API-Call / WS-Connect mitgeschickt. Beim Zurückkehren wird der Name automatisch
erkannt.

Der Server legt den Spieler per **Upsert** in der DB an (existiert der Name bereits, wird nichts überschrieben). Nach
Spielende werden Wins/Losses **automatisch** für beide Spieler aktualisiert – kein manuelles Eintragen nötig.

### REST-API (HTTP)

| Methode | Pfad                     | Beschreibung                                       |
|---------|--------------------------|----------------------------------------------------|
| GET     | `/`                      | Name-Eingabe oder Lobby (je nach Cookie)           |
| GET     | `/game/:roomID`          | Spielfeld-Seite (HTML)                             |
| GET     | `/highscores`            | Highscore-Seite (HTML)                             |
| GET     | `/rules`                 | Spielanleitung (HTML, statisch)                    |
| POST    | `/api/player/register`   | Name registrieren / Cookie setzen                  |
| POST    | `/api/room/create`       | Raum erstellen → `{roomID, playerToken}`           |
| POST    | `/api/room/join/:roomID` | Raum beitreten → `{playerToken}`                   |
| GET     | `/api/highscores`        | Highscores abrufen (JSON, sortiert nach Wins desc) |

### WebSocket-Protokoll

Verbindung: `WS /ws/:roomID` mit Header `X-Player-Token` (oder Query-Parameter).

**Client → Server:**

| Typ      | Payload                  | Beschreibung                     |
|----------|--------------------------|----------------------------------|
| `place`  | `{ pos: int }`           | Stein setzen (Phase 1)           |
| `move`   | `{ from: int, to: int }` | Stein ziehen (Phase 2/3)         |
| `remove` | `{ pos: int }`           | Gegnerstein entfernen nach Mühle |

**Server → Client (broadcast an beide Spieler):**

| Typ             | Payload                                  | Beschreibung                        |
|-----------------|------------------------------------------|-------------------------------------|
| `waiting`       | –                                        | Warte auf zweiten Spieler           |
| `game_start`    | `{ yourPlayer: 1\|2, opponent: string }` | Spiel beginnt (Name des Gegners)    |
| `state_update`  | `GameState`                              | Neuer Spielzustand nach jedem Zug   |
| `game_over`     | `{ winner: 1\|2, stats: PlayerStats }`   | Spielende + aktualisierte Statistik |
| `error`         | `{ message: string }`                    | Ungültiger Zug o.ä.                 |
| `opponent_left` | –                                        | Gegner hat Verbindung getrennt      |

### Spielzustand & Room

```go
type Room struct {
ID          string
Players     [2]*Client // nil = noch nicht verbunden
PlayerNames [2]string // Namen aus Cookie, gesetzt beim WS-Connect
Game        *GameState
Status      RoomStatus // Waiting, InProgress, Finished
}

type GameState struct {
Board      [24]int8 // 0=leer, 1=Spieler1, 2=Spieler2
Phase      Phase    // Placing, Moving, Flying
Turn       int8   // 1 oder 2
Placed     [2]int // noch zu setzende Steine
Removed    [2]int // entfernte Steine
MustRemove bool   // nach Mühle: Gegnerstein entfernen
}

type PlayerStats struct {
Name   string
Wins   int
Losses int
}
```

Alle aktiven Räume leben im **Hub** (In-Memory-Map, mutex-gesichert). Räume werden nach Spielende oder
Verbindungsabbruch aufgeräumt. Bei `game_over` schreibt der Hub die Ergebnisse automatisch in die DB (Upsert für beide
Spielernamen).

### Datenbank-Schema (SQLite)

```sql
CREATE TABLE players
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL UNIQUE,
    wins       INTEGER NOT NULL DEFAULT 0,
    losses     INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME         DEFAULT CURRENT_TIMESTAMP
);
```

Nach Spielende wird per Upsert aktualisiert:

```sql
INSERT INTO players (name, wins, losses)
VALUES (?, 1, 0) ON CONFLICT(name) DO
UPDATE SET wins = wins + 1;

INSERT INTO players (name, wins, losses)
VALUES (?, 0, 1) ON CONFLICT(name) DO
UPDATE SET losses = losses + 1;
```

Die Highscore-Tabelle zeigt: **Rang, Name, Wins, Losses, Win-Rate** – sortiert nach `wins DESC`, bei Gleichstand nach
`losses ASC`.

---

## Frontend

### Technologie-Stack

| Komponente  | Technologie                                            |
|-------------|--------------------------------------------------------|
| Markup      | HTML5 via `html/template`                              |
| Styling     | TailwindCSS 4 (lokales Standalone-Binary unter `bin/`) |
| Reaktivität | AlpineJS                                               |
| Spiellogik  | Natives JavaScript                                     |

### TailwindCSS 4 – Lokales Binary

TailwindCSS wird **nicht** per CDN eingebunden, sondern als heruntergeladenes Standalone-Binary lokal ausgeführt. Das
Binary liegt unter `bin/tailwindcss` und wird nicht in Git eingecheckt (`.gitignore`).

**Download (einmalig, plattformabhängig):**

```bash
# macOS (arm64)
curl -sLo bin/tailwindcss https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-arm64
chmod +x bin/tailwindcss

# macOS (x64)
curl -sLo bin/tailwindcss https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-x64
chmod +x bin/tailwindcss

# Linux (x64)
curl -sLo bin/tailwindcss https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
chmod +x bin/tailwindcss
```

**CSS-Build:**

```bash
./bin/tailwindcss -i static/css/input.css -o static/css/app.css --minify
```

Das generierte `static/css/app.css` wird via `embed.FS` in das Go-Binary eingebettet. Im Dockerfile wird das Binary für
Linux heruntergeladen und der CSS-Build als eigener Layer ausgeführt, bevor `go build` aufgerufen wird.

**Taskfile.yml (Auszug):**

```yaml
version: '3'

tasks:
  tw:install:
    desc: TailwindCSS Binary herunterladen
    cmds:
      - mkdir -p bin
      - curl -sLo bin/tailwindcss https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-{{OS}}-{{ARCH}}
      - chmod +x bin/tailwindcss

  tw:build:
    desc: CSS einmalig generieren
    cmds:
      - ./bin/tailwindcss -i static/css/input.css -o static/css/app.css --minify

  tw:watch:
    desc: CSS im Watch-Modus generieren (Entwicklung)
    cmds:
      - ./bin/tailwindcss -i static/css/input.css -o static/css/app.css --watch

  build:
    desc: Go-Binary bauen (CSS muss vorher generiert sein)
    deps: [ tw:build ]
    cmds:
      - go build -o bin/muehle ./...

  run:
    desc: Anwendung starten
    deps: [ tw:build ]
    cmds:
      - go run ./...

  test:
    desc: Tests ausführen
    cmds:
      - go test ./...
```

### Spielfeld-Darstellung

Das Mühle-Brett wird als **SVG** im HTML gerendert. Die 24 Positionen sind als klickbare Kreise dargestellt. Linien
zwischen den Positionen werden als SVG-`<line>`-Elemente gezeichnet.

```
a7 ---- b7 ---- c7
|       |       |
a5 -- b5 -- c5  |
|    |    |     |
a4-b4    c4-d4  ...
```

Die 24 Felder werden über Koordinaten (Column + Row) adressiert und in einer statischen JSON-Map auf SVG-Koordinaten
gemappt.

### AlpineJS Komponente

```javascript
Alpine.data('muehle', () => ({
    gameState: null,       // vom Server geladener Zustand
    selectedPos: null,     // aktuell ausgewählte Position
    message: '',           // Statusmeldung für den Spieler

    async newGame() { ...
    },
    async clickPosition(pos) { ...
    },
    async removeStone(pos) { ...
    },
    async saveHighscore(name) { ...
    },
}))
```

### AlpineJS Komponente

```javascript
Alpine.data('muehle', () => ({
    gameState: null,        // vom Server per WS empfangener Zustand
    myPlayer: null,         // 1 oder 2 (vom Server bei game_start gesetzt)
    selectedPos: null,      // aktuell ausgewählte Position (Phase 2/3)
    message: '',            // Statusmeldung für den Spieler
    ws: null,               // WebSocket-Verbindung

    connect(roomID, playerToken) { ...
    },   // WS verbinden
    onMessage(event) { ...
    },               // WS-Nachrichten verarbeiten
    clickPosition(pos) { ...
    },             // Klick auf Brett-Position
    async saveHighscore(name) { ...
    },
}))
```

### Seiten

1. **Name-Eingabe (`/`)** – Einmaliger Einstieg: Name eingeben → Cookie setzen → weiter zur Lobby. Bei bekanntem Cookie
   direkt zur Lobby.
2. **Lobby (`/lobby`)** – Spiel erstellen (→ Raum-Link teilen) oder per Raum-ID beitreten; eigene Stats (Wins/Losses)
   sichtbar; Link zur Spielanleitung
3. **Spielfeld (`/game/:roomID`)** – Spielfeld, Spieler-Anzeige, Wartescreen bis Gegner kommt; nach Spielende:
   Ergebnis + aktualisierte Stats
4. **Highscores (`/highscores`)** – Tabelle: Rang, Name, Wins, Losses, Win-Rate; sortiert nach Wins
5. **Spielanleitung (`/rules`)** – Statische Seite mit Regelübersicht (Phasen, Mühle, Spielziel), Brettdiagramm

---

## UI / Design

### Design-Richtung

Dunkles, atmosphärisches Interface mit räumlicher Tiefe – kein flaches Minimal-Design. Anlehnung an klassische
Brettspiel-Ästhetik, modernisiert mit Glass-Morphism und subtilen Lichteffekten.

### Farbpalette

| Rolle         | Farbe                        | Verwendung                             |
|---------------|------------------------------|----------------------------------------|
| Hintergrund   | `#0f0e17` (tiefschwarz-lila) | Body, Page-Hintergrund                 |
| Surface       | `#1a1a2e` (dunkelblau)       | Karten, Panels                         |
| Glass         | `rgba(255,255,255,0.05)`     | Glass-Morphism-Elemente                |
| Akzent        | `#e8a045` (warmes Amber)     | Buttons, aktive Felder, Highlights     |
| Spieler 1     | `#f0f0f0` (Elfenbein)        | Steine Spieler 1                       |
| Spieler 2     | `#1a1a1a` (Anthrazit)        | Steine Spieler 2                       |
| Mühle-Glow    | `#e8a045` mit Glow-Effekt    | Hervorhebung einer geschlossenen Mühle |
| Text primär   | `#fffffe`                    | Überschriften                          |
| Text sekundär | `#a7a9be`                    | Labels, Beschreibungen                 |

### Spielbrett

Das SVG-Brett erhält eine visuelle Aufwertung:

- **Hintergrund:** Dunkel mit leichtem radialen Gradient (Mitte heller) – wirkt wie Tiefenbeleuchtung
- **Linien:** Amber-farbig (`#e8a045`), leicht glühend via SVG `filter: drop-shadow`
- **Felder (Punkte):** Gefüllte Kreise mit `stroke`, hover-Highlight via CSS-Transition
- **Steine:** Große Kreise mit radialem Gradient (Lichtreflex oben links) + `drop-shadow` für 3D-Optik
- **Mühle-Animation:** Steine einer aktiven Mühle pulsieren mit einem Amber-Glow (`@keyframes mill-pulse`)
- **Auswahl:** Ausgewählter Stein hat einen Ring + sanftes Glow, erlaubte Zielfelder leuchten auf

### Typografie

- **Font:** `Inter` (via lokale Einbindung oder `@font-face`) – klar, modern, gut lesbar
- **Überschriften:** Größer, Letter-Spacing, leicht gedimmt
- **Spielstatus-Nachricht:** Zentriert, groß, mit Fade-In-Animation bei Änderung

### Glass-Morphism-Elemente

Panels (Lobby-Karten, Spieler-Info, Highscore-Tabelle) erhalten:

```css
background:

rgba
(
255
,
255
,
255
,
0.05
)
;
backdrop-filter:

blur
(
12
px

)
;
border:

1
px solid

rgba
(
255
,
255
,
255
,
0.08
)
;
border-radius:

1
rem

;
box-shadow:

0
8
px

32
px

rgba
(
0
,
0
,
0
,
0.4
)
;
```

### Animationen & Übergänge

| Element           | Animation                                               |
|-------------------|---------------------------------------------------------|
| Stein setzen      | `scale 0 → 1` + kurzes Bounce (`cubic-bezier`)          |
| Stein entfernen   | `scale 1 → 0` + `opacity 1 → 0`                         |
| Mühle geschlossen | Pulsierendes Glow auf den 3 Steinen (`mill-pulse`)      |
| Zug-Highlight     | Erlaubte Felder faden ein (`opacity 0 → 0.6`)           |
| Statusmeldung     | Fade-In bei jeder Änderung                              |
| Wartescreen       | Animierter Spinner / pulsierende Punkte                 |
| Spielende-Banner  | Slide-In von oben mit Gewinner-Hervorhebung             |
| Seitenübergänge   | Sanftes `opacity`-Crossfade via AlpineJS `x-transition` |

### Navigation

Einheitliche Top-Navigation auf allen Seiten: Logo/Titel links, Links zu Lobby / Highscores / Regeln rechts. Auf dem
Spielfeld zusätzlich: Spieler-Namen, Steinzähler, aktueller Spieler-Indikator.

---

## Spielfeld-Koordinaten

Die 24 Positionen des Bretts werden intern als Integer 0–23 kodiert:

```
Außenring:  0  1  2  3  4  5  6  7
Mittelring: 8  9 10 11 12 13 14 15
Innenring: 16 17 18 19 20 21 22 23
```

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

Mühlen (alle 16 möglichen Dreierreihen) sind als statisches Array im Backend kodiert.

---

## Spielanleitung (`/rules`)

Statische HTML-Seite, kein JavaScript nötig. Inhalt:

### Ziel des Spiels

Reduziere die Steine deines Gegners auf 2, oder blockiere alle seine Züge.

### Das Spielbrett

3 konzentrische Quadrate mit je 8 Feldern, verbunden durch Linien an den Mitten der Seiten. Insgesamt 24 Felder.

### Spielphasen

| Phase            | Bedingung                                          | Aktion                                                               |
|------------------|----------------------------------------------------|----------------------------------------------------------------------|
| **1 – Setzen**   | Jeder Spieler hat noch Steine in der Hand (je 9)   | Abwechselnd einen Stein auf ein freies Feld setzen                   |
| **2 – Ziehen**   | Alle Steine wurden gesetzt, Spieler hat > 3 Steine | Einen eigenen Stein auf ein direkt benachbartes freies Feld schieben |
| **3 – Springen** | Spieler hat genau 3 Steine übrig                   | Einen eigenen Stein auf ein beliebiges freies Feld setzen            |

### Mühle

Drei eigene Steine in einer geraden Linie = **Mühle**. Der Spieler darf sofort einen beliebigen Stein des Gegners vom
Brett entfernen.

- Steine aus einer geschlossenen Mühle des Gegners dürfen nur entfernt werden, wenn keine anderen Steine verfügbar sind.
- Eine Mühle kann durch Wegziehen und Zurückziehen erneut gebildet werden.

### Spielende

- Ein Spieler hat nur noch **2 Steine** → Niederlage
- Ein Spieler kann **keinen Zug** mehr machen (alle Steine blockiert) → Niederlage

---

## Dockerfile

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app

# TailwindCSS Binary herunterladen und CSS bauen
RUN curl -sLo bin/tailwindcss https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64 \
    && chmod +x bin/tailwindcss

COPY . .
RUN ./bin/tailwindcss -i static/css/input.css -o static/css/app.css --minify
RUN go build -o muehle ./...

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/muehle .
EXPOSE 8080
CMD ["./muehle"]
```

Da `modernc.org/sqlite` kein CGO benötigt, ist ein reines Alpine-Image ohne gcc ausreichend. Das TailwindCSS-Binary wird
nur im Builder-Stage benötigt und landet nicht im finalen Image.

---

## Implementierungsreihenfolge

1. **Projektstruktur & go.mod** – Module anlegen, Abhängigkeiten definieren, `.gitignore` für `bin/`
2. **Taskfile & TailwindCSS Setup** – `Taskfile.yml` anlegen, Binary via `task tw:install` herunterladen, `input.css`
   anlegen, CSS-Build prüfen
3. **Spiellogik** (`internal/game/`) – Board, Regeln, Zustandsmaschine
4. **Hub & WebSocket** (`internal/hub/`) – Room-Registry, Client-Verwaltung, Message-Loop, Broadcast
5. **Repository** (`internal/repository/`) – SQLite-Anbindung, Highscore-CRUD
6. **Handler & Router** – Gin-Setup, Lobby/WS/Highscore-Handler, Template-Rendering
7. **Templates & SVG-Board** – Lobby, Spielfeld-SVG, Highscore-Seite
8. **AlpineJS Frontend-Logik** – WebSocket-Client, Klick-Handling, Wartescreen, Zustandsanzeige
9. **Embedding** – `embed.FS` einbinden, Binary-Test
10. **Dockerfile** – Multi-Stage-Build mit TailwindCSS-Download, Smoke-Test
