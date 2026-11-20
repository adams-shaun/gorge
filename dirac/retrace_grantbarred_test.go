package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRetraceGrantBarredAdditionalCosts: Six's granted retrace is priced as
// the printed base plus the discard alone (buildPickCost's retrace branch
// returns the retrace cost, not the spell's own additional parts). A printed
// Retrace is safe because kgen's lowerRetrace refuses it beside another cast
// route or additional cost; a granted one lands on any nonland permanent card
// in the graveyard, so retraceGrantBarred must withhold the offer (stall
// "retracegrant") for each of those, or the kernel drops a cost gorge still
// announces and pays.
func TestRetraceGrantBarredAdditionalCosts(t *testing.T) {
	deck := []cardset.Entry{{Name: "Six", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Forest", Count: 10}}
	base, da, _, err := cardsettest.Set(t, "offzonegrants").Matchup(deck, deck)
	if err != nil {
		t.Skip(err)
	}
	disc := kop.Cost{Flags: kop.CostDiscard, N: 1}
	cases := []struct {
		name string
		mut  func(cd *kop.CardDef)
	}{
		{"none", func(*kop.CardDef) {}},
		// (kicker is not here: a kicked cast is a hand-only offer, so a Kicker
		// card's retrace cast is the plain one in both engines --
		// TestRetraceGrantKickerNotBarred; nor is warp, a hand-only permission:
		// TestRetraceGrantWarpNotBarred)
		{"plot", func(cd *kop.CardDef) { cd.HasPlot = true }},
		{"sneak", func(cd *kop.CardDef) { cd.HasSneak = true }},
		{"impending", func(cd *kop.CardDef) { cd.Impending = 4 }},
		{"bestow", func(cd *kop.CardDef) { cd.HasBestow = true }},
		{"spellcost", func(cd *kop.CardDef) { cd.SpellCost = disc }},
		{"altadd", func(cd *kop.CardDef) { cd.HasAltAdd = true }},
		{"offspring", func(cd *kop.CardDef) { cd.Offspring = true }},
		{"gift", func(cd *kop.CardDef) { cd.Gift = true }},
		// (morph is not here either: the face-down morph cast is a hand-and-
		// command-zone offer, so a Morph card's retrace cast is the plain
		// face-up one in both engines -- TestRetraceGrantMorphNotBarred)
		{"casualty", func(cd *kop.CardDef) { cd.Casualty = 1 }},
		{"bargain", func(cd *kop.CardDef) { cd.Bargain = true }},
		{"optcost", func(cd *kop.CardDef) { cd.OptCost = disc }},
		{"teamwork", func(cd *kop.CardDef) { cd.Teamwork = 1 }},
		{"spree", func(cd *kop.CardDef) { cd.HasModeCost = true }},
		{"madness", func(cd *kop.CardDef) { cd.HasMadness = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tbl := *base // the card is mutated on a private copy of the table
			tbl.Cards = slices.Clone(base.Cards)
			id := map[string]CardID{}
			for _, c := range da {
				id[tbl.Cards[c].Name] = c
			}
			giant := id["Hill Giant"]
			tc.mut(&tbl.Cards[giant])
			sc := newScenTableLib(t, &tbl, id["Forest"], Config{NoElide: true})
			sc.perm(id["Six"], 0)
			sc.s.refreshTy(sc.k)
			grave := sc.grave(giant, 0, 1)[0]
			if !sc.s.retraceLive(sc.k, grave) {
				t.Fatalf("precondition: Six must grant the graveyard Hill Giant retrace")
			}
			wantBarred := tc.name != "none"
			if got := retraceGrantBarred(sc.k, giant); got != wantBarred {
				t.Fatalf("retraceGrantBarred = %v, want %v", got, wantBarred)
			}
			var d Decision
			var av avail
			sc.s.offerRetraceCasts(sc.k, &d, 0, true, &av)
			for i := 0; i < int(d.N); i++ {
				if wantBarred && d.Options[i].Kind() == AkCast {
					t.Fatalf("a retrace cast of a card with %s was offered: %v", tc.name, d.Options[i])
				}
			}
			if got := sc.s.StallReason() == StallRetraceGrant; got != wantBarred {
				t.Fatalf("stall %q: retracegrant stall = %v, want %v", StallName(sc.s.StallReason()), got, wantBarred)
			}
		})
	}
}

// TestRetraceGrantMorphNotBarred: the face-down morph cast is a hand-and-
// command-zone offer (face_facedown.go's offerFaceDownHand; the graveyard
// retrace walk has no face-down arm, and gorge's is hand-only too), so a
// Morph card's retrace cast from the graveyard is the plain face-up one in
// both engines: the printed base plus the land discard, never face down
// (morph is not an additional cost on the face-up cast). Six's granted
// retrace on a morph card (Shieldhide Dragon's shape) is therefore offered,
// not withheld as a retracegrant stall.
func TestRetraceGrantMorphNotBarred(t *testing.T) {
	deck := []cardset.Entry{{Name: "Six", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Forest", Count: 10}}
	base, da, _, err := cardsettest.Set(t, "offzonegrants").Matchup(deck, deck)
	if err != nil {
		t.Skip(err)
	}
	tbl := *base // the card is mutated on a private copy of the table
	tbl.Cards = slices.Clone(base.Cards)
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	giant := id["Hill Giant"]
	tbl.Cards[giant].Cost = kop.ManaCost{Generic: 3, Pips: [kop.NumManaKinds]uint8{kop.G: 1}} // green: the fixture's only source colour
	tbl.Cards[giant].MorphCost = kop.ManaCost{Generic: 3}                                     // Shieldhide Dragon's shape: a face-down morph cost
	sc := newScenTableLib(t, &tbl, id["Forest"], Config{NoElide: true})
	sc.perm(id["Six"], 0)
	for i := 0; i < 4; i++ {
		sc.perm(id["Forest"], 0) // the printed {3}{G} (Hill Giant)
	}
	sc.hand(id["Forest"], 0) // the retrace cast's discardable land
	sc.s.refreshTy(sc.k)
	grave := sc.grave(giant, 0, 1)[0]
	if !sc.s.retraceLive(sc.k, grave) {
		t.Fatalf("precondition: Six must grant the graveyard Hill Giant retrace")
	}
	if retraceGrantBarred(sc.k, giant) {
		t.Fatal("retraceGrantBarred: a morph card's granted retrace is withheld")
	}
	var d Decision
	var av avail
	sc.s.offerRetraceCasts(sc.k, &d, 0, true, &av)
	if r := sc.s.StallReason(); r == StallRetraceGrant {
		t.Fatalf("stall %q on a morph card's granted retrace", StallName(r))
	}
	var casts, faceDown int
	found := false
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != AkCast {
			continue
		}
		casts++
		if a.Mode()&castFaceDown != 0 {
			faceDown++
			continue
		}
		if a.Source() == TCode(grave) && a.Target() == TCode(retraceTgt) && a.Mode() == 2 {
			found = true
		}
	}
	if casts != 1 || !found {
		t.Fatalf("want exactly one plain retrace cast of the morph card, got %d casts (found=%v, %d options)", casts, found, d.N)
	}
	if faceDown != 0 {
		t.Fatalf("a face-down cast of the graveyard morph card was offered (%d options)", d.N)
	}
}
