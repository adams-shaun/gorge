package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The fuse lockstep (CR 702.102): constructed matchups over the fuse
// fixture pool, together holding every lowered fuse card. gorge offers the
// fused cast only when the floating pool pays it (its planner prices no
// "fuse" mode), and the lockstep has no manual mana: the decks carry {X}
// spells whose lowered witness taps for the largest X, so a smaller X leaves
// the mana floating for a fused cast (an instant one with the {X} spell
// still on the stack, a sorcery one after it resolves). The decks carry
// artifacts and enchantments for Wear // Tear, creatures on both sides for
// the halves' targets and a counterspell. The counters read gorge's event
// log; the mutations must diverge.
var kfuseDecks = [][2][]cardset.Entry{{
	{
		{Name: "Wear", Count: 4}, {Name: "Turn", Count: 4}, {Name: "Protect", Count: 4},
		{Name: "Blaze", Count: 4}, {Name: "Mind Spring", Count: 1},
		{Name: "Ornithopter", Count: 3}, {Name: "Pacifism", Count: 3},
		{Name: "Island", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 5},
	},
	{
		{Name: "Alive", Count: 4}, {Name: "Down", Count: 4}, {Name: "Ready", Count: 4},
		{Name: "Stream of Life", Count: 4}, {Name: "Channel the Suns", Count: 2},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Glorious Anthem", Count: 1}, {Name: "Bonesplitter", Count: 1},
		{Name: "Forest", Count: 7}, {Name: "Swamp", Count: 6}, {Name: "Plains", Count: 5},
	},
}, {
	{
		{Name: "Far", Count: 4}, {Name: "Toil", Count: 4}, {Name: "Mind Spring", Count: 3},
		{Name: "Exsanguinate", Count: 2}, {Name: "Blaze", Count: 2}, {Name: "Coral Merfolk", Count: 3},
		{Name: "Wind Drake", Count: 2}, {Name: "Counterspell", Count: 1},
		{Name: "Island", Count: 7}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Give", Count: 4}, {Name: "Profit", Count: 4}, {Name: "Beck", Count: 4},
		{Name: "Stream of Life", Count: 4}, {Name: "Channel the Suns", Count: 2}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Forest", Count: 7}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 3},
	},
}, {
	// the costliest fused spells, beside twenty lands and more {X} spells
	{
		{Name: "Turn", Count: 4}, {Name: "Give", Count: 4},
		{Name: "Stream of Life", Count: 4}, {Name: "Mind Spring", Count: 3}, {Name: "Blaze", Count: 3},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Forest", Count: 7}, {Name: "Island", Count: 7}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Down", Count: 4}, {Name: "Ready", Count: 4},
		{Name: "Stream of Life", Count: 4}, {Name: "Channel the Suns", Count: 2}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Forest", Count: 8}, {Name: "Plains", Count: 8}, {Name: "Swamp", Count: 7},
	},
}, {
	// the three-colour fused spells, each deck one carrier, its {X} spell
	// and twenty-three lands in the carrier's colours
	{
		{Name: "Ready", Count: 9}, {Name: "Stream of Life", Count: 8}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Forest", Count: 6}, {Name: "Plains", Count: 8}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Beck", Count: 6}, {Name: "Stream of Life", Count: 8}, {Name: "Coral Merfolk", Count: 3},
		{Name: "Forest", Count: 8}, {Name: "Island", Count: 9}, {Name: "Plains", Count: 6},
	},
}}

// fuseCardFaces reports the halves of a split card printed with fuse, or nil.
func fuseCardFaces(o *state.Object) []*cards.Face {
	if o == nil || o.Card == nil || o.Card.AlternateMode != cards.AlternateSplit || len(o.Card.Faces) != 2 ||
		o.Card.Faces[0] == nil || o.Card.Faces[1] == nil ||
		!o.Card.Faces[0].HasKeyword("Fuse") && !o.Card.Faces[1].HasKeyword("Fuse") {
		return nil
	}
	return o.Card.Faces
}

// fuseHandCost is the least combined mana value of the fuse cards p holds
// (CR 702.102c), or -1 when p holds none.
func fuseHandCost(g *state.Game, p state.PlayerID) int {
	least := -1
	for _, id := range g.Zone(state.ZHand, p) {
		faces := fuseCardFaces(g.Obj(id))
		if faces == nil {
			continue
		}
		if mv := int(faces[0].ManaValue() + faces[1].ManaValue()); least < 0 || mv < least {
			least = mv
		}
	}
	return least
}

// untappedLands counts p's untapped lands.
func untappedLands(g *state.Game, p state.PlayerID) int {
	n := 0
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && !o.Tapped && o.Face() != nil && o.Face().IsLand() {
			n++
		}
	}
	return n
}

// manaSpell reports whether f is one of the decks' mana floaters: an {X}
// spell or Channel the Suns.
func manaSpell(f *cards.Face) bool {
	return f != nil && (strings.HasPrefix(f.ManaCost, "X ") || f.Name == "Channel the Suns")
}

// fusePolicy casts a fused split spell whenever one is offered. While the
// seat holds a fuse card it floats the mana for it, and keeps that card (a
// half cast alone would spend it) and its mana spells for that (the mana
// spells it never casts otherwise, so one is still in hand when a fuse card
// comes): an {X} spell announced at its smallest X once its
// untapped lands cover the fused cost beside the spell's own pips (the
// witness taps them all), or Channel the Suns for a fused cost of at most
// five. Else it plays or casts something, else it answers at random.
func fusePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	xc, _ := chooseCode("x")
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, a := range c.D.Options {
			if a.Kind() == ActCastFace && a.Mode()&0x0f == faceCodeFuse {
				return a
			}
		}
		hoard := false
		if c.G != nil {
			g, p := c.G.E.G, state.PlayerID(c.D.Player)
			if cost := fuseHandCost(g, p); cost >= 0 {
				hoard = true
				least := -1
				for i, a := range c.D.Options {
					if a.Kind() == ActChoose+CanonKind(xc) && (least < 0 || a.Mode() < c.D.Options[least].Mode()) {
						least = i
					}
				}
				if least >= 0 {
					return c.D.Options[least]
				}
				lands := untappedLands(g, p)
				for _, a := range c.D.Options {
					if a.Kind() != ActCast {
						continue
					}
					o := canonObj(c, a.Src())
					if o == nil || o.Face() == nil {
						continue
					}
					if f := o.Face(); manaSpell(f) && (f.Name == "Channel the Suns" && cost <= 5 || f.Name != "Channel the Suns" && lands >= cost+int(f.ManaValue())) {
						return a
					}
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActPlayLand {
				if o := canonObj(c, a.Src()); k == ActCast && o != nil && (manaSpell(o.Face()) || hoard && fuseCardFaces(o) != nil) {
					continue
				}
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			if hoard { // only the held cards are left to cast: pass
				for _, a := range c.D.Options {
					if a.Kind() == ActPass {
						return a
					}
				}
			}
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// fuseTally counts the fuse routes. The counters survive across games.
type fuseTally struct {
	ref    *GorgeSide
	frames map[uint64]bool

	offers, emptyPoolOffers    int
	resolved, toGrave, toOther int
	halfCasts                  int
	perCard                    map[string]int
}

func (t *fuseTally) start() {
	t.ref = nil
	t.frames = map[uint64]bool{}
	if t.perCard == nil {
		t.perCard = map[string]int{}
	}
}

// check counts the fused offers at every priority frame: gorge prices none
// with its planner, so each is paid by the actor's floating pool.
func (t *fuseTally) check(ref *GorgeSide, k Side) *Divergence {
	t.ref = ref
	fr := ref.fr
	if fr == nil || fr.d == nil || fr.d.Kind != "priority" || t.frames[fr.seq] {
		return nil
	}
	t.frames[fr.seq] = true
	g := ref.E.G
	actor := state.PlayerID(fr.d.Player)
	for range kernelFaceRefs(k, faceCodeFuse) {
		t.offers++
		if g.Players[actor].Pool.Total() == 0 {
			t.emptyPoolOffers++
		}
	}
	return nil
}

// scan reads the finished game's event log: gorge's fused resolution is one
// Resolve naming both halves (rules split.go resolveFused).
func (t *fuseTally) scan() {
	if t.ref == nil {
		return
	}
	g := t.ref.E.G
	fused := map[state.ObjID]bool{}
	for i := range t.ref.E.L.Events {
		e := &t.ref.E.L.Events[i]
		switch e.Kind {
		case events.PutOnStack:
			if faces := fuseCardFaces(g.Obj(e.Obj)); faces != nil && (e.Text == faces[0].Name || e.Text == faces[1].Name) {
				t.halfCasts++
			}
		case events.Resolve:
			faces := fuseCardFaces(g.Obj(e.Obj))
			if faces != nil && e.Text == faces[0].Name+" // "+faces[1].Name {
				fused[e.Obj] = true
				t.resolved++
				t.perCard[faces[0].Name]++
			}
		case events.MoveZone:
			if e.From != state.ZStack || !fused[e.Obj] {
				break
			}
			delete(fused, e.Obj)
			if e.To == state.ZGraveyard {
				t.toGrave++
			} else {
				t.toOther++
			}
		}
	}
}

func kfuseSetup(t *testing.T, mi int) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("fuse")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kfuseDecks[mi] {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kfuseDecks[mi][0], kfuseDecks[mi][1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

func kfuseRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config, tally *fuseTally) Result {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"fuse-a", "fuse-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := fusePolicy(seed)
	if seed%8 == 1 {
		pol = RandomPolicy(seed)
	}
	var check func(*GorgeSide, Side) *Divergence
	if tally != nil {
		tally.start()
		check = tally.check
	}
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 5000, MaxTurns: 80, Check: check})
	if tally != nil {
		tally.scan()
	}
	CloseSide(k)
	return res
}

// TestFuseLockstep: the kernel matches gorge over the fuse pool, every
// carrier resolves fused, and no fused cast is offered without the floating
// pool to pay it.
func TestFuseLockstep(t *testing.T) {
	t.Parallel()
	var tally fuseTally
	const seeds = 256
	var carriers []string
	capacity := 0
	for mi := range kfuseDecks {
		reg, set, decks, tbl := kfuseSetup(t, mi)
		for seed := uint64(1); seed <= seeds; seed++ {
			res := kfuseRun(t, reg, set, decks, tbl, seed, aftermathCfg(seed), &tally)
			switch {
			case res.StallUnmodelled:
				t.Errorf("matchup %d seed %d: unmodelled stall: %s", mi, seed, res.Stall)
			case strings.HasPrefix(res.Stall, KernelStallPrefix):
				capacity++
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		for _, c := range tbl.Cards {
			if c.FaceMode == kop.FaceSplit && c.Face == 0 && c.FuseDef != 0 {
				carriers = append(carriers, c.Name)
			}
		}
	}
	t.Logf("%d seeds per matchup (%d kernel capacity stalls): fused offers %d (%d with an empty pool); fused resolutions %d, per carrier %v; to the graveyard %d, elsewhere %d; single-half casts %d",
		seeds, capacity, tally.offers, tally.emptyPoolOffers, tally.resolved, tally.perCard, tally.toGrave, tally.toOther, tally.halfCasts)
	for _, c := range []struct {
		name      string
		got, want int
	}{
		{"fused offers", tally.offers, 40},
		{"fused resolutions", tally.resolved, 30},
		{"fused spells resting in the graveyard", tally.toGrave, 30},
		{"single-half casts of a fuse card", tally.halfCasts, 30},
	} {
		if c.got < c.want {
			t.Errorf("%s: %d, want at least %d", c.name, c.got, c.want)
		}
	}
	if tally.emptyPoolOffers != 0 {
		t.Errorf("fused offers with an empty pool: %d, want 0 (gorge's planner prices no fused cast)", tally.emptyPoolOffers)
	}
	if capacity > seeds/8 {
		t.Errorf("%d kernel capacity stalls, want at most %d", capacity, seeds/8)
	}
	distinct := map[string]bool{}
	for _, name := range carriers {
		distinct[name] = true
	}
	if len(distinct) != 11 {
		t.Errorf("the matchups hold %d fuse carriers %v, want the pool's 11", len(distinct), carriers)
	}
	for name := range distinct {
		if got := tally.perCard[name]; got < 1 {
			t.Errorf("carrier %s: %d fused resolutions, want at least 1", name, got)
		}
	}
}

// TestFuseMutation: each mutation must diverge on some seed.
func TestFuseMutation(t *testing.T) {
	reg, set, decks, tbl := kfuseSetup(t, 0)
	muts := []struct {
		name string
		on   *bool
	}{
		{"fused cast offered from the mana census", &dirac.DebugFuseAnyPool},
		{"fused cast withheld", &dirac.DebugFuseNoOffer},
	}
	for _, mu := range muts {
		*mu.on = true
		t.Cleanup(func() { *mu.on = false }) // a t.Fatal inside must not leave it set for the parallel tests
		diverged := false
		for seed := uint64(1); seed <= 128 && !diverged; seed++ {
			res := kfuseRun(t, reg, set, decks, tbl, seed, aftermathCfg(seed), nil)
			if diverged = res.Div != nil; diverged {
				t.Logf("mutation %q: seed %d diverged at decision %d: %s %s", mu.name, seed, res.Div.At, res.Div.What, res.Div.Field)
			}
		}
		*mu.on = false
		if !diverged {
			t.Errorf("mutation %q never diverged", mu.name)
		}
	}
}
