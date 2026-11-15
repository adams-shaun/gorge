package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
)

// The Silverquill Silencer lockstep over the namecast fixture pool
// (dirac/cardset/pools/fixtures/namecast): its ETBReplacement DB$ NameCard
// names a card of the opponent's, and its `T:Mode$ SpellCast | ValidCard$
// Card.NamedCard | ValidActivatingPlayer$ Opponent` trigger reads the
// listener's own chosen byte -- gorge namePredicate reads
// g.Obj(sc.Source).ChosenName with the trigger's source, Silencer -- so the
// opponent casting the named spell drains 3 life and draws its controller a
// card. This is the Card.NamedCard read outside a resolution (Demonic
// Consultation's dig reads its own resolution's answer instead), so
// castFilterOK matches the spell with the listener as the filter's source
// context (dirac/trig_castbound.go).
//
// Precondition: seat A's policy names Dark Ritual every Silencer entry asks
// for and seat B casts Dark Ritual whenever it can (its Swamps pay its {B}),
// so the trigger is reached several times; the drain is counted as a turn in
// which seat A drew a card beside its draw step (Silencer's trigger is the
// matchup's only other draw). The mutation subtest empties the trigger's
// ValidCard$ filter in the kernel table alone, so the kernel fires it on
// every opponent cast where gorge fires it only on the named spell.
func TestNameCardSilencerCast(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, asks, drains := runNameCast(t, 12, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, %d drain turns, asks %v", run.games, run.divs, run.stalls, drains, asks)
		if run.divs != 0 || run.stalls != 0 {
			t.Fatalf("%d divergences, %d stalls (first: %s)", run.divs, run.stalls, run.first)
		}
		if asks["Silverquill Silencer"] < 3 {
			t.Errorf("Silverquill Silencer asked its name in only %d turns, want at least 3", asks["Silverquill Silencer"])
		}
		if run.casts["Dark Ritual"] < 3 {
			t.Errorf("Dark Ritual was cast only %d times, want at least 3", run.casts["Dark Ritual"])
		}
		if drains < 1 {
			t.Errorf("the named-cast trigger never drained in %d games", run.games)
		}
	})
	t.Run("mutation", func(t *testing.T) {
		run, _, _ := runNameCast(t, 6, false, func(tbl *kop.Table) {
			// a ValidCard$ no card of the matchup can match: the trigger's
			// filter read is the only thing that fires it, so the kernel
			// under-fires where gorge fires on the named spell
			tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TBattle})
			f := kop.FilterID(len(tbl.Filters) - 1)
			for i := range tbl.Cards {
				for j := range tbl.Cards[i].Abilities {
					if a := &tbl.Cards[i].Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigSpellCast {
						a.Trig.Filter = f
					}
				}
			}
		})
		run.requireDivergence(t, "a SpellCast ValidCard$ read from the wrong source")
	})
}

// ncCastDecks is the namecast matchup: seat A plays Silverquill Silencer and
// names Dark Ritual (seat B's spell), seat B casts Dark Ritual with its
// Swamps.
var ncCastDecks = [2][]cardset.Entry{
	{{Name: "Silverquill Silencer", Count: 12}, {Name: "Plains", Count: 14}, {Name: "Swamp", Count: 14}},
	{{Name: "Dark Ritual", Count: 12}, {Name: "Swamp", Count: 28}},
}

// runNameCast plays seeds 1..seeds of the namecast matchup, gorge leading,
// under nameCastPolicy, and counts the turns of gorge's log in which
// Silverquill Silencer asked for a name (nameCardAsks) and the casts of the
// game (the picks' source cards). mutate, when set, rewrites the matchup's
// table before the games are played (the mutation subtest's feature switch).
func runNameCast(t *testing.T, seeds int, report bool, mutate func(*kop.Table)) (fixtureRun, map[string]int, int) {
	t.Helper()
	reg, set, decks, tbl := p3Setup(t, "namecast", ncCastDecks)
	// Silencer's ask names a nonland card (ValidCards$ Card.nonLand), so the
	// table offers Table.NameCards, not NameCardsAny
	if len(tbl.NameCards) == 0 {
		t.Fatal("precondition: the matchup table lists no nameable cards")
	}
	if mutate != nil {
		mutate(tbl)
	}
	// namesB is the deciding seat A's offer of seat B's own card names: the
	// ask's universe is the matchup's cards, and a name seat B's deck holds
	// is one seat B can cast.
	namesB := map[uint16]bool{}
	for _, c := range decks[1] {
		namesB[canon.NameValue(cardName(c))] = true
	}
	run := fixtureRun{casts: map[string]int{}}
	asks := map[string]int{}
	drains := 0
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		m, err := NewMatchup(reg, [2]string{"namecast-a", "namecast-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		if m.NameUniverse == nil { // the harness default: the matchup's own cards
			t.Fatal("precondition: NewMatchup left a name asker's matchup on the registry universe")
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, Config{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, nameCastPolicy(seed, namesB), RunConfig{MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		for _, a := range res.Actions { // the picks: the casts, by their card
			if a.Kind() == ActCast {
				if n := refCardName(g, a.Src()); n != "" {
					run.casts[n]++
				}
			}
		}
		run.games++
		nameCardAsks(t, g, asks)
		drains += nameCastDrains(g)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			run.divs++
			msg := strings.TrimSpace(d.What + " " + d.Field + " " + d.Detail)
			if run.first == "" {
				run.first = msg
			}
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || run.divs >= 3 {
				break
			}
		}
	}
	return run, asks, drains
}

// nameCastPolicy answers seat A's name ask with a card of seat B's deck list
// and otherwise plays casts, attacks and lands at random (nameCardPolicy's
// shape, without the Demonic Consultation casts).
func nameCastPolicy(seed uint64, namesB map[uint16]bool) Policy {
	rnd := RandomPolicy(seed)
	nameKind := ActChoose + CanonKind(mustChooseCode("name"))
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case nameKind:
				if c.D.Player&1 == 0 && namesB[uint16(a.Tgt())<<8|uint16(a.Mode())] {
					buf = append(buf, a) // seat A names Dark Ritual
				}
			case ActAttack, ActCast, ActCastFace, ActPlayLand:
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// nameCastDrains counts the turns of g's log in which seat A drew a card
// beside its draw step: more than one events.Draw of player 0 in a turn is
// Silverquill Silencer's named-cast trigger (the matchup's only other draw).
// The opening hands' draws come before the first turn and are skipped.
func nameCastDrains(g *GorgeSide) int {
	turns := 0
	inTurn, open := 0, true
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.TurnChange:
			if !open && inTurn > 1 {
				turns++
			}
			inTurn = 0
			open = false
		case events.Draw:
			if e.Player == 0 {
				inTurn++
			}
		}
	}
	if !open && inTurn > 1 {
		turns++
	}
	return turns
}
