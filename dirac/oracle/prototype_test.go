package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The prototype lockstep (CR 702.160, 718.3): Rust Goliath is cast for its
// printed {10} (a colourless 10/10 artifact creature) or prototyped for
// {3}{G}{G} (a green 3/5). gorge offers the second as a "cast" option with
// Mode "prototype"; the kernel casts it as the card's prototype face
// (kop.FacePrototype, dirac/face_castface.go), whose def carries the inset
// cost, colour and power/toughness, so Essence Scatter and Negate read the
// right spell, Shock reads the toughness, and the card is its printed self
// again in every zone but the stack and the battlefield. The fixture pool is
// dirac/cardset/pools/fixtures/prototype.
var prototypeDecks = []fixtureDecks{{
	{
		{Name: "Rust Goliath", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 5},
		{Name: "Giant Growth", Count: 4}, {Name: "Forest", Count: 18}, {Name: "Mountain", Count: 5},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Essence Scatter", Count: 3},
		{Name: "Negate", Count: 3}, {Name: "Unsummon", Count: 3}, {Name: "Wind Drake", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
}, {
	{
		{Name: "Rust Goliath", Count: 4}, {Name: "Sol Ring", Count: 4}, {Name: "Gilded Lotus", Count: 2},
		{Name: "Thran Dynamo", Count: 2}, {Name: "Worn Powerstone", Count: 2}, {Name: "Mind Stone", Count: 2},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Forest", Count: 20},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Essence Scatter", Count: 3},
		{Name: "Negate", Count: 3}, {Name: "Unsummon", Count: 3}, {Name: "Wind Drake", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
}}

// protoStats are the preconditions the lockstep measures on gorge's side.
type protoStats struct {
	protoCasts, frontCasts int                  // Rust Goliath casts the policy took, by route
	protoSeen, frontSeen   map[state.ObjID]bool // this game's battlefield objects, by designation
	protoPerms, frontPerms int                  // those objects over the games played
	protoSpells            int                  // decision points at which a prototyped Rust Goliath was on the stack
}

// protoPolicy plays randomly but casts Rust Goliath whenever it is offered:
// the printed cast always, the prototyped cast three times in four.
func protoPolicy(seed uint64, st *protoStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x70726f74)) // "prot"
	proto, _ := faceCastCode("prototype")
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority && len(c.G.E.G.Stack) == 0 {
			for _, a := range c.D.Options {
				if refCardName(c.G, a.Src()) != "Rust Goliath" {
					continue
				}
				if a.Kind() == ActCast && a.Mode()&15 == 0 {
					st.frontCasts++
					return a
				}
				if a.Kind() == ActCastFace && a.Mode() == proto && prng.IntN(4) != 0 {
					st.protoCasts++
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

// protoObserve records the Rust Goliath objects gorge shows prototyped and
// not on the battlefield, and the prototyped ones on the stack.
func protoObserve(ref *GorgeSide, st *protoStats) {
	g := ref.E.G
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil || o.Face().Name != "Rust Goliath" {
				continue
			}
			if o.CastFlags&state.FlagPrototype != 0 {
				st.protoSeen[id] = true
			} else {
				st.frontSeen[id] = true
			}
		}
	}
	for _, id := range g.Stack {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Rust Goliath" && o.CastFlags&state.FlagPrototype != 0 {
			st.protoSpells++
		}
	}
}

// runProtoLockstep plays seeds 1..seeds of prototypeDecks, gorge leading,
// alternating the matchup and the option surface by seed; mutate rewrites
// each matchup's table first.
func runProtoLockstep(t *testing.T, seeds int, report bool, mutate func(tbl *kop.Table)) (fixtureRun, *protoStats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("prototype")
	if err != nil {
		t.Fatal(err)
	}
	decks := make([][2][]*cards.Card, len(prototypeDecks))
	tbls := make([]*kop.Table, len(prototypeDecks))
	for mi, md := range prototypeDecks {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for range e.Count {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		if mutate != nil {
			mutate(tbl)
		}
		tbls[mi] = tbl
	}
	run := fixtureRun{casts: map[string]int{}}
	st := &protoStats{protoSeen: map[state.ObjID]bool{}, frontSeen: map[state.ObjID]bool{}}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		mi := int(seed/2) % len(prototypeDecks)
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"proto-a", "proto-b"}, decks[mi], 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbls[mi], cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		check := func(ref *GorgeSide, _ Side) *Divergence {
			protoObserve(ref, st)
			return nil
		}
		res := LockstepLed(m, seed, g, k, protoPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
		CloseSide(k)
		st.protoPerms, st.frontPerms = st.protoPerms+len(st.protoSeen), st.frontPerms+len(st.frontSeen)
		clear(st.protoSeen)
		clear(st.frontSeen)
		run.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			run.stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
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
	return run, st
}

// TestPrototypeLowers plays Rust Goliath both ways in lockstep with gorge: no
// divergence, and the pool actually casts it prototyped and printed.
func TestPrototypeLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	run, st := runProtoLockstep(t, 300, true, nil)
	t.Logf("%d games, %d divergences, %d stalls; prototype casts %d, printed casts %d; prototyped permanents %d, printed permanents %d, prototype stack decisions %d",
		run.games, run.divs, run.stalls, st.protoCasts, st.frontCasts, st.protoPerms, st.frontPerms, st.protoSpells)
	if st.protoCasts < 40 || st.protoPerms < 20 || st.protoSpells < 20 {
		t.Errorf("the pool barely plays the prototype: %d casts, %d permanents, %d stack decisions", st.protoCasts, st.protoPerms, st.protoSpells)
	}
	if st.frontPerms < 3 {
		t.Errorf("the pool never casts the printed Rust Goliath: %d permanents", st.frontPerms)
	}
}

// TestPrototypeMutation: a kernel without the prototype face, or whose
// prototype face keeps the printed 10/10, must diverge from gorge.
func TestPrototypeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, m := range []struct {
		what string
		fn   func(cd *kop.CardDef, tbl *kop.Table)
	}{
		{"no prototype face", func(cd *kop.CardDef, _ *kop.Table) { cd.FaceMode, cd.Faces = kop.FaceNone, nil }},
		{"the prototype keeps the printed power", func(cd *kop.CardDef, tbl *kop.Table) { tbl.Cards[cd.Faces[1]].Power = 10 }},
	} {
		t.Run(m.what, func(t *testing.T) {
			run, _ := runProtoLockstep(t, 300, false, func(tbl *kop.Table) {
				tbl.Cards = append([]kop.CardDef(nil), tbl.Cards...) // the set's own table is shared
				for i := range tbl.Cards {
					if cd := &tbl.Cards[i]; cd.Name == "Rust Goliath" && cd.FaceMode == kop.FacePrototype && cd.Face == 0 {
						m.fn(cd, tbl)
					}
				}
			})
			run.requireDivergence(t, m.what)
		})
	}
}
