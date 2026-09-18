package main

import (
	"fmt"
	"sort"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

const traceSchemaVersion = 1

type traceDecisionMeta struct {
	PairIndex int
	Pair      string
	GameIndex int
	Seed      uint64
	Policy    string
}

type gameTrace struct {
	Decisions []traceDecisionV1
	Terminal  traceGameV1
}

func newGameTrace() *gameTrace { return &gameTrace{} }

type traceDecisionV1 struct {
	RecordType    string                 `json:"record_type"`
	SchemaVersion int                    `json:"schema_version"`
	PairIndex     int                    `json:"pair_index"`
	Pair          string                 `json:"pair"`
	GameIndex     int                    `json:"game_index"`
	Seed          uint64                 `json:"seed"`
	Sequence      uint64                 `json:"decision_sequence"`
	Seat          state.PlayerID         `json:"seat"`
	Policy        string                 `json:"policy"`
	Kind          decision.Kind          `json:"kind"`
	Min           int                    `json:"min"`
	Max           int                    `json:"max"`
	Options       []traceOptionV1        `json:"options"`
	Choices       []int                  `json:"choices"`
	TargetEffect  *decision.TargetEffect `json:"target_effect,omitempty"`
	Board         traceBoardV1           `json:"board"`
}

type traceOptionV1 struct {
	Index        int            `json:"index"`
	Kind         string         `json:"kind"`
	Object       state.ObjID    `json:"object,omitempty"`
	Player       state.PlayerID `json:"player"`
	Attacker     state.ObjID    `json:"attacker,omitempty"`
	Required     bool           `json:"required,omitempty"`
	Group        string         `json:"group,omitempty"`
	AltCostIndex int            `json:"alt_cost_index,omitempty"`
	CastMode     string         `json:"cast_mode,omitempty"`
	Amount       int            `json:"amount,omitempty"`
	AbilityIndex int            `json:"ability_index,omitempty"`
}

type traceBoardV1 struct {
	SchemaVersion int                `json:"schema_version"`
	IsMain        bool               `json:"is_main"`
	Mana          [6]int32           `json:"mana_wubrgc"`
	Cards         []traceCardV1      `json:"cards"`
	Life          []traceLifeV1      `json:"life"`
	Creatures     []traceCreatureV1  `json:"creatures"`
	Commanders    []traceCommanderV1 `json:"commanders"`
	Stack         []traceStackV1     `json:"stack"`
}

type traceCardV1 struct {
	CardID        state.ObjID `json:"card_id"`
	Creature      bool        `json:"creature"`
	Power         int32       `json:"power"`
	ManaValue     int32       `json:"mana_value"`
	Basic         bool        `json:"basic"`
	AttachedTo    state.ObjID `json:"attached_to,omitempty"`
	Activated     int32       `json:"activated"`
	ManaCost      string      `json:"mana_cost"`
	Castable      bool        `json:"castable"`
	OnBattlefield bool        `json:"on_battlefield"`
	InstantSpeed  bool        `json:"instant_speed"`
	Produces      [6]int32    `json:"produces_wubrgc"`
	ProducesAny   bool        `json:"produces_any"`
	Indeterminate bool        `json:"produces_indeterminate"`
	Counter       bool        `json:"counter"`
}

type traceLifeV1 struct {
	Player state.PlayerID `json:"player"`
	Life   int32          `json:"life"`
}

type traceCreatureV1 struct {
	Object     state.ObjID    `json:"object"`
	Controller state.PlayerID `json:"controller"`
	Power      int32          `json:"power"`
	Toughness  int32          `json:"toughness"`
	Damage     int32          `json:"damage"`
	Tapped     bool           `json:"tapped"`
	Keywords   []string       `json:"keywords"`
}

type traceCommanderDamageV1 struct {
	Player state.PlayerID `json:"player"`
	Damage int32          `json:"damage"`
}

type traceCommanderV1 struct {
	Object        state.ObjID              `json:"object"`
	Casts         int32                    `json:"casts"`
	InCommandZone bool                     `json:"in_command_zone"`
	Damage        []traceCommanderDamageV1 `json:"damage"`
}

type traceStackV1 struct {
	Object     state.ObjID    `json:"object"`
	Controller state.PlayerID `json:"controller"`
	IsSpell    bool           `json:"is_spell"`
}

type traceGameV1 struct {
	RecordType    string `json:"record_type"`
	SchemaVersion int    `json:"schema_version"`
}

func (g *gameTrace) record(d *decision.Decision, in decision.Intent, b *botpolicy.Board, meta traceDecisionMeta) error {
	r := traceDecisionV1{
		RecordType: "decision-v1", SchemaVersion: traceSchemaVersion,
		PairIndex: meta.PairIndex, Pair: meta.Pair, GameIndex: meta.GameIndex, Seed: meta.Seed,
		Sequence: d.Seq, Seat: d.Player, Policy: meta.Policy, Kind: d.Kind, Min: d.Min, Max: d.Max,
		Choices: append([]int(nil), in.Choices...), Board: projectTraceBoard(b),
	}
	if d.TargetEffect != nil {
		r.TargetEffect = &decision.TargetEffect{API: d.TargetEffect.API}
		if d.TargetEffect.Damage != nil {
			r.TargetEffect.Damage = &decision.DamageEffect{}
			if d.TargetEffect.Damage.Amount != nil {
				n := *d.TargetEffect.Damage.Amount
				r.TargetEffect.Damage.Amount = &n
			}
		}
	}
	for _, o := range d.Options {
		r.Options = append(r.Options, traceOptionV1{
			Index: o.Index, Kind: o.Kind, Object: o.Obj, Player: o.Player, Attacker: o.Attacker,
			Required: o.Required, Group: o.Group, AltCostIndex: o.AltCostIndex, CastMode: o.Mode,
			Amount: o.Amount, AbilityIndex: o.Ability,
		})
	}
	if err := validateTraceRecord(r); err != nil {
		return err
	}
	g.Decisions = append(g.Decisions, r)
	return nil
}

func projectTraceBoard(b *botpolicy.Board) traceBoardV1 {
	r := traceBoardV1{SchemaVersion: traceSchemaVersion, IsMain: b.IsMain, Mana: b.Pool}
	cardIDs := sortedObjIDs(b.Cards)
	for _, id := range cardIDs {
		c := b.Cards[id]
		r.Cards = append(r.Cards, traceCardV1{CardID: id, Creature: c.Creature, Power: c.Power, ManaValue: c.CMC, Basic: c.Basic, AttachedTo: c.AttachedTo, Activated: c.Activated, ManaCost: c.ManaCost, Castable: c.Castable, OnBattlefield: c.OnBattlefield, InstantSpeed: c.InstantSpeed, Produces: c.Produces.Colour, ProducesAny: c.Produces.Any, Indeterminate: c.Produces.Indeterminate, Counter: c.Counter})
	}
	players := make([]int, 0, len(b.Life))
	for p := range b.Life {
		players = append(players, int(p))
	}
	sort.Ints(players)
	for _, p := range players {
		r.Life = append(r.Life, traceLifeV1{Player: state.PlayerID(p), Life: b.Life[state.PlayerID(p)]})
	}
	for _, id := range sortedObjIDs(b.Creatures) {
		c := b.Creatures[id]
		r.Creatures = append(r.Creatures, traceCreatureV1{Object: id, Controller: c.Controller, Power: c.Power, Toughness: c.Toughness, Damage: c.Damage, Tapped: c.Tapped, Keywords: append([]string(nil), c.Keywords...)})
	}
	for _, id := range sortedObjIDs(b.Commanders) {
		c := b.Commanders[id]
		v := traceCommanderV1{Object: id, Casts: c.Casts, InCommandZone: c.InCommandZone}
		ps := make([]int, 0, len(c.Damage))
		for p := range c.Damage {
			ps = append(ps, int(p))
		}
		sort.Ints(ps)
		for _, p := range ps {
			v.Damage = append(v.Damage, traceCommanderDamageV1{Player: state.PlayerID(p), Damage: c.Damage[state.PlayerID(p)]})
		}
		r.Commanders = append(r.Commanders, v)
	}
	for _, s := range b.Stack {
		r.Stack = append(r.Stack, traceStackV1{Object: s.ID, Controller: s.Controller, IsSpell: s.IsSpell})
	}
	return r
}

func sortedObjIDs[V any](m map[state.ObjID]V) []state.ObjID {
	ids := make([]state.ObjID, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func validateTraceRecord(v any) error {
	switch r := v.(type) {
	case traceDecisionV1:
		if r.SchemaVersion != traceSchemaVersion {
			return fmt.Errorf("decision trace schema version %d is unsupported", r.SchemaVersion)
		}
		if r.RecordType != "decision-v1" {
			return fmt.Errorf("invalid decision trace record type %q", r.RecordType)
		}
		if r.Min < 0 || r.Max < r.Min {
			return fmt.Errorf("invalid decision choice bounds %d..%d", r.Min, r.Max)
		}
		for i, o := range r.Options {
			if o.Index != i {
				return fmt.Errorf("decision option %d has index %d", i, o.Index)
			}
		}
		for _, choice := range r.Choices {
			if choice < 0 || choice >= len(r.Options) {
				return fmt.Errorf("decision choice %d out of range", choice)
			}
		}
		if r.Board.SchemaVersion != traceSchemaVersion {
			return fmt.Errorf("board trace schema version %d is unsupported", r.Board.SchemaVersion)
		}
		return nil
	case traceGameV1:
		if r.SchemaVersion != traceSchemaVersion {
			return fmt.Errorf("game trace schema version %d is unsupported", r.SchemaVersion)
		}
		if r.RecordType != "game-v1" {
			return fmt.Errorf("invalid game trace record type %q", r.RecordType)
		}
		return nil
	default:
		return fmt.Errorf("unsupported trace record %T", v)
	}
}
