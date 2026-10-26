package effects

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPumpDefinedKWSubstitutesTheChoice covers the corpus's DefinedKW$ shapes
// this build can substitute: ChosenColor (Skrelv, Mondo Gecko, Sungold
// Sentinel: Hexproof:ChosenColor) and ChosenType (Illusionary Presence,
// Giant Slug: Landwalk:ChosenType), the no-chosen-player ChosenPlayer case
// (no keywords, no Note), the loud refusal when a colour choice was not
// recorded, and one genuinely unknown selector, which stays loud and leaves
// the keywords as written.
func TestPumpDefinedKWSubstitutesTheChoice(t *testing.T) {
	cases := []struct {
		name   string
		script string
		colour string
		typ    string
		want   []string
		note   bool
	}{
		{"colour", "DB$ Pump | Defined$ Self | KW$ Hexproof:ChosenColor & Toxic:1 | DefinedKW$ ChosenColor", "U", "", []string{"Hexproof:blue", "Toxic:1"}, false},
		{"type", "DB$ Pump | Defined$ Self | KW$ Landwalk:ChosenType | DefinedKW$ ChosenType", "", "Swamp", []string{"Landwalk:Swamp"}, false},
		{"no colour recorded", "DB$ Pump | Defined$ Self | KW$ Hexproof:ChosenColor & Toxic:1 | DefinedKW$ ChosenColor", "", "", []string{"Toxic:1"}, true},
		{"no chosen player", "DB$ Pump | Defined$ Self | KW$ Protection:Player.PlayerUID_ChosenPlayerUID:ChosenPlayerName | DefinedKW$ ChosenPlayer", "", "", nil, false},
		{"unknown selector", "DB$ Pump | Defined$ Self | KW$ Hexproof:ChosenColor | DefinedKW$ Frobnicate", "", "", []string{"Hexproof:ChosenColor"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, ids := board(t)
			h := &fakeHost{g: g}
			src := ids["myBear"]
			g.Obj(src).ChosenColor, g.Obj(src).ChosenType = tc.colour, tc.typ
			if g.Obj(src).Zone != state.ZBattlefield {
				t.Fatal("precondition: the pump source must be on the battlefield")
			}
			Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, tc.script))
			var got []string
			for _, ce := range h.continuous {
				got = append(got, ce.AddKeywords...)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("granted keywords = %v, want %v", got, tc.want)
			}
			noted := false
			for _, ev := range h.log {
				if ev.Kind == events.Note {
					noted = true
				}
			}
			if noted != tc.note {
				t.Fatalf("Note emitted = %v, want %v", noted, tc.note)
			}
		})
	}
}

// TestPumpDefinedKWPlayerValuesSubstitute is the player-valued half: Forge's
// Opponent, You, ChosenPlayer, TargetedPlayer and qualifier selectors all
// resolve to seats, and the KW$ ChosenPlayerUID/ChosenPlayerName tokens are
// replaced once per seat. A player selector that resolves to nobody (no
// chosen player) drops the token-bearing keywords with no Note, because the
// card's own text has nothing to grant.
func TestPumpDefinedKWPlayerValuesSubstitute(t *testing.T) {
	const protection = "Protection:Player.PlayerUID_ChosenPlayerUID:ChosenPlayerName"
	cases := []struct {
		name    string
		defined string
		ctx     func(src, foe state.ObjID) *Ctx
		chosen  []state.Target
		want    []string
		note    bool
	}{
		{
			name:    "opponent grants one per opponent",
			defined: "Opponent",
			ctx:     func(src, foe state.ObjID) *Ctx { return &Ctx{Source: src, Controller: 0} },
			want:    []string{"Protection:Player.PlayerUID_1:b", "Protection:Player.PlayerUID_2:c"},
		},
		{
			name:    "you",
			defined: "You",
			ctx:     func(src, foe state.ObjID) *Ctx { return &Ctx{Source: src, Controller: 0} },
			want:    []string{"Protection:Player.PlayerUID_0:a"},
		},
		{
			name:    "chosen player",
			defined: "ChosenPlayer",
			ctx:     func(src, foe state.ObjID) *Ctx { return &Ctx{Source: src, Controller: 0} },
			chosen:  []state.Target{{Player: 1, IsPlayer: true}},
			want:    []string{"Protection:Player.PlayerUID_1:b"},
		},
		{
			name:    "targeted player",
			defined: "TargetedPlayer",
			ctx: func(src, foe state.ObjID) *Ctx {
				return &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Player: 2, IsPlayer: true}}}
			},
			want: []string{"Protection:Player.PlayerUID_2:c"},
		},
		{
			name:    "remembered controller opponent",
			defined: "RememberedController.Opponent",
			ctx: func(src, foe state.ObjID) *Ctx {
				return &Ctx{Source: src, Controller: 0, Remembered: []state.Target{{Obj: foe}}}
			},
			want: []string{"Protection:Player.PlayerUID_1:b"},
		},
		{
			name:    "no chosen player grants nothing quietly",
			defined: "ChosenPlayer",
			ctx:     func(src, foe state.ObjID) *Ctx { return &Ctx{Source: src, Controller: 0} },
			want:    nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, src, foe := definedKWPlayerHost(t)
			h.g.Obj(src).Chosen = tc.chosen
			script := "DB$ Pump | Defined$ Self | KW$ Flying & " + protection + " | DefinedKW$ " + tc.defined
			Resolve(h, tc.ctx(src, foe), sa(t, script))
			var got []string
			for _, ce := range h.continuous {
				got = append(got, ce.AddKeywords...)
			}
			want := tc.want
			if len(want) != 0 {
				want = append([]string{"Flying"}, want...)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("granted keywords = %v, want %v", got, want)
			}
			noted := false
			for _, ev := range h.log {
				if ev.Kind == events.Note {
					noted = true
				}
			}
			if noted != tc.note {
				t.Fatalf("Note emitted = %v, want %v", noted, tc.note)
			}
		})
	}
}

// definedKWPlayerHost is a 3-seat fake host: the pump source controlled by
// seat 0 and one opponent permanent controlled by seat 1. Seat 2 is the
// remaining opponent for Opponent ordering.
func definedKWPlayerHost(t *testing.T) (*fakeHost, state.ObjID, state.ObjID) {
	t.Helper()
	h := newHost(t, 3)
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	src.Zone = state.ZBattlefield
	foe := h.g.AddObject(mkCard(t, "Name:Foe\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
	foe.Zone = state.ZBattlefield
	return h, src.ID, foe.ID
}

// TestPumpDefinedKWFivePlayerCarriersNoNote resolves the five corpus carriers
// of a player-valued DefinedKW$ and pins both halves: the four object pumps
// register the substituted protection keyword (one per resolved opponent, or
// the one chosen player), and none of them reaches the old "cannot
// substitute" Note. The player pumps (Eon Frolicker's DBPump, Noble
// Heritage's DBPump, Guardian Archon's DBPump) only need the loud Note gone:
// their grant lands on a player, not an object.
func TestPumpDefinedKWFivePlayerCarriersNoNote(t *testing.T) {
	const notSubstitutable = "is not a choice this engine can substitute"
	type carrier struct {
		name   string
		card   string
		svar   string
		object bool
		ctx    func(src, target, foe state.ObjID) *Ctx
		chosen []state.Target
		want   []string
	}
	cases := []carrier{
		{
			name: "Courageous Resolve", card: "Courageous Resolve", object: true,
			ctx: func(src, target, foe state.ObjID) *Ctx {
				return &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: target}}}
			},
			want: []string{"Protection:Player.PlayerUID_1:b", "Protection:Player.PlayerUID_2:c"},
		},
		{
			name: "Eon Frolicker DBPump", card: "Eon Frolicker", svar: "DBPump",
			ctx: func(src, target, foe state.ObjID) *Ctx {
				return &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
			},
		},
		{
			name: "Eon Frolicker DBPumpWalkers", card: "Eon Frolicker", svar: "DBPumpWalkers", object: true,
			ctx: func(src, target, foe state.ObjID) *Ctx {
				return &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
			},
			want: []string{"Protection:Player.PlayerUID_1:b"},
		},
		{
			name: "Noble Heritage DBPump", card: "Noble Heritage", svar: "DBPump",
			ctx: func(src, target, foe state.ObjID) *Ctx {
				return &Ctx{Source: src, Controller: 0, Remembered: []state.Target{{Obj: foe}}}
			},
		},
		{
			name: "Guardian Archon ability", card: "Guardian Archon", object: true,
			ctx: func(src, target, foe state.ObjID) *Ctx {
				return &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: target}}}
			},
			chosen: []state.Target{{Player: 1, IsPlayer: true}},
			want:   []string{"Protection:Player.PlayerUID_1:b"},
		},
		{
			name: "Guardian Archon DBPump", card: "Guardian Archon", svar: "DBPump",
			ctx: func(src, target, foe state.ObjID) *Ctx {
				return &Ctx{Source: src, Controller: 0}
			},
			chosen: []state.Target{{Player: 1, IsPlayer: true}},
		},
		{
			name: "Cliffside Rescuer ability", card: "Cliffside Rescuer", object: true,
			ctx: func(src, target, foe state.ObjID) *Ctx {
				return &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: target}}}
			},
			want: []string{"Protection:Player.OpponentOf PlayerUID_0:all opponents of a"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			card, s := corpusSA(t, tc.card, tc.svar)
			h := newHost(t, 3)
			src := h.g.AddObject(card, 0).ID
			target := h.g.AddObject(mkCard(t, "Name:Target\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0).ID
			walker := h.g.AddObject(mkCard(t, "Name:Walker\nManaCost:2 U\nTypes:Legendary Planeswalker Test\nOracle:x\n"), 0).ID
			foe := h.g.AddObject(mkCard(t, "Name:Foe\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1).ID
			for _, id := range []state.ObjID{src, target, walker, foe} {
				h.g.Obj(id).Zone = state.ZBattlefield
			}
			h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src, target, walker})
			h.g.SetZone(state.ZBattlefield, 1, []state.ObjID{foe})
			h.g.Obj(src).Chosen = tc.chosen
			// Eon's walker pump targets a planeswalker, not the creature.
			pumpTarget := target
			if tc.svar == "DBPumpWalkers" {
				pumpTarget = walker
			}
			Resolve(h, tc.ctx(src, pumpTarget, foe), s)
			var got []string
			for _, ce := range h.continuous {
				if tc.object && ce.Source == pumpTarget {
					got = append(got, ce.AddKeywords...)
				}
			}
			if tc.object && !slices.Equal(got, tc.want) {
				t.Fatalf("granted keywords = %v, want %v", got, tc.want)
			}
			for _, ev := range h.log {
				if ev.Kind == events.Note && strings.Contains(ev.Text, notSubstitutable) {
					t.Fatalf("unexpected cannot-substitute Note: %q", ev.Text)
				}
			}
		})
	}
}
