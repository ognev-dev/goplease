package game

import (
	"errors"
	"math/rand"
	"sync"

	"github.com/google/uuid"
	"github.com/ognev-dev/goplease/app/ds"
)

// PlacementAction is sent by a player to place a unit on the board.
type PlacementAction struct {
	UnitID ds.ID
	Col    int
	Row    int
}

// TurnResult is returned after EndTurn is called.
type TurnResult struct {
	IsOver   bool
	Winner   ds.ID // player ID, or "" for draw
	Reason   EndReason
	NewPhase Phase
}

// Room holds the full state of one match.
type Room struct {
	mu sync.Mutex

	ID      string
	Board   Board
	Players [2]*Player

	CurrentTurn  int
	ActivePlayer int // 0 or 1 whose turn is
	Phase        Phase

	// Pending placements for current turn (committed on EndTurn)
	pendingPlacements []PlacementAction
}

func NewRoom(p1, p2 *Player) *Room {
	return &Room{
		ID:           uuid.NewString(),
		Players:      [2]*Player{p1, p2},
		CurrentTurn:  0,
		ActivePlayer: rand.Intn(2),
		Phase:        PhaseUnitPlacement,
		Board:        NewBoard(),
	}
}

// ─── Placement ───────────────────────────────────────────────────────────────

// PlaceUnit attempts to place a unit on the board.
// Returns error if the placement is invalid.
func (r *Room) PlaceUnit(playerID ds.ID, place PlacementAction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, idx := r.playerByID(playerID)
	if p == nil {
		return errors.New("unknown player")
	}
	if r.Phase != PhaseUnitPlacement || r.ActivePlayer != idx {
		return errors.New("not your turn to place")
	}

	// TODO

	return nil
}

// RecallUnit removes a placed unit back to hand (only during placement phase).
func (r *Room) RecallUnit(playerID ds.ID, row, col int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, idx := r.playerByID(playerID)
	if p == nil {
		return errors.New("unknown player")
	}
	if r.Phase != PhaseUnitPlacement || r.ActivePlayer != idx {
		return errors.New("not your turn to place")
	}
	u := r.Board.UnitAt(row, col)
	if u == nil || u.OwnerID != playerID {
		return errors.New("no owned unit at that cell")
	}
	r.Board.ClearUnit(row, col)

	return nil
}

// ─── End Turn (triggers simulation) ──────────────────────────────────────────

// EndTurn ends the current player's turn
func (r *Room) EndTurn(playerID ds.ID) (*TurnResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, idx := r.playerByID(playerID)
	if p == nil {
		return nil, errors.New("unknown player")
	}
	if r.Phase != PhaseUnitPlacement || r.ActivePlayer != idx {
		return nil, errors.New("not your turn")
	}

	// TODO
	return nil, nil
}

func (r *Room) playerByID(id ds.ID) (*Player, int) {
	for i, p := range r.Players {
		if p.ID == id {
			return p, i
		}
	}
	return nil, -1
}
