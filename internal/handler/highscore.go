package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pmoscode/Nine-Men-s-Morris/internal/repository"
)

type HighscoreHandler struct {
	repo *repository.PlayerRepository
}

func NewHighscore(repo *repository.PlayerRepository) *HighscoreHandler {
	return &HighscoreHandler{repo: repo}
}

func (h *HighscoreHandler) List(c *gin.Context) {
	players, err := h.repo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Datenbankfehler"})
		return
	}
	c.JSON(http.StatusOK, players)
}
