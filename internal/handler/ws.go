package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"muehle/internal/hub"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins for simplicity
	},
}

type WSHandler struct {
	hub *hub.Hub
}

func NewWS(h *hub.Hub) *WSHandler {
	return &WSHandler{hub: h}
}

func (h *WSHandler) Handle(c *gin.Context) {
	roomID := c.Param("roomID")
	token := c.Query("token")

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token fehlt"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	h.hub.Connect(conn, roomID, token)
}

func (h *WSHandler) HandleSpectator(c *gin.Context) {
	roomID := c.Param("roomID")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	h.hub.ConnectSpectator(conn, roomID)
}
