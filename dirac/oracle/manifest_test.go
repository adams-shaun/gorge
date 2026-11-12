package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The kmanifest lockstep: plain Manifest, the DSK ManifestDread cards that
// lower, and the turn-face-up procedures a manifested card offers (CR
// 701.40b-d). The decks are mostly creatures so manifested cards can be
// turned up for their mana cost, and Zoetic Cavern is the land with Morph
// that proves the keyword alternative is offered when the mana-cost
// procedure is withheld (CR 701.40c-d). Ornithopter is a {0}-cost creature
// and Dryad Arbor prints "no cost"; both are offered (gorge rejects only an
// EMPTY printed cost and prices "no cost" as free); Break Down the Door and
// Twist Reality nest ManifestDread in a Charm; Killer's Mask is a
// RememberManifested$ dread rider; Mastery of the Unseen and Temur War
// Shaman carry TurnFaceUp triggers. There is no Tapped$/Shuffle$ Cloak
// carrier in the corpus that lowers (Become Anonymous and Expose the Culprit
// are refused), so that rider stays lowering-tested only
// (TestCloakWardAndTurnUpOffers).
var kmanifestDecks = [][2][]cardset.Entry{{
	{
		{Name: "Manifest Dread", Count: 4}, {Name: "Soul Summons", Count: 4},
		{Name: "Cryptic Coat", Count: 4}, {Name: "Defiant Survivor", Count: 4},
		{Name: "Unsettling Twins", Count: 2}, {Name: "Innocuous Rat", Count: 2},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Mind Stone", Count: 1},
		{Name: "Mox Emerald", Count: 2},
		{Name: "Mox Pearl", Count: 1}, {Name: "Mox Jet", Count: 1},
		{Name: "Mox Ruby", Count: 1}, {Name: "Mox Sapphire", Count: 1},
		{Name: "Priest of Urabrask", Count: 3}, {Name: "Zoetic Cavern", Count: 2},
		{Name: "Ornithopter", Count: 2}, {Name: "Dryad Arbor", Count: 1},
		{Name: "Break Down the Door", Count: 2}, {Name: "Mastery of the Unseen", Count: 1},
		{Name: "Killer's Mask", Count: 1},
	},
	{
		{Name: "Unnerving Grasp", Count: 4}, {Name: "Reality Shift", Count: 4},
		{Name: "Cryptic Coat", Count: 4}, {Name: "Innocuous Rat", Count: 2},
		{Name: "Unsettling Twins", Count: 3}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Mind Stone", Count: 2}, {Name: "Mox Sapphire", Count: 2}, {Name: "Mox Jet", Count: 2},
		{Name: "Mox Emerald", Count: 2}, {Name: "Mox Pearl", Count: 1},
		{Name: "Mox Ruby", Count: 1}, {Name: "Priest of Gix", Count: 4},
		{Name: "Zoetic Cavern", Count: 2},
		{Name: "Twist Reality", Count: 2}, {Name: "Temur War Shaman", Count: 1},
		{Name: "Ornithopter", Count: 2},
	},
}}

type kmanifestCounts struct {
	entries, turnUp, manaUp, keywordUp int
	cloak                              int
}

func kmanifestMox(n string) bool {
	switch n {
	case "Mox Emerald", "Mox Sapphire", "Mox Jet", "Mox Ruby", "Mox Pearl":
		return true
	}
	return false
}

// kmanifestDown counts the gorge battlefield's face-down permanents.
func kmanifestDown(c *PickCtx) int {
	if c.G == nil {
		return 0
	}
	n := 0
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range c.G.E.G.Zone(state.ZBattlefield, p) {
			if o := c.G.E.G.Obj(id); o != nil && o.FaceDown {
				n++
			}
		}
	}
	return n
}

// kmanifestCloaked counts the gorge battlefield's cloaked permanents.
func kmanifestCloaked(c *PickCtx) int {
	if c.G == nil {
		return 0
	}
	n := 0
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range c.G.E.G.Zone(state.ZBattlefield, p) {
			if o := c.G.E.G.Obj(id); o != nil && o.Cloaked {
				n++
			}
		}
	}
	return n
}

// kmanifestPolicy prefers a face-down turn-up whenever offered (so the
// procedures actually run) and otherwise random. Each policy call adds the
// rise in face-down and cloaked permanents to the manifest-entry and cloak
// counts (kept per game: the policy is rebuilt each seed); the actions
// classify the turn-up procedure from the ActSpecial mode's high nibble.
func kmanifestPolicy(seed uint64, counts *kmanifestCounts) Policy {
	rnd := RandomPolicy(seed)
	var up, mox, priest, manifest, coat Canon
	haveUp, haveMox, havePriest, haveManifest, haveCoat := false, false, false, false, false
	down, cloaked, priests, coats := 0, 0, 0, 0
	return PolicyFunc(func(c *PickCtx) Canon {
		if n := kmanifestDown(c); n > down {
			counts.entries += n - down
		}
		down = kmanifestDown(c)
		if n := kmanifestCloaked(c); n > cloaked {
			counts.cloak += n - cloaked
		}
		cloaked = kmanifestCloaked(c)
		haveUp, haveMox, havePriest, haveManifest, haveCoat = false, false, false, false, false
		for _, a := range c.D.Options {
			n := optionFaceName(c, a)
			switch {
			case a.Kind() == ActSpecial:
				up, haveUp = a, true

			case a.Kind() == ActCast && kmanifestMox(n):
				mox, haveMox = a, true
			case a.Kind() == ActCast && (n == "Priest of Urabrask" || n == "Priest of Gix"):
				priest, havePriest = a, true
			case a.Kind() == ActCast && n == "Cryptic Coat":
				coat, haveCoat = a, true
			case a.Kind() == ActCast && (n == "Manifest Dread" || n == "Soul Summons" || n == "Reality Shift" || n == "Unnerving Grasp"):
				manifest, haveManifest = a, true
			}
		}
		switch {
		case haveMox:
			return mox
		case havePriest && priests < 4:
			priests++
			return priest
		case haveCoat && coats < 4:
			coats++
			return coat
		case haveManifest:
			return manifest
		}
		if haveUp {
			proc := up.Mode() >> 4
			counts.turnUp++
			switch proc {
			case 1, 2:
				counts.manaUp++
			case 3, 4, 5:
				counts.keywordUp++
			}
			return up
		}
		return rnd.Pick(c)
	})
}

func TestManifestLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("manifest")
	if err != nil {
		t.Fatal(err)
	}
	var counts kmanifestCounts
	games := 0
	for mi, md := range kmanifestDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 1024; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"manifest-a", "manifest-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := kmanifestPolicy(seed, &counts)
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, manifest entries %d, cloak entries %d, turn-ups %d (mana %d, keyword %d)",
		games, counts.entries, counts.cloak, counts.turnUp, counts.manaUp, counts.keywordUp)
	if counts.entries < 50 {
		t.Errorf("only %d manifest entries", counts.entries)
	}
	if counts.manaUp < 10 {
		t.Errorf("only %d mana-cost turn-ups", counts.manaUp)
	}
	if counts.keywordUp < 1 {
		t.Error("no keyword-alternative turn-up")
	}
	if counts.cloak < 10 {
		t.Errorf("only %d cloak entries", counts.cloak)
	}
}

// TestManifestMutation: with the noncreature mana-cost turn-up switched on
// (dirac.DebugTurnUpNoncreature) the same games diverge from gorge, which
// never offers it (CR 701.40b/701.58b: a CREATURE CARD only). It proves the
// lockstep sees the withholding the mutation removes.
func TestManifestMutation(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("manifest")
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugTurnUpNoncreature = true
	defer func() { dirac.DebugTurnUpNoncreature = false }()
	md := kmanifestDecks[0]
	var decks [2][]*cards.Card
	for s, d := range md {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 128; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"manifest-a", "manifest-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var counts kmanifestCounts
		res := Lockstep(m, seed, k, kmanifestPolicy(seed, &counts), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d: mutation diverges at decision %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("offering the noncreature mana-cost turn-up diverged in no game: the lockstep does not see the withholding")
}
