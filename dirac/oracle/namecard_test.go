package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The mid-resolution card-name choice (gorge effNameCard, engine/effects/
// cardflow.go; kernel ExtNameCard, dirac/repl_namecard.go) and its consumers,
// in lockstep over the namecard fixture pool (dirac/cardset/pools/fixtures/
// namecard): Demonic Consultation's `SP$ NameCard | Defined$ You |
// SubAbility$ DBExile` (the chosen name read by a DigUntil Valid$
// Card.NamedCard in the same chain), and Cheering Fanatic's trigger-body
// `DB$ NameCard` whose Effect's ReduceCost static reads ValidCard$
// Card.NamedCard and whose `DB$ Cleanup | ClearNamedCard$ True` wipes the
// source so the next attack asks anew.
//
// Both decks' cards are the ask's names: NewMatchup narrows gorge's name
// universe to the matchup's own cards whenever one of them asks
// (holdsNameChooserN), so the kernel's Table.NameCardsAny list and gorge's
// sorted names are the same list in the same order.
//
// Precondition: the policy casts Demonic Consultation whenever it can and
// attacks with Cheering Fanatic whenever it can, so both name asks are
// reached (gorge's events.Choose Counter "name") several times per card. The
// mutation subtest drops the kernel's answer store (DebugNameCardNoStore), so
// Demonic Consultation's dig reveals its whole library where gorge finds the
// named card.
func TestNameCardLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, asks := runNameCard(t, 24, true)
		t.Logf("%d games, %d divergences, %d stalls, name asks %v", run.games, run.divs, run.stalls, asks)
		if run.divs != 0 || run.stalls != 0 {
			t.Fatalf("%d divergences, %d stalls (first: %s)", run.divs, run.stalls, run.first)
		}
		if asks["Demonic Consultation"] < 3 {
			t.Errorf("Demonic Consultation asked its name in only %d turns, want at least 3", asks["Demonic Consultation"])
		}
		if asks["Cheering Fanatic"] < 3 {
			t.Errorf("Cheering Fanatic asked its name in only %d turns, want at least 3", asks["Cheering Fanatic"])
		}
	})
	t.Run("mutation", func(t *testing.T) {
		old := dirac.DebugNameCardNoStore
		dirac.DebugNameCardNoStore = true
		defer func() { dirac.DebugNameCardNoStore = old }()
		run, _ := runNameCard(t, 8, false)
		run.requireDivergence(t, "a dropped NameCard answer")
	})
}

// ncDecks is the namecard matchup: Demonic Consultation's deck names a card
// its own library holds (the Swamps its dig reveals), Cheering Fanatic's
// attacks every turn.
var ncDecks = [2][]cardset.Entry{
	{{Name: "Demonic Consultation", Count: 10}, {Name: "Swamp", Count: 30}},
	{{Name: "Cheering Fanatic", Count: 10}, {Name: "Mountain", Count: 30}},
}

// runNameCard plays seeds 1..seeds of the namecard matchup, gorge leading,
// under nameCardPolicy, and counts per name-asking card the turns of gorge's
// log in which it asked (gorge events.Choose Counter "name", the source card
// named).
func runNameCard(t *testing.T, seeds int, report bool) (fixtureRun, map[string]int) {
	t.Helper()
	reg, set, decks, tbl := p3Setup(t, "namecard", ncDecks)
	if len(tbl.NameCardsAny) == 0 {
		t.Fatal("precondition: the matchup table lists no nameable cards")
	}
	if n := cardNameCount(decks, "Demonic Consultation"); n != 10 || cardNameCount(decks, "Cheering Fanatic") != 10 {
		t.Fatalf("precondition: bad deck lists (%d, %d)", n, cardNameCount(decks, "Cheering Fanatic"))
	}
	// names is each seat's own card names, the name ask of a card that seat
	// casts or triggers: the ask is the caster's, over the matchup's cards,
	// and a name its own library holds is one its dig can find.
	names := [2]map[uint16]bool{}
	for s := range decks {
		names[s] = map[uint16]bool{}
		for _, c := range decks[s] {
			names[s][canon.NameValue(cardName(c))] = true
		}
	}
	run := fixtureRun{casts: map[string]int{}}
	asks := map[string]int{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"namecard-a", "namecard-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		if m.NameUniverse == nil { // the harness default: the matchup's own cards
			t.Fatal("precondition: NewMatchup left a name asker's matchup on the registry universe")
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, nameCardPolicy(seed, names), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		run.games++
		nameCardAsks(t, g, asks)
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
	return run, asks
}

// nameCardPolicy casts Demonic Consultation, attacks with Cheering Fanatic,
// answers a name ask with a card of the deciding deck's own list, and
// otherwise plays randomly.
func nameCardPolicy(seed uint64, names [2]map[uint16]bool) Policy {
	rnd := RandomPolicy(seed)
	castKind := ActChoose + CanonKind(mustChooseCode("name"))
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch k := a.Kind(); {
			case k == castKind:
				if names[c.D.Player&1][uint16(a.Tgt())<<8|uint16(a.Mode())] {
					buf = append(buf, a)
				}
			case k == ActAttack:
				buf = append(buf, a)
			case k == ActCast || k == ActCastFace || k == ActPlayLand:
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

// nameCardAsks counts, per asking card, the turns in which gorge asked for a
// card name on its behalf (events.Choose Counter "name", the ask's source).
func nameCardAsks(t *testing.T, g *GorgeSide, asks map[string]int) {
	t.Helper()
	seen := map[string]bool{}
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.TurnChange:
			for n := range seen {
				asks[n]++
			}
			seen = map[string]bool{}
		case events.Choose:
			if e.Counter != "name" {
				continue
			}
			o := g.E.G.Obj(e.Obj)
			if o == nil || o.Card == nil {
				continue
			}
			seen[o.Face().Name] = true
		}
	}
	for n := range seen {
		asks[n]++
	}
}

func cardNameCount(decks [2][]*cards.Card, name string) int {
	n := 0
	for _, d := range decks {
		for _, c := range d {
			if cardName(c) == name {
				n++
			}
		}
	}
	return n
}
