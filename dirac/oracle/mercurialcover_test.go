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

// A Mercurial Pretender copy's AddAbilities$ is a copiable value (CR 707.9b),
// so it goes when a newer copy layer covers the copy and comes back when the
// layer under it is given back (CR 613.1a, 707.2): Fleeting Reflection makes
// the Pretender copy a copy of another creature until end of turn (the
// bounce is gone while it lasts, back at the next priority after the cleanup
// step), True Polymorph for good (the bounce is gone). A second Pretender
// entering as a copy of the first one's copy has the ability once (CR 707.3).
// The expected answer at every decision is gorge's; the policy never
// activates the Pretender, so the offers (compared at every priority) are the
// row.
var kmercurialCoverDecks = [2][]cardset.Entry{
	{
		{Name: "Mercurial Pretender", Count: 8}, {Name: "Grizzly Bears", Count: 2}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Fleeting Reflection", Count: 10}, {Name: "True Polymorph", Count: 2},
		{Name: "Island", Count: 13}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Plains", Count: 16}, {Name: "Forest", Count: 16},
	},
}

// kmercurialCoverPolicy: with a Pretender copy out, cast a copier (Fleeting
// Reflection, then True Polymorph), picking the Pretender copy as its first
// target and as another Pretender's template; never activate a Pretender; else
// cast or play anything.
func kmercurialCoverPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf, want []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		out := kmercurialCopyOut(c.G)
		if c.D.Kind != DPriority {
			prefer := []string{"Grizzly Bears"} // the first Pretender copies a bear
			if out {
				prefer = []string{"Mercurial Pretender", "Llanowar Elves"} // a copy of the copy, an Elves to copy
			}
			for _, name := range prefer {
				for _, o := range c.D.Options {
					if o.Kind() >= ActChoose && o.Mode() == 0 && m.InstanceCard(int(o.Src())) == name {
						return o
					}
				}
			}
			return rnd.Pick(c)
		}
		buf, want = buf[:0], want[:0]
		for _, o := range c.D.Options {
			switch o.Kind() {
			case ActCast:
				n := m.InstanceCard(int(o.Src()))
				if out && (n == "Fleeting Reflection" || n == "True Polymorph" || n == "Llanowar Elves" || n == "Mercurial Pretender") {
					want = append(want, o)
				}
				buf = append(buf, o)
			case ActPlayLand:
				buf = append(buf, o)
			}
		}
		if len(want) > 0 {
			// a second Pretender first (it copies the first's copy), then an
			// Elves to copy, then the copiers
			for _, n := range []string{"Mercurial Pretender", "Llanowar Elves", "Fleeting Reflection", "True Polymorph"} {
				for _, o := range want {
					if m.InstanceCard(int(o.Src())) == n && (n != "Mercurial Pretender" || kmercurialCopies(c.G) < 2) {
						return o
					}
				}
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

// kmercurialCopies is the number of seat 0's Pretenders that entered as copies.
func kmercurialCopies(g *GorgeSide) (n int) {
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		if o := g.E.G.Obj(id); o != nil && kmercurialPrinted(o) && o.Face() != nil && o.Face().Name != "Mercurial Pretender" {
			n++
		}
	}
	return n
}

// TestMercurialPretenderCoveredLockstep: 50 games, 0 divergences, with
// Pretender copies covered by an until-end-of-turn and a permanent copy.
func TestMercurialPretenderCoveredLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var gd [2][]*cards.Card
	for s, d := range kmercurialCoverDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	set := cardset.Raw(printedDelveSet(t, names))
	tbl, _, _, err := set.Matchup(kmercurialCoverDecks[0], kmercurialCoverDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var st dirac.CloneCopyStats
	dirac.DebugCloneCopyStats = &st
	defer func() { dirac.DebugCloneCopyStats = nil }()
	const n = 50
	covered, chained, div := 0, 0, 0
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"mpv-a", "mpv-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seenCov, seenChain := map[state.ObjID]bool{}, map[state.ObjID]bool{}
		check := func(ref *GorgeSide, _ Side) *Divergence {
			copies := 0
			for _, id := range ref.E.G.Zone(state.ZBattlefield, 0) {
				o := ref.E.G.Obj(id)
				if o == nil || !kmercurialPrinted(o) || o.Face() == nil {
					continue
				}
				switch o.Face().Name {
				case "Llanowar Elves":
					if !seenCov[id] {
						seenCov[id] = true
						covered++ // a Pretender (copy) under a newer copy layer
					}
				case "Grizzly Bears":
					if copies++; copies == 2 && !seenChain[id] {
						seenChain[id] = true
						chained++ // a second Pretender copy out
					}
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, kmercurialCoverPolicy(m, seed), RunConfig{Config: Config{}, MaxDecisions: 2000, MaxTurns: 20, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %d divergences, %d Pretender copies covered, %d seats with two Pretender copies, %d permanent and %d until-end-of-turn copies, %d re-based",
		n, div, covered, chained, st.Perm, st.EOT, st.Rebased)
	if covered < 3 || st.Rebased < 1 {
		t.Errorf("%d Pretender copies covered, %d re-based: want at least 3 and 1 (the covered layers must be exercised)", covered, st.Rebased)
	}
}

// A printed ability whose text is also the body of an SVar of its card
// (Scorn-Blade Berserker's "{1}, Sacrifice this creature: Draw a card.", the
// Backup grant's ABDraw) is a printed ability, not a copy's grant: the
// oracle's canonicalisation of a copy grant keys on the fold's mark, not on
// the text.
func TestPrintedAbilityIsNotACopyGrant(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	card, ok := reg.Lookup("Scorn-Blade Berserker")
	bear, ok2 := reg.Lookup("Grizzly Bears")
	mp, ok3 := reg.Lookup("Mercurial Pretender")
	if !ok || !ok2 || !ok3 {
		t.Skip("corpus lacks Scorn-Blade Berserker, Grizzly Bears or Mercurial Pretender")
	}
	g := state.NewGame([]string{"a", "b"})
	o := g.AddObject(card, 0)
	o.Zone = state.ZBattlefield
	f := o.Face()
	printed := -1
	for i, sa := range f.Abilities {
		for _, body := range f.SVars {
			if sa.Kind == "AB" && sa.Line != "" && strings.TrimSpace(body) == strings.TrimSpace(sa.Line) {
				printed = i
			}
		}
	}
	if printed < 0 {
		t.Fatal("precondition: no printed ability of Scorn-Blade Berserker has the text of an SVar body")
	}
	if copyGrantedAbilityOf(o, printed) {
		t.Fatal("a printed ability whose text is an SVar body reads as a copy's grant")
	}
	// and a copy's grant does
	b := g.AddObject(bear, 0)
	b.Zone = state.ZBattlefield
	x := events.CopyExceptions{Abilities: [][2]string{{"MercurialBounce", mp.Faces[0].SVars["MercurialBounce"]}}}
	events.Apply(g, events.Event{Kind: events.CopyExcept, Obj: b.ID, Text: x.Text()})
	if len(b.Face().Abilities) != 1 || !copyGrantedAbilityOf(b, 0) {
		t.Fatalf("the bear's copy has %d abilities, granted %v, want one granted", len(b.Face().Abilities), copyGrantedAbilityOf(b, 0))
	}
}
