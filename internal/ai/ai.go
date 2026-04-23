package ai

import (
	"math/rand"
	"muehle/internal/game"
)

const (
	winScore  = 100_000
	loseScore = -100_000
)

// Move represents a single action chosen by the AI.
type Move struct {
	Type string // "place" | "move" | "remove"
	Pos  int    // target position (place / remove)
	From int    // source position (move)
	To   int    // destination position (move)
}

// BestMove returns the best legal move for aiPlayer.
// depth controls search depth; easy adds randomisation for variety.
func BestMove(gs *game.GameState, aiPlayer int8, depth int, easy bool) Move {
	moves := generateMoves(gs)
	if len(moves) == 0 {
		return Move{}
	}
	if len(moves) == 1 {
		return moves[0]
	}

	// Easy: randomly skip the search 40 % of the time.
	if easy && rand.Float64() < 0.4 {
		return moves[rand.Intn(len(moves))]
	}

	best := moves[0]
	bestVal := loseScore - 1
	alpha := loseScore - 1
	beta := winScore + 1

	for _, m := range moves {
		child := applyMove(gs, m)
		val := minimax(child, depth-1, alpha, beta, aiPlayer)
		if val > bestVal {
			bestVal = val
			best = m
		}
		if val > alpha {
			alpha = val
		}
		if beta <= alpha {
			break
		}
	}
	return best
}

// minimax returns the heuristic value of gs from aiPlayer's perspective.
// Maximising when it is aiPlayer's turn, minimising when it is the opponent's.
func minimax(gs *game.GameState, depth, alpha, beta int, aiPlayer int8) int {
	if depth == 0 || gs.Phase == game.PhaseOver {
		return evaluate(gs, aiPlayer)
	}

	moves := generateMoves(gs)
	if len(moves) == 0 {
		return evaluate(gs, aiPlayer)
	}

	if gs.Turn == aiPlayer {
		best := loseScore - 1
		for _, m := range moves {
			val := minimax(applyMove(gs, m), depth-1, alpha, beta, aiPlayer)
			if val > best {
				best = val
			}
			if val > alpha {
				alpha = val
			}
			if beta <= alpha {
				break
			}
		}
		return best
	}

	best := winScore + 1
	for _, m := range moves {
		val := minimax(applyMove(gs, m), depth-1, alpha, beta, aiPlayer)
		if val < best {
			best = val
		}
		if val < beta {
			beta = val
		}
		if beta <= alpha {
			break
		}
	}
	return best
}

// evaluate scores the position from aiPlayer's perspective.
func evaluate(gs *game.GameState, aiPlayer int8) int {
	opp := int8(3 - aiPlayer)

	if gs.Phase == game.PhaseOver {
		if gs.Winner == aiPlayer {
			return winScore
		}
		return loseScore
	}

	aiTotal := gs.TotalStones(aiPlayer)
	oppTotal := gs.TotalStones(opp)

	score := 0

	// Stone count
	score += 20 * (aiTotal - oppTotal)

	// Mill count
	score += 30 * (countMills(gs.Board, aiPlayer) - countMills(gs.Board, opp))

	// Potential mills (two own stones + one empty gap)
	score += 10 * (countPotential(gs.Board, aiPlayer) - countPotential(gs.Board, opp))

	// Mobility in move phase
	if gs.Phase == game.PhaseMove {
		score += 2 * (len(game.ValidMoves(gs, aiPlayer)) - len(game.ValidMoves(gs, opp)))
	}

	return score
}

func countMills(board [24]int8, p int8) int {
	n := 0
	for _, m := range game.Mills {
		if board[m[0]] == p && board[m[1]] == p && board[m[2]] == p {
			n++
		}
	}
	return n
}

func countPotential(board [24]int8, p int8) int {
	n := 0
	for _, m := range game.Mills {
		own, empty := 0, 0
		for _, pos := range m {
			switch board[pos] {
			case p:
				own++
			case 0:
				empty++
			}
		}
		if own == 2 && empty == 1 {
			n++
		}
	}
	return n
}

// generateMoves returns all legal moves for the side to move in gs.
func generateMoves(gs *game.GameState) []Move {
	player := gs.Turn
	opp := int8(3 - player)
	var moves []Move

	if gs.MustRemove {
		for i, v := range gs.Board {
			if v == opp && game.CanRemove(gs.Board, i, player) {
				moves = append(moves, Move{Type: "remove", Pos: i})
			}
		}
		return moves
	}

	switch gs.Phase {
	case game.PhasePlace:
		for i, v := range gs.Board {
			if v == 0 {
				moves = append(moves, Move{Type: "place", Pos: i})
			}
		}
	case game.PhaseMove:
		for _, mv := range game.ValidMoves(gs, player) {
			moves = append(moves, Move{Type: "move", From: mv[0], To: mv[1]})
		}
	}
	return moves
}

// applyMove clones gs and applies m, returning the new state.
// GameState contains only value types so a struct copy is a full deep copy.
func applyMove(gs *game.GameState, m Move) *game.GameState {
	clone := *gs
	switch m.Type {
	case "place":
		game.ApplyPlace(&clone, m.Pos, gs.Turn)
	case "move":
		game.ApplyMove(&clone, m.From, m.To, gs.Turn)
	case "remove":
		game.ApplyRemove(&clone, m.Pos, gs.Turn)
	}
	return &clone
}
