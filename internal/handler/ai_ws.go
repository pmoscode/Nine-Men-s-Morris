package handler

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	aiengine "muehle/internal/ai"
	"muehle/internal/game"
	"muehle/internal/model"
)

const (
	aiWriteWait  = 10 * time.Second
	aiPongWait   = 60 * time.Second
	aiPingPeriod = (aiPongWait * 9) / 10
)

// AIWSHandler creates single-player AI game sessions over WebSocket.
type AIWSHandler struct{}

func NewAIWS() *AIWSHandler { return &AIWSHandler{} }

func (h *AIWSHandler) Handle(c *gin.Context) {
	difficulty := c.DefaultQuery("difficulty", "medium")
	depth, easy := difficultySettings(difficulty)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	s := &aiSession{
		gs:       game.NewGameState(),
		aiPlayer: 2, // human is always player 1
		depth:    depth,
		easy:     easy,
		conn:     conn,
		send:     make(chan []byte, 64),
	}

	go s.writePump()
	go s.readPump(difficulty)
}

func difficultySettings(d string) (depth int, easy bool) {
	switch d {
	case "easy":
		return 2, true
	case "hard":
		return 6, false
	default: // medium
		return 4, false
	}
}

func difficultyLabel(d string) string {
	switch d {
	case "easy":
		return "Leicht"
	case "hard":
		return "Schwer"
	default:
		return "Mittel"
	}
}

// ── AI session ────────────────────────────────────────────────────────────────

type aiSession struct {
	gs       *game.GameState
	aiPlayer int8
	depth    int
	easy     bool
	conn     *websocket.Conn
	send     chan []byte
}

func (s *aiSession) enqueue(msg model.ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ai marshal: %v", err)
		return
	}
	select {
	case s.send <- data:
	default:
		log.Printf("ai send buffer full, dropping")
	}
}

func (s *aiSession) sendState() {
	s.enqueue(model.ServerMessage{Type: model.MsgStateUpdate, Payload: s.gs})
}

func (s *aiSession) sendGameOver() {
	s.enqueue(model.ServerMessage{
		Type: model.MsgGameOver,
		Payload: model.GameOverPayload{
			Winner: int(s.gs.Winner),
			Stats:  []model.PlayerStats{},
		},
	})
}

// readPump is the main game loop for an AI session.
func (s *aiSession) readPump(difficulty string) {
	defer s.conn.Close()

	s.conn.SetReadLimit(512)
	s.conn.SetReadDeadline(time.Now().Add(aiPongWait))
	s.conn.SetPongHandler(func(string) error {
		s.conn.SetReadDeadline(time.Now().Add(aiPongWait))
		return nil
	})

	// Send game start immediately — no waiting for a second player.
	s.enqueue(model.ServerMessage{
		Type: model.MsgGameStart,
		Payload: model.GameStartPayload{
			YourPlayer: 1,
			Opponent:   "KI (" + difficultyLabel(difficulty) + ")",
			StartedAt:  time.Now().Unix(),
		},
	})
	s.sendState()

	for {
		_, raw, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ai ws read: %v", err)
			}
			return
		}
		if s.gs.Phase == game.PhaseOver {
			continue // game already ended, ignore stray messages
		}
		var msg model.ClientMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		s.handleHumanMove(msg)
	}
}

func (s *aiSession) writePump() {
	ticker := time.NewTicker(aiPingPeriod)
	defer func() {
		ticker.Stop()
		s.conn.Close()
	}()
	for {
		select {
		case data, ok := <-s.send:
			s.conn.SetWriteDeadline(time.Now().Add(aiWriteWait))
			if !ok {
				s.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := s.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			s.conn.SetWriteDeadline(time.Now().Add(aiWriteWait))
			if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *aiSession) handleHumanMove(msg model.ClientMessage) {
	human := int8(1)
	var errMsg string

	switch msg.Type {
	case "place":
		errMsg = game.ApplyPlace(s.gs, msg.Pos, human)
	case "move":
		errMsg = game.ApplyMove(s.gs, msg.From, msg.To, human)
	case "remove":
		errMsg = game.ApplyRemove(s.gs, msg.Pos, human)
	default:
		return
	}

	if errMsg != "" {
		s.enqueue(model.ServerMessage{
			Type:    model.MsgError,
			Payload: map[string]string{"message": errMsg},
		})
		return
	}

	s.sendState()

	if s.gs.Phase == game.PhaseOver {
		s.sendGameOver()
		return
	}

	// Let the AI respond for as long as it is its turn
	// (handles the case where the AI closes a mill and must also remove).
	for s.gs.Phase != game.PhaseOver && s.gs.Turn == s.aiPlayer {
		time.Sleep(450 * time.Millisecond)

		move := aiengine.BestMove(s.gs, s.aiPlayer, s.depth, s.easy)
		switch move.Type {
		case "place":
			game.ApplyPlace(s.gs, move.Pos, s.aiPlayer)
		case "move":
			game.ApplyMove(s.gs, move.From, move.To, s.aiPlayer)
		case "remove":
			game.ApplyRemove(s.gs, move.Pos, s.aiPlayer)
		}

		s.sendState()

		if s.gs.Phase == game.PhaseOver {
			s.sendGameOver()
			return
		}
	}
}
