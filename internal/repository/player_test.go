package repository

import (
	"testing"
)

// ── calcElo ───────────────────────────────────────────────────────────────────

func TestCalcElo_EqualRatings(t *testing.T) {
	// Both at 1200: expected = 0.5 → delta = round(32 * 0.5) = 16
	wDelta, lDelta := calcElo(1200, 1200)
	if wDelta != 16 {
		t.Errorf("expected winner delta 16, got %d", wDelta)
	}
	if lDelta != -16 {
		t.Errorf("expected loser delta -16, got %d", lDelta)
	}
}

func TestCalcElo_HigherRatedWins(t *testing.T) {
	// Higher rated (1500) beats lower (1200): expected ≈ 0.849 → delta ≈ 5
	wDelta, lDelta := calcElo(1500, 1200)
	if wDelta != 5 {
		t.Errorf("expected winner delta 5, got %d", wDelta)
	}
	if lDelta != -5 {
		t.Errorf("expected loser delta -5, got %d", lDelta)
	}
}

func TestCalcElo_LowerRatedWins(t *testing.T) {
	// Lower rated (1000) beats higher (1400): expected ≈ 0.091 → delta ≈ 29
	wDelta, lDelta := calcElo(1000, 1400)
	if wDelta != 29 {
		t.Errorf("expected winner delta 29, got %d", wDelta)
	}
	if lDelta != -29 {
		t.Errorf("expected loser delta -29, got %d", lDelta)
	}
}

func TestCalcElo_DeltasAreSymmetric(t *testing.T) {
	wDelta, lDelta := calcElo(1200, 1200)
	if wDelta != -lDelta {
		t.Errorf("deltas should be symmetric: %d vs %d", wDelta, lDelta)
	}
}

// ── RecordResult (integration) ────────────────────────────────────────────────

func TestRecordResult_UpdatesWinsLossesAndElo(t *testing.T) {
	repo, err := NewPlayerRepository(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	defer repo.Close()

	if err := repo.Upsert("alice"); err != nil {
		t.Fatalf("upsert alice: %v", err)
	}
	if err := repo.Upsert("bob"); err != nil {
		t.Fatalf("upsert bob: %v", err)
	}

	wDelta, lDelta, err := repo.RecordResult("alice", "bob")
	if err != nil {
		t.Fatalf("RecordResult: %v", err)
	}

	// Both start at eloDefault (1200), so delta should be 16 / -16.
	if wDelta != 16 {
		t.Errorf("expected winner delta 16, got %d", wDelta)
	}
	if lDelta != -16 {
		t.Errorf("expected loser delta -16, got %d", lDelta)
	}

	alice, err := repo.Get("alice")
	if err != nil || alice == nil {
		t.Fatalf("get alice: %v", err)
	}
	if alice.Wins != 1 || alice.Losses != 0 {
		t.Errorf("alice: expected wins=1 losses=0, got %d/%d", alice.Wins, alice.Losses)
	}
	if alice.Elo != eloDefault+16 {
		t.Errorf("alice elo: expected %d, got %d", eloDefault+16, alice.Elo)
	}

	bob, err := repo.Get("bob")
	if err != nil || bob == nil {
		t.Fatalf("get bob: %v", err)
	}
	if bob.Wins != 0 || bob.Losses != 1 {
		t.Errorf("bob: expected wins=0 losses=1, got %d/%d", bob.Wins, bob.Losses)
	}
	if bob.Elo != eloDefault-16 {
		t.Errorf("bob elo: expected %d, got %d", eloDefault-16, bob.Elo)
	}
}

func TestRecordResult_EloFloorAt100(t *testing.T) {
	repo, err := NewPlayerRepository(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	defer repo.Close()

	// Force bob to a very low ELO (just above the floor) by directly inserting.
	if _, err := repo.db.Exec(
		`INSERT INTO players (name, elo) VALUES ('alice', 1200)`,
	); err != nil {
		t.Fatalf("insert alice: %v", err)
	}
	if _, err := repo.db.Exec(
		`INSERT INTO players (name, elo) VALUES ('bob', 110)`,
	); err != nil {
		t.Fatalf("insert bob: %v", err)
	}

	if _, _, err := repo.RecordResult("alice", "bob"); err != nil {
		t.Fatalf("RecordResult: %v", err)
	}

	bob, err := repo.Get("bob")
	if err != nil || bob == nil {
		t.Fatalf("get bob: %v", err)
	}
	if bob.Elo < 100 {
		t.Errorf("bob elo should not drop below 100, got %d", bob.Elo)
	}
}

func TestRecordResult_AutoCreatesUnknownPlayers(t *testing.T) {
	repo, err := NewPlayerRepository(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	defer repo.Close()

	// Neither player exists yet — RecordResult should create them.
	if _, _, err := repo.RecordResult("new1", "new2"); err != nil {
		t.Fatalf("RecordResult for unknown players: %v", err)
	}

	p, err := repo.Get("new1")
	if err != nil || p == nil {
		t.Fatal("new1 should have been created by RecordResult")
	}
}
