package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The residual five of the hosted-Commander filter-property gap (parent
// ticket cli-20261127T093648Z-c355c4bd, residual ticket
// cli-20261127T093648Z-c355c4bd-residual): Kaust, Eyes of the Glade
// (turnedFaceUpThisTurn), Cut Short (Planeswalker.activated,Creature.tapped),
// Emrakul, the World Anew (GainControl AllValid$ beside ValidTgts$), Sink into
// Stupor (Permanent.nonLand+OppCtrl,Card.inZoneStack+OppCtrl) and the two
// TargetedByTarget$Valid carriers: Not of This World (Count$Compare CheckTgt
// GE1.7.0 gating its ReduceCost amount) and Bane's Contingency (the same head
// gating a Counter's scry-and-draw rider). All six lower (StatusUnverified): the
// nested target count is kop.ExCountTgtEntry, and TestTargetedByTargetLockstepMutation
// holds the two carriers to the read. The four cards the parent ticket lowered
// are pinned in hosted_cmdr_filter_props_lockstep_test.go, not here.
var hostedFilterPropsResidualCards = []struct {
	name string
	want cardset.Status
	// wantReasonSub: for a refused card, the refusal must name this shape.
	// Ignored when the card lowered.
	wantReasonSub string
	// wantCast: the harness can cast the card's shape in the pinned decks.
	wantCast bool
}{
	{"Kaust, Eyes of the Glade", cardset.StatusUnverified, "", true},
	// Cut Short's convoke enablers play no game here: like the parent
	// ticket's Kasla and Joyful, the oracle's priced priority walk does not
	// offer a convoke cast to the policy, so the status pin is the regression.
	{"Cut Short", cardset.StatusUnverified, "", false},
	{"Emrakul, the World Anew", cardset.StatusUnverified, "", false},
	// Sink into Stupor needs a permanent or spell the OTHER seat controls to
	// have a legal target, so its cast is seed-dependent; the status pin and
	// the clean lockstep games are the regression.
	{"Sink into Stupor", cardset.StatusUnverified, "", false},
	// Not of This World's reduction needs a 7-power creature the opponent's
	// Lightning Bolt takes as its target, so the deck holds Vorstclaw; the
	// read's liveness is TestTargetedByTargetLockstepMutation's, not this row's.
	{"Not of This World", cardset.StatusUnverified, "", false},
	// Bane's Contingency's scry-and-draw rider reads the same head over the
	// commander the countered spell targets (Card.IsCommander, which a duel's
	// seats never carry: the clean games, not a fired rider, are the pin).
	{"Bane's Contingency", cardset.StatusUnverified, "", false},
}

// hostedFilterPropsResidualDecks pairs each card with the enablers its shape
// needs: morph creatures beside Kaust so something arrives face down for his
// {T} turn-face-up, untapped creatures beside Cut Short so convoke has
// payers, a mass of lands beside Emrakul (its 12-mana cast is not required),
// an opponent-controlled permanent and stack spell beside Sink, and beside the
// two nested-target carriers a spell that targets a permanent you control:
// Vorstclaw (a 7/7, so Not of This World's Card.powerGE7+YouCtrl spec can hold)
// and Grizzled Wolverine (a legendary creature for Bane's Contingency's
// commander read).
var hostedFilterPropsResidualDecks = map[string][2][]cardset.Entry{
	"Kaust, Eyes of the Glade": {
		{{Name: "Kaust, Eyes of the Glade", Count: 4}, {Name: "Ivory Giant", Count: 8},
			{Name: "Raging Gorilla", Count: 8}, {Name: "Plains", Count: 7},
			{Name: "Mountain", Count: 7}, {Name: "Forest", Count: 6}},
		{{Name: "Kaust, Eyes of the Glade", Count: 4}, {Name: "Ivory Giant", Count: 8},
			{Name: "Raging Gorilla", Count: 8}, {Name: "Plains", Count: 7},
			{Name: "Mountain", Count: 7}, {Name: "Forest", Count: 6}},
	},
	"Cut Short": {
		{{Name: "Cut Short", Count: 8}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Plains", Count: 12}, {Name: "Island", Count: 12}},
		{{Name: "Cut Short", Count: 8}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Plains", Count: 12}, {Name: "Island", Count: 12}},
	},
	"Emrakul, the World Anew": {
		{{Name: "Emrakul, the World Anew", Count: 4}, {Name: "Plains", Count: 12},
			{Name: "Island", Count: 12}, {Name: "Mountain", Count: 12}},
		{{Name: "Emrakul, the World Anew", Count: 4}, {Name: "Plains", Count: 12},
			{Name: "Island", Count: 12}, {Name: "Mountain", Count: 12}},
	},
	"Sink into Stupor": {
		{{Name: "Sink into Stupor", Count: 8}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Island", Count: 24}},
		{{Name: "Sink into Stupor", Count: 8}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Island", Count: 24}},
	},
	// Vorstclaw rather than a 2/2: the reduction's spec is Card.powerGE7, so a
	// Bear's target never holds and the read would never move a price.
	"Not of This World": {
		{{Name: "Not of This World", Count: 8}, {Name: "Lightning Bolt", Count: 8},
			{Name: "Vorstclaw", Count: 8}, {Name: "Forest", Count: 6},
			{Name: "Island", Count: 5}, {Name: "Mountain", Count: 5}},
		{{Name: "Not of This World", Count: 8}, {Name: "Lightning Bolt", Count: 8},
			{Name: "Vorstclaw", Count: 8}, {Name: "Forest", Count: 6},
			{Name: "Island", Count: 5}, {Name: "Mountain", Count: 5}},
	},
	// Bane's Contingency is {U}{U}, so the legendary creature beside it is the
	// red Grizzled Wolverine: the opponent's Lightning Bolt can aim at it, and
	// the countered spell then targets a permanent its caster does not control.
	"Bane's Contingency": {
		{{Name: "Bane's Contingency", Count: 8}, {Name: "Lightning Bolt", Count: 8},
			{Name: "Grizzled Wolverine", Count: 8}, {Name: "Island", Count: 8},
			{Name: "Mountain", Count: 8}},
		{{Name: "Bane's Contingency", Count: 8}, {Name: "Lightning Bolt", Count: 8},
			{Name: "Grizzled Wolverine", Count: 8}, {Name: "Island", Count: 8},
			{Name: "Mountain", Count: 8}},
	},
}

// TestHostedCmdrFilterPropsResidual builds one custom table over the residual
// five and their enablers, pins each card's status (lowered, or refused by
// name with the shape named), and plays seeded lockstep games of every
// lowered card against the kernel table.
func TestHostedCmdrFilterPropsResidual(t *testing.T) {
	t.Parallel()
	// printedDelveSet reads the corpus directly (cardset.CorpusDir), so a fresh
	// clone with no .cards must skip before it gets there: this test is a
	// corpus test, never a pass or a failure without one.
	corpusDirOrSkip(t)
	var all []string
	for _, c := range hostedFilterPropsResidualCards {
		all = append(all, c.name)
		d := hostedFilterPropsResidualDecks[c.name]
		for s := range d {
			for _, e := range d[s] {
				all = append(all, e.Name)
			}
		}
	}
	set := printedDelveSet(t, [2][]string{all})
	for _, c := range hostedFilterPropsResidualCards {
		i, ok := set.Lookup(c.name)
		if !ok {
			t.Fatalf("%s not in the table", c.name)
		}
		got := set.Status(i)
		if got != c.want {
			t.Errorf("%s: status %s, want %s (reason: %s)", c.name, got, c.want, set.Reason(i))
			continue
		}
		if c.wantReasonSub != "" && !strings.Contains(set.Reason(i), c.wantReasonSub) {
			t.Errorf("%s: refusal %q does not name the shape %q", c.name, set.Reason(i), c.wantReasonSub)
		}
	}

	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range hostedFilterPropsResidualCards {
		if c.want != cardset.StatusUnverified {
			continue
		}
		decks := hostedFilterPropsResidualDecks[c.name]
		var gd [2][]*cards.Card
		for s := range decks {
			for _, e := range decks[s] {
				card, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					gd[s] = append(gd[s], card)
				}
			}
			if len(gd[s]) != 40 {
				t.Fatalf("%s deck %d: %d cards", c.name, s, len(gd[s]))
			}
		}
		t.Run(c.name, func(t *testing.T) {
			clean, casts, div := lockstepSeeds(t, reg, set, "hosted-filter-props-residual", decks, gd, 1, 24, false, nil, nil, hostedFilterPropsPolicy)
			if div != 0 {
				t.Fatalf("%s: %d of %d seeds diverged", c.name, div, 24)
			}
			if c.wantCast && casts == 0 {
				t.Fatalf("%s: no spell was cast in %d clean games", c.name, clean)
			}
			t.Logf("%s: %d clean games, %d casts/activations", c.name, clean, casts)
		})
	}
}
