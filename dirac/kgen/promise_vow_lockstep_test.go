package kgen

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Promise of Loyalty (hosted deck cavalry-charge; dirac gap
// cli-20261127T065130Z-0be3787f): SP$ RepeatEach over every player whose body
// is a PutCounter | Choices$ Creature.ControlledBy Player.IsRemembered |
// Chooser$ / Placer$ Player.IsRemembered | CounterType$ VOW | RememberCards$
// True and a SacrificeAll of Creature.!IsRemembered+ControlledBy
// Player.IsRemembered, then a permanent DB$ Effect (RememberObjects$
// Remembered, ForgetOnMoved$ Battlefield, ForgetCounter$ VOW) whose CantAttack
// static (ValidCard$ Card.IsRemembered, Target$ You,Planeswalker.YouCtrl)
// keeps each vowed creature off the caster while its vow counter stands.
//
// The lockstep plays the real card from the corpus, both seats on the same
// white deck, gorge leading the decisions. The scan is the precondition: a
// seed counts only when a Promise resolved while a player held two or more
// creatures (the per-player election asked) and a creature carrying a vow
// counter then stood in an attackers decision (the restriction was live).

var promiseVowFillers = []string{"Savannah Lions", "Armored Pegasus", "Suntail Hawk", "Plains"}

func promiseVowDecks() [2][]cardset.Entry {
	d0 := []cardset.Entry{
		{Name: "Promise of Loyalty", Count: 4},
		{Name: "Savannah Lions", Count: 8},
		{Name: "Armored Pegasus", Count: 4},
		{Name: "Plains", Count: 24},
	}
	d1 := []cardset.Entry{
		{Name: "Promise of Loyalty", Count: 4},
		{Name: "Suntail Hawk", Count: 8},
		{Name: "Armored Pegasus", Count: 4},
		{Name: "Plains", Count: 24},
	}
	return [2][]cardset.Entry{slices.Clone(d0), slices.Clone(d1)}
}

// TestPromiseOfLoyaltyVowLowering: the card lowers with no refusal, as the
// per-player counted pick, the remembering counter, the sacrifice of the
// rest and one capturing permanent emblem.
func TestPromiseOfLoyaltyVowLowering(t *testing.T) {
	t.Parallel()
	ratchetCards(t)
	c := lowerCorpusCard(t, "Promise of Loyalty")
	if !promiseVowKindServed() {
		// Blocked on the counter family's capacity: the card refuses once, by
		// name, at its Effect, after the election's loop lowered.
		if len(c.Refusals) != 1 || !strings.Contains(c.Refusals[0].Reason, `counter kind "VOW"`) {
			t.Fatalf("Promise of Loyalty refusals %v: want the one counter-kind refusal", c.Refusals)
		}
		var sawRepeat, sawPick, sawPut, sawSac bool
		for _, a := range c.Abilities {
			for _, in := range a.Prog {
				sawRepeat = sawRepeat || in.Op == "RepeatEach" && in.Obj.Kind == rEachPlayer
				sawPick = sawPick || in.Op == "ChooseCard" && in.CS == "PutCounterPick" && in.Obj.Kind == rEach
				sawPut = sawPut || in.Op == "PutCounter" && in.CS == "VOW" && in.Obj.Kind == rChosen && in.B.N == 2
				sawSac = sawSac || in.Op == "SacrificeAll" && in.F != nil
			}
		}
		if !sawRepeat || !sawPick || !sawPut || !sawSac {
			t.Errorf("Promise of Loyalty's election lowered repeat %v pick %v put %v sacrifice %v", sawRepeat, sawPick, sawPut, sawSac)
		}
		return
	}
	for _, rf := range c.Refusals {
		t.Errorf("Promise of Loyalty refused: %s", rf.Reason)
	}
	var sawRepeat, sawPick, sawPut, sawSac, sawEmblem bool
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			switch in.Op {
			case "RepeatEach":
				sawRepeat = in.Obj.Kind == rEachPlayer
			case "ChooseCard":
				sawPick = in.CS == "PutCounterPick" && in.Obj.Kind == rEach
			case "PutCounter":
				sawPut = in.CS == "VOW" && in.Obj.Kind == rChosen && in.B.N == 2
			case "SacrificeAll":
				sawSac = in.F != nil
			case "CreateEmblem":
				sawEmblem = true
			}
		}
	}
	if !sawRepeat || !sawPick || !sawPut || !sawSac || !sawEmblem {
		t.Errorf("Promise of Loyalty lowered repeat %v pick %v put %v sacrifice %v emblem %v", sawRepeat, sawPick, sawPut, sawSac, sawEmblem)
	}
	if len(c.Emblems) != 1 {
		t.Fatalf("Promise of Loyalty lowered %d emblems, want 1", len(c.Emblems))
	}
	if e := c.Emblems[0]; !e.CaptureRem || !e.CaptureLapse || e.CaptureCounter != "VOW" || e.UntilEOT {
		t.Errorf("Promise of Loyalty emblem %+v: want a permanent remembered-set capture lapsing on the battlefield and on the last vow counter", e)
	}
}

// promiseVowKindServed reports whether the kernel has a counter kind for the
// vow counter. CtrVow is deferred past kop.NumCounterKinds3 (the counter
// family's frozen cover, ordinals.tsv), so until the cover moves the card
// refuses by name and the lockstep below has nothing to play.
func promiseVowKindServed() bool {
	_, err := counterKind("VOW")
	return err == nil
}

type promiseVowSeen struct {
	asked, restricted bool
}

// promiseVowGame is one game's scan state.
type promiseVowGame struct {
	inGY map[state.ObjID]bool
	// armed: a Promise reached a graveyard while some player held two or more
	// creatures at the previous pick (the election was asked).
	prevMaxCreatures int
	prevPromiseGY    bool
	asked            bool
}

func promiseVowScan(g *state.Game, gm *promiseVowGame) (vowed bool) {
	if g == nil {
		return false
	}
	promiseGY, maxCr := false, 0
	for p := state.PlayerID(0); p < 2; p++ {
		n := 0
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o == nil || o.Card == nil {
				continue
			}
			if o.Card.Faces[0].IsCreature() {
				n++
			}
			if o.Counter("VOW") > 0 {
				vowed = true
			}
		}
		maxCr = max(maxCr, n)
		for _, id := range g.Zone(state.ZGraveyard, p) {
			if o := g.Obj(id); o != nil && o.Card != nil && o.Card.Faces[0].Name == "Promise of Loyalty" {
				promiseGY = true
			}
		}
	}
	if promiseGY && !gm.prevPromiseGY && gm.prevMaxCreatures >= 2 {
		gm.asked = true
	}
	gm.prevPromiseGY, gm.prevMaxCreatures = promiseGY, maxCr
	return vowed
}

func promiseVowPolicy(seed uint64, seen *promiseVowSeen) oracle.Policy {
	rnd := oracle.RandomPolicy(seed)
	var gm promiseVowGame
	return oracle.PolicyFunc(func(c *oracle.PickCtx) oracle.Canon {
		if c.G != nil {
			vowed := promiseVowScan(c.G.E.G, &gm)
			if gm.asked {
				seen.asked = true
			}
			if vowed && c.D.Kind == oracle.DAttackers {
				seen.restricted = true
			}
		}
		buf := make([]oracle.Canon, 0, 8)
		for _, a := range c.D.Options {
			switch c.D.Kind {
			case oracle.DAttackers:
				if a.Kind() == oracle.ActAttack || a.Kind() == oracle.ActDone || a.Kind() == oracle.ActPass {
					buf = append(buf, a)
				}
			case oracle.DBlockers:
				if a.Kind() == oracle.ActBlock || a.Kind() == oracle.ActDone || a.Kind() == oracle.ActPass {
					buf = append(buf, a)
				}
			default:
				switch a.Kind() {
				case oracle.ActCast, oracle.ActCastFace, oracle.ActActivate, oracle.ActPlayLand:
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 || len(buf) == len(c.D.Options) {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&oracle.PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// TestPromiseOfLoyaltyVowLockstep plays the card decision-for-decision with
// gorge over the seeds.
func TestPromiseOfLoyaltyVowLockstep(t *testing.T) {
	t.Parallel()
	if !promiseVowKindServed() {
		t.Skip("blocked: the kernel has no counter kind for VOW (kop.CtrVow is deferred past the frozen cover kop.NumCounterKinds3); the counter-kind cover ticket (dirac-counter-kind-cover) lifts it")
	}
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("no corpus: %v", err)
	}
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadCardNames(append([]string{"Promise of Loyalty"}, promiseVowFillers...), cardsDir)
	if err != nil {
		t.Fatalf("fixture pool: %v", err)
	}
	pd, _, _, err := tableOf(p, "promise-vow-fixture", nil)
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	decks := promiseVowDecks()
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s := range gd {
		for _, e := range decks[s] {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	const seeds = 32
	var seen promiseVowSeen
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := oracle.NewMatchup(reg, [2]string{"vow-a", "vow-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := oracle.Config{}
		if seed%4 == 0 {
			cfg = oracle.Config{GorgeAutopaySurface: true}
		}
		k, err := oracle.SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := promiseVowPolicy(seed, &seen)
		if seed%8 == 0 {
			pol = oracle.RandomPolicy(seed)
		}
		res := oracle.Lockstep(m, seed, k, pol, oracle.RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		oracle.CloseSide(k)
		if d := res.Div; d != nil {
			div++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
	if !seen.asked || !seen.restricted {
		t.Fatalf("over %d seeds the card was played %+v: the lockstep never reached the election with two creatures and an attackers decision under a vow", seeds, seen)
	}
	if div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
	t.Logf("Promise of Loyalty played %+v over %d seeds", seen, seeds)
}
