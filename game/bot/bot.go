package bot

import (
	"github.com/ognev-dev/goplease/game"
)

// Bot implements a simple greedy AI.
type Bot struct{}

func New() *Bot { return &Bot{} }

// TakeTurn places units and calls EndTurn on behalf of the bot player.
// Called from a goroutine; the room's internal mutex handles concurrency.
func (b *Bot) TakeTurn(room *game.Room, p *game.Player) {
	if room.Phase != game.PhaseUnitPlacement {
		return
	}

	// TODO
}

func findFreeCell(room *game.Room, minRow, maxRow int) (col, row int, ok bool) {
	for r := minRow; r <= maxRow; r++ {
		for c := 0; c < game.BoardColumns; c++ {
			if room.Board.UnitAt(c, r) == nil {
				return c, r, true
			}
		}
	}
	return 0, 0, false
}
