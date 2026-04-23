package hub

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"muehle/internal/game"
	"muehle/internal/model"
	"muehle/internal/repository"
)

const reconnectGrace = 60 * time.Second

type RoomStatus int

const (
	RoomWaiting    RoomStatus = iota // waiting for second player
	RoomInProgress                   // game ongoing
	RoomFinished                     // game over
)

type Room struct {
	ID              string
	Tokens          [2]string // playerToken[0] = player 1, [1] = player 2
	PlayerNames     [2]string
	Players         [2]*Client // nil when disconnected
	Game            *game.GameState
	Status          RoomStatus
	StartedAt       time.Time      // set when both players first connect
	cleanupTimers   [2]*time.Timer // per-slot grace-period timers
	RematchReady    [2]bool        // which players have requested rematch
	finishTimer     *time.Timer    // cleanup timer after game over
	AllowSpectators bool
	spectators      []*Client
	mu              sync.Mutex
}

type Hub struct {
	rooms map[string]*Room
	mu    sync.RWMutex
	repo  *repository.PlayerRepository
}

func New(repo *repository.PlayerRepository) *Hub {
	return &Hub{
		rooms: make(map[string]*Room),
		repo:  repo,
	}
}

// CreateRoom creates a new room, assigns player 1 token and returns roomID + token.
func (h *Hub) CreateRoom(playerName string, allowSpectators bool) (roomID, token string) {
	roomID = uuid.NewString()[:8]
	token = uuid.NewString()

	room := &Room{
		ID:              roomID,
		Tokens:          [2]string{token, ""},
		PlayerNames:     [2]string{playerName, ""},
		Game:            game.NewGameState(),
		Status:          RoomWaiting,
		AllowSpectators: allowSpectators,
	}

	h.mu.Lock()
	h.rooms[roomID] = room
	h.mu.Unlock()

	return roomID, token
}

// JoinRoom assigns player 2 to the room and returns their token.
// Returns false if the room is full, finished, or doesn't exist.
func (h *Hub) JoinRoom(roomID, playerName string) (token string, ok bool) {
	h.mu.RLock()
	room, exists := h.rooms[roomID]
	h.mu.RUnlock()

	if !exists {
		return "", false
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	// Allow if slot 2 is not yet assigned.
	if room.Status == RoomFinished || room.Tokens[1] != "" {
		return "", false
	}

	token = uuid.NewString()
	room.Tokens[1] = token
	room.PlayerNames[1] = playerName
	return token, true
}

// Connect attaches a WebSocket connection to the appropriate room slot.
// Supports initial connections and reconnections (same token, slot was nil).
func (h *Hub) Connect(conn *websocket.Conn, roomID, token string) {
	h.mu.RLock()
	room, exists := h.rooms[roomID]
	h.mu.RUnlock()

	if !exists {
		conn.Close()
		return
	}

	room.mu.Lock()

	slot := -1
	for i, t := range room.Tokens {
		if t == token {
			slot = i
			break
		}
	}
	if slot == -1 {
		room.mu.Unlock()
		conn.Close()
		return
	}

	// Cancel any pending cleanup timer for this slot (reconnect case).
	if room.cleanupTimers[slot] != nil {
		room.cleanupTimers[slot].Stop()
		room.cleanupTimers[slot] = nil
	}

	isReconnect := room.Status == RoomInProgress

	client := newClient(conn, h, roomID, room.PlayerNames[slot], slot+1)
	room.Players[slot] = client

	bothConnected := room.Players[0] != nil && room.Players[1] != nil
	if bothConnected && room.Status == RoomWaiting {
		room.Status = RoomInProgress
		room.StartedAt = time.Now()
	}

	p1 := room.Players[0]
	p2 := room.Players[1]
	names := room.PlayerNames
	gs := room.Game
	startedAt := room.StartedAt.Unix()
	room.mu.Unlock()

	go client.WritePump()
	go client.ReadPump()

	switch {
	case isReconnect:
		// Notify the waiting opponent that the player has reconnected.
		reconnOpponent := p2
		if slot == 1 {
			reconnOpponent = p1
		}
		if reconnOpponent != nil {
			reconnOpponent.Send(model.ServerMessage{Type: model.MsgOpponentReconnected})
		}
		// Restore game state for the reconnecting player.
		client.Send(model.ServerMessage{
			Type:    model.MsgGameStart,
			Payload: model.GameStartPayload{YourPlayer: slot + 1, Opponent: names[1-slot], StartedAt: startedAt},
		})
		client.Send(model.ServerMessage{Type: model.MsgStateUpdate, Payload: gs})

	case bothConnected:
		// Both players just connected for the first time – start the game.
		if p1 != nil {
			p1.Send(model.ServerMessage{
				Type:    model.MsgGameStart,
				Payload: model.GameStartPayload{YourPlayer: 1, Opponent: names[1], StartedAt: startedAt},
			})
		}
		if p2 != nil {
			p2.Send(model.ServerMessage{
				Type:    model.MsgGameStart,
				Payload: model.GameStartPayload{YourPlayer: 2, Opponent: names[0], StartedAt: startedAt},
			})
		}
		h.broadcastState(room)

	default:
		client.Send(model.ServerMessage{Type: model.MsgWaiting})
	}
}

// handleMessage processes a ClientMessage from a connected client.
func (h *Hub) handleMessage(c *Client, msg model.ClientMessage) {
	h.mu.RLock()
	room, exists := h.rooms[c.roomID]
	h.mu.RUnlock()
	if !exists {
		return
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	if c.isSpectator {
		return // spectators cannot send game messages
	}

	if msg.Type == "rematch" {
		h.handleRematch(room, c)
		return
	}

	if room.Status != RoomInProgress {
		return
	}

	gs := room.Game
	player := int8(c.PlayerSlot)
	var errMsg string

	switch msg.Type {
	case "place":
		errMsg = game.ApplyPlace(gs, msg.Pos, player)
	case "move":
		errMsg = game.ApplyMove(gs, msg.From, msg.To, player)
	case "remove":
		errMsg = game.ApplyRemove(gs, msg.Pos, player)
	default:
		c.Send(model.ServerMessage{Type: model.MsgError, Payload: map[string]string{"message": "Unbekannte Aktion"}})
		return
	}

	if errMsg != "" {
		c.Send(model.ServerMessage{Type: model.MsgError, Payload: map[string]string{"message": errMsg}})
		return
	}

	if gs.Phase == game.PhaseOver {
		room.Status = RoomFinished
		h.finishGame(room)
		return
	}

	h.broadcastStateUnlocked(room)
}

// handleDisconnect is called when a client's ReadPump exits.
// It starts a grace-period timer; if the player reconnects in time, the timer
// is cancelled in Connect(). Otherwise the opponent is notified and the room cleaned up.
func (h *Hub) handleDisconnect(c *Client) {
	h.mu.RLock()
	room, exists := h.rooms[c.roomID]
	h.mu.RUnlock()
	if !exists {
		return
	}

	room.mu.Lock()

	// Spectator disconnect: just remove from the slice, no grace period needed.
	if c.isSpectator {
		for i, s := range room.spectators {
			if s == c {
				room.spectators = append(room.spectators[:i], room.spectators[i+1:]...)
				break
			}
		}
		room.mu.Unlock()
		return
	}

	slot := c.PlayerSlot - 1

	// Ignore stale disconnect events (e.g. from a replaced connection).
	if room.Players[slot] != c {
		room.mu.Unlock()
		return
	}

	room.Players[slot] = nil
	opponent := room.Players[1-slot]
	wasInProgress := room.Status == RoomInProgress
	isFinished := room.Status == RoomFinished
	room.mu.Unlock()

	// Finished rooms are cleaned up by finishTimer — no reconnect grace period needed.
	if isFinished {
		return
	}

	// Inform the opponent immediately so they know a reconnect attempt is underway.
	if opponent != nil && wasInProgress {
		opponent.Send(model.ServerMessage{Type: model.MsgOpponentDisconnected})
	}

	// Start grace-period timer for both waiting and in-progress rooms so that
	// a page refresh or brief disconnect does not immediately kill the room.
	room.mu.Lock()
	room.cleanupTimers[slot] = time.AfterFunc(reconnectGrace, func() {
		if opponent != nil {
			opponent.Send(model.ServerMessage{Type: model.MsgOpponentLeft})
		}
		h.mu.Lock()
		delete(h.rooms, c.roomID)
		h.mu.Unlock()
		log.Printf("room %s cleaned up after reconnect grace period", c.roomID)
	})
	room.mu.Unlock()
}

func (h *Hub) finishGame(room *Room) {
	gs := room.Game
	winnerIdx := gs.Winner - 1
	loserIdx := 1 - winnerIdx

	winnerName := room.PlayerNames[winnerIdx]
	loserName := room.PlayerNames[loserIdx]

	winnerDelta, loserDelta, err := h.repo.RecordResult(winnerName, loserName)
	if err != nil {
		log.Printf("record result error: %v", err)
	}

	winner, _ := h.repo.Get(winnerName)
	loser, _ := h.repo.Get(loserName)

	payload := model.GameOverPayload{
		Winner: int(gs.Winner),
		Stats:  []model.PlayerStats{},
	}
	if winner != nil {
		payload.Stats = append(payload.Stats, model.PlayerStats{Name: winner.Name, Wins: winner.Wins, Losses: winner.Losses, Elo: winner.Elo, EloDelta: winnerDelta})
	}
	if loser != nil {
		payload.Stats = append(payload.Stats, model.PlayerStats{Name: loser.Name, Wins: loser.Wins, Losses: loser.Losses, Elo: loser.Elo, EloDelta: loserDelta})
	}

	msg := model.ServerMessage{Type: model.MsgGameOver, Payload: payload}
	for _, p := range room.Players {
		if p != nil {
			p.Send(msg)
		}
	}
	for _, s := range room.spectators {
		s.Send(msg)
	}

	// Keep the room alive for 10 minutes so players can request a rematch.
	room.finishTimer = time.AfterFunc(10*time.Minute, func() {
		h.mu.Lock()
		delete(h.rooms, room.ID)
		h.mu.Unlock()
		log.Printf("room %s cleaned up after finish grace period", room.ID)
	})
}

// handleRematch processes a rematch request from a player in a finished room.
// room.mu must be held by caller.
func (h *Hub) handleRematch(room *Room, c *Client) {
	if room.Status != RoomFinished {
		return
	}
	slot := c.PlayerSlot - 1
	if room.RematchReady[slot] {
		return // already requested
	}
	room.RematchReady[slot] = true

	if room.RematchReady[0] && room.RematchReady[1] {
		// Both players want a rematch — reset the game in-place.
		if room.finishTimer != nil {
			room.finishTimer.Stop()
			room.finishTimer = nil
		}
		room.Game = game.NewGameState()
		room.Status = RoomInProgress
		room.StartedAt = time.Now()
		room.RematchReady = [2]bool{}
		startedAt := room.StartedAt.Unix()
		names := room.PlayerNames
		p1 := room.Players[0]
		p2 := room.Players[1]

		if p1 != nil {
			p1.Send(model.ServerMessage{
				Type:    model.MsgGameStart,
				Payload: model.GameStartPayload{YourPlayer: 1, Opponent: names[1], StartedAt: startedAt},
			})
		}
		if p2 != nil {
			p2.Send(model.ServerMessage{
				Type:    model.MsgGameStart,
				Payload: model.GameStartPayload{YourPlayer: 2, Opponent: names[0], StartedAt: startedAt},
			})
		}
		h.broadcastStateUnlocked(room)
		return
	}

	// Only this player has requested — notify the opponent.
	opponent := room.Players[1-slot]
	if opponent != nil {
		opponent.Send(model.ServerMessage{Type: model.MsgRematchOffer})
	}
}

func (h *Hub) broadcastState(room *Room) {
	room.mu.Lock()
	defer room.mu.Unlock()
	h.broadcastStateUnlocked(room)
}

func (h *Hub) broadcastStateUnlocked(room *Room) {
	msg := model.ServerMessage{Type: model.MsgStateUpdate, Payload: room.Game}
	for _, p := range room.Players {
		if p != nil {
			p.Send(msg)
		}
	}
	for _, s := range room.spectators {
		s.Send(msg)
	}
}

// RoomExists returns true if the given room is waiting for a second player.
func (h *Hub) RoomExists(roomID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	r, ok := h.rooms[roomID]
	if !ok {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Status == RoomWaiting && r.Tokens[1] == ""
}

// ListRooms returns info about all non-finished rooms.
func (h *Hub) ListRooms() []model.RoomInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	list := make([]model.RoomInfo, 0, len(h.rooms))
	for _, r := range h.rooms {
		r.mu.Lock()
		if r.Status == RoomFinished {
			r.mu.Unlock()
			continue
		}
		info := model.RoomInfo{
			ID:              r.ID,
			Player1:         r.PlayerNames[0],
			AllowSpectators: r.AllowSpectators,
			SpectatorCount:  len(r.spectators),
		}
		if r.Status == RoomInProgress {
			info.Status = "playing"
			info.Player2 = r.PlayerNames[1]
		} else {
			info.Status = "waiting"
		}
		r.mu.Unlock()
		list = append(list, info)
	}
	return list
}

// RoomAllowsSpectators returns true if the room exists, is in progress, and allows spectators.
func (h *Hub) RoomAllowsSpectators(roomID string) bool {
	h.mu.RLock()
	r, ok := h.rooms[roomID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.AllowSpectators && r.Status == RoomInProgress
}

// ConnectSpectator attaches a read-only WebSocket connection to a room.
func (h *Hub) ConnectSpectator(conn *websocket.Conn, roomID string) {
	h.mu.RLock()
	room, exists := h.rooms[roomID]
	h.mu.RUnlock()

	if !exists {
		conn.Close()
		return
	}

	room.mu.Lock()
	if !room.AllowSpectators || room.Status != RoomInProgress {
		room.mu.Unlock()
		conn.Close()
		return
	}

	client := newSpectatorClient(conn, h, roomID)
	room.spectators = append(room.spectators, client)
	gs := room.Game
	names := room.PlayerNames
	startedAt := room.StartedAt.Unix()
	room.mu.Unlock()

	go client.WritePump()
	go client.ReadPump()

	client.Send(model.ServerMessage{
		Type: model.MsgGameStart,
		Payload: model.GameStartPayload{
			YourPlayer: 0,
			Player1:    names[0],
			Player2:    names[1],
			StartedAt:  startedAt,
		},
	})
	client.Send(model.ServerMessage{Type: model.MsgStateUpdate, Payload: gs})
}
