package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The hosted-Commander filter-property gap (ticket
// cli-20261127T093648Z-c355c4bd): nine cards kgen refused on a filter word or
// value head the grammar did not know. Kasla, the Broken Halo
// (hasKeywordConvoke), Valgavoth, Harrower of Souls (Opponent.Active),
// Joyful Stormsculptor (Battle.OppProtect) and Deluxe Dragster (OwnedBy
// TriggeredTarget) now lower and play. The other five keep their named
// lowering refusal and a residual ticket; they are not pinned here, so a
// future lowering does not fail this test (the acceptance ratchets catch a
// status change).
var hostedFilterPropsCards = []struct {
	name string
	want cardset.Status
	// wantCast: the harness can cast the card's shape (a non-convoke spell).
	// The oracle's priced priority walk does not offer a convoke cast to the
	// policy, so Kasla's and Joyful's convoke enablers play no game here; the
	// status pin is their regression.
	wantCast bool
}{
	{"Kasla, the Broken Halo", cardset.StatusUnverified, false},
	// Evidenced from clean games (cli-20261127T093705Z-1fed1794).
	{"Valgavoth, Harrower of Souls", cardset.StatusSupported, true},
	{"Joyful Stormsculptor", cardset.StatusSupported, false},
	{"Deluxe Dragster", cardset.StatusUnverified, true},
}

// hostedFilterPropsDecks pairs each lowered card with the enablers its shape
// needs: a convoke spell beside Kasla and Joyful, a battle for Joyful's
// Battle.OppProtect sweep, a burn spell that makes a NON-active opponent lose
// life beside Valgavoth, and a graveyard instant beside Deluxe Dragster.
var hostedFilterPropsDecks = map[string][2][]cardset.Entry{
	"Kasla, the Broken Halo": {
		{{Name: "Kasla, the Broken Halo", Count: 4}, {Name: "Knight-Errant of Eos", Count: 8},
			{Name: "Plains", Count: 10}, {Name: "Island", Count: 9}, {Name: "Mountain", Count: 9}},
		{{Name: "Kasla, the Broken Halo", Count: 4}, {Name: "Knight-Errant of Eos", Count: 8},
			{Name: "Plains", Count: 10}, {Name: "Island", Count: 9}, {Name: "Mountain", Count: 9}},
	},
	"Valgavoth, Harrower of Souls": {
		{{Name: "Valgavoth, Harrower of Souls", Count: 4}, {Name: "Lightning Bolt", Count: 8},
			{Name: "Sign in Blood", Count: 8}, {Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10}},
		{{Name: "Valgavoth, Harrower of Souls", Count: 4}, {Name: "Lightning Bolt", Count: 8},
			{Name: "Sign in Blood", Count: 8}, {Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10}},
	},
	"Joyful Stormsculptor": {
		{{Name: "Joyful Stormsculptor", Count: 4}, {Name: "Knight-Errant of Eos", Count: 8},
			{Name: "Invasion of Kaladesh", Count: 4}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
			{Name: "Plains", Count: 8}},
		{{Name: "Joyful Stormsculptor", Count: 4}, {Name: "Knight-Errant of Eos", Count: 8},
			{Name: "Invasion of Kaladesh", Count: 4}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
			{Name: "Plains", Count: 8}},
	},
	"Deluxe Dragster": {
		{{Name: "Deluxe Dragster", Count: 4}, {Name: "Lightning Bolt", Count: 8},
			{Name: "Grizzly Bears", Count: 8}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 10}},
		{{Name: "Deluxe Dragster", Count: 4}, {Name: "Lightning Bolt", Count: 8},
			{Name: "Grizzly Bears", Count: 8}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 10}},
	},
}

// TestHostedCmdrFilterPropsLockstep builds one custom table over the nine
// cards (and their enablers), pins each card's status, and plays seeded
// lockstep games of every lowered card against the kernel table.
func TestHostedCmdrFilterPropsLockstep(t *testing.T) {
	t.Parallel()
	// printedDelveSet reads the corpus directly (cardset.CorpusDir), so a fresh
	// clone with no .cards must skip before it gets there: this test is a
	// corpus test, never a pass or a failure without one.
	corpusDirOrSkip(t)
	var all []string
	for _, c := range hostedFilterPropsCards {
		all = append(all, c.name)
		d := hostedFilterPropsDecks[c.name]
		for s := range d {
			for _, e := range d[s] {
				all = append(all, e.Name)
			}
		}
	}
	set := printedDelveSet(t, [2][]string{all})
	for _, c := range hostedFilterPropsCards {
		i, ok := set.Lookup(c.name)
		if !ok {
			t.Fatalf("%s not in the table", c.name)
		}
		if got := set.Status(i); got != c.want {
			t.Errorf("%s: status %s, want %s (reason: %s)", c.name, got, c.want, set.Reason(i))
		}
	}

	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range hostedFilterPropsCards {
		if c.want != cardset.StatusUnverified && c.want != cardset.StatusSupported {
			continue // only a playable card has games
		}
		decks := hostedFilterPropsDecks[c.name]
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
			clean, casts, div := lockstepSeeds(t, reg, set, "hosted-filter-props", decks, gd, 1, 24, false, nil, nil, hostedFilterPropsPolicy)
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

// hostedFilterPropsPolicy plays lands, then prefers casting and activating
// (a Vehicle's crew included) and attacking, so a deck of five-drop cards
// actually reaches the shapes the lockstep is about. Every pick inside a
// class is still the seeded random pick.
func hostedFilterPropsPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var lands [2]int
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind != DPriority || c.G == nil {
			return rnd.Pick(c)
		}
		var play, cast, attack, pass []Canon
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActPlayLand:
				play = append(play, a)
			case ActCast, ActCastFace, ActActivate:
				cast = append(cast, a)
			case ActAttack:
				attack = append(attack, a)
			case ActPass:
				pass = append(pass, a)
			}
		}
		p := c.D.Player
		pick := func(opts []Canon) Canon {
			d := *c.D
			d.Options = opts
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		switch {
		case len(play) > 0 && lands[p] < 6:
			lands[p]++
			return pick(play)
		case len(cast) > 0:
			return pick(cast)
		case len(attack) > 0:
			return pick(attack)
		case len(pass) > 0:
			return pick(pass)
		}
		return rnd.Pick(c)
	})
}
