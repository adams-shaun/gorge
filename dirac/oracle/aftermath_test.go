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

// The aftermath lockstep (CR 702.127a): constructed matchups over the
// aftermath fixture pool, together holding every lowered aftermath card. Each
// deck carries aftermath split cards in the colours of their graveyard
// halves beside the loot, discard and self-mill that put them in the
// graveyard, creatures of power 1..5 and flyers for their halves, and
// counterspells, so the right half is cast from the graveyard, resolves and
// is countered. The counters read gorge's event log; the mutations must
// diverge.
var kaftermathDecks = [][2][]cardset.Entry{{
	{
		{Name: "Dusk", Count: 3}, {Name: "Farm", Count: 3}, {Name: "Never", Count: 3},
		{Name: "Careful Study", Count: 3}, {Name: "Stitcher's Supplier", Count: 3},
		{Name: "Mind Rot", Count: 2}, {Name: "Counterspell", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Walking Corpse", Count: 2},
		{Name: "Serra Angel", Count: 1}, {Name: "Wind Drake", Count: 2}, {Name: "Coral Merfolk", Count: 2},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 4},
	},
	{
		{Name: "Heaven", Count: 4}, {Name: "Faithless Looting", Count: 3}, {Name: "Tormenting Voice", Count: 3},
		{Name: "Mulch", Count: 2}, {Name: "Shock", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Hill Giant", Count: 3}, {Name: "Giant Spider", Count: 2}, {Name: "Gray Ogre", Count: 2},
		{Name: "Essence Scatter", Count: 2}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 5},
		{Name: "Island", Count: 2},
	},
}, {
	// the graveyard halves in black, red and white: Finish's sacrifice,
	// Ribbons' {X}, Fame, Chance's discard-and-draw, Victory, Rubble's
	// three land targets and Ruin
	{
		{Name: "Start", Count: 2}, {Name: "Cut", Count: 2}, {Name: "Claim", Count: 2}, {Name: "Leave", Count: 2},
		{Name: "Onward", Count: 2}, {Name: "Reduce", Count: 2}, {Name: "Road", Count: 2},
		{Name: "Faithless Looting", Count: 2}, {Name: "Tormenting Voice", Count: 2}, {Name: "Stitcher's Supplier", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Walking Corpse", Count: 2}, {Name: "Gray Ogre", Count: 1},
		{Name: "Shock", Count: 1}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Plains", Count: 4},
	},
	// the graveyard halves in green, white, blue and black: Fight's two
	// targets, Authority, Despair, Feed, Believe, the instant Mind and
	// Oblivion
	{
		{Name: "Prepare", Count: 2}, {Name: "Appeal", Count: 2}, {Name: "Driven", Count: 2}, {Name: "Mouth", Count: 2},
		{Name: "Reason", Count: 2}, {Name: "Spring", Count: 2}, {Name: "Consign", Count: 2},
		{Name: "Careful Study", Count: 2}, {Name: "Mulch", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Giant Spider", Count: 2}, {Name: "Coral Merfolk", Count: 2}, {Name: "Counterspell", Count: 1},
		{Name: "Essence Scatter", Count: 1}, {Name: "Forest", Count: 6}, {Name: "Island", Count: 4},
		{Name: "Plains", Count: 2}, {Name: "Swamp", Count: 2},
	},
}}

// aftermathPolicy casts an aftermath half whenever one is offered, else
// plays or casts something (the loot and self-mill fill the graveyard), else
// answers at random.
func aftermathPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, a := range c.D.Options {
			if a.Kind() == ActCastFace && a.Mode()&0x0f == faceCodeAftermath {
				return a
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActPlayLand {
				buf = append(buf, a)
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

// aftermathCard reports the halves of a split card whose right half has
// aftermath, or nil.
func aftermathCard(o *state.Object) []*cards.Face {
	if o == nil || o.Card == nil || o.Card.AlternateMode != cards.AlternateSplit || len(o.Card.Faces) != 2 ||
		o.Card.Faces[1] == nil || !o.Card.Faces[1].HasKeyword("Aftermath") {
		return nil
	}
	return o.Card.Faces
}

// aftermathTally counts the aftermath routes over gorge's event log. The
// counters survive across games.
type aftermathTally struct {
	ref *GorgeSide

	casts, fronts         int
	toExile, toElsewhere  int
	countered, fizzled    int
	frontToGrave          int
	offeredFromGraveyard  int
	offeredOutsideGrave   int
	perCard               map[string]int
	frames                map[uint64]bool
	emptyPoolOffers       int
	targetedAftermathCast int
}

func (t *aftermathTally) start() {
	t.ref = nil
	t.frames = map[uint64]bool{}
	if t.perCard == nil {
		t.perCard = map[string]int{}
	}
}

// check counts the aftermath offers at every priority frame: each names a
// card in its caster's graveyard.
func (t *aftermathTally) check(ref *GorgeSide, k Side) *Divergence {
	t.ref = ref
	fr := ref.fr
	if fr == nil || fr.d == nil || fr.d.Kind != "priority" || t.frames[fr.seq] {
		return nil
	}
	t.frames[fr.seq] = true
	g := ref.E.G
	actor := state.PlayerID(fr.d.Player)
	for _, r := range kernelFaceRefs(k, faceCodeAftermath) {
		o := g.Obj(ref.id.obj[r])
		if o != nil && o.Zone == state.ZGraveyard && o.Owner == actor {
			t.offeredFromGraveyard++
		} else {
			t.offeredOutsideGrave++
		}
		if g.Players[actor].Pool.Total() == 0 {
			t.emptyPoolOffers++
		}
	}
	return nil
}

// scan reads the finished game's event log.
func (t *aftermathTally) scan() {
	if t.ref == nil {
		return
	}
	g := t.ref.E.G
	cast := map[state.ObjID]bool{}
	front := map[state.ObjID]bool{}
	for i := range t.ref.E.L.Events {
		e := &t.ref.E.L.Events[i]
		switch e.Kind {
		case events.PutOnStack:
			faces := aftermathCard(g.Obj(e.Obj))
			switch {
			case faces == nil:
			case e.Text == faces[1].Name:
				cast[e.Obj] = true
				delete(front, e.Obj)
				t.casts++
				t.perCard[faces[0].Name]++
			case e.Text == faces[0].Name:
				front[e.Obj] = true
				delete(cast, e.Obj)
				t.fronts++
			}
		case events.TargetsChosen:
			if cast[e.Obj] && len(e.IDs) > 0 {
				t.targetedAftermathCast++
			}
		case events.MoveZone:
			if e.From != state.ZStack {
				break
			}
			if front[e.Obj] && e.To == state.ZGraveyard {
				t.frontToGrave++
			}
			delete(front, e.Obj)
			if !cast[e.Obj] {
				break
			}
			delete(cast, e.Obj)
			if e.To != state.ZExile {
				t.toElsewhere++
				break
			}
			t.toExile++
			switch {
			case strings.HasPrefix(e.Text, "countered"):
				t.countered++
			case strings.HasPrefix(e.Text, "fizzled"):
				t.fizzled++
			}
		}
	}
}

func kaftermathSetup(t *testing.T, mi int) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("aftermath")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kaftermathDecks[mi] {
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
	tbl, _, _, err := set.Matchup(kaftermathDecks[mi][0], kaftermathDecks[mi][1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

func kaftermathRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config, tally *aftermathTally) Result {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"aftermath-a", "aftermath-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := aftermathPolicy(seed)
	if seed%4 == 1 {
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

// aftermathCfg alternates the surfaces: the autopay surface has no floating
// mana to pay a face with, so every fourth seed plays it.
func aftermathCfg(seed uint64) Config {
	if seed%4 == 0 {
		return Config{GorgeAutopaySurface: true}
	}
	return Config{}
}

// TestAftermathLockstep: the kernel matches gorge over the aftermath pool,
// and every route of the keyword is reached: the graveyard cast of each
// carrier, the exile on resolution and on a counter, and the front half's
// ordinary cast from the hand into the graveyard.
func TestAftermathLockstep(t *testing.T) {
	t.Parallel()
	var tally aftermathTally
	const seeds = 128
	var carriers []string
	capacity := 0
	for mi := range kaftermathDecks {
		reg, set, decks, tbl := kaftermathSetup(t, mi)
		for seed := uint64(1); seed <= seeds; seed++ {
			res := kaftermathRun(t, reg, set, decks, tbl, seed, aftermathCfg(seed), &tally)
			switch {
			case res.StallUnmodelled:
				t.Errorf("matchup %d seed %d: unmodelled stall: %s", mi, seed, res.Stall)
			case strings.HasPrefix(res.Stall, KernelStallPrefix):
				// a declared capacity (Leave's "any number of target
				// permanents" past maxXTgt): the game ends as a draw, as
				// kfuzz counts it
				capacity++
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		// Every carrier in the table must reach the stack as its aftermath
		// half. The list is the table's, so a carrier added to a deck is
		// covered.
		for _, c := range tbl.Cards {
			if c.FaceMode == kop.FaceSplit && c.Face == 0 && len(c.Faces) == 2 && tbl.Cards[c.Faces[1]].Aftermath {
				carriers = append(carriers, c.Name)
			}
		}
	}
	t.Logf("%d seeds per matchup (%d kernel capacity stalls): aftermath casts %d (%d targeted), per carrier %v; front casts %d (%d to the graveyard); aftermath left the stack for exile %d (%d countered, %d fizzled), elsewhere %d; offers from the caster's graveyard %d (%d with an empty pool), elsewhere %d",
		seeds, capacity, tally.casts, tally.targetedAftermathCast, tally.perCard, tally.fronts, tally.frontToGrave,
		tally.toExile, tally.countered, tally.fizzled, tally.toElsewhere,
		tally.offeredFromGraveyard, tally.emptyPoolOffers, tally.offeredOutsideGrave)
	for _, c := range []struct {
		name      string
		got, want int
	}{
		{"aftermath casts", tally.casts, 40},
		{"aftermath casts with a target", tally.targetedAftermathCast, 10},
		{"front casts", tally.fronts, 40},
		{"front casts resting in the graveyard", tally.frontToGrave, 20},
		{"aftermath exiles leaving the stack", tally.toExile, 30},
		{"countered aftermath spells exiled", tally.countered, 1},
		{"aftermath offers from the graveyard", tally.offeredFromGraveyard, 40},
		{"aftermath offers with an empty pool", tally.emptyPoolOffers, 10},
	} {
		if c.got < c.want {
			t.Errorf("%s: %d, want at least %d", c.name, c.got, c.want)
		}
	}
	for _, c := range []struct {
		name string
		got  int
	}{
		{"aftermath spells leaving the stack for a zone other than exile", tally.toElsewhere},
		{"aftermath offers outside the caster's graveyard", tally.offeredOutsideGrave},
	} {
		if c.got != 0 {
			t.Errorf("%s: %d, want 0", c.name, c.got)
		}
	}
	if capacity > seeds/8 {
		t.Errorf("%d kernel capacity stalls, want at most %d: the matchups stop comparing too early", capacity, seeds/8)
	}
	if len(carriers) != 18 {
		t.Errorf("the matchups hold %d aftermath carriers %v, want the pool's 18", len(carriers), carriers)
	}
	for _, name := range carriers {
		if got := tally.perCard[name]; got < 1 {
			t.Errorf("carrier %s: %d aftermath casts, want at least 1", name, got)
		}
	}
}

// TestAftermathMutation: each mutation must diverge on some seed.
func TestAftermathMutation(t *testing.T) {
	reg, set, decks, tbl := kaftermathSetup(t, 0)
	muts := []struct {
		name string
		on   *bool
	}{
		{"resolved aftermath spell rests in the graveyard", &dirac.DebugAftermathGraveyard},
		{"aftermath half offered from the hand", &dirac.DebugAftermathFromHand},
	}
	for _, mu := range muts {
		*mu.on = true
		t.Cleanup(func() { *mu.on = false }) // a t.Fatal inside must not leave it set for the parallel tests
		diverged := false
		for seed := uint64(1); seed <= 128 && !diverged; seed++ {
			res := kaftermathRun(t, reg, set, decks, tbl, seed, aftermathCfg(seed), nil)
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
