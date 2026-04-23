package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"muehle/internal/hub"
	"muehle/internal/model"
	"muehle/internal/repository"
)

type LobbyHandler struct {
	hub  *hub.Hub
	repo *repository.PlayerRepository
}

func NewLobby(h *hub.Hub, repo *repository.PlayerRepository) *LobbyHandler {
	return &LobbyHandler{hub: h, repo: repo}
}

func (h *LobbyHandler) Register(c *gin.Context) {
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name erforderlich"})
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name darf nicht leer sein"})
		return
	}
	if len(name) > 30 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name zu lang (max. 30 Zeichen)"})
		return
	}

	if err := h.repo.Upsert(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbankfehler"})
		return
	}

	c.SetCookie("player_name", name, 60*60*24*365, "/", "", true, false)
	c.JSON(http.StatusOK, gin.H{"name": name})
}

func (h *LobbyHandler) CreateRoom(c *gin.Context) {
	name, err := c.Cookie("player_name")
	if err != nil || name == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Kein Spielername"})
		return
	}

	var body struct {
		AllowSpectators bool `json:"allowSpectators"`
	}
	_ = c.ShouldBindJSON(&body) // optional body; ignore binding errors

	roomID, token := h.hub.CreateRoom(name, body.AllowSpectators)
	c.SetCookie("room_token_"+roomID, token, 60*60*24*7, "/", "", true, false)
	c.JSON(http.StatusOK, gin.H{"roomID": roomID, "token": token})
}

func (h *LobbyHandler) ListRooms(c *gin.Context) {
	rooms := h.hub.ListRooms()
	if rooms == nil {
		rooms = []model.RoomInfo{}
	}
	c.JSON(http.StatusOK, rooms)
}

func (h *LobbyHandler) JoinRoom(c *gin.Context) {
	name, err := c.Cookie("player_name")
	if err != nil || name == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Kein Spielername"})
		return
	}

	roomID := c.Param("roomID")
	token, ok := h.hub.JoinRoom(roomID, name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Raum nicht gefunden oder bereits voll"})
		return
	}

	c.SetCookie("room_token_"+roomID, token, 60*60*24*7, "/", "", true, false)
	c.JSON(http.StatusOK, gin.H{"roomID": roomID, "token": token})
}
