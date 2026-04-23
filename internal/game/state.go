package game

type Phase int

const (
	PhasePlace Phase = iota // players are still placing stones
	PhaseMove               // stones are moved to adjacent fields
	PhaseOver               // game finished
)

type GameState struct {
	Board      [24]int8 // 0 = empty, 1 = player 1, 2 = player 2
	Phase      Phase
	Turn       int8   // 1 or 2
	ToPlace    [2]int // stones still in hand (index 0 = player 1)
	Removed    [2]int // stones removed from board (index 0 = player 1)
	MustRemove bool   // true after forming a mill: current player must remove an opponent stone
	Winner     int8   // 0 = no winner yet, 1 or 2
}

func NewGameState() *GameState {
	return &GameState{
		Turn:    1,
		Phase:   PhasePlace,
		ToPlace: [2]int{9, 9},
	}
}

// OnBoard returns the number of stones player p has on the board.
func (gs *GameState) OnBoard(p int8) int {
	count := 0
	for _, v := range gs.Board {
		if v == p {
			count++
		}
	}
	return count
}

// TotalStones returns placed + in-hand stones for player p.
func (gs *GameState) TotalStones(p int8) int {
	return gs.OnBoard(p) + gs.ToPlace[p-1]
}
