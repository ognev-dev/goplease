package game

import (
	"github.com/ognev-dev/goplease/app/ds"
	"github.com/ognev-dev/goplease/game/unit"
)

type Player struct {
	ID          ds.ID        `json:"id"`
	Name        string       `json:"name"`
	IsBot       bool         `json:"is_bot"`
	PlayerIndex int          `json:"-"`     // 0 or 1
	Units       []*unit.Unit `json:"units"` // units at hand
}

func NewPlayer(id ds.ID, name string, index int, isBot bool, units []*unit.Unit) *Player {
	p := &Player{
		ID:          id,
		Name:        name,
		IsBot:       isBot,
		PlayerIndex: index,
		Units:       units,
	}

	return p
}

func (p *Player) HasUnits(board *Board) bool {
	if len(p.Units) > 0 {
		return true
	}

	for col := 0; col < BoardColumns; col++ {
		for row := 0; row < BoardRows; row++ {
			u := board.UnitAt(col, row)
			if u != nil && u.OwnerID == p.ID {
				return true
			}
		}
	}
	return false
}
