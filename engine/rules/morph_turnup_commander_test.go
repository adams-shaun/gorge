package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// A Morph card's cost sacrifices another creature. The chosen commander
// belongs to seat 1 but is controlled by seat 0. Under CR 903.9a the
// sacrifice is not replaced: the commander goes to its owner's graveyard, the
// morph turns face up at once, and the owner (seat 1, not the controller) is
// asked by the state-based check that follows.
func TestMorphTurnFaceUpSacrificedCommanderAsksItsOwnerAfterwards(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		pick int
		zone state.Zone
	}{{"accept", 0, state.ZCommand}, {"decline", 1, state.ZGraveyard}} {
		t.Run(tc.name, func(t *testing.T) {
			reg := searchTestRegistry(t)
			morph := card(t, "Name:Sacrifice morph fixture\nManaCost:0\nTypes:Creature\nPT:2/2\nK:Morph:Sac<1/Creature.Other>\nOracle:x\n")
			forest := searchCorpusCard(t, reg, "Forest")
			cmd0, cmd1 := card(t, tinyCmdSrc), card(t, tinyCmdSrc)
			decks := [][]*cards.Card{{cmd0, morph}, {cmd1}}
			for p := range decks {
				for len(decks[p]) < 40 {
					decks[p] = append(decks[p], forest)
				}
			}
			cfg := seatZeroStart(Config{Seed: 9127, Names: []string{"turn-up", "owner"},
				Decks: decks, Commanders: [][]int{{0}, {0}}, Tokens: reg.Tokens,
				StartingLife: 40, Format: FormatCommander})
			e := New(cfg)
			e.Advance()
			toMain1(t, e)
			id := searchMoveByNameSeat(t, e, 0, "Sacrifice morph fixture", state.ZBattlefield)
			e.emit(events.Event{Kind: events.TurnFaceDown, Obj: id})
			e.emit(events.Event{Kind: events.CastInfo, Obj: id, Counter: events.FlagsString(state.FlagMorphed)})
			mf, ok := morphFaceUpCost(e.G.Obj(id))
			if !ok || len(mf.cost.Sac) != 1 || mf.cost.Sac[0].N != 1 {
				t.Fatalf("precondition: fixture turn-up cost = %+v, want Sac<1/Creature.Other>", mf.cost)
			}
			cmd := fieldCommander(t, e, 1, 0)
			e.emit(events.Event{Kind: events.ControlChange, Obj: cmd, Player: 0})
			if o := e.G.Obj(cmd); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 || o.Owner != 1 {
				t.Fatalf("precondition: sacrifice candidate = %+v, want seat 1 commander controlled by seat 0", o)
			}
			if cands := pay.SacrificeCostCandidates(asPayer(e), 0, id, mf.cost.Sac[0], false); !turnUpContainsObj(cands, cmd) {
				t.Fatalf("precondition: commander %d not in Creature.Other sacrifice candidates: %v", cmd, cands)
			}
			if e.commanderZoneReplacementApplies(events.Sacrifice(cmd)) {
				t.Fatalf("precondition: a sacrifice of commander %d must not be replaced (CR 903.9a)", cmd)
			}
			mark := len(e.L.Events)
			if !e.morphTurnUpPayable(0, id, mf.cost, costMods{}) {
				t.Fatal("precondition: turn-up cost is not payable")
			}
			e.priorityRound()
			submit(t, e, turnFaceUpIndex(t, e, id))
			d := e.Pending()
			if d == nil || d.Kind != decision.KCommanderZone || d.Player != 1 {
				t.Fatalf("pending = %+v, want seat 1's (the owner's) commander-zone choice", d)
			}
			if e.G.Obj(id).FaceDown {
				t.Fatal("morph still face down: the sacrifice cost is paid at once under CR 903.9a")
			}
			if got := e.G.Obj(cmd).Zone; got != state.ZGraveyard || !contains(e.G.Zone(state.ZGraveyard, 1), cmd) {
				t.Fatalf("sacrificed commander zone = %v, want its owner's (seat 1) graveyard", got)
			}
			submit(t, e, tc.pick)
			if got := e.G.Obj(cmd).Zone; got != tc.zone {
				t.Fatalf("commander zone = %v, want %v", got, tc.zone)
			}
			moveAt, flipAt := -1, -1
			for i, ev := range e.L.Events[mark:] {
				if ev.Kind == events.MoveZone && ev.Obj == cmd && ev.To == state.ZGraveyard && moveAt < 0 {
					moveAt = i
				}
				if ev.Kind == events.TurnFaceUp && ev.Obj == id {
					flipAt = i
				}
			}
			if moveAt < 0 || flipAt <= moveAt {
				t.Fatalf("sacrifice move index=%d TurnFaceUp index=%d; want the sacrifice first", moveAt, flipAt)
			}
			replayCheck(t, e, cfg)
		})
	}
}
