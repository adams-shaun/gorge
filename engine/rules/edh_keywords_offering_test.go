package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Patron of the Nezumi is the real corpus Offering carrier this file plays:
// printed {5}{B}{B}, K:Offering:Rat (CR 702.46). Blast-Furnace Hellkite
// (K:Offering:Artifact, the pre-2007 wording) is asserted alongside it. The
// sacrifice fuels are freely authored fixtures (never corpus .txt, per the
// licensing rule): a Rat whose {1}{B} cost reduces the Patron colour-
// inclusively to {4}{B} (its black pip takes a printed black pip first, its
// generic spills onto the {5}), and a Myr whose {2} cost reduces the Hellkite
// to {5}{R}{R}.
const (
	offeringRatSrc = "Name:Offering Rat\nManaCost:1 B\nTypes:Creature Rat\nPT:1/1\nOracle:x\n"
	offeringMyrSrc = "Name:Offering Myr\nManaCost:2\nTypes:Artifact Creature Myr\nPT:2/1\nOracle:x\n"
	// offeringGruntSrc is the keyword-LESS control: a plain creature that
	// must never be castable off turn, so the offers asserted below are the
	// keywords', not the walk's baseline.
	offeringGruntSrc = "Name:Offering Grunt\nManaCost:2 R\nTypes:Creature Orc\nPT:2/2\nOracle:x\n"
)

// castOfferingSacrificing submits the "offering" cast option for id, answers
// the mandatory sacrifice ask with sacObj, and drains the cast (the 601.2g
// mana window and priority) until the spell has resolved onto the battlefield.
// It fails on any decision shape it does not recognise, so a vacuous setup
// cannot slip through.
func castOfferingSacrificing(t *testing.T, e *Engine, id, sacObj state.ObjID) {
	t.Helper()
	submitChoices(t, e, castModeOption(t, e, id, "offering"))
	for i := 0; i < 80; i++ {
		if o := e.G.Obj(id); o != nil && o.Zone == state.ZBattlefield && len(e.G.Stack) == 0 {
			return
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("offering cast stalled with no pending decision (stack depth %d)", len(e.G.Stack))
		}
		switch d.Kind {
		case decision.KChoose:
			idx := -1
			// The mandatory sacrifice ask: pick exactly the permanent under
			// test.
			for _, o := range d.Options {
				if o.Kind == "sacrifice" && o.Obj == sacObj {
					idx = o.Index
				}
			}
			// The 601.2g mana window: the pool already covers the reduced
			// cost, so take "done".
			if idx < 0 {
				for _, o := range d.Options {
					if o.Kind == "done" {
						idx = o.Index
					}
				}
			}
			if idx < 0 {
				t.Fatalf("unexpected KChoose during offering cast: %+v", d)
			}
			submitChoices(t, e, idx)
		case decision.KTarget:
			// Patron of the Nezumi's cast trigger ("whenever a permanent is
			// put into an opponent's graveyard") has no targets; a target ask
			// here is not part of the flow under test.
			t.Fatalf("unexpected target ask during offering cast: %+v", d)
		case decision.KPriority:
			passOnceP(t, e)
		default:
			t.Fatalf("unexpected decision during offering cast: %+v", d)
		}
	}
	t.Fatal("offering cast never resolved")
}

// TestEDHKeywordsOfferingCastReducesBySacrificedManaCost plays Patron of the
// Nezumi's printed Rat offering (CR 702.46) through a real cast: the
// sacrificed Rat's mana cost — colour-inclusive — reduces the printed
// {5}{B}{B} to {4}{B}, the Rat is sacrificed, and the pool's REMAINING
// COMPOSITION (1B 1C out of a 2B 5C funding) proves both pips were priced.
func TestEDHKeywordsOfferingCastReducesBySacrificedManaCost(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 4242, []string{"Patron of the Nezumi"}, []string{offeringRatSrc}, nil)
	// The primitive must be registered: without it the coverage census
	// rejects every Offering card as unsupported, so reverting the
	// registration alone fails this test.
	if !effects.Supported()["kw:Offering"] {
		t.Fatal(`effects.Supported() is missing "kw:Offering"`)
	}
	patron := findCardObj(t, e, 0, "Patron of the Nezumi", state.ZHand)
	rat := findCardObj(t, e, 0, "Offering Rat", state.ZBattlefield)

	// Preconditions: the spell is in hand, the sacrifice is a controlled
	// battlefield Rat, and its printed cost is the colour-inclusive reduction
	// under test.
	pf := e.G.Obj(patron)
	if pf == nil || pf.Zone != state.ZHand || pf.Controller != 0 {
		t.Fatalf("Patron of the Nezumi not in seat 0's hand: %+v", pf)
	}
	ro := e.G.Obj(rat)
	if ro == nil || ro.Zone != state.ZBattlefield || ro.Controller != 0 {
		t.Fatalf("Offering Rat not a controlled battlefield creature: %+v", ro)
	}
	if rc := ParseCost(ro.Face().ManaCost); rc.Generic != 1 || rc.Colored[state.MB] != 1 {
		t.Fatalf("Offering Rat cost %+v, want {1}{B}", rc)
	}
	// Sanity on the composition the offering arm charges: the printed mana
	// cost plus the mandatory Rat sacrifice.
	if bc := pay.RawBaseCost(asPayer(e), 0, patron); bc.Generic != 5 || bc.Colored[state.MB] != 2 {
		t.Fatalf("Patron printed cost %+v, want {5}{B}{B}", bc)
	}

	// Fund the pool with the full printed {5}{B}{B} plus one spare of each
	// component: 3 black + 6 generic. If the Rat's {1}{B} reduction is
	// applied colour-inclusively, the cast pays {4}{B} (one black pip + four
	// generic) and 2B 2C remain.
	addMana(t, e, 0, "BBCCCCCBC")
	pool := e.G.Players[0].Pool
	if pool[state.MB] != 3 || pool[state.MC] != 6 || pool.Total() != 9 {
		t.Fatalf("pool funded %+v, want 3B 6C", pool)
	}

	castOfferingSacrificing(t, e, patron, rat)

	if o := e.G.Obj(rat); o.Zone != state.ZGraveyard {
		t.Fatalf("sacrificed Rat in %s, want graveyard", o.Zone)
	}
	// The remaining pool's composition, not just its total: the reduced
	// {4}{B} must actually have taken one black pip (the Rat's coloured
	// reduction) and four generic.
	pool = e.G.Players[0].Pool
	if pool[state.MB] != 2 || pool[state.MC] != 2 || pool.Total() != 4 {
		t.Fatalf("mana remaining %+v, want 2B 2C ({5}{B}{B} reduced by the Rat's {1}{B})", pool)
	}
	sacrificed := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.Obj == rat && ev.From == state.ZBattlefield &&
			ev.To == state.ZGraveyard && ev.Text == "sacrificed" {
			sacrificed = true
		}
	}
	if !sacrificed {
		t.Fatal("no sacrifice MoveZone for the offering cost in the log")
	}
	replayCheck(t, e, cfg)
}

// TestEDHKeywordsOfferingInstantTimingAndTypeFilter proves the Offering
// permission ("any time you could cast an instant") and its type filter:
// off turn, Blast-Furnace Hellkite (K:Offering:Artifact) and Patron of the
// Nezumi (K:Offering:Rat) are each offered exactly one cast option — the
// "offering" one — with a permanent of the named type to sacrifice, and
// NOTHING when there is none (Blast-Furnace Hellkite with no artifact,
// Patron with no Rat).
func TestEDHKeywordsOfferingInstantTimingAndTypeFilter(t *testing.T) {
	t.Parallel()
	e, _, _ := altCostEngine(t, 77, []string{"Blast-Furnace Hellkite", "Patron of the Nezumi"},
		[]string{offeringMyrSrc, offeringRatSrc, offeringGruntSrc}, nil)
	hk := findCardObj(t, e, 0, "Blast-Furnace Hellkite", state.ZHand)
	pt := findCardObj(t, e, 0, "Patron of the Nezumi", state.ZHand)
	// The fuels are moved onto the battlefield (findCardObj relocates a
	// seeded card wherever genesis shuffled it).
	findCardObj(t, e, 0, "Offering Myr", state.ZBattlefield)
	findCardObj(t, e, 0, "Offering Rat", state.ZBattlefield)
	// Sanity control, seeded on the battlefield and moved to the hand.
	grunt := findCardObj(t, e, 0, "Offering Grunt", state.ZHand)
	addMana(t, e, 0, "RRRRRRRRRBB")
	e.G.Active = 1
	offs := offTurnCastModes(t, e, 0)
	if _, ok := offs[grunt]; ok {
		t.Fatal("the keyword-less creature must not be castable off turn")
	}
	if modes := offs[hk]; len(modes) != 1 || modes[0] != "offering" {
		t.Fatalf("Blast-Furnace Hellkite off-turn cast modes = %v, want exactly [offering]", modes)
	}
	if modes := offs[pt]; len(modes) != 1 || modes[0] != "offering" {
		t.Fatalf("Patron of the Nezumi off-turn cast modes = %v, want exactly [offering]", modes)
	}

	// Without a permanent of the named type the offer is withheld entirely.
	e2, _, _ := altCostEngine(t, 78, []string{"Blast-Furnace Hellkite", "Patron of the Nezumi"},
		[]string{offeringMyrSrc}, nil)
	hk2 := findCardObj(t, e2, 0, "Blast-Furnace Hellkite", state.ZHand)
	pt2 := findCardObj(t, e2, 0, "Patron of the Nezumi", state.ZHand)
	// Only the Myr is moved: the Rat stays buried in the deck, so the
	// Patron's Rat offer must be withheld.
	findCardObj(t, e2, 0, "Offering Myr", state.ZBattlefield)
	addMana(t, e2, 0, "RRRRRRRRRBB")
	e2.G.Active = 1
	offs2 := offTurnCastModes(t, e2, 0)
	if modes := offs2[hk2]; len(modes) != 1 || modes[0] != "offering" {
		t.Fatalf("Blast-Furnace Hellkite with a Myr present = %v, want exactly [offering]", modes)
	}
	if modes := offs2[pt2]; len(modes) != 0 {
		t.Fatalf("Patron of the Nezumi with no Rat present = %v, want no cast offer", modes)
	}
}

// offTurnCastModes maps each hand object id to the cast modes seat p is
// offered for it at instant timing (the caller has already parked the walk on
// an off-turn priority point).
func offTurnCastModes(t *testing.T, e *Engine, p state.PlayerID) map[state.ObjID][]string {
	t.Helper()
	modes := map[state.ObjID][]string{}
	for _, id := range e.G.Zone(state.ZHand, p) {
		for _, o := range e.legalActions(p) {
			if o.Kind == "cast" && o.Obj == id {
				modes[id] = append(modes[id], o.Mode)
			}
		}
	}
	return modes
}

// The corpus carrier assertion (the coverage census is cmd/forgec's
// TestEDHKeywordsSupported): the Patron script really carries the keyword
// this file plays, so a corpus rename cannot silently vacate the test.
func TestEDHKeywordsOfferingCarriersPrinted(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	patron, ok := reg.Lookup("Patron of the Nezumi")
	if !ok {
		t.Fatal("Patron of the Nezumi not in the corpus")
	}
	if got, ok := patron.Faces[0].KeywordParam("Offering"); !ok || got != "Rat" {
		t.Fatalf("Patron Offering param = %q (%v), want Rat", got, ok)
	}
	hk, ok := reg.Lookup("Blast-Furnace Hellkite")
	if !ok {
		t.Fatal("Blast-Furnace Hellkite not in the corpus")
	}
	if got, ok := hk.Faces[0].KeywordParam("Offering"); !ok || got != "Artifact" {
		t.Fatalf("Blast-Furnace Hellkite Offering param = %q (%v), want Artifact", got, ok)
	}
}
