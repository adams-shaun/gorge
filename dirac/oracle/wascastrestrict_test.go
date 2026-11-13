package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The spell cast-provenance spend restriction lockstep
// (CR 106.6 / 601.2): Mm'menon, the Right Hand grants every artifact its
// controller has "{T}: Add {U}, spent only on a spell cast from anywhere
// other than your hand" (RestrictValid$ Spell.!wasCastFromYourHand). gorge
// reads the restriction PRE-push through castProvenanceAdmitsPending, which
// denies the hand family for an object off the stack even when negated, so
// its prefix search never counts the granted mana toward an offered play: the
// kernel's grant search (grantAdmits) reads the play the same way. The
// mechanism is counted from gorge's log like kmrScan: a restricted ManaAdd
// names its restriction and funds the push made since the last decision (or
// the next one when floated before any push). A spend is legal only on a
// spell pushed from anywhere but a hand, and never on an activated ability.
//
// Interdimensional Web Watch (Spell.wasCastFromExile on a Combo Any x2
// production) is the same restriction on a source
// gorge's planner cannot price (PaymentPlanChoiceColours: a Combo Any has no
// colour list): it is a census gap whose activation the prefix search
// branches on, a pair of colours per answer, and its mana floats restricted
// afterwards. TestWascastrestrictWebWatch counts the spends from gorge's
// log; the kernel follows the search (dirac/mana_window.go restrWideAnyMana).

var kwrDecks = [2][]cardset.Entry{
	{ // A: W/U, no restricted mana
		{Name: "Savannah Lions", Count: 8}, {Name: "Healer's Hawk", Count: 6}, {Name: "Coral Merfolk", Count: 6},
		{Name: "Opt", Count: 3}, {Name: "Otterball Antics", Count: 3}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 6},
	},
	{ // B: U/R, Mm'menon over its artifacts, the adventure and flashback casters
		{Name: "Mm'menon, the Right Hand", Count: 4}, {Name: "Ornithopter", Count: 3}, {Name: "Memnite", Count: 3},
		{Name: "Frolicking Familiar", Count: 3}, {Name: "Otterball Antics", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Opt", Count: 2}, {Name: "Coral Merfolk", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 7},
	},
}

const (
	kwrNotHandLegal = iota // Mm'menon's granted mana funds a spell cast from outside a hand
	kwrNotHandBad          // ...a spell cast from a hand or an activated ability (must stay 0)
	kwrN
)

var kwrNames = [kwrN]string{"granted mana on a spell not from hand", "granted mana elsewhere"}

// kwrScan walks gorge's log: a restricted positive ManaAdd carrying one of
// the two provenance restrictions funds the push that follows it.
func kwrScan(g *GorgeSide, n *[kwrN]int) {
	if g == nil {
		return
	}
	type pend struct {
		valid string
	}
	var pending []pend
	var cur *events.Event
	credit := func(push *events.Event, valid string) {
		if !strings.Contains(valid, "!wasCastFromYourHand") {
			return
		}
		if push.Kind == events.PutOnStack && push.From != state.ZHand {
			n[kwrNotHandLegal]++
		} else {
			n[kwrNotHandBad]++
		}
	}
	for i := range g.E.L.Events {
		ev := &g.E.L.Events[i]
		switch ev.Kind {
		case events.DecisionMade:
			cur = nil
		case events.ManaAdd:
			if ev.Amount <= 0 {
				continue
			}
			if valid, _, _, _, ok := events.ManaRestrictionFromText(restrSourceCutPersistent(ev.Text)); ok {
				if cur != nil {
					credit(cur, valid)
				} else {
					pending = append(pending, pend{valid})
				}
			}
		case events.ManaClear, events.TurnChange:
			pending = pending[:0]
		case events.PutOnStack, events.AbilityPush:
			cur = ev
			for _, p := range pending {
				credit(ev, p.valid)
			}
			pending = pending[:0]
		}
	}
}

func kwrSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	reg, _ := kcastSetup(t)
	set, err := cardset.RawByName("wascastspell")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, kcastDeck(t, reg, kwrDecks)
}

func TestWascastrestrictLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks := kwrSetup(t)
	tbl, _, _, err := set.Matchup(kwrDecks[0], kwrDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var fires [kwrN]int
	var st dirac.RestrictGrantStats
	dirac.DebugRestrGrantStats = &st
	defer func() { dirac.DebugRestrGrantStats = nil }()
	games, divs := 0, 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kwr-a", "kwr-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, map[string]int{})
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		kwrScan(gs, &fires)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if res.Div != nil {
			divs++
			if divs <= 3 {
				d := res.Div
				lo := max(d.At-10, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, %d divergent; restricted spends %v %v; grant search %+v", games, divs, kwrNames, fires, st)
	if st.Reads < 30 {
		t.Errorf("restricted grants read by a prefix search: %d, want at least 30", st.Reads)
	}
	if fires[kwrNotHandBad] != 0 {
		t.Errorf("%s: %d times, want 0", kwrNames[kwrNotHandBad], fires[kwrNotHandBad])
	}
}

// TestWascastrestrictMutation: with dirac.DebugRestrProvIgnored every
// provenance restriction holds, which must diverge from gorge. The mutation
// reads on the restriction path only (never a SpellCast trigger or a spell's
// condition).
func TestWascastrestrictMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, decks := kwrSetup(t)
	tbl, _, _, err := set.Matchup(kwrDecks[0], kwrDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugRestrProvIgnored
	dirac.DebugRestrProvIgnored = true
	defer func() { dirac.DebugRestrProvIgnored = old }()
	for seed := uint64(1); seed <= 64; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kwr-a", "kwr-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, eagerPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("mutation diverged: seed %d at %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("mutation did not diverge")
}

// Interdimensional Web Watch (T: Add two mana of any one color, spent only on
// a spell cast from exile): gorge's planner fails closed on a Combo Any source
// (PaymentPlanChoiceColours), so it is a manual-window source its offer walk
// treats as unpriced. The deck puts the source beside the cards cast from
// exile (Sentinel of Lost Lore's adventure, Otterball Antics' flashback).
var kwrWebDecks = [2][]cardset.Entry{
	{ // A: W/U, Web Watch over its exile casts
		{Name: "Interdimensional Web Watch", Count: 4}, {Name: "Sentinel of Lost Lore", Count: 3}, {Name: "Otterball Antics", Count: 3},
		{Name: "Savannah Lions", Count: 4}, {Name: "Healer's Hawk", Count: 4}, {Name: "Coral Merfolk", Count: 4},
		{Name: "Opt", Count: 2}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 8},
	},
	{ // B: W/U, no restricted mana
		{Name: "Savannah Lions", Count: 6}, {Name: "Healer's Hawk", Count: 5}, {Name: "Coral Merfolk", Count: 5},
		{Name: "Opt", Count: 3}, {Name: "Plains", Count: 10}, {Name: "Island", Count: 11},
	},
}

const (
	kwrExileLegal = iota // Web Watch's mana funds a spell cast from exile
	kwrExileBad          // ...anything else (must stay 0)
	kwrWebN
)

var kwrWebNames = [kwrWebN]string{"web watch mana on a spell from exile", "web watch mana elsewhere"}

// kwrWebScan is kwrScan for the exile restriction: a restricted ManaAdd whose
// valid text names wasCastFromExile funds the push that follows it, and is
// legal only on a PutOnStack from exile.
func kwrWebScan(g *GorgeSide, n *[kwrWebN]int) {
	if g == nil {
		return
	}
	var pending []string
	var cur *events.Event
	credit := func(push *events.Event, valid string) {
		if !strings.Contains(valid, "wasCastFromExile") {
			return
		}
		if push.Kind == events.PutOnStack && push.From == state.ZExile {
			n[kwrExileLegal]++
		} else {
			n[kwrExileBad]++
		}
	}
	for i := range g.E.L.Events {
		ev := &g.E.L.Events[i]
		switch ev.Kind {
		case events.DecisionMade:
			cur = nil
		case events.ManaAdd:
			if ev.Amount <= 0 {
				continue
			}
			if valid, _, _, _, ok := events.ManaRestrictionFromText(restrSourceCutPersistent(ev.Text)); ok {
				if cur != nil {
					credit(cur, valid)
				} else {
					pending = append(pending, valid)
				}
			}
		case events.ManaClear, events.TurnChange:
			pending = pending[:0]
		case events.PutOnStack, events.AbilityPush:
			cur = ev
			for _, p := range pending {
				credit(ev, p)
			}
			pending = pending[:0]
		}
	}
}

// kwrWebRun plays seeds 1..n of the Web Watch matchup and returns the games,
// the first divergences logged, the divergence count and gorge's spend counts.
func kwrWebRun(t *testing.T, n uint64, log bool) (divs int, fires [kwrWebN]int) {
	t.Helper()
	reg, _ := kcastSetup(t)
	set, err := cardset.RawByName("wascastspell")
	if err != nil {
		t.Fatal(err)
	}
	decks := kcastDeck(t, reg, kwrWebDecks)
	tbl, _, _, err := set.Matchup(kwrWebDecks[0], kwrWebDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kww-a", "kww-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, map[string]int{})
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		kwrWebScan(gs, &fires)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if res.Div != nil {
			divs++
			if log && divs <= 3 {
				d := res.Div
				lo := max(d.At-10, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return divs, fires
}

func TestWascastrestrictWebWatch(t *testing.T) {
	t.Parallel()
	divs, fires := kwrWebRun(t, 256, true)
	t.Logf("256 games, %d divergent; web watch spends %v %v", divs, kwrWebNames, fires)
	if fires[kwrExileLegal] < 10 {
		t.Errorf("%s: %d times, want at least 10", kwrWebNames[kwrExileLegal], fires[kwrExileLegal])
	}
	if fires[kwrExileBad] != 0 {
		t.Errorf("%s: %d times, want 0", kwrWebNames[kwrExileBad], fires[kwrExileBad])
	}
}

// TestWascastrestrictWebWatchMutation: with dirac.DebugWideAnyPriced the
// Web Watch source is an ordinary restricted planner unit the kernel counts
// for the offer, which must diverge from gorge (it lists a cast from exile
// only through the prefix search). The mutation reads on that source's
// classification alone, never the provenance helper or the Mm'menon grant.
func TestWascastrestrictWebWatchMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugWideAnyPriced
	dirac.DebugWideAnyPriced = true
	defer func() { dirac.DebugWideAnyPriced = old }()
	divs, _ := kwrWebRun(t, 64, false)
	if divs == 0 {
		t.Fatal("mutation did not diverge")
	}
	t.Logf("mutation diverged in %d of 64 games", divs)
}
