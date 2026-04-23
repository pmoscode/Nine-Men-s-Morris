package model

import "fmt"

type Player struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Wins   int    `json:"wins"`
	Losses int    `json:"losses"`
	Elo    int    `json:"elo"`
}

func (p *Player) WinRate() string {
	total := p.Wins + p.Losses
	if total == 0 {
		return "–"
	}
	rate := float64(p.Wins) / float64(total) * 100
	return fmt.Sprintf("%.0f%%", rate)
}
