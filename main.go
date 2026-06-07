package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pmoscode/Nine-Men-s-Morris/internal/handler"
	"github.com/pmoscode/Nine-Men-s-Morris/internal/hub"
	"github.com/pmoscode/Nine-Men-s-Morris/internal/repository"
)

//go:embed templates/* static/*
var embeddedFiles embed.FS

func dbPath() string {
	if p := os.Getenv("DB_PATH"); p != "" {
		return p
	}
	return filepath.Join(".", "database", "muehle.db")
}

func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return ":" + p
	}
	return ":8080"
}

// trustedProxies reads TRUSTED_PROXIES as a comma-separated list.
// Empty or unset returns nil (trust no proxies).
func trustedProxies() []string {
	v := os.Getenv("TRUSTED_PROXIES")
	if v == "" {
		return nil
	}
	var proxies []string
	for _, p := range strings.Split(v, ",") {
		if t := strings.TrimSpace(p); t != "" {
			proxies = append(proxies, t)
		}
	}
	return proxies
}

func main() {
	db := dbPath()
	if err := os.MkdirAll(filepath.Dir(db), 0755); err != nil {
		log.Fatalf("db dir: %v", err)
	}
	repo, err := repository.NewPlayerRepository(db)
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	defer repo.Close()

	h := hub.New(repo)

	pageHandler, err := handler.NewPage(embeddedFiles, repo, h)
	if err != nil {
		log.Fatalf("template init: %v", err)
	}

	lobbyHandler := handler.NewLobby(h, repo)
	wsHandler := handler.NewWS(h)
	aiWSHandler := handler.NewAIWS()
	hsHandler := handler.NewHighscore(repo)

	r := gin.Default()
	if err := r.SetTrustedProxies(trustedProxies()); err != nil {
		log.Fatalf("trusted proxies: %v", err)
	}

	// Static files from embedded FS
	staticSubFS, err := fs.Sub(embeddedFiles, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	r.StaticFS("/static", http.FS(staticSubFS))

	// Pages
	r.GET("/", pageHandler.Index)
	r.GET("/lobby", pageHandler.Lobby)
	r.GET("/game/:roomID", pageHandler.Game)
	r.GET("/spectate/:roomID", pageHandler.Spectate)
	r.GET("/ai", pageHandler.AI)
	r.GET("/highscores", pageHandler.Highscores)
	r.GET("/rules", pageHandler.Rules)

	// API
	r.POST("/api/player/register", lobbyHandler.Register)
	r.POST("/api/room/create", lobbyHandler.CreateRoom)
	r.POST("/api/room/join/:roomID", lobbyHandler.JoinRoom)
	r.GET("/api/rooms", lobbyHandler.ListRooms)
	r.GET("/api/highscores", hsHandler.List)

	// WebSocket
	r.GET("/ws/:roomID", wsHandler.Handle)
	r.GET("/ws/:roomID/spectate", wsHandler.HandleSpectator)
	r.GET("/ai/ws", aiWSHandler.Handle)

	addr := port()
	log.Printf("Nine Men's Morris server listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
