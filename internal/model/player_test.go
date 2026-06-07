package model_test

import (
	"testing"

	"github.com/pmoscode/Nine-Men-s-Morris/internal/model"
)

func TestWinRate_ZeroGames(t *testing.T) {
	p := &model.Player{Wins: 0, Losses: 0}
	if got := p.WinRate(); got != "–" {
		t.Errorf("expected \"–\", got %q", got)
	}
}

func TestWinRate_AllWins(t *testing.T) {
	p := &model.Player{Wins: 5, Losses: 0}
	if got := p.WinRate(); got != "100%" {
		t.Errorf("expected \"100%%\", got %q", got)
	}
}

func TestWinRate_NoWins(t *testing.T) {
	p := &model.Player{Wins: 0, Losses: 4}
	if got := p.WinRate(); got != "0%" {
		t.Errorf("expected \"0%%\", got %q", got)
	}
}

func TestWinRate_Mixed(t *testing.T) {
	p := &model.Player{Wins: 3, Losses: 1} // 75%
	if got := p.WinRate(); got != "75%" {
		t.Errorf("expected \"75%%\", got %q", got)
	}
}

func TestWinRate_Rounds(t *testing.T) {
	p := &model.Player{Wins: 1, Losses: 2} // 33.3... → rounds to 33%
	if got := p.WinRate(); got != "33%" {
		t.Errorf("expected \"33%%\", got %q", got)
	}
}
