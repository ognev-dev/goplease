package unit

import (
	"github.com/ognev-dev/goplease/app/ds"
	"github.com/ognev-dev/goplease/game/ability"
)

type Template struct {
	ID           int
	Name         string
	Description  string
	HP           int
	Attack       int
	AttackRange  int
	Shield       int
	MovePoints   int
	ActionPoints int
	Abilities    []ability.ID
}

type Unit struct {
	ID         ds.ID  `json:"id"`
	TemplateID int    `json:"template_id"`
	OwnerID    ds.ID  `json:"owner_id"`
	Name       string `json:"name"`

	MaxHP         int `json:"max_hp"`
	CurrentHP     int `json:"current_hp"`
	CurrentShield int `json:"current_shield"`

	AP int `json:"ap"` // Action Points
	MP int `json:"mp"` // Move Points

	// board position, -1 - in hand
	Row int `json:"row"`
	Col int `json:"col"`

	Abilities []ability.ID       `json:"abilities"`
	Cooldowns map[ability.ID]int `json:"cooldowns"`
}

func NewUnitFromTemplate(t Template, playerID ds.ID) *Unit {
	return &Unit{
		ID:            ds.NewID(),
		TemplateID:    t.ID,
		OwnerID:       playerID,
		Name:          t.Name,
		MaxHP:         t.HP,
		CurrentHP:     t.HP,
		CurrentShield: t.Shield,
		AP:            t.ActionPoints,
		MP:            t.MovePoints,
		Row:           -1,
		Col:           -1,
		Abilities:     t.Abilities,
		Cooldowns:     make(map[ability.ID]int),
	}
}

func (u *Unit) IsAlive() bool { return u.CurrentHP > 0 }
func (u *Unit) InHand() bool  { return u.Row == -1 }
