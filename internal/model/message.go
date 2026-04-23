package model

type MsgType string

const (
	MsgWaiting              MsgType = "waiting"
	MsgGameStart            MsgType = "game_start"
	MsgStateUpdate          MsgType = "state_update"
	MsgGameOver             MsgType = "game_over"
	MsgError                MsgType = "error"
	MsgOpponentLeft         MsgType = "opponent_left"
	MsgOpponentDisconnected MsgType = "opponent_disconnected"
	MsgOpponentReconnected  MsgType = "opponent_reconnected"
	MsgRematchOffer         MsgType = "rematch_offer"
)

// ClientMessage is sent from browser → server over WebSocket.
type ClientMessage struct {
	Type string `json:"type"`
	Pos  int    `json:"pos,omitempty"`
	From int    `json:"from,omitempty"`
	To   int    `json:"to,omitempty"`
}

// ServerMessage is sent from server → browser over WebSocket.
type ServerMessage struct {
	Type    MsgType `json:"type"`
	Payload any     `json:"payload,omitempty"`
}

type GameStartPayload struct {
	YourPlayer int    `json:"yourPlayer"` // 0 for spectators
	Opponent   string `json:"opponent"`
	Player1    string `json:"player1,omitempty"` // filled for spectators
	Player2    string `json:"player2,omitempty"` // filled for spectators
	StartedAt  int64  `json:"startedAt"`
}

type RoomInfo struct {
	ID              string `json:"id"`
	Player1         string `json:"player1"`
	Player2         string `json:"player2,omitempty"`
	Status          string `json:"status"` // "waiting" | "playing"
	AllowSpectators bool   `json:"allowSpectators"`
	SpectatorCount  int    `json:"spectatorCount"`
}

type GameOverPayload struct {
	Winner int           `json:"winner"`
	Stats  []PlayerStats `json:"stats"`
}

type PlayerStats struct {
	Name     string `json:"name"`
	Wins     int    `json:"wins"`
	Losses   int    `json:"losses"`
	Elo      int    `json:"elo"`
	EloDelta int    `json:"eloDelta"`
}
