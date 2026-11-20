package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The as-enters copy of a permanent that is not a creature or an artifact and
// the as-enters or on-battlefield copy with NewName$ (the clonenoncreature
// fixture's three classes): a land, an enchantment and an Aura copy enter with
// the copied card's characteristics (CR 707.2), an Aura copy is asked its
// CR 303.4f host as it enters, a planeswalker copy enters with the copied
// card's printed loyalty (CR 306.5b) and a NewName$ copy carries the named
// name as a copy exception (CR 707.9b). The lockstep compares the two sides'
// option lists at every decision, so the copy election's template order, the
// Aura copy's host candidates and the NewName copy's name are checked
// decision by decision.

// clonenoncreaturePodDecks are the four-seat decks: two carrier decks (the
// as-enters copy carriers over the permanents they copy) and two template
// decks (a creature, an Aura, a non-Aura enchantment, a nonbasic land and a
// planeswalker), alternating so every seat can copy something and every
// carrier sees a template. Nothing here makes a token or a copy of a copy, so
// the object order the two engines compare stays the fixture's own.
func clonenoncreaturePodDecks() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		clonenoncreatureCarrierDeck(),
		clonenoncreatureTemplateDeck(),
		clonenoncreatureCarrierDeck(),
		clonenoncreatureTemplateDeck(),
	}
}

// clonenoncreatureDecks are the two-seat decks, the same carrier and template
// decks at two seats.
func clonenoncreatureDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		clonenoncreatureCarrierDeck(),
		clonenoncreatureTemplateDeck(),
	}
}

// clonenoncreatureCarrierDeck is a 40-card deck of the four as-enters copy
// carriers plus the permanents they can copy.
func clonenoncreatureCarrierDeck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Clever Impersonator", Count: 4}, {Name: "Copy Land", Count: 3},
		{Name: "Copy Enchantment", Count: 3}, {Name: "Mirrormade", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Divine Favor", Count: 2},
		{Name: "Ajani's Mantra", Count: 2}, {Name: "Command Tower", Count: 2},
		{Name: "Liliana of the Veil", Count: 2},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 6}, {Name: "Forest", Count: 1}, {Name: "Swamp", Count: 4},
	}
}

// clonenoncreatureTemplateDeck is a 40-card deck of the permanents the
// carriers copy: a creature, an Aura, a non-Aura enchantment, a nonbasic land
// and a planeswalker.
func clonenoncreatureTemplateDeck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Divine Favor", Count: 4},
		{Name: "Ajani's Mantra", Count: 4}, {Name: "Command Tower", Count: 4},
		{Name: "Liliana of the Veil", Count: 2},
		{Name: "Plains", Count: 7}, {Name: "Island", Count: 7},
		{Name: "Forest", Count: 4}, {Name: "Swamp", Count: 4},
	}
}

// cloneCopies counts a finished gorge log's as-enters copies by the template's
// kind: an entered object whose current face is not its printed card's face
// entered as a copy (CR 707.2), classified Saga, Battle, planeswalker, Aura,
// land and other enchantment (a Copy Land copy is Land and Enchantment, so it
// counts as a land).
type cloneCopies struct {
	land, enchant, aura, planeswalker int
	saga, battle                      int // a Saga or Battle copy's entry state
	chameleon, irma                   int // the NewName$ copies, by the name they carry
}

func countCloneCopies(g *GorgeSide) (c cloneCopies) {
	for _, e := range g.E.L.Events {
		if e.Kind != events.ClonePermanent {
			continue
		}
		switch e.Text {
		case "Chameleon, Master of Disguise":
			c.chameleon++
		case "Irma, Part-Time Mutant":
			c.irma++
		}
		o := g.E.G.Obj(e.Obj)
		if o == nil {
			continue
		}
		f := o.Face()
		if o.Zone != state.ZBattlefield && o.LeftCopyFace != nil {
			f = o.LeftCopyFace // a copy that has since left still happened
		}
		if f == nil {
			continue
		}
		switch {
		case slices.Contains(f.Types, "Saga"):
			c.saga++
		case f.IsBattle():
			c.battle++
		case f.IsPlaneswalker():
			c.planeswalker++
		case f.IsEnchantment() && slices.Contains(f.Types, "Aura"):
			c.aura++
		case f.IsLand():
			c.land++
		case f.IsEnchantment():
			c.enchant++
		}
	}
	return c
}

// clonenoncreatureChecks reports a seed's counts and fails on a divergence, an
// unmodelled stall or a kernel stall.
func clonenoncreatureChecks(t *testing.T, seed uint64, res Result, c cloneCopies) {
	t.Helper()
	if res.Div != nil {
		ventureFail(t, seed, res)
	}
	if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
	}
	t.Logf("seed %d: %d decisions, over %v %s; copies %+v", seed, res.Decisions, res.Over, res.Stall, c)
}

// clonenoncreaturePolicy casts, plays a land or activates whenever offered
// (the carriers are the game's spells), else prefers the as-enters copy
// election's templates over "Enter as itself", rotating the template kind the
// election prefers (planeswalker, Aura, land, enchantment) so the seeds cover
// all four; a copy of no preferred kind still copies (the rotation is a
// preference, never a decline). When the gorge side is absent the policy plays
// random.
func clonenoncreaturePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	isAura := func(f *cards.Face) bool { return f.IsEnchantment() && slices.Contains(f.Types, "Aura") }
	isOtherEnchant := func(f *cards.Face) bool { return f.IsEnchantment() && !slices.Contains(f.Types, "Aura") }
	pref := [4]func(*cards.Face) bool{
		(*cards.Face).IsPlaneswalker, isAura, (*cards.Face).IsLand, isOtherEnchant,
	}
	cloneSeq := 0
	needPW := false
	var done [4]bool
	cardName := func(c *PickCtx, a Canon) string {
		o, ok := c.G.GorgeOption(a)
		if !ok || o.Obj == 0 {
			return ""
		}
		obj := c.G.E.G.Obj(o.Obj)
		if obj == nil || obj.Card == nil || len(obj.Card.Faces) == 0 {
			return ""
		}
		return obj.Card.Faces[0].Name
	}
	pwOnBoard := func(c *PickCtx) bool {
		for p := 0; p < len(c.G.E.G.Players); p++ {
			for _, id := range c.G.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
				if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().IsPlaneswalker() {
					return true
				}
			}
		}
		return false
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DTarget {
			for _, a := range c.D.Options {
				if a.Kind() == ActTarget {
					return a
				}
			}
		}
		if c.D.Kind == DTriggerOptional {
			for _, a := range c.D.Options {
				if a.Kind() == ActYes {
					return a
				}
			}
		}
		var acts []Canon
		for _, a := range c.D.Options {
			k := a.Kind()
			if (k == ActCast || k == ActCastFace) && c.G != nil &&
				cardName(c, a) == "Clever Impersonator" && !pwOnBoard(c) {
				continue // hold the planeswalker-copy carrier until a planeswalker is there
			}
			if k == ActCast || k == ActCastFace || k == ActActivate || k == ActPlayLand {
				acts = append(acts, a)
			}
		}
		if len(acts) == 0 {
			for _, a := range c.D.Options {
				if a.Kind() == ActPass {
					return a
				}
			}
		}
		if len(acts) > 0 {
			d := *c.D
			d.Options = acts
			pick := rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			if c.G != nil && cardName(c, pick) == "Clever Impersonator" && pwOnBoard(c) {
				needPW = true
			}
			return pick
		}
		if c.G == nil {
			return rnd.Pick(c)
		}
		var tpls []Canon
		for _, a := range c.D.Options {
			if o, ok := c.G.GorgeOption(a); ok && o.Kind == "clone" && o.Obj != 0 {
				tpls = append(tpls, a)
			}
		}
		if len(tpls) == 0 {
			return rnd.Pick(c)
		}
		filter := func(want func(*cards.Face) bool) (out []Canon) {
			for _, a := range tpls {
				if o, _ := c.G.GorgeOption(a); o.Obj != 0 {
					if f := c.G.E.G.Obj(o.Obj).Face(); f != nil && want(f) {
						out = append(out, a)
					}
				}
			}
			return out
		}
		var chosen []Canon
		if needPW {
			chosen = filter((*cards.Face).IsPlaneswalker)
			if len(chosen) > 0 {
				needPW, done[0] = false, true
			}
		}
		if len(chosen) == 0 {
			for i := range pref {
				if done[i] {
					continue
				}
				if chosen = filter(pref[i]); len(chosen) > 0 {
					done[i] = true
					break
				}
			}
		}
		if len(chosen) == 0 {
			chosen = filter(pref[cloneSeq%4])
			cloneSeq++
		}
		if len(chosen) == 0 {
			chosen = tpls
		}
		d := *c.D
		d.Options = chosen
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// clonenoncreaturePlay plays one seed at seats decks, returning the divergence
// result and the copies the gorge log reached.
func clonenoncreaturePlay(t *testing.T, set cardset.Set, decks [][]cardset.Entry, seed uint64, maxTurns int32) (Result, cloneCopies) {
	t.Helper()
	reg := synthReg(t)
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(decks[0]), expand(decks[1])},
		Seed: seed, PassElision: true, Surface: "full"}
	if len(decks) > 2 {
		r.MoreDecks = []string{"x2", "x3"}
		r.MoreCards = [][]string{expand(decks[2]), expand(decks[3])}
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	var c cloneCopies
	rc.Finish = func(g *GorgeSide) { c = countCloneCopies(g) }
	tbl, _, err := set.MatchupN(decks)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, seed, lead, k, clonenoncreaturePolicy(seed), rc)
	return res, c
}

func clonenoncreatureFixture(t *testing.T) cardset.Set {
	t.Helper()
	set, err := cardset.RawByName("clonenoncreature")
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// TestCloneNonCreatureFourSeatLockstep plays four-seat pods of the
// clonenoncreature fixture: the carriers' as-enters copies of a land, an
// enchantment, an Aura and a planeswalker happen over the seeds, each with the
// copied card's characteristics, the Aura copy attached by its CR 303.4f ask
// and the planeswalker copy entering with the copied card's printed loyalty.
// No divergence and no stall.
func TestCloneNonCreatureFourSeatLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	set := clonenoncreatureFixture(t)
	decks := clonenoncreaturePodDecks()
	var sum cloneCopies
	for _, seed := range []uint64{1, 2, 3, 5} {
		res, c := clonenoncreaturePlay(t, set, decks[:], seed, 35)
		clonenoncreatureChecks(t, seed, res, c)
		sum.land += c.land
		sum.enchant += c.enchant
		sum.aura += c.aura
		sum.planeswalker += c.planeswalker
	}
	if sum.land == 0 || sum.enchant == 0 || sum.aura == 0 || sum.planeswalker == 0 {
		t.Fatalf("copies over the seeds = %+v: want a land, an enchantment, an Aura and a planeswalker copy", sum)
	}
}

// TestCloneNonCreatureTwoSeatLockstep is the same carriers at two seats.
func TestCloneNonCreatureTwoSeatLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	set := clonenoncreatureFixture(t)
	decks := clonenoncreatureDecks()
	var sum cloneCopies
	for _, seed := range []uint64{1, 2, 3, 4, 5, 6} {
		res, c := clonenoncreaturePlay(t, set, [][]cardset.Entry{decks[0], decks[1]}, seed, 35)
		clonenoncreatureChecks(t, seed, res, c)
		sum.land += c.land
		sum.enchant += c.enchant
		sum.aura += c.aura
		sum.planeswalker += c.planeswalker
	}
	if sum.land == 0 || sum.enchant == 0 || sum.aura == 0 || sum.planeswalker == 0 {
		t.Fatalf("copies over the seeds = %+v: want a land, an enchantment, an Aura and a planeswalker copy", sum)
	}
}

// cloneNewNameDecks are the two-seat NewName$ decks: Chameleon, Master of
// Disguise (an as-enters copy with NewName$) and Irma, Part-Time Mutant (a
// resolving Clone with NewName$ and GainThisAbility$) over Grizzly Bears, with
// no other copy effect (so the copy target is never a copy).
func cloneNewNameDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Chameleon, Master of Disguise", Count: 4}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Forest", Count: 10}, {Name: "Island", Count: 10},
			{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 4},
		},
		{
			{Name: "Grizzly Bears", Count: 8}, {Name: "Forest", Count: 12},
			{Name: "Plains", Count: 12}, {Name: "Island", Count: 8},
		},
	}
}

// cloneResolvingNewNameDecks are the two-seat resolving-Clone NewName$ decks:
// Irma, Part-Time Mutant (a resolving Clone with NewName$ and
// GainThisAbility$) over Grizzly Bears, with no other copy effect.
func cloneResolvingNewNameDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Irma, Part-Time Mutant", Count: 4}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Forest", Count: 10}, {Name: "Island", Count: 10},
			{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 4},
		},
		{
			{Name: "Grizzly Bears", Count: 8}, {Name: "Forest", Count: 12},
			{Name: "Plains", Count: 12}, {Name: "Island", Count: 8},
		},
	}
}

// TestCloneNonCreatureResolvingNewNameLockstep plays Irma's resolving Clone:
// the copy is named Irma (CR 707.9b) and keeps her begin-combat trigger, which
// the kernel lists as a latent trigger. No divergence and no stall; the copy
// happens over the seeds.
func TestCloneNonCreatureResolvingNewNameLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	set := clonenoncreatureFixture(t)
	decks := cloneResolvingNewNameDecks()
	var sum cloneCopies
	for _, seed := range []uint64{1, 2, 3, 4} {
		res, c := clonenoncreaturePlay(t, set, [][]cardset.Entry{decks[0], decks[1]}, seed, 35)
		clonenoncreatureChecks(t, seed, res, c)
		sum.irma += c.irma
	}
	if sum.irma == 0 {
		t.Fatalf("NewName$ copies over the seeds = %+v: want an Irma copy", sum)
	}
}

// TestCloneNonCreatureNewNameLockstep plays the two NewName$ paths: Chameleon's
// as-enters copy is named Chameleon (CR 707.9b) and Irma's resolving Clone is
// named Irma while keeping her own begin-combat trigger. Both names are copy
// exceptions, not the copied card's name, and the kept trigger is the kernel's
// latent one. No divergence and no stall; both copies happen over the seeds.
func TestCloneNonCreatureNewNameLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	set := clonenoncreatureFixture(t)
	decks := cloneNewNameDecks()
	var sum cloneCopies
	for _, seed := range []uint64{1, 2, 3, 4} {
		res, c := clonenoncreaturePlay(t, set, [][]cardset.Entry{decks[0], decks[1]}, seed, 35)
		clonenoncreatureChecks(t, seed, res, c)
		sum.chameleon += c.chameleon
		sum.irma += c.irma
	}
	if sum.chameleon == 0 {
		t.Fatalf("NewName$ copies over the seeds = %+v: want a Chameleon copy", sum)
	}
}

// TestCloneNonCreatureLoyaltyMutation is the mutation test: a planeswalker
// copy entering with no loyalty counters dies to the state-based action, where
// gorge keeps it, so the run must diverge.
func TestCloneNonCreatureLoyaltyMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	set := clonenoncreatureFixture(t)
	decks := clonenoncreatureDecks()
	dirac.DebugCloneNoLoyalty = true
	defer func() { dirac.DebugCloneNoLoyalty = false }()
	for seed := uint64(1); seed <= 12; seed++ {
		res, c := clonenoncreaturePlay(t, set, [][]cardset.Entry{decks[0], decks[1]}, seed, 35)
		t.Logf("mutation seed %d: copies %+v, divergence %v", seed, c, res.Div != nil)
		if c.planeswalker == 0 {
			continue // this seed never copies a planeswalker, so the mutation has nothing to break
		}
		if res.Div == nil {
			t.Fatalf("seed %d: a loyalty-less planeswalker copy (%+v) diverged on no decision", seed, c)
		}
		return
	}
	t.Fatal("no seed copied a planeswalker, so the mutation was not exercised")
}
