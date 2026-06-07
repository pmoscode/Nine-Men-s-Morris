package hub

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pmoscode/Nine-Men-s-Morris/internal/model"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Client represents a single WebSocket connection (one player in a room).
type Client struct {
	PlayerName  string
	PlayerSlot  int // 1 or 2; 0 for spectators
	isSpectator bool
	conn        *websocket.Conn
	send        chan []byte
	hub         *Hub
	roomID      string
}

func newClient(conn *websocket.Conn, h *Hub, roomID, name string, slot int) *Client {
	return &Client{
		PlayerName: name,
		PlayerSlot: slot,
		conn:       conn,
		send:       make(chan []byte, 64),
		hub:        h,
		roomID:     roomID,
	}
}

func newSpectatorClient(conn *websocket.Conn, h *Hub, roomID string) *Client {
	return &Client{
		isSpectator: true,
		conn:        conn,
		send:        make(chan []byte, 64),
		hub:         h,
		roomID:      roomID,
	}
}

// Send encodes msg as JSON and queues it for writing.
func (c *Client) Send(msg model.ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("marshal error: %v", err)
		return
	}
	select {
	case c.send <- data:
	default:
		log.Printf("client %s send buffer full, dropping message", c.PlayerName)
	}
}

// ReadPump reads messages from the WebSocket and dispatches them to the hub.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.handleDisconnect(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ws read error: %v", err)
			}
			break
		}

		var msg model.ClientMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.Send(model.ServerMessage{Type: model.MsgError, Payload: map[string]string{"message": "Invalid message"}})
			continue
		}

		c.hub.handleMessage(c, msg)
	}
}

// WritePump drains the send channel and writes to the WebSocket.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case data, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
