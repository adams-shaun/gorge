//go:build manabrew

package mbtest

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/manabrew"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDamageAssignPromptedAndAnsweredInGame drives seeded repo-deck games
// through the ManaBrew mapping until it has watched a real combat
// damage-division ask (engine/rules/combat.go's askNextDivision, CR 510.1c)
// cross the wire, and pins four things:
//
//   - a double-blocked attacker's division ask poses the ManaBrew protocol's
//     own chooseCombatDamageAssignment prompt (never the generic label
//     selection the ask used to fall back to), and the
//     combatDamageAssignmentDecision answer is accepted and actually dealt:
//     every blocker assigned damage is gone or shows at least that much
//     damage, exactly as gorge's native decision path would have dealt the
//     same option;
//   - a trample attacker never appears as the ask's attacker: gorge poses
//     the division only for non-trample attackers (a blocked trample
//     attacker's lethal-then-remainder assignment is decided
//     deterministically engine-side, CR 510.1c with 702.19b/c), so the
//     brief's "with trample" half is the engine's no-ask behaviour, not a
//     second wire prompt;
//   - no game records an unmapped decision or a rejected answer, so the
//     mapping's Prompt and TranslateResponse halves agree with the engine's
//     own legality gate;
//   - every division ask observed was answered as a
//     combatDamageAssignmentDecision, so the census's
//     posed:chooseCombatDamageAssignment count covers them all.
func TestDamageAssignPromptedAndAnsweredInGame(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const maxGames = 400
	census := NewCensus()
	client := NewFirstLegalClient()
	divisions := 0
	trampleDivisions := 0
	var assigned []struct {
		id  state.ObjID
		dmg int
	}
	names := testutil.LegacyDeckNames()
	for g := 0; g < maxGames && divisions == 0; g++ {
		seed := uint64(3000 + g)
		p1 := names[int(seed)%len(names)]
		p2 := names[(int(seed)+1)%len(names)]
		cfg := rules.Config{Seed: seed, Names: []string{p1, p2},
			Decks:  [][]*cards.Card{testutil.RepoDeck(t, reg, p1), testutil.RepoDeck(t, reg, p2)},
			Tokens: reg.Tokens, NameUniverse: reg.Universe()}
		table := "damage-assign"
		before := divisions
		// The pick answers every division ask the way the mock client does
		// (everything to the first offered blocker, mbtest/mock.go's
		// answerCombatDamageAssignment): the attacker's whole power lands on
		// one blocker and the other takes none of it, and the round trip
		// exercises the dedicated prompt AND its answer.
		pick := func(v view.View, d *decision.Decision) (mb.PromptOutputValue, bool) {
			switch d.Kind {
			case decision.KAttackers:
				// Declare one attacker, so attacks are known to be declared
				// at all (the first-legal client's own attack policy leaves
				// every declaration empty in these decks). An ask with any
				// required attacker, or a chosen pair carrying a charge or an
				// alone restriction, goes back to the client whole.
				for _, o := range d.Options {
					if o.Required || o.AttackMust || o.AttackMustTwo {
						return nil, false
					}
				}
				for _, o := range d.Options {
					if o.AloneRestricted || o.Value != 0 || o.CostLife != 0 ||
						o.CostPhyrexian != 0 || o.CostTaps != 0 {
						continue
					}
					if o.Player == 0 && o.Battle == 0 {
						continue
					}
					tid := "player-" + strconv.Itoa(int(o.Player))
					return mb.DeclareAttackersDecision{Assignments: []mb.AttackerAssignment{
						{AttackerID: "o" + strconv.FormatUint(uint64(o.Obj), 10), TargetID: tid}}}, true
				}
				return nil, false
			case decision.KPriority:
				// Cast anything the pool can pay for, then play a land, then
				// activate a source, all only in this seat's own main phases
				// (castpolicy.go's plan, without a spell filter), so boards
				// actually develop; everything else passes. The wire-only
				// first-legal client cannot express this: it passes every
				// priority and the game ends with empty boards.
				cast, land, act := -1, -1, -1
				for _, o := range d.Options {
					switch o.Kind {
					case "cast":
						if cast < 0 {
							cast = o.Index
						}
					case "play_land":
						if land < 0 {
							land = o.Index
						}
					case "activate":
						if act < 0 {
							act = o.Index
						}
					}
				}
				if cast >= 0 {
					return mb.ActOutput{ActionID: "opt-" + strconv.Itoa(cast)}, true
				}
				if ownMain(v) && land >= 0 {
					return mb.ActOutput{ActionID: "opt-" + strconv.Itoa(land)}, true
				}
				if ownMain(v) && act >= 0 {
					return mb.ActOutput{ActionID: "opt-" + strconv.Itoa(act)}, true
				}
				return mb.PassOutput{}, true
			case decision.KBlockers:
				// The mock client's blocker policy never voluntarily
				// double-blocks, so the fixture does: on a turn with no
				// required block, two charge-free creatures gang up on one
				// attacker. That is what puts a CR 510.1c division ask on
				// the wire at all.
				if out, ok := doubleBlockPick(d); ok {
					return out, true
				}
				return nil, false
			case decision.KChoose:
				if len(d.Options) == 0 || d.Options[0].Kind != "division" {
					return nil, false
				}
				atk, ids, ok := divisionPickTargets(v, d)
				if !ok {
					return nil, false
				}
				pw := d.Options[len(d.Options)-1].Amount
				if pw <= 0 {
					return nil, false
				}
				divisions++
				if c := battlefieldLookup(v, atk); c != nil {
					for _, kw := range c.Keywords {
						if strings.EqualFold(kw, "Trample") {
							trampleDivisions++
						}
					}
				}
				assigned = append(assigned, struct {
					id  state.ObjID
					dmg int
				}{id: parseWireID(ids[0]), dmg: int(pw)})
				return mb.DamageAssignmentDecision{Assignments: []mb.DamageAssignment{
					{AssigneeID: ids[0], Damage: int(pw)}}}, true
			}
			return nil, false
		}
		seats := []seat.Seat{
			&TranslatingSeat{Translator: manabrew.New(table, int64(seed), nil), Client: client,
				Table: table, Census: census, Pick: pick},
			&TranslatingSeat{Translator: manabrew.New(table, int64(seed), nil), Client: client,
				Table: table, Census: census, Pick: pick},
		}
		outcome, e, err := bench.PlayGame(cfg, seats, censusMaxTurns, censusMaxIntents, bench.Hooks{})
		if err != nil {
			t.Fatalf("seats=2 seed=%d: PlayGame: %v", seed, err)
		}
		census.AddGame()
		if bench.IsAbort(outcome.StallOn) {
			t.Fatalf("seats=2 seed=%d: engine abort (%s): %s", seed, outcome.StallOn, outcome.Livelock)
		}
		// The assigned damage must actually have landed: each assignee is
		// either gone from the battlefield (it died of it) or still showing
		// at least the assigned amount (first-strike and regular passes may
		// both have asked, so >= is the honest shape).
		for _, a := range assigned[len(assigned)-(divisions-before):] {
			if o := e.G.Obj(a.id); o != nil && o.Zone == state.ZBattlefield && int(o.Damage) < a.dmg {
				t.Fatalf("seed=%d: assigned %d damage to o%d but it shows %d", seed, a.dmg, a.id, o.Damage)
			}
		}
	}
	t.Logf("census: %s (chooseAttackers=%d)", census.Summary(), census.Posed["chooseAttackers"])
	if census.Games == 0 {
		t.Fatal("census recorded zero games -- the .cards corpus is not reachable in this worktree")
	}
	if divisions == 0 {
		t.Fatalf("no combat damage-division ask was observed across %d games; widen the seed scan", census.Games)
	}
	if trampleDivisions != 0 {
		t.Fatalf("%d division ask(s) came from a trample attacker: gorge poses the CR 510.1c division only for non-trample attackers", trampleDivisions)
	}
	if n := census.TotalUnmapped(); n > 0 {
		t.Fatalf("census recorded %d unmapped decision(s): %v", n, census.Unmapped)
	}
	if n := census.TotalRejected(); n > 0 {
		t.Fatalf("census recorded %d rejected answer(s): %v", n, census.Rejected)
	}
	if posed := census.Posed["chooseCombatDamageAssignment"]; posed < divisions {
		t.Fatalf("divisions asked=%d but only %d chooseCombatDamageAssignment prompt(s) were posed: %v",
			divisions, posed, census.Posed)
	}
}

// doubleBlockPick declares two charge-free blockers on one attacker (a
// declaration the mock client's policy never makes), so the game reaches a
// CR 510.1c division ask. It only fires when the ask carries no required
// pair and the chosen attacker's bounds admit a two-creature team with no
// charge; otherwise the client answers as usual. Decision.Validate stays
// the one legality fence for what this declares.
func doubleBlockPick(d *decision.Decision) (mb.PromptOutputValue, bool) {
	for _, o := range d.Options {
		if o.Required || o.BlockMust || o.AttackMust || o.AttackMustTwo {
			return nil, false
		}
	}
	for _, o := range d.Options {
		if o.MinBlockers > 2 || o.MaxBlockers == 1 {
			continue
		}
		if o.Value != 0 || o.CostLife != 0 || o.CostPhyrexian != 0 || o.CostTaps != 0 {
			continue
		}
		// The second blocker: another charge-free option on the same
		// attacker, a different creature, both within the attacker's
		// two-creature bound.
		for _, p := range d.Options {
			if p.Attacker != o.Attacker || p.Obj == o.Obj || p.Index == o.Index {
				continue
			}
			if p.Value != 0 || p.CostLife != 0 || p.CostPhyrexian != 0 || p.CostTaps != 0 {
				continue
			}
			if o.MaxBlockers > 0 && o.MaxBlockers < 2 {
				continue
			}
			return mb.DeclareBlockersDecision{Assignments: []mb.BlockerAssignment{
				{BlockerID: "o" + strconv.FormatUint(uint64(o.Obj), 10), AttackerID: "o" + strconv.FormatUint(uint64(o.Attacker), 10)},
				{BlockerID: "o" + strconv.FormatUint(uint64(p.Obj), 10), AttackerID: "o" + strconv.FormatUint(uint64(p.Attacker), 10)},
			}}, true
		}
	}
	return nil, false
}

// divisionPickTargets resolves the division ask's attacker and its offered
// blocker ids off the seat's view the same way the prompt builder does
// (server/manabrew's divisionRecipientsOf): the attacker's view BlockedBy,
// in order. It is a fixture's re-derivation of a documented projection, not
// a second mapping.
func divisionPickTargets(v view.View, d *decision.Decision) (state.ObjID, []string, bool) {
	atk := d.Options[0].Obj
	if atk == 0 {
		return 0, nil, false
	}
	for _, p := range v.Players {
		for i := range p.Battlefield {
			if p.Battlefield[i].ID != atk {
				continue
			}
			ids := make([]string, 0, len(p.Battlefield[i].BlockedBy))
			for _, bid := range p.Battlefield[i].BlockedBy {
				if bid == 0 {
					return 0, nil, false
				}
				ids = append(ids, "o"+strconv.FormatUint(uint64(bid), 10))
			}
			if len(ids) < 2 {
				return 0, nil, false
			}
			return atk, ids, true
		}
	}
	return 0, nil, false
}

// battlefieldLookup is the fixture's battlefield-only lookup by id (the
// same scope server/manabrew's battlefieldCard resolves in).
func battlefieldLookup(v view.View, id state.ObjID) *view.CardView {
	for _, p := range v.Players {
		for i := range p.Battlefield {
			if p.Battlefield[i].ID == id {
				return &p.Battlefield[i]
			}
		}
	}
	return nil
}

// parseWireID reads a wire card id ("o12") back to its object id.
func parseWireID(s string) state.ObjID {
	rest, ok := strings.CutPrefix(s, "o")
	if !ok {
		return 0
	}
	n, err := strconv.ParseUint(rest, 10, 32)
	if err != nil {
		return 0
	}
	return state.ObjID(n)
}
