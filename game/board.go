package game

import "github.com/ognev-dev/goplease/game/unit"

const (
	BoardRows    = 8
	BoardColumns = 16
	SafeZoneSize = 2 // columns at each end that are "safe zones"
)

func NewBoard() Board {
	b := Board{}
	for r := range BoardRows {
		for c := range BoardColumns {
			b[r][c] = &BoardCell{}
		}
	}

	return b
}

// Board is a 2D grid of a game board
type Board [BoardRows][BoardColumns]*BoardCell

func (b *Board) CellAt(row, col int) *BoardCell {
	if !b.InBounds(row, col) {
		return nil
	}
	return b[row][col]
}

func (b *Board) UnitAt(row, col int) *unit.Unit {
	cell := b.CellAt(row, col)
	if cell == nil {
		return nil
	}

	return cell.Unit
}

func (b *Board) PlaceUnit(row, col int, u *unit.Unit) bool {
	cell := b.CellAt(row, col)
	if cell == nil {
		return false
	}

	cell.Unit = u

	return true
}

func (b *Board) ClearUnit(row, col int) {
	cell := b.CellAt(row, col)
	if cell != nil {
		cell.Unit = nil
	}
}

func (b *Board) InBounds(row, col int) bool {
	return row >= 0 && row < BoardRows && col >= 0 && col < BoardColumns
}

// EnemySafeZone returns true if the cell belongs to the given playerIndex (0 or 1)
// enemy's safe zone
func EnemySafeZone(row int, ownerIndex int) bool {
	if ownerIndex == 0 {
		return row >= BoardRows-SafeZoneSize
	}
	return row < SafeZoneSize
}

// PlacementZone returns the valid placement rows for a player.
func PlacementZone(playerIndex int) (minRow, maxRow int) {
	if playerIndex == 0 {
		return 0, SafeZoneSize - 1
	}
	return BoardRows - SafeZoneSize, BoardRows - 1
}

type BoardCell struct {
	Unit       *unit.Unit `json:"unit"`
	IsSafeZone bool       `json:"is_safe_zone"`
}
