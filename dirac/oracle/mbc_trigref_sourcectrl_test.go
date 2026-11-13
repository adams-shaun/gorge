package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC trigger-referents-6 family in lockstep against gorge
// (pool dirac/cardset/pools/fixtures/mbctrigref6), one deck pair per
// spelling kgen now lowers:
//
//   - a DamageDone trigger whose OptionalDecider$ and Draw Defined$ are
//     TriggeredSourceController (Edric, Spymaster of Trest: "its controller
//     may draw a card"): the CR 603.5 ask and the draw belong to the damage
//     source's controller (kop.Tf5OptSrcCtrl, RefTriggerTgtCtrl).
//   - ChangeZone Defined$ TriggeredNewCardLKICopy to LibraryPosition$ 2 from
//     a source's own dies-or-exile trigger (God-Eternal Oketra): the card
//     goes third from the top (kop.LibThird), killed by Doom Blade and
//     exiled by Swords to Plowshares.
//   - TargetsWithDefinedController$ TriggeredAttackedTarget (Gornog, the Red
//     Reaper: "target creature that player controls becomes a Coward"): the
//     target slot is held to the defending player's creatures
//     (kop.F2DefendingCtrl).
//
// Each subtest first asserts the carrier is playable in the pool (it is
// refused without the lowering), then plays the lockstep games with the
// comparison at every decision gorge's, and asserts some game saw the
// carrier's own trigger do its work in gorge's event log: Edric's and
// Gornog's trigger queued, Oketra's card put into a library from the
// graveyard or exile.

const mbcTrigRef6Pool = "mbctrigref6"

var mbcTrigRef6Cases = []struct {
	carrier string
	decks   [2][]cardset.Entry
	// seen reports whether event e of the gorge game is the carrier's
	// trigger at work
	seen func(ref *GorgeSide, e events.Event) bool
}{
	{"Edric, Spymaster of Trest", [2][]cardset.Entry{
		{
			{Name: "Edric, Spymaster of Trest", Count: 4}, {Name: "Grizzly Bears", Count: 8}, {Name: "Giant Spider", Count: 4},
			{Name: "Coral Merfolk", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 10},
		},
		{
			{Name: "Hill Giant", Count: 8}, {Name: "Mogg Flunkies", Count: 6}, {Name: "Lightning Bolt", Count: 6},
			{Name: "Mountain", Count: 20},
		},
	}, trigQueuedBy("Edric, Spymaster of Trest")},
	{"God-Eternal Oketra", [2][]cardset.Entry{
		{
			{Name: "God-Eternal Oketra", Count: 4}, {Name: "Savannah Lions", Count: 8}, {Name: "Glory Seeker", Count: 6},
			{Name: "Serra Angel", Count: 2}, {Name: "Plains", Count: 20},
		},
		{
			{Name: "Swords to Plowshares", Count: 6}, {Name: "Doom Blade", Count: 6}, {Name: "Glory Seeker", Count: 6},
			{Name: "Plains", Count: 11}, {Name: "Swamp", Count: 11},
		},
	}, func(ref *GorgeSide, e events.Event) bool {
		// the card itself goes from the graveyard or exile into a library
		o := ref.E.G.Obj(e.Obj)
		return e.Kind == events.MoveZone && e.To == state.ZLibrary && (e.From == state.ZGraveyard || e.From == state.ZExile) &&
			o != nil && o.Face() != nil && o.Face().Name == "God-Eternal Oketra"
	}},
	{"Gornog, the Red Reaper", [2][]cardset.Entry{
		{
			{Name: "Gornog, the Red Reaper", Count: 4}, {Name: "Minotaur Warrior", Count: 8}, {Name: "Fearless Halberdier", Count: 8},
			{Name: "Mountain", Count: 20},
		},
		{
			{Name: "Grizzly Bears", Count: 8}, {Name: "Giant Spider", Count: 6}, {Name: "Forest", Count: 26},
		},
	}, trigQueuedBy("Gornog, the Red Reaper")},
}

// trigQueuedBy is the seen test of a carrier whose only triggered ability is
// the one under test: a trigger of that card was put on the stack.
func trigQueuedBy(name string) func(*GorgeSide, events.Event) bool {
	return func(ref *GorgeSide, e events.Event) bool {
		o := ref.E.G.Obj(e.Obj)
		return e.Kind == events.TriggerPush && o != nil && o.Face() != nil && o.Face().Name == name
	}
}

func TestMBCTriggerReferents6Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, tc := range mbcTrigRef6Cases {
		t.Run(tc.carrier, func(t *testing.T) {
			t.Parallel()
			reg, set, gd := lockstepPool(t, mbcTrigRef6Pool, tc.decks)
			for s, d := range gd {
				if !SetDeckSupported(set, d) {
					t.Fatalf("seat %d: %s not playable in the %s pool", s, tc.carrier, mbcTrigRef6Pool)
				}
			}
			tbl, _, _, err := set.Matchup(tc.decks[0], tc.decks[1])
			if err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			reached := 0
			for seed := uint64(1); seed <= 80; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%4 == 0}
				m, err := NewMatchup(reg, [2]string{mbcTrigRef6Pool + "-a", mbcTrigRef6Pool + "-b"}, gd, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				pol := castFirstPolicy(seed)
				if seed%8 == 0 {
					pol = RandomPolicy(seed)
				}
				rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
				hit, next := false, 0
				rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
					evs := ref.E.L.Events
					for ; next < len(evs); next++ {
						hit = hit || tc.seen(ref, evs[next])
					}
					return nil
				}
				res := Lockstep(m, seed, k, pol, rc)
				CloseSide(k)
				if err := unmodelledStallError(t.Name(), &res); err != nil {
					t.Errorf("seed %d: %v", seed, err)
				}
				if d := res.Div; d != nil {
					from := max(d.At-12, 0)
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nlast actions %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, actionStrings(res.Actions[from:d.At]))
				}
				if hit {
					mu.Lock()
					reached++
					mu.Unlock()
				}
			}
			if reached == 0 {
				t.Fatalf("%s: no game saw its trigger at work: the pool does not exercise it", tc.carrier)
			}
			t.Logf("%s: %d of 80 games saw the trigger at work", tc.carrier, reached)
		})
	}
}
