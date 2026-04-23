package game

// Board positions 0–23:
//
//	Outer ring:  0  1  2  3  4  5  6  7  (clockwise: TL top TR right BR bot BL left)
//	Middle ring: 8  9 10 11 12 13 14 15
//	Inner ring: 16 17 18 19 20 21 22 23
//
// Visual layout:
//
//	 0 -------- 1 -------- 2
//	 |          |          |
//	 |  8 ----- 9 ----- 10 |
//	 |  |       |       |  |
//	 |  |  16--17--18   |  |
//	 |  |   |       |   |  |
//	 7  15  23      19  11  3
//	 |  |   |       |   |  |
//	 |  |  22--21--20   |  |
//	 |  |       |       |  |
//	 |  14----13-----12 |
//	 |          |          |
//	 6 -------- 5 -------- 4

// Adjacent lists which positions each position connects to.
var Adjacent = [24][]int{
	0:  {1, 7},
	1:  {0, 2, 9},
	2:  {1, 3},
	3:  {2, 4, 11},
	4:  {3, 5},
	5:  {4, 6, 13},
	6:  {5, 7},
	7:  {6, 0, 15},
	8:  {9, 15},
	9:  {1, 8, 10, 17},
	10: {9, 11},
	11: {3, 10, 12, 19},
	12: {11, 13},
	13: {5, 12, 14, 21},
	14: {13, 15},
	15: {7, 14, 8, 23},
	16: {17, 23},
	17: {16, 18, 9},
	18: {17, 19},
	19: {18, 20, 11},
	20: {19, 21},
	21: {20, 22, 13},
	22: {21, 23},
	23: {22, 16, 15},
}

// Mills contains all 16 possible lines of three.
var Mills = [16][3]int{
	// Outer ring
	{0, 1, 2},
	{2, 3, 4},
	{4, 5, 6},
	{6, 7, 0},
	// Middle ring
	{8, 9, 10},
	{10, 11, 12},
	{12, 13, 14},
	{14, 15, 8},
	// Inner ring
	{16, 17, 18},
	{18, 19, 20},
	{20, 21, 22},
	{22, 23, 16},
	// Cross connections (all three rings)
	{1, 9, 17},
	{3, 11, 19},
	{5, 13, 21},
	{7, 15, 23},
}

// SVGPositions maps position index → (cx, cy) on a 600×600 viewBox.
var SVGPositions = [24][2]int{
	// Outer ring
	0: {50, 50},
	1: {300, 50},
	2: {550, 50},
	3: {550, 300},
	4: {550, 550},
	5: {300, 550},
	6: {50, 550},
	7: {50, 300},
	// Middle ring
	8:  {150, 150},
	9:  {300, 150},
	10: {450, 150},
	11: {450, 300},
	12: {450, 450},
	13: {300, 450},
	14: {150, 450},
	15: {150, 300},
	// Inner ring
	16: {250, 250},
	17: {300, 250},
	18: {350, 250},
	19: {350, 300},
	20: {350, 350},
	21: {300, 350},
	22: {250, 350},
	23: {250, 300},
}

// Lines lists all board edges as pairs of position indices (for SVG rendering).
var Lines [][2]int

func init() {
	seen := map[[2]int]bool{}
	for pos, neighbors := range Adjacent {
		for _, nb := range neighbors {
			a, b := pos, nb
			if a > b {
				a, b = b, a
			}
			key := [2]int{a, b}
			if !seen[key] {
				seen[key] = true
				Lines = append(Lines, key)
			}
		}
	}
}
