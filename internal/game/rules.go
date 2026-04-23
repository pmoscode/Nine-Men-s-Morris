package game

// FormsMill returns true if placing/moving player's stone to pos completes a mill.
func FormsMill(board [24]int8, pos int, player int8) bool {
	for _, mill := range Mills {
		inMill := false
		for _, p := range mill {
			if p == pos {
				inMill = true
				break
			}
		}
		if !inMill {
			continue
		}
		// Check all three positions of the mill.
		complete := true
		for _, p := range mill {
			if p == pos {
				continue // the stone we just placed
			}
			if board[p] != player {
				complete = false
				break
			}
		}
		if complete {
			return true
		}
	}
	return false
}

// IsInMill returns true if the stone at pos (owned by player) is part of any mill.
func IsInMill(board [24]int8, pos int, player int8) bool {
	if board[pos] != player {
		return false
	}
	for _, mill := range Mills {
		found := false
		for _, p := range mill {
			if p == pos {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		complete := true
		for _, p := range mill {
			if board[p] != player {
				complete = false
				break
			}
		}
		if complete {
			return true
		}
	}
	return false
}

// MillPositions returns all positions that are part of any mill for player.
func MillPositions(board [24]int8, player int8) map[int]bool {
	result := map[int]bool{}
	for _, mill := range Mills {
		complete := true
		for _, p := range mill {
			if board[p] != player {
				complete = false
				break
			}
		}
		if complete {
			for _, p := range mill {
				result[p] = true
			}
		}
	}
	return result
}

// CanRemove returns true if the opponent's stone at pos may be removed.
// Stones in a closed mill may only be removed when all opponent stones are in mills.
func CanRemove(board [24]int8, pos int, remover int8) bool {
	opponent := int8(3) - remover
	if board[pos] != opponent {
		return false
	}
	if !IsInMill(board, pos, opponent) {
		return true
	}
	// Stone is in a mill – only allowed if all opponent stones are in mills.
	for i, v := range board {
		if v == opponent && !IsInMill(board, i, opponent) {
			return false
		}
	}
	return true
}

// ValidMoves returns all valid (from, to) moves for player in move/fly phase.
func ValidMoves(gs *GameState, player int8) [][2]int {
	var moves [][2]int
	flying := gs.OnBoard(player) == 3
	for from, v := range gs.Board {
		if v != player {
			continue
		}
		if flying {
			for to, t := range gs.Board {
				if t == 0 && to != from {
					moves = append(moves, [2]int{from, to})
				}
			}
		} else {
			for _, to := range Adjacent[from] {
				if gs.Board[to] == 0 {
					moves = append(moves, [2]int{from, to})
				}
			}
		}
	}
	return moves
}

// ApplyPlace places player's stone at pos, updates phase/turn/mustRemove.
// Returns an error string if the move is illegal, empty string on success.
func ApplyPlace(gs *GameState, pos int, player int8) string {
	if gs.Phase != PhasePlace {
		return "Nicht in der Setzphase"
	}
	if gs.Turn != player {
		return "Nicht dein Zug"
	}
	if gs.MustRemove {
		return "Du musst zuerst einen Stein entfernen"
	}
	if pos < 0 || pos > 23 {
		return "Ungültige Position"
	}
	if gs.Board[pos] != 0 {
		return "Position bereits belegt"
	}
	if gs.ToPlace[player-1] == 0 {
		return "Keine Steine mehr zum Setzen"
	}

	gs.Board[pos] = player
	gs.ToPlace[player-1]--

	if FormsMill(gs.Board, pos, player) {
		gs.MustRemove = true
		// Don't switch turn yet – player must remove first.
		return ""
	}

	gs.advanceTurnAndPhase()
	return ""
}

// ApplyMove moves player's stone from→to, updates phase/turn/mustRemove.
func ApplyMove(gs *GameState, from, to int, player int8) string {
	if gs.Phase != PhaseMove {
		return "Nicht in der Zugphase"
	}
	if gs.Turn != player {
		return "Nicht dein Zug"
	}
	if gs.MustRemove {
		return "Du musst zuerst einen Stein entfernen"
	}
	if from < 0 || from > 23 || to < 0 || to > 23 {
		return "Ungültige Position"
	}
	if gs.Board[from] != player {
		return "Kein eigener Stein auf dieser Position"
	}
	if gs.Board[to] != 0 {
		return "Zielposition ist belegt"
	}

	flying := gs.OnBoard(player) == 3
	if !flying {
		adjacent := false
		for _, nb := range Adjacent[from] {
			if nb == to {
				adjacent = true
				break
			}
		}
		if !adjacent {
			return "Feld nicht benachbart"
		}
	}

	gs.Board[from] = 0
	gs.Board[to] = player

	if FormsMill(gs.Board, to, player) {
		gs.MustRemove = true
		return ""
	}

	gs.advanceTurnAndPhase()
	return ""
}

// ApplyRemove removes the opponent's stone at pos.
func ApplyRemove(gs *GameState, pos int, player int8) string {
	if !gs.MustRemove {
		return "Kein Stein zu entfernen"
	}
	if gs.Turn != player {
		return "Nicht dein Zug"
	}
	if !CanRemove(gs.Board, pos, player) {
		return "Stein kann nicht entfernt werden"
	}

	opponent := int8(3) - player
	gs.Board[pos] = 0
	gs.Removed[opponent-1]++
	gs.MustRemove = false

	gs.advanceTurnAndPhase()
	return ""
}

// advanceTurnAndPhase switches the turn and checks whether the game phase
// or winner should be updated.
func (gs *GameState) advanceTurnAndPhase() {
	gs.Turn = 3 - gs.Turn

	// Check if placement phase is over.
	if gs.Phase == PhasePlace && gs.ToPlace[0] == 0 && gs.ToPlace[1] == 0 {
		gs.Phase = PhaseMove
	}

	if gs.Phase == PhaseMove {
		gs.checkWinner()
	}
}

// checkWinner sets gs.Winner and gs.Phase if a win condition is met.
func (gs *GameState) checkWinner() {
	current := gs.Turn
	opponent := int8(3) - current

	// Lose by having fewer than 3 stones (only counts after placement).
	if gs.ToPlace[current-1] == 0 && gs.OnBoard(current) < 3 {
		gs.Winner = opponent
		gs.Phase = PhaseOver
		return
	}
	if gs.ToPlace[opponent-1] == 0 && gs.OnBoard(opponent) < 3 {
		gs.Winner = current
		gs.Phase = PhaseOver
		return
	}

	// Lose by having no valid moves.
	if len(ValidMoves(gs, current)) == 0 && gs.OnBoard(current) > 3 {
		gs.Winner = opponent
		gs.Phase = PhaseOver
	}
}
