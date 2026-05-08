package game

const MaxTurns = 20

type Phase string

const (
	PhaseUnitPlacement Phase = "unit_placement" // current player places units
	PhaseUnitActing    Phase = "unit_acting"    // current player playing with unit
	PhaseGameOver      Phase = "game_over"
)

type EndReason string

const (
	EndNoUnits   EndReason = "no_units"
	EndTurnLimit EndReason = "turn_limit"
)

type NewGamePayload struct {
	RoomID   string  `json:"room_id"`
	Phase    Phase   `json:"phase"`
	IsMyTurn bool    `json:"is_my_turn"`
	Board    Board   `json:"board"`
	Player   *Player `json:"player"`
	Opponent string  `json:"opponent"`
}
