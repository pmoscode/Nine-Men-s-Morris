package ai

import (
	"testing"

	"muehle/internal/game"
)

// ── countMills ────────────────────────────────────────────────────────────────

func TestCountMills_Empty(t *testing.T) {
	var b [24]int8
	if n := countMills(b, 1); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestCountMills_One(t *testing.T) {
	var b [24]int8
	b[0], b[1], b[2] = 1, 1, 1 // mill [0,1,2]
	if n := countMills(b, 1); n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
}

func TestCountMills_Two(t *testing.T) {
	var b [24]int8
	b[0], b[1], b[2] = 1, 1, 1  // mill [0,1,2]
	b[8], b[9], b[10] = 1, 1, 1 // mill [8,9,10]
	if n := countMills(b, 1); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

func TestCountMills_OpponentNotCounted(t *testing.T) {
	var b [24]int8
	b[0], b[1], b[2] = 2, 2, 2 // opponent's mill
	if n := countMills(b, 1); n != 0 {
		t.Errorf("opponent mill should not count for player 1, got %d", n)
	}
}

// ── countPotential ────────────────────────────────────────────────────────────

func TestCountPotential_None(t *testing.T) {
	var b [24]int8
	if n := countPotential(b, 1); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestCountPotential_One(t *testing.T) {
	var b [24]int8
	b[0], b[1] = 1, 1 // two own stones; position 2 empty → potential mill [0,1,2]
	if n := countPotential(b, 1); n != 1 {
		t.Errorf("expected 1 potential mill, got %d", n)
	}
}

func TestCountPotential_Blocked(t *testing.T) {
	var b [24]int8
	b[0], b[1] = 1, 1 // two own stones; position 2 blocked by opponent
	b[2] = 2
	if n := countPotential(b, 1); n != 0 {
		t.Errorf("blocked line should not count as potential, got %d", n)
	}
}

// ── generateMoves ─────────────────────────────────────────────────────────────

func TestGenerateMoves_PlacePhase(t *testing.T) {
	gs := game.NewGameState() // PhasePlace, all 24 positions empty
	moves := generateMoves(gs)
	if len(moves) != 24 {
		t.Errorf("expected 24 place moves on empty board, got %d", len(moves))
	}
	for _, m := range moves {
		if m.Type != "place" {
			t.Errorf("expected type=place, got %q", m.Type)
		}
	}
}

func TestGenerateMoves_MustRemove(t *testing.T) {
	gs := game.NewGameState()
	gs.MustRemove = true
	gs.Board[5] = 2  // free opponent stone
	gs.Board[10] = 2 // another free opponent stone
	moves := generateMoves(gs)
	if len(moves) != 2 {
		t.Errorf("expected 2 remove moves, got %d", len(moves))
	}
	for _, m := range moves {
		if m.Type != "remove" {
			t.Errorf("expected type=remove, got %q", m.Type)
		}
	}
}

func TestGenerateMoves_MovePhase(t *testing.T) {
	gs := game.NewGameState()
	gs.Phase = game.PhaseMove
	gs.ToPlace = [2]int{0, 0}
	gs.Board[0] = 1 // P1 at 0; Adjacent[0] = {1, 7}, both empty → 2 moves
	gs.Board[8] = 2
	gs.Board[9] = 2
	gs.Board[10] = 2
	gs.Board[11] = 2
	moves := generateMoves(gs)
	if len(moves) != 2 {
		t.Errorf("expected 2 move moves from pos 0, got %d", len(moves))
	}
	for _, m := range moves {
		if m.Type != "move" {
			t.Errorf("expected type=move, got %q", m.Type)
		}
	}
}

// ── applyMove ─────────────────────────────────────────────────────────────────

func TestApplyMove_NonDestructive(t *testing.T) {
	gs := game.NewGameState()
	original := *gs

	child := applyMove(gs, Move{Type: "place", Pos: 0})

	if *gs != original {
		t.Error("applyMove mutated the original GameState")
	}
	if child.Board[0] != 1 {
		t.Error("stone not placed in child state")
	}
}

// ── BestMove ─────────────────────────────────────────────────────────────────

func TestBestMove_ReturnsValidMoveInPlacePhase(t *testing.T) {
	gs := game.NewGameState()
	m := BestMove(gs, 1, 2, false)
	if m.Type != "place" {
		t.Errorf("expected place move, got %q", m.Type)
	}
	if m.Pos < 0 || m.Pos > 23 {
		t.Errorf("invalid position %d", m.Pos)
	}
}

func TestBestMove_PrefersMillCompletion(t *testing.T) {
	// P1 (AI) has stones at 0 and 1; placing at 2 completes mill [0,1,2].
	// That position should score highest due to the mill bonus in evaluate.
	gs := game.NewGameState()
	gs.Board[0], gs.Board[1] = 1, 1
	gs.ToPlace[0] = 7 // 2 stones placed, 7 remaining

	m := BestMove(gs, 1, 2, false)
	if m.Type != "place" || m.Pos != 2 {
		t.Errorf("expected place at 2 to complete mill [0,1,2], got type=%q pos=%d", m.Type, m.Pos)
	}
}

func TestBestMove_ReturnsValidMoveInMovePhase(t *testing.T) {
	gs := game.NewGameState()
	gs.Phase = game.PhaseMove
	gs.ToPlace = [2]int{0, 0}
	gs.Board[0] = 1
	gs.Board[1] = 1
	gs.Board[2] = 1
	gs.Board[8] = 2
	gs.Board[9] = 2
	gs.Board[10] = 2
	gs.Board[11] = 2

	m := BestMove(gs, 1, 2, false)
	if m.Type != "move" {
		t.Errorf("expected move type, got %q", m.Type)
	}
	if m.From < 0 || m.From > 23 || m.To < 0 || m.To > 23 {
		t.Errorf("invalid move positions from=%d to=%d", m.From, m.To)
	}
}
