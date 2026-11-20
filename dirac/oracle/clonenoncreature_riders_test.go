package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The riders of the as-enters non-creature copy (gap4-03 findings): a copied
// Saga enters with its lore counter (CR 702.151a), a copied Battle with its
// defense counters and protector (CR 310.4, 310.10), a copied Aura whose own
// pre-entry choice follows the CR 303.4f host ask, a clone that elects an Aura
// template on a blink's return, and a NewName$ copy at four seats. Each test is
// a lockstep over decks built for its one shape.

// cloneRiderOnBoard counts permanents of the kind the rider policy waits for.
func cloneRiderOnBoard(c *PickCtx, want func(*cards.Face) bool) int {
	if c.G == nil {
		return 0
	}
	n := 0
	for p := 0; p < len(c.G.E.G.Players); p++ {
		for _, id := range c.G.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := c.G.E.G.Obj(id); o != nil && o.Face() != nil && want(o.Face()) {
				n++
			}
		}
	}
	return n
}

// cloneRiderIsAura and cloneRiderIsSaga classify a face for the rider policy.
func cloneRiderIsAura(f *cards.Face) bool {
	return f.IsEnchantment() && slices.Contains(f.Types, "Aura")
}

func cloneRiderIsSaga(f *cards.Face) bool { return slices.Contains(f.Types, "Saga") }

// cloneRiderIsCopyAura is a battlefield Aura that entered as a copy.
func cloneRiderIsCopyAura(o *state.Object) bool {
	return o.CopyFace != nil && o.Face() != nil && cloneRiderIsAura(o.Face())
}

// cloneRiderPolicy drives the rider decks: it holds each carrier until the
// template it needs is on the battlefield, keeps the board at one creature
// where an Aura's host ask must have exactly one candidate, and aims a blink at
// a copied Aura instead of a printed one. Everything else is the shared random
// policy, so the clone election still faces the real option list.
func cloneRiderPolicy(seed uint64, holdCreatures bool) Policy {
	rnd := RandomPolicy(seed)
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
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		if c.D.Kind == DTarget {
			var first Canon
			haveFirst := false
			for _, a := range c.D.Options {
				if a.Kind() != ActTarget {
					continue
				}
				if !haveFirst {
					first, haveFirst = a, true
				}
				if o, ok := c.G.GorgeOption(a); ok && o.Obj != 0 {
					if obj := c.G.E.G.Obj(o.Obj); obj != nil && cloneRiderIsCopyAura(obj) {
						return a // the blink's target: the Aura copy, not a printed Aura
					}
				}
			}
			if haveFirst {
				return first
			}
		}
		if c.D.Kind == DTriggerOptional {
			for _, a := range c.D.Options {
				if a.Kind() == ActYes {
					return a
				}
			}
		}
		creatures := cloneRiderOnBoard(c, (*cards.Face).IsCreature)
		auras := cloneRiderOnBoard(c, cloneRiderIsAura)
		sagas := cloneRiderOnBoard(c, cloneRiderIsSaga)
		battles := cloneRiderOnBoard(c, (*cards.Face).IsBattle)
		var acts []Canon
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActCastFace:
				switch cardName(c, a) {
				case "Copy Enchantment":
					// wait for an enchantment template; when the template is
					// Flickering Ward keep exactly one host so the template's
					// own choice, not the two-host stall, is exercised
					if sagas == 0 && auras == 0 {
						continue
					}
					if cloneRiderOnBoard(c, func(f *cards.Face) bool {
						return f.IsEnchantment() && slices.Contains(f.Types, "Aura") && f.Name == "Flickering Ward"
					}) > 0 && creatures != 1 {
						continue
					}
				case "Clever Impersonator":
					if auras == 0 && battles == 0 {
						continue
					}
				case "Grizzly Bears":
					if holdCreatures && creatures >= 1 {
						continue // one host is enough for the Aura-choice ask
					}
				case "Flickering Ward", "Divine Favor":
					if creatures == 0 {
						continue
					}
				case "Flicker":
					if !cloneRiderAnyCopyAura(c) {
						continue // wait for the Aura copy to blink
					}
				}
				acts = append(acts, a)
			case ActActivate, ActPlayLand:
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
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		// the clone election: prefer the template this deck exercises (a
		// Battle, a Saga, then an Aura), else copy one of whatever is there.
		var tpls []Canon
		for _, a := range c.D.Options {
			if o, ok := c.G.GorgeOption(a); ok && o.Kind == "clone" && o.Obj != 0 {
				tpls = append(tpls, a)
			}
		}
		if len(tpls) > 0 {
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
			for _, want := range []func(*cards.Face) bool{(*cards.Face).IsBattle, cloneRiderIsSaga, cloneRiderIsAura} {
				if chosen := filter(want); len(chosen) > 0 {
					d := *c.D
					d.Options = chosen
					return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
				}
			}
			d := *c.D
			d.Options = tpls
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		return rnd.Pick(c)
	})
}

// cloneRiderAnyCopyAura reports a battlefield Aura that is a copy.
func cloneRiderAnyCopyAura(c *PickCtx) bool {
	if c.G == nil {
		return false
	}
	for p := 0; p < len(c.G.E.G.Players); p++ {
		for _, id := range c.G.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := c.G.E.G.Obj(id); o != nil && cloneRiderIsCopyAura(o) {
				return true
			}
		}
	}
	return false
}

// cloneRiderPlay is clonenoncreaturePlay with the rider policy.
func cloneRiderPlay(t *testing.T, set cardset.Set, decks [][]cardset.Entry, seed uint64, hold bool) (Result, cloneCopies) {
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
	rc.MaxDecisions, rc.MaxTurns = 8000, 35
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
	return LockstepLed(m, seed, lead, k, cloneRiderPolicy(seed, hold), rc), c
}

// cloneSagaDecks: Copy Enchantment copies Binding the Old Gods. The Saga copy
// must enter with a lore counter so chapter I (a target ask) fires.
func cloneSagaDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Copy Enchantment", Count: 4}, {Name: "Binding the Old Gods", Count: 4},
			{Name: "Grizzly Bears", Count: 1}, {Name: "Plains", Count: 8},
			{Name: "Island", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Swamp", Count: 7},
		},
		{
			{Name: "Binding the Old Gods", Count: 4}, {Name: "Grizzly Bears", Count: 1},
			{Name: "Plains", Count: 10}, {Name: "Forest", Count: 10},
			{Name: "Swamp", Count: 8}, {Name: "Island", Count: 6},
		},
	}
}

// cloneBattleDecks: Clever Impersonator copies Invasion of Segovia. The Battle
// copy must enter with its printed defense counters and a protector, so the
// ETB token trigger and the attackable battle line up.
func cloneBattleDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Clever Impersonator", Count: 4}, {Name: "Invasion of Segovia", Count: 4},
			{Name: "Island", Count: 14}, {Name: "Plains", Count: 10},
			{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
		},
		{
			{Name: "Invasion of Segovia", Count: 4}, {Name: "Island", Count: 12},
			{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 6},
		},
	}
}

// cloneAuraChoiceDecks: Copy Enchantment copies Flickering Ward, whose own
// "as this enters, choose a color" must follow the CR 303.4f host ask. The
// board is held to one creature so the one-host path is exercised.
func cloneAuraChoiceDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Copy Enchantment", Count: 4}, {Name: "Flickering Ward", Count: 4},
			{Name: "Grizzly Bears", Count: 2}, {Name: "Plains", Count: 10},
			{Name: "Island", Count: 10}, {Name: "Forest", Count: 6}, {Name: "Swamp", Count: 4},
		},
		{
			{Name: "Plains", Count: 14}, {Name: "Forest", Count: 14},
			{Name: "Island", Count: 8}, {Name: "Swamp", Count: 4},
		},
	}
}

// cloneBlinkDecks: Flicker blinks a Clever Impersonator that entered as a copy
// of Divine Favor. The returned copy must ask its host again (CR 303.4f), not
// stop the kernel.
func cloneBlinkDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Clever Impersonator", Count: 4}, {Name: "Divine Favor", Count: 4},
			{Name: "Flicker", Count: 4}, {Name: "Grizzly Bears", Count: 2},
			{Name: "Plains", Count: 10}, {Name: "Island", Count: 10},
			{Name: "Forest", Count: 6}, {Name: "Swamp", Count: 4},
		},
		{
			{Name: "Grizzly Bears", Count: 1}, {Name: "Plains", Count: 14},
			{Name: "Forest", Count: 14}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 3},
		},
	}
}

// TestCloneNonCreatureSagaBattleLockstep copies a Saga and a Battle at two
// seats (a Battle is two-seat in the kernel). Both copies must carry their
// entry state: the Saga's lore counter and the Battle's defense and protector.
func TestCloneNonCreatureSagaBattleLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	sagaSet := clonenoncreatureFixture(t)
	sagaDecks := cloneSagaDecks()
	var sum cloneCopies
	for _, seed := range []uint64{1, 2, 3, 4, 5, 6} {
		res, c := cloneRiderPlay(t, sagaSet, sagaDecks[:], seed, false)
		clonenoncreatureChecks(t, seed, res, c)
		sum.saga += c.saga
	}
	if sum.saga == 0 {
		t.Fatalf("copies over the seeds = %+v: want a Saga copy", sum)
	}
	// A Battle is two-seat in the kernel, so its pool is separate from the
	// four-seat fixture.
	battleSet, err := cardset.RawByName("clonenoncreaturebattle")
	if err != nil {
		t.Fatal(err)
	}
	battleDecks := cloneBattleDecks()
	for _, seed := range []uint64{1, 2, 3, 4, 5, 6} {
		res, c := cloneRiderPlay(t, battleSet, battleDecks[:], seed, false)
		clonenoncreatureChecks(t, seed, res, c)
		sum.battle += c.battle
	}
	if sum.battle == 0 {
		t.Fatalf("copies over the seeds = %+v: want a Battle copy", sum)
	}
}

// TestCloneNonCreatureAuraChoiceLockstep copies Flickering Ward with one legal
// host: the template's own colour choice is asked before the move, after the
// (absent) host ask.
func TestCloneNonCreatureAuraChoiceLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	set := clonenoncreatureFixture(t)
	decks := cloneAuraChoiceDecks()
	var sum cloneCopies
	for seed := uint64(1); seed <= 24; seed++ {
		res, c := cloneRiderPlay(t, set, decks[:], seed, true)
		clonenoncreatureChecks(t, seed, res, c)
		sum.aura += c.aura
	}
	if sum.aura == 0 {
		t.Fatalf("copies over the seeds = %+v: want a Flickering Ward copy", sum)
	}
}

// TestCloneNonCreatureBlinkAuraLockstep blinks an Aura copy: its return asks
// the CR 303.4f host again instead of stopping at StallCopyAllAsk.
func TestCloneNonCreatureBlinkAuraLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	set := clonenoncreatureFixture(t)
	decks := cloneBlinkDecks()
	var sum cloneCopies
	for seed := uint64(1); seed <= 24; seed++ {
		res, c := cloneRiderPlay(t, set, decks[:], seed, false)
		clonenoncreatureChecks(t, seed, res, c)
		sum.aura += c.aura
	}
	if sum.aura == 0 {
		t.Fatalf("copies over the seeds = %+v: want an Aura copy to blink", sum)
	}
}

// TestCloneNonCreatureFourSeatNewNameLockstep is the NewName$ paths at four
// seats: Chameleon's as-enters copy and Irma's resolving Clone both carry the
// named card's name (CR 707.9b) into the legend rule.
func TestCloneNonCreatureFourSeatNewNameLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	set := clonenoncreatureFixture(t)
	decks := [4][]cardset.Entry{
		cloneNewNameDecks()[0], cloneNewNameDecks()[1],
		cloneResolvingNewNameDecks()[0], cloneResolvingNewNameDecks()[1],
	}
	var sum cloneCopies
	for _, seed := range []uint64{1, 2, 3, 4} {
		res, c := cloneRiderPlay(t, set, decks[:], seed, false)
		clonenoncreatureChecks(t, seed, res, c)
		sum.chameleon += c.chameleon
		sum.irma += c.irma
	}
	if sum.chameleon == 0 || sum.irma == 0 {
		t.Fatalf("NewName$ copies over the seeds = %+v: want a Chameleon and an Irma copy", sum)
	}
}
