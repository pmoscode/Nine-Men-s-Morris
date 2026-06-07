package repository

import (
	"database/sql"
	"github.com/pmoscode/Nine-Men-s-Morris/internal/model"
	"math"
	"strings"

	_ "modernc.org/sqlite"
)

const eloK = 32
const eloDefault = 1200

type PlayerRepository struct {
	db *sql.DB
}

func NewPlayerRepository(dsn string) (*PlayerRepository, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	repo := &PlayerRepository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *PlayerRepository) Close() error {
	return r.db.Close()
}

func (r *PlayerRepository) migrate() error {
	_, err := r.db.Exec(`
		PRAGMA journal_mode=WAL;
		CREATE TABLE IF NOT EXISTS players (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT    NOT NULL UNIQUE,
			wins       INTEGER NOT NULL DEFAULT 0,
			losses     INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return err
	}
	// Idempotent: add elo column if it doesn't exist yet.
	_, err = r.db.Exec(`ALTER TABLE players ADD COLUMN elo INTEGER NOT NULL DEFAULT 1200`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}
	return nil
}

// Upsert creates a player if they don't exist yet (no stats change on first registration).
func (r *PlayerRepository) Upsert(name string) error {
	_, err := r.db.Exec(
		`INSERT INTO players (name) VALUES (?) ON CONFLICT(name) DO NOTHING`,
		name,
	)
	return err
}

// RecordResult increments wins/losses and updates ELO for both players.
// Returns the ELO delta (positive for winner, negative for loser).
func (r *PlayerRepository) RecordResult(winner, loser string) (winnerDelta, loserDelta int, err error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	// Ensure both rows exist.
	for _, name := range []string{winner, loser} {
		if _, err := tx.Exec(
			`INSERT INTO players (name) VALUES (?) ON CONFLICT(name) DO NOTHING`,
			name,
		); err != nil {
			return 0, 0, err
		}
	}

	// Fetch current ELO ratings.
	var winnerElo, loserElo int
	if err := tx.QueryRow(`SELECT elo FROM players WHERE name = ?`, winner).Scan(&winnerElo); err != nil {
		return 0, 0, err
	}
	if err := tx.QueryRow(`SELECT elo FROM players WHERE name = ?`, loser).Scan(&loserElo); err != nil {
		return 0, 0, err
	}

	winnerDelta, loserDelta = calcElo(winnerElo, loserElo)
	newWinnerElo := winnerElo + winnerDelta
	newLoserElo := loserElo + loserDelta
	if newLoserElo < 100 {
		newLoserElo = 100
	}

	if _, err := tx.Exec(
		`UPDATE players SET wins = wins + 1, elo = ? WHERE name = ?`,
		newWinnerElo, winner,
	); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(
		`UPDATE players SET losses = losses + 1, elo = ? WHERE name = ?`,
		newLoserElo, loser,
	); err != nil {
		return 0, 0, err
	}

	return winnerDelta, loserDelta, tx.Commit()
}

// calcElo returns (winnerDelta, loserDelta) using standard ELO with K=32.
func calcElo(winnerElo, loserElo int) (int, int) {
	expected := 1.0 / (1.0 + math.Pow(10, float64(loserElo-winnerElo)/400.0))
	delta := int(math.Round(eloK * (1.0 - expected)))
	return delta, -delta
}

// List returns all players sorted by ELO desc.
func (r *PlayerRepository) List() ([]model.Player, error) {
	rows, err := r.db.Query(`
		SELECT id, name, wins, losses, elo
		FROM players
		ORDER BY elo DESC, wins DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []model.Player
	for rows.Next() {
		var p model.Player
		if err := rows.Scan(&p.ID, &p.Name, &p.Wins, &p.Losses, &p.Elo); err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

// Get returns a single player by name.
func (r *PlayerRepository) Get(name string) (*model.Player, error) {
	row := r.db.QueryRow(
		`SELECT id, name, wins, losses, elo FROM players WHERE name = ?`,
		name,
	)
	var p model.Player
	if err := row.Scan(&p.ID, &p.Name, &p.Wins, &p.Losses, &p.Elo); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}
