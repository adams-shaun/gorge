package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// tgteachopp pool, whose carriers read TargetsForEachPlayer$ True over an
// opponents-only filter (TargetMin$ 0 | TargetMax$ OneEach). In two players
// that is "up to one target" (dirac/kgen/lower_targeteachopponent.go):
//
//   - Celebrate the Mountain-king: ETB exile until it leaves;
//   - Lich's Relic: the paid ImmediateTrigger destroy (creature or
//     planeswalker: The Theorist is in both decks);
//   - Omega, Heartless Evolution: ETB tap, stun counters on Defined$ Targeted;
//   - The Theorist, Jace Beleren: the -2 bounce to hand.

var ktgDecks = [2][]cardset.Entry{
	{
		{Name: "Celebrate the Mountain-king", Count: 4}, {Name: "The Theorist, Jace Beleren", Count: 4},
		{Name: "Lich's Relic", Count: 3}, {Name: "Ornithopter", Count: 3}, {Name: "Memnite", Count: 3},
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 9}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Omega, Heartless Evolution", Count: 4}, {Name: "Lich's Relic", Count: 3},
		{Name: "The Theorist, Jace Beleren", Count: 2}, {Name: "Grizzly Bears", Count: 6}, {Name: "Ornithopter", Count: 3},
		{Name: "Forest", Count: 9}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 5},
	},
}

const (
	ktgCelebrate = "Celebrate the Mountain-king"
	ktgRelic     = "Lich's Relic"
	ktgOmega     = "Omega, Heartless Evolution"
	ktgTheorist  = "The Theorist, Jace Beleren"
)

func ktgSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tgteachopp")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ktgDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(ktgDecks[0], ktgDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

// ktgScan counts, from gorge's event log, the carriers' moves: an object
// leaving the battlefield (or tapping) while the latest pushed trigger or
// ability is the carrier's. A Celebrate entry whose exile trigger moved
// nothing is a zero-target resolution (only entries before the final turn
// are counted, so the trigger has resolved).
func ktgScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	name := func(id state.ObjID) string {
		if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
			return ob.Face().Name
		}
		return ""
	}
	evs := g.E.L.Events
	last := len(evs)
	for last > 0 && evs[last-1].Kind != events.TurnChange {
		last--
	}
	var trig string
	enters, exiles := 0, 0
	for i, ev := range evs {
		switch ev.Kind {
		case events.TriggerPush, events.AbilityPush:
			trig = name(ev.Obj)
		case events.PutOnStack, events.StepChange, events.TurnChange:
			trig = ""
		case events.Tap:
			if trig == ktgOmega && name(ev.Obj) != ktgOmega {
				n["omega:tap"]++
			}
		case events.MoveZone:
			if ev.From != state.ZBattlefield {
				if ev.To == state.ZBattlefield && name(ev.Obj) == ktgCelebrate && i < last {
					enters++
				}
				continue
			}
			switch {
			case trig == ktgCelebrate && ev.To == state.ZExile:
				n["celebrate:exile"]++
				if i < last {
					exiles++
				}
			case trig == ktgRelic && ev.To == state.ZGraveyard && name(ev.Obj) != ktgRelic:
				n["relic:destroy"]++
			case trig == ktgTheorist && ev.To == state.ZHand:
				n["theorist:bounce"]++
			}
		}
	}
	if enters > exiles {
		n["celebrate:zero"] += enters - exiles
	}
}

func TestTgteachoppLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := ktgSetup(t)
	played := map[string]int{}
	n := map[string]int{}
	stalls := map[string]int{}
	games := 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktg-a", "ktg-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		res := Lockstep(m, seed, k, trackingEagerPolicy(seed, played, &gs), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		ktgScan(gs, n)
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, counts %v; kernel stalls %v; carrier plays %v", games, n, stalls, played)
	for _, w := range []struct {
		key string
		min int
	}{{"celebrate:exile", 10}, {"celebrate:zero", 5}, {"omega:tap", 10}, {"theorist:bounce", 5}} {
		if n[w.key] < w.min {
			t.Errorf("%s = %d, want at least %d", w.key, n[w.key], w.min)
		}
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}

// TestTgteachoppMutation rewrites Celebrate the Mountain-king's target slot
// to a forced target (Min 1: UpTo cleared) and requires the lockstep to
// diverge: gorge still offers the zero-target choice.
func TestTgteachoppMutation(t *testing.T) {
	reg, set, decks, tbl := ktgSetup(t)
	mut := *tbl
	mut.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	hit := false
	for i := range mut.Cards {
		if mut.Cards[i].Name != ktgCelebrate {
			continue
		}
		abs := append([]kop.Ability(nil), mut.Cards[i].Abilities...)
		for j := range abs {
			if abs[j].NTargets == 1 && abs[j].Targets[0].UpTo {
				abs[j].Targets[0].UpTo = false
				hit = true
			}
		}
		mut.Cards[i].Abilities = abs
	}
	if !hit {
		t.Fatal("no Celebrate the Mountain-king up-to-one target slot in the table")
	}
	for seed := uint64(1); seed <= 128; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktg-a", "ktg-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, &mut, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		res := Lockstep(m, seed, k, trackingEagerPolicy(seed, map[string]int{}, &gs), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("diverged at seed %d decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("forced-target mutation did not diverge in 128 seeds")
}
