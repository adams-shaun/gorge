package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestUmbraArmorGraveyardOrderReprosLockstep replays a game where a lethal-
// damage SBA batch destroys an umbra Aura whose host it saves. The Aura must
// enter the graveyard at its host's battlefield position (gorge's per-victim
// walk), not at its own slot.
func TestUmbraArmorGraveyardOrderReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "umbra_armor_graveyard_order")
}

// umbraOrderDeck is a focused W/U/R pod: Memnites for the umbra Auras to
// enchant (1/1 under Felidar, 2/2 under Eel Umbra; both die to Pyroclasm's 2
// damage), Pyroclasm to kill several creatures in one SBA batch, and lands.
func umbraOrderDeck() []string {
	var d []string
	for range 12 {
		d = append(d, "Memnite")
	}
	for range 8 {
		d = append(d, "Felidar Umbra")
	}
	for range 4 {
		d = append(d, "Eel Umbra")
	}
	for range 4 {
		d = append(d, "Pyroclasm")
	}
	for range 5 {
		d = append(d, "Plains")
	}
	for range 4 {
		d = append(d, "Island")
	}
	for range 3 {
		d = append(d, "Mountain")
	}
	return d
}

// umbraOrderPolicy casts in preference to playing a land, so the deck reaches
// an enchanted creature and a sweep inside the pod's turn cap; other decisions
// fall to the random policy.
func umbraOrderPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var cast, land []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DPriority {
			cast, land = cast[:0], land[:0]
			for _, o := range c.D.Options {
				switch o.Kind() {
				case ActCast:
					cast = append(cast, o)
				case ActPlayLand:
					land = append(land, o)
				}
			}
			if len(cast) > 0 {
				d := *c.D
				d.Options = cast
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
			if len(land) > 0 {
				d := *c.D
				d.Options = land
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
			if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPass }); ok {
				return a
			}
		}
		return rnd.Pick(c)
	})
}

// umbraOrderTally counts umbra armor answers: the Aura went to the graveyard
// and the host it enchanted stayed on the battlefield. It tracks attachment
// from the Attach events so the host is known after the Aura has moved.
type umbraOrderTally struct {
	last   int
	hostOf map[state.ObjID]state.ObjID
	spares int
}

func (u *umbraOrderTally) reset() {
	u.last = 0
	u.hostOf = map[state.ObjID]state.ObjID{}
}

func (u *umbraOrderTally) check(g *GorgeSide, _ Side) *Divergence {
	evs := g.E.L.Events
	for ; u.last < len(evs); u.last++ {
		e := &evs[u.last]
		switch e.Kind {
		case events.Attach:
			if len(e.IDs) > 0 {
				u.hostOf[e.Obj] = e.IDs[0]
			} else {
				delete(u.hostOf, e.Obj)
			}
		case events.MoveZone:
			if e.From != state.ZBattlefield || e.To != state.ZGraveyard {
				continue
			}
			o := g.E.G.Obj(e.Obj)
			if o == nil || o.Face() == nil || !o.Face().HasKeyword("Umbra armor") {
				continue
			}
			host := u.hostOf[e.Obj]
			if host == 0 {
				continue
			}
			if h := g.E.G.Obj(host); h != nil && h.Zone == state.ZBattlefield {
				u.spares++
			}
		}
	}
	return nil
}

// TestUmbraArmorGraveyardOrderFourSeat plays 20 four-seat pods: a damage
// sweep kills several creatures at once, an umbra Aura saves its host, and
// both engines must agree on the Aura's place among the casualties in every
// seat's walk.
func TestUmbraArmorGraveyardOrderFourSeat(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := umbraOrderDeck()
	tally := &umbraOrderTally{}
	base := fourSeatSetPlay(t, printedDelveSet(t, [2][]string{deck, deck}),
		[4][]string{deck, deck, deck, deck}, nil, tally.check, nil,
		func(seed uint64) Policy { return umbraOrderPolicy(seed) })
	play := func(seed uint64) Result {
		tally.reset()
		return base(seed)
	}
	const seeds = 20
	fourSeatAtkClean(t, play, seeds)
	t.Logf("%d umbra armor answers saved their host", tally.spares)
	if tally.spares == 0 {
		t.Fatal("no umbra armor answer saved its host: the mechanism was never reached")
	}
}

// TestUmbraArmorGraveyardOrderMutation restores the old own-slot order build:
// the recorded game then diverges at the graveyard order again.
func TestUmbraArmorGraveyardOrderMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "umbra_armor_graveyard_order.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugUmbraOwnSlotOrder
	dirac.DebugUmbraOwnSlotOrder = true
	defer func() { dirac.DebugUmbraOwnSlotOrder = old }()
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatal("the own-slot order mutation produced no divergence")
	}
	t.Logf("mutation diverged at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
}

// umbraDestroyAllDeck is the same pod with Wrath of God instead of a damage
// sweep: the destroy-all's own pre-read answer must put each Aura at its
// host's position just as the lethal-damage pass does.
func umbraDestroyAllDeck() []string {
	var d []string
	for range 12 {
		d = append(d, "Memnite")
	}
	for range 8 {
		d = append(d, "Felidar Umbra")
	}
	for range 4 {
		d = append(d, "Eel Umbra")
	}
	for range 4 {
		d = append(d, "Wrath of God")
	}
	for range 8 {
		d = append(d, "Plains")
	}
	for range 4 {
		d = append(d, "Island")
	}
	return d
}

// TestUmbraArmorGraveyardOrderDestroyAll checks the destroy-all choke point
// (shieldUmbraAllLeave): the Aura goes at its host's position, and the old
// own-slot build diverges.
func TestUmbraArmorGraveyardOrderDestroyAll(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := umbraDestroyAllDeck()
	tally := &umbraOrderTally{}
	base := fourSeatSetPlay(t, printedDelveSet(t, [2][]string{deck, deck}),
		[4][]string{deck, deck, deck, deck}, nil, tally.check, nil,
		func(seed uint64) Policy { return umbraOrderPolicy(seed) })
	play := func(seed uint64) Result {
		tally.reset()
		return base(seed)
	}
	const seeds = 12
	fourSeatAtkClean(t, play, seeds)
	t.Logf("%d umbra armor answers saved their host in a destroy-all", tally.spares)
	if tally.spares == 0 {
		t.Fatal("no destroy-all umbra armor answer saved its host: the mechanism was never reached")
	}
	old := dirac.DebugUmbraOwnSlotOrder
	dirac.DebugUmbraOwnSlotOrder = true
	defer func() { dirac.DebugUmbraOwnSlotOrder = old }()
	for seed := uint64(1); seed <= seeds; seed++ {
		if res := play(seed); res.Div != nil {
			t.Logf("mutation: seed %d diverges at %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("the own-slot order produced no divergence in a destroy-all game")
}

// umbraVictimAuraDeck: white creatures and white umbra Auras with Anarchy
// (destroy all white permanents), so the Aura is itself a victim of the
// destroy-all and sits in a lower or higher seat than the host it saves.
func umbraVictimAuraDeck() []string {
	var d []string
	for range 12 {
		d = append(d, "Savannah Lions")
	}
	for range 10 {
		d = append(d, "Hyena Umbra")
	}
	for range 4 {
		d = append(d, "Anarchy")
	}
	for range 7 {
		d = append(d, "Plains")
	}
	for range 7 {
		d = append(d, "Mountain")
	}
	return d
}

// TestUmbraArmorGraveyardOrderDestroyAllVictimAura covers the destroy-all
// whose victims include the umbra Aura itself: gorge destroys it at its own
// place when that comes before its host's and at the host's otherwise, so the
// engines must agree across both seat orders, and an order build that always
// moves the Aura to its host (the bug this guards) or never does diverges.
func TestUmbraArmorGraveyardOrderDestroyAllVictimAura(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := umbraVictimAuraDeck()
	tally := &umbraOrderTally{}
	base := fourSeatSetPlay(t, printedDelveSet(t, [2][]string{deck, deck}),
		[4][]string{deck, deck, deck, deck}, nil, tally.check, nil,
		func(seed uint64) Policy { return umbraOrderPolicy(seed) })
	play := func(seed uint64) Result {
		tally.reset()
		return base(seed)
	}
	const seeds = 20
	fourSeatAtkClean(t, play, seeds)
	t.Logf("%d umbra armor answers saved their host beside a victim Aura", tally.spares)
	if tally.spares == 0 {
		t.Fatal("no destroy-all umbra armor answer saved its host: the mechanism was never reached")
	}
	old := dirac.DebugUmbraOwnSlotOrder
	dirac.DebugUmbraOwnSlotOrder = true
	defer func() { dirac.DebugUmbraOwnSlotOrder = old }()
	for seed := uint64(1); seed <= seeds; seed++ {
		if res := play(seed); res.Div != nil {
			t.Logf("mutation: seed %d diverges at %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("the own-slot order produced no divergence in a victim-Aura destroy-all game")
}
