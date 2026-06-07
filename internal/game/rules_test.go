package game_test

import (
	"testing"

	"github.com/pmoscode/Nine-Men-s-Morris/internal/game"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func freshState() *game.GameState {
	return game.NewGameState()
}

// place stones for the given player at each position (no rule checks).
func setStones(gs *game.GameState, player int8, positions ...int) {
	for _, p := range positions {
		gs.Board[p] = player
	}
}

// ── FormsMill ─────────────────────────────────────────────────────────────────

func TestFormsMill_CompletesOuter(t *testing.T) {
	var b [24]int8
	b[0], b[1] = 1, 1
	if !game.FormsMill(b, 2, 1) {
		t.Error("expected mill [0,1,2]")
	}
}

func TestFormsMill_CrossMill(t *testing.T) {
	var b [24]int8
	b[1], b[9] = 1, 1 // mill [1,9,17]
	if !game.FormsMill(b, 17, 1) {
		t.Error("expected cross mill [1,9,17]")
	}
}

func TestFormsMill_Incomplete(t *testing.T) {
	var b [24]int8
	b[0] = 1 // only one stone in [0,1,2]
	if game.FormsMill(b, 2, 1) {
		t.Error("did not expect mill with only two stones in line")
	}
}

func TestFormsMill_WrongPlayer(t *testing.T) {
	var b [24]int8
	b[0], b[1] = 2, 2 // opponent stones
	if game.FormsMill(b, 2, 1) {
		t.Error("mill with opponent stones should not count")
	}
}

// ── IsInMill ─────────────────────────────────────────────────────────────────

func TestIsInMill_True(t *testing.T) {
	var b [24]int8
	b[0], b[1], b[2] = 1, 1, 1
	if !game.IsInMill(b, 1, 1) {
		t.Error("stone at 1 should be in mill [0,1,2]")
	}
}

func TestIsInMill_False(t *testing.T) {
	var b [24]int8
	b[0], b[1] = 1, 1 // incomplete mill
	if game.IsInMill(b, 0, 1) {
		t.Error("stone at 0 should not be in an incomplete mill")
	}
}

// ── CanRemove ────────────────────────────────────────────────────────────────

func TestCanRemove_FreeOpponentStone(t *testing.T) {
	var b [24]int8
	b[5] = 2
	if !game.CanRemove(b, 5, 1) {
		t.Error("should be able to remove free opponent stone")
	}
}

func TestCanRemove_OwnStone(t *testing.T) {
	var b [24]int8
	b[5] = 1
	if game.CanRemove(b, 5, 1) {
		t.Error("should not be able to remove own stone")
	}
}

func TestCanRemove_MillStoneWithFreeOpponentExists(t *testing.T) {
	var b [24]int8
	// Opponent (P2) has a complete mill at [8,9,10] and a free stone at 3.
	b[8], b[9], b[10] = 2, 2, 2
	b[3] = 2 // free opponent stone
	// Cannot remove mill stone 8 because free stone 3 exists.
	if game.CanRemove(b, 8, 1) {
		t.Error("should not remove mill stone when free opponent stones exist")
	}
}

func TestCanRemove_MillStoneAllInMills(t *testing.T) {
	var b [24]int8
	// Opponent (P2) has two complete mills: [8,9,10] and [10,11,12].
	b[8], b[9], b[10], b[11], b[12] = 2, 2, 2, 2, 2
	// All P2 stones are in mills; any mill stone is now removable.
	if !game.CanRemove(b, 9, 1) {
		t.Error("should remove mill stone when all opponent stones are in mills")
	}
}

// ── ApplyPlace ───────────────────────────────────────────────────────────────

func TestApplyPlace_Valid(t *testing.T) {
	gs := freshState()
	if err := game.ApplyPlace(gs, 0, 1); err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	if gs.Board[0] != 1 {
		t.Error("stone not placed")
	}
	if gs.Turn != 2 {
		t.Error("turn should advance to player 2")
	}
	if gs.ToPlace[0] != 8 {
		t.Error("ToPlace[0] should be 8")
	}
}

func TestApplyPlace_OccupiedPosition(t *testing.T) {
	gs := freshState()
	game.ApplyPlace(gs, 0, 1)
	// P2's turn; they try to place on the occupied square.
	if err := game.ApplyPlace(gs, 0, 2); err == "" {
		t.Error("expected error for occupied position")
	}
}

func TestApplyPlace_WrongTurn(t *testing.T) {
	gs := freshState() // P1's turn
	if err := game.ApplyPlace(gs, 0, 2); err == "" {
		t.Error("expected error for wrong-turn placement")
	}
}

func TestApplyPlace_OutOfBounds(t *testing.T) {
	gs := freshState()
	if err := game.ApplyPlace(gs, 24, 1); err == "" {
		t.Error("expected error for out-of-bounds position")
	}
	if err := game.ApplyPlace(gs, -1, 1); err == "" {
		t.Error("expected error for negative position")
	}
}

func TestApplyPlace_WrongPhase(t *testing.T) {
	gs := freshState()
	gs.Phase = game.PhaseMove
	if err := game.ApplyPlace(gs, 0, 1); err == "" {
		t.Error("expected error when placing in move phase")
	}
}

func TestApplyPlace_FormsMillSetsMustRemove(t *testing.T) {
	gs := freshState()
	setStones(gs, 1, 0, 1)    // P1 stones at 0 and 1
	gs.ToPlace[0] = 7         // already placed 2
	game.ApplyPlace(gs, 2, 1) // completes mill [0,1,2]
	if !gs.MustRemove {
		t.Error("MustRemove should be true after forming a mill")
	}
	if gs.Turn != 1 {
		t.Error("turn must stay with P1 until removal is done")
	}
}

func TestApplyPlace_NoExtraStoneWhenHandEmpty(t *testing.T) {
	gs := freshState()
	gs.ToPlace[0] = 0
	if err := game.ApplyPlace(gs, 0, 1); err == "" {
		t.Error("expected error when player has no stones left to place")
	}
}

// ── ApplyMove ────────────────────────────────────────────────────────────────

func newMovePhaseState() *game.GameState {
	gs := freshState()
	gs.Phase = game.PhaseMove
	gs.ToPlace = [2]int{0, 0}
	return gs
}

func TestApplyMove_ValidAdjacentMove(t *testing.T) {
	gs := newMovePhaseState()
	setStones(gs, 1, 0) // P1 at 0; adjacent: 1, 7
	if err := game.ApplyMove(gs, 0, 1, 1); err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	if gs.Board[0] != 0 || gs.Board[1] != 1 {
		t.Error("stone not moved correctly")
	}
	if gs.Turn != 2 {
		t.Error("turn should advance")
	}
}

func TestApplyMove_NonAdjacentBlocked(t *testing.T) {
	gs := newMovePhaseState()
	setStones(gs, 1, 0)
	// 0 and 5 are not adjacent
	if err := game.ApplyMove(gs, 0, 5, 1); err == "" {
		t.Error("expected error for non-adjacent move")
	}
}

func TestApplyMove_FlyingPhase(t *testing.T) {
	gs := newMovePhaseState()
	// P1 has exactly 3 stones on board → flying allowed
	setStones(gs, 1, 0, 1, 2)
	setStones(gs, 2, 8, 9, 10, 11, 12) // P2 has enough stones
	// In flying mode P1 can jump anywhere
	if err := game.ApplyMove(gs, 0, 23, 1); err != "" {
		t.Fatalf("unexpected error in flying phase: %s", err)
	}
}

func TestApplyMove_OccupiedTarget(t *testing.T) {
	gs := newMovePhaseState()
	setStones(gs, 1, 0)
	setStones(gs, 2, 1) // P2 occupies target
	if err := game.ApplyMove(gs, 0, 1, 1); err == "" {
		t.Error("expected error when moving to occupied square")
	}
}

func TestApplyMove_WrongPlayer(t *testing.T) {
	gs := newMovePhaseState()
	setStones(gs, 2, 0)
	gs.Turn = 1
	if err := game.ApplyMove(gs, 0, 1, 2); err == "" {
		t.Error("expected error when wrong player moves")
	}
}

func TestApplyMove_WrongPhase(t *testing.T) {
	gs := freshState() // PhasePlace
	if err := game.ApplyMove(gs, 0, 1, 1); err == "" {
		t.Error("expected error when moving in place phase")
	}
}

// ── ApplyRemove ──────────────────────────────────────────────────────────────

func TestApplyRemove_ValidRemove(t *testing.T) {
	gs := newMovePhaseState()
	setStones(gs, 1, 0, 1, 2)
	setStones(gs, 2, 8, 9, 10, 11, 12)
	gs.MustRemove = true
	if err := game.ApplyRemove(gs, 8, 1); err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	if gs.Board[8] != 0 {
		t.Error("stone should be removed")
	}
	if gs.MustRemove {
		t.Error("MustRemove should be cleared")
	}
}

func TestApplyRemove_NotMustRemove(t *testing.T) {
	gs := newMovePhaseState()
	setStones(gs, 2, 5)
	if err := game.ApplyRemove(gs, 5, 1); err == "" {
		t.Error("expected error when MustRemove is false")
	}
}

func TestApplyRemove_OwnStone(t *testing.T) {
	gs := newMovePhaseState()
	setStones(gs, 1, 5)
	gs.MustRemove = true
	if err := game.ApplyRemove(gs, 5, 1); err == "" {
		t.Error("expected error when removing own stone")
	}
}

// ── Win conditions ────────────────────────────────────────────────────────────

func TestWinner_TooFewStones(t *testing.T) {
	gs := newMovePhaseState()
	// P2 has only 2 stones on board; after P2 moves, P1 should win.
	// Actually checkWinner is called after turn switches, so we make P2 move
	// and P1 is now checked: P1 must have ≥3 stones too or P2 wins first.
	setStones(gs, 1, 0, 1, 2, 3, 4) // P1 safe
	setStones(gs, 2, 8, 14)         // P2 only 2 stones (ToPlace already 0)

	// P1's turn; P1 moves to trigger checkWinner on P2 next time.
	game.ApplyMove(gs, 3, 11, 1) // valid adjacent move (3 adj: 2,4,11)

	if gs.Phase != game.PhaseOver {
		t.Error("expected PhaseOver when opponent has < 3 stones")
	}
	if gs.Winner != 1 {
		t.Errorf("expected winner=1, got %d", gs.Winner)
	}
}

func TestWinner_NoValidMoves(t *testing.T) {
	gs := newMovePhaseState()
	// Trap P2: 4 stones (>3 so flying is not active), completely surrounded.
	// P2 at 16,17,18,20; all their adjacencies are blocked by P1.
	//   Adjacent[16]={17,23}  → 17=P2, 23 blocked by P1
	//   Adjacent[17]={16,18,9}→ 16=P2, 18=P2, 9 blocked by P1
	//   Adjacent[18]={17,19}  → 17=P2, 19 blocked by P1
	//   Adjacent[20]={19,21}  → 19=P1, 21 blocked by P1
	setStones(gs, 2, 16, 17, 18, 20)
	setStones(gs, 1, 23, 9, 19, 21)

	// P1 also has a stone at 0 that can move to 7 (adjacent, empty).
	// P1 moves 0→7: turn advances to P2, checkWinner detects P2 is stuck.
	setStones(gs, 1, 0)
	game.ApplyMove(gs, 0, 7, 1)

	if gs.Phase != game.PhaseOver {
		t.Errorf("expected PhaseOver when player has no valid moves, got phase %v", gs.Phase)
	}
	if gs.Winner != 1 {
		t.Errorf("expected winner=1, got %d", gs.Winner)
	}
}

// ── ValidMoves ───────────────────────────────────────────────────────────────

func TestValidMoves_AdjacentOnly(t *testing.T) {
	gs := newMovePhaseState()
	setStones(gs, 1, 0)        // P1 at 0; Adjacent[0] = {1,7}
	setStones(gs, 2, 8, 9, 10) // enough P2 stones

	moves := game.ValidMoves(gs, 1)
	if len(moves) != 2 {
		t.Errorf("expected 2 valid moves from pos 0, got %d", len(moves))
	}
}

func TestValidMoves_FlyingPhase(t *testing.T) {
	gs := newMovePhaseState()
	setStones(gs, 1, 0, 1, 2) // exactly 3 stones → flying
	setStones(gs, 2, 8, 9, 10, 11, 12)

	moves := game.ValidMoves(gs, 1)
	// Flying: P1 can move to any empty square (24 - 8 occupied = 16 empty)
	if len(moves) != 3*16 {
		t.Errorf("expected 48 flying moves, got %d", len(moves))
	}
}
