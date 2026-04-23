package handler

import (
	"html/template"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
	"muehle/internal/game"
	"muehle/internal/hub"
	"muehle/internal/repository"
)

// BoardPos is passed to the game template for SVG rendering.
type BoardPos struct {
	I  int
	X  int
	Y  int
	RX int // reflex highlight X (X-6)
	RY int // reflex highlight Y (Y-6)
	SX int // shadow X (X+2)
	SY int // shadow Y (Y+3)
}

type PageHandler struct {
	tmpls map[string]*template.Template
	repo  *repository.PlayerRepository
	hub   *hub.Hub
}

var TemplateFuncs = template.FuncMap{
	"add": func(a, b int) int { return a + b },
}

func NewPage(embeddedFS fs.FS, repo *repository.PlayerRepository, h *hub.Hub) (*PageHandler, error) {
	pages := []string{"index", "lobby", "game", "highscores", "rules"}
	tmpls := make(map[string]*template.Template, len(pages))

	for _, name := range pages {
		t, err := template.New("").Funcs(TemplateFuncs).ParseFS(embeddedFS,
			"templates/layout.html",
			"templates/"+name+".html",
		)
		if err != nil {
			return nil, err
		}
		tmpls[name] = t
	}
	return &PageHandler{tmpls: tmpls, repo: repo, hub: h}, nil
}

func (h *PageHandler) render(c *gin.Context, page string, data gin.H) {
	t := h.tmpls[page]
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(c.Writer, "layout", data); err != nil {
		_ = c.Error(err)
	}
}

func (h *PageHandler) Index(c *gin.Context) {
	name, _ := c.Cookie("player_name")
	if name != "" {
		next := c.Query("next")
		if next != "" {
			c.Redirect(http.StatusFound, next)
		} else {
			c.Redirect(http.StatusFound, "/lobby")
		}
		return
	}
	h.render(c, "index", gin.H{})
}

func (h *PageHandler) Lobby(c *gin.Context) {
	name, err := c.Cookie("player_name")
	if err != nil || name == "" {
		c.Redirect(http.StatusFound, "/")
		return
	}
	h.render(c, "lobby", gin.H{"PlayerName": name})
}

func boardPositions() []BoardPos {
	positions := make([]BoardPos, 24)
	for i, p := range game.SVGPositions {
		positions[i] = BoardPos{
			I: i,
			X: p[0], Y: p[1],
			RX: p[0] - 6, RY: p[1] - 6,
			SX: p[0] + 2, SY: p[1] + 3,
		}
	}
	return positions
}

func (h *PageHandler) Game(c *gin.Context) {
	name, err := c.Cookie("player_name")
	if err != nil || name == "" {
		c.Redirect(http.StatusFound, "/?next="+c.Request.URL.RequestURI())
		return
	}

	roomID := c.Param("roomID")
	cookieName := "room_token_" + roomID
	token, _ := c.Cookie(cookieName)

	if token == "" {
		// Player arrived via share link – auto-join server-side before rendering.
		var ok bool
		token, ok = h.hub.JoinRoom(roomID, name)
		if !ok {
			c.Redirect(http.StatusFound, "/lobby")
			return
		}
		c.SetCookie(cookieName, token, 60*60*24*7, "/", "", false, true)
	}

	h.render(c, "game", gin.H{
		"PlayerName":   name,
		"RoomID":       roomID,
		"Token":        token,
		"Positions":    boardPositions(),
		"Spectator":    false,
		"AIDifficulty": "",
	})
}

func (h *PageHandler) Highscores(c *gin.Context) {
	players, err := h.repo.List()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	h.render(c, "highscores", gin.H{"Players": players})
}

func (h *PageHandler) Spectate(c *gin.Context) {
	roomID := c.Param("roomID")
	if !h.hub.RoomAllowsSpectators(roomID) {
		c.Redirect(http.StatusFound, "/lobby")
		return
	}
	playerName, _ := c.Cookie("player_name")
	h.render(c, "game", gin.H{
		"PlayerName":   playerName,
		"RoomID":       roomID,
		"Token":        "",
		"Positions":    boardPositions(),
		"Spectator":    true,
		"AIDifficulty": "",
	})
}

func (h *PageHandler) AI(c *gin.Context) {
	name, err := c.Cookie("player_name")
	if err != nil || name == "" {
		c.Redirect(http.StatusFound, "/?next="+c.Request.URL.RequestURI())
		return
	}
	difficulty := c.DefaultQuery("difficulty", "medium")
	switch difficulty {
	case "easy", "medium", "hard":
	default:
		difficulty = "medium"
	}
	h.render(c, "game", gin.H{
		"PlayerName":   name,
		"RoomID":       "",
		"Token":        "",
		"Positions":    boardPositions(),
		"Spectator":    false,
		"AIDifficulty": difficulty,
	})
}

func (h *PageHandler) Rules(c *gin.Context) {
	h.render(c, "rules", gin.H{})
}
