package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Surveil-X lockstep (S1): Cerebral Download's Surveil X
// grows with the controller's artifact count. Above eight the kernel's
// res.peek window is full, so the kernel stalls "tables" (gorge asks the full
// N); at eight or less both sides pose the same ask. The pool's only
// Surveil/Scry source is Cerebral Download, so every KArrange decision here
// is its ask and Max is X.
var ksilentcapsArtifacts = map[string]bool{
	"Ornithopter": true, "Memnite": true, "Phyrexian Walker": true,
	"Darksteel Relic": true, "Spellbook": true,
}

var ksilentcapsDeck = []cardset.Entry{
	{Name: "Cerebral Download", Count: 4},
	{Name: "Ornithopter", Count: 4},
	{Name: "Memnite", Count: 4},
	{Name: "Phyrexian Walker", Count: 4},
	{Name: "Darksteel Relic", Count: 4},
	{Name: "Spellbook", Count: 4},
	{Name: "Island", Count: 16},
}

type ksilentcapsStats struct {
	lowAsks       int  // KArrange asks with Max <= 8
	highAsks      int  // KArrange asks with Max >= 9 (gorge asks after the kernel stalls)
	highCast      bool // the policy cast Cerebral Download with X >= 9 and a library of >= 9
	lowGames      int
	tableGames    int // games the kernel ended in a tables stall
	highCastGames int
}

func ksilentcapsPolicy(seed uint64, st *ksilentcapsStats) Policy {
	rnd := RandomPolicy(seed)
	var lastArrange uint64
	var hasLast bool
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			if d := c.G.Pending(); d != nil && d.Kind == decision.KArrange {
				if !hasLast || d.Seq != lastArrange {
					hasLast, lastArrange = true, d.Seq
					if d.Max <= 8 {
						st.lowAsks++
					} else {
						st.highAsks++
					}
				}
			}
		}
		var land, artifact, cereb, attack Canon
		haveLand, haveArtifact, haveCereb, haveAttack := false, false, false, false
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActPlayLand:
				land, haveLand = a, true
			case ActCast:
				name := ""
				if c.G != nil {
					if o, ok := c.G.GorgeOption(a); ok {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
							name = ob.Face().Name
						}
					}
				}
				switch {
				case name == "Cerebral Download":
					cereb, haveCereb = a, true
				case ksilentcapsArtifacts[name]:
					artifact, haveArtifact = a, true
				}
			case ActAttack:
				attack, haveAttack = a, true
			}
		}
		switch {
		case haveLand:
			return land
		case haveArtifact:
			return artifact
		case haveCereb && (seed%2 == 0 || ksilentcapsArtifactCount(c) >= 9):
			if ksilentcapsArtifactCount(c) >= 9 && ksilentcapsLibrarySize(c) >= 9 {
				st.highCast = true
			}
			return cereb
		case haveAttack:
			return attack
		default:
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActPass, ActDone, ActKeep:
					return a
				}
			}
			return rnd.Pick(c)
		}
	})
}

func ksilentcapsLibrarySize(c *PickCtx) int {
	if c.G == nil {
		return 0
	}
	d := c.G.Pending()
	if d == nil {
		return 0
	}
	return len(c.G.E.G.Zone(state.ZLibrary, d.Player))
}

func ksilentcapsArtifactCount(c *PickCtx) int {
	if c.G == nil {
		return 0
	}
	d := c.G.Pending()
	if d == nil {
		return 0
	}
	n := 0
	for _, id := range c.G.E.G.Zone(state.ZBattlefield, d.Player) {
		ob := c.G.E.G.Obj(id)
		if ob != nil && ob.Face() != nil && ksilentcapsArtifacts[ob.Face().Name] {
			n++
		}
	}
	return n
}

func TestSilentcapsLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("silentcaps")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := 0; s < 2; s++ {
		for _, e := range ksilentcapsDeck {
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
	tbl, _, _, err := set.Matchup(ksilentcapsDeck, ksilentcapsDeck)
	if err != nil {
		t.Fatal(err)
	}
	const seeds = 48
	games := 0
	var all ksilentcapsStats
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{"ksilentcaps-a", "ksilentcaps-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var st ksilentcapsStats
		res := Lockstep(m, seed, k, ksilentcapsPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		if st.lowAsks > 0 {
			st.lowGames = 1
			all.lowGames++
		}
		if res.Stall == KernelStallPrefix+"tables" {
			st.tableGames = 1
			all.tableGames++
		}
		if st.highCast {
			all.highCastGames++
		}
		all.lowAsks += st.lowAsks
		all.highAsks += st.highAsks
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %s", seed, d.At, d.What, d.Field, d.Detail)
		}
	}
	t.Logf("%d games, %d low asks (%d games), %d games cast at X >= 9 with a library of >= 9, %d games ended in a tables stall",
		games, all.lowAsks, all.lowGames, all.highCastGames, all.tableGames)
	if all.lowGames < 10 {
		t.Errorf("%d games posed a Max <= 8 surveil ask, want at least 10", all.lowGames)
	}
	if all.lowAsks < 10 {
		t.Errorf("%d Max <= 8 surveil asks, want at least 10", all.lowAsks)
	}
	if all.highCastGames < 10 {
		t.Errorf("%d games cast Cerebral Download at X >= 9 with a library of >= 9, want at least 10", all.highCastGames)
	}
	if all.tableGames < 10 {
		t.Errorf("%d games ended in a tables stall, want at least 10", all.tableGames)
	}
}
