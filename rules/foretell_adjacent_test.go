package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestForetoldPredicateMatchesFlaggedExile(t *testing.T) {
	t.Parallel()
	g := state.NewGame([]string{"A"})
	o := g.AddObject(card(t, "Name:Foretold\nManaCost:3\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	o.Zone = state.ZExile
	o.CastFlags |= state.FlagForetold
	if !effects.MatchesSpecCtx(g, "Card.foretold+YouOwn", o.ID, effects.SpecContext{}) {
		t.Fatal("foretold object predicate did not match a flagged exiled card")
	}
	o.CastFlags = 0
	if effects.MatchesSpecCtx(g, "Card.foretold+YouOwn", o.ID, effects.SpecContext{}) {
		t.Fatal("foretold object predicate matched an unflagged card")
	}
}

func TestForetoldCostUsesPrintedManaMinusTwo(t *testing.T) {
	t.Parallel()
	f := card(t, "Name:Marked\nManaCost:4 W\nTypes:Creature\nPT:2/2\nOracle:x\n").Faces[0]
	got, ok := foretellCost(f)
	if !ok {
		t.Fatal("printed-cost ForetoldCost shape was not priceable")
	}
	if got.Generic != 2 || got.Colored[state.MW] != 1 {
		t.Fatalf("foretell cost = %+v, want {2}{W}", got)
	}
}

// countForetoldInExile counts seat p's exile cards carrying FlagForetold.
func countForetoldInExile(e *Engine, p state.PlayerID) int {
	n := 0
	for _, id := range e.G.Zone(state.ZExile, p) {
		if o := e.G.Obj(id); o != nil && o.CastFlags&state.FlagForetold != 0 {
			n++
		}
	}
	return n
}

// foretellByLabel foretells the hand card named by the option's label -- the
// labelled variant of foretellIt, for a board where the foretell action's own
// exile queues triggers (Ranar's Spirit mint) that must drain before the
// caller reads state. Returns the exiled object.
func foretellByLabel(t *testing.T, e *Engine, label string) *state.Object {
	t.Helper()
	opts := e.legalActions(0)
	idx := optionByLabel(opts, label)
	if idx < 0 {
		t.Fatalf("no foretell option %q in %+v", label, opts)
	}
	opt := opts[idx]
	e.beginCast(0, opt)
	e.priorityRound()
	answerQuiet(t, e, 60)
	o := e.G.Obj(opt.Obj)
	if o.Zone != state.ZExile || o.CastFlags&state.FlagForetold == 0 || !o.FaceDown {
		t.Fatalf("%s: zone=%s flags=%#x faceDown=%v, want exile/FlagForetold/face-down",
			label, o.Zone, o.CastFlags, o.FaceDown)
	}
	return o
}

// TestEtherealValkyrieMarksExiledHandCardForetold drives the real corpus
// card end to end: its ETB trigger draws and then exiles a hand card with
// Foretold$ True | ForetoldCost$ True, the one-event marker encoding lands
// FlagForetold through events.Apply, and on a later turn the card -- which
// carries NO K:Foretell line of its own -- is offered the foretell-cast at
// its mana cost less {2} and the cast charges exactly that. This is the
// regression pin for the no-keyword route: the old keyword-only gate in
// legal.go's exile walk withheld the offer entirely.
func TestEtherealValkyrieMarksExiledHandCardForetold(t *testing.T) {
	t.Parallel()
	marked := card(t, "Name:Marked One\nManaCost:4 W\nTypes:Creature\nPT:2/2\nOracle:x\n")
	e := handEngine(t, corpusAlternativeCard(t, "Ethereal Valkyrie"), marked)
	hand := e.G.Zone(state.ZHand, 0)
	vid, mid := hand[0], hand[1]
	if _, ok := e.G.Obj(mid).Face().KeywordParam("Foretell"); ok {
		t.Fatal("precondition: the marked card must have no K:Foretell line of its own")
	}
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MU], e.G.Players[0].Pool[state.MW] = 4, 1, 1
	castMode(t, e, vid, "")
	finishCast(t, e, vid)
	if v := e.G.Obj(vid); v.Zone != state.ZBattlefield {
		t.Fatalf("valkyrie in %s, want battlefield", v.Zone)
	}
	e.priorityRound()
	answerQuiet(t, e, 60)
	// Precondition for the deterministic exile pick: the trigger drew a
	// Mountain (the fixture deck is all Mountains) AFTER the marked card, so
	// the first-eligible hand pick is the marked one and the Mountain stays.
	if mo := e.G.Obj(mid); mo.Zone != state.ZExile || !mo.FaceDown || mo.CastFlags&state.FlagForetold == 0 {
		t.Fatalf("marked card: zone=%s faceDown=%v flags=%#x, want exile/face-down/FlagForetold",
			mo.Zone, mo.FaceDown, mo.CastFlags)
	}
	mountains := 0
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Mountain" {
			mountains++
		}
	}
	if mountains != 1 {
		t.Fatalf("hand holds %d Mountains after the trigger, want exactly the drawn 1", mountains)
	}
	if got := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.MoveZone && ev.To == state.ZExile && ev.Counter == "exiled_with_face_down_foretold"
	}); got != 1 {
		t.Fatalf("effect-granted foretelling emitted %d marker MoveZone events, want exactly 1", got)
	}
	if got := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.MoveZone && ev.To == state.ZExile && ev.Counter == "exiled_with_face_down"
	}); got != 0 {
		t.Fatalf("the effect grant reused the {2} action's counter %d times, want 0 (log-distinguishable)", got)
	}
	// Later turn, unfunded: {2}{W} is not payable from an empty pool -- no offer.
	driveToTurn3Main(t, e)
	if optionByLabel(e.legalActions(0), "Cast Marked One (foretold)") >= 0 {
		t.Error("foretell cast offered at {2}{W} from an empty pool")
	}
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MW] = 2, 1
	submitOption(t, e, "foretell_cast", "Cast Marked One (foretold)")
	if p := e.G.Players[0].Pool; p[state.MC] != 0 || p[state.MW] != 0 {
		t.Fatalf("foretell cast charged pool %v, want exactly the derived {2}{W}", p)
	}
	finishCast(t, e, mid)
	if mo := e.G.Obj(mid); mo.Zone != state.ZBattlefield {
		t.Fatalf("cast marked card in %s, want battlefield", mo.Zone)
	}
}

// TestForetoldSoldierBattlefieldExileSurvivesFlagReset drives the corpus
// card's damage trigger: the soldier deals combat damage on turn 3 and its
// trigger exiles ITSELF from the battlefield. This is the CastFlags-reset
// hazard's regression pin -- Move zeroes a battlefield leaver's cast flags
// before the decode switch re-ORs FlagForetold from the marker counter -- and
// the later cast pays the card's own K:Foretell colon cost {1}{G}.
func TestForetoldSoldierBattlefieldExileSurvivesFlagReset(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "The Foretold Soldier"))
	sid := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MG] = 2, 2
	castMode(t, e, sid, "")
	finishCast(t, e, sid)
	if s := e.G.Obj(sid); s.Zone != state.ZBattlefield {
		t.Fatalf("soldier in %s, want battlefield", s.Zone)
	}
	// Combat on turn 3 (the soldier is summoning sick on turn 1): it attacks
	// alone against a Mountains-only defender, so it is unblocked (the
	// must-block requirement has no creature to bind) and its 6 damage lands.
	e.priorityRound()
	driveToStep(t, e, 3, 0, state.StepDeclareAttackers)
	e.askAttackers()
	submitAttackers(t, e, sid)
	if got := e.G.Players[1].Life; got != 14 {
		t.Fatalf("defender life = %d, want 14 (the 6/2 soldier hit unblocked)", got)
	}
	e.priorityRound()
	answerQuiet(t, e, 60)
	// THE HAZARD: the battlefield->exile Move reset the flags; the marker
	// decode re-applied the designation after it.
	if s := e.G.Obj(sid); s.Zone != state.ZExile || !s.FaceDown || s.CastFlags&state.FlagForetold == 0 {
		t.Fatalf("soldier after its damage trigger: zone=%s faceDown=%v flags=%#x, want exile/face-down/FlagForetold",
			s.Zone, s.FaceDown, s.CastFlags)
	}
	if got := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.MoveZone && ev.To == state.ZExile && ev.Counter == "exiled_with_face_down_foretold"
	}); got != 1 {
		t.Fatalf("the soldier's self-exile emitted %d marker MoveZone events, want 1", got)
	}
	// Same turn: the marker's TurnChange scan withholds the cast offer.
	if optionByLabel(e.legalActions(0), "Cast The Foretold Soldier (foretold)") >= 0 {
		t.Error("foretell cast offered on the exile turn itself")
	}
	// Later turn: the K:Foretell colon param prices the cast {1}{G}.
	e.priorityRound()
	driveToStep(t, e, 5, 0, state.StepMain1)
	if optionByLabel(e.legalActions(0), "Cast The Foretold Soldier (foretold)") >= 0 {
		t.Error("foretell cast offered from an empty pool")
	}
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MG] = 1, 1
	submitOption(t, e, "foretell_cast", "Cast The Foretold Soldier (foretold)")
	if p := e.G.Players[0].Pool; p[state.MC] != 0 || p[state.MG] != 0 {
		t.Fatalf("foretell cast charged pool %v, want exactly {1}{G}", p)
	}
	finishCast(t, e, sid)
	if s := e.G.Obj(sid); s.Zone != state.ZBattlefield {
		t.Fatalf("cast soldier in %s, want battlefield", s.Zone)
	}
}

// TestForetoldPredicateCountsThroughNikoChapterI drives the corpus Saga:
// with two foretold cards owned in exile, Niko Defies Destiny's chapter I
// gains 2 life per foretold card -- the Count$ValidExile
// Card.foretold+YouOwn/Times.2 head reading the predicate through the
// Count$ValidExile walk.
func TestForetoldPredicateCountsThroughNikoChapterI(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Niko Defies Destiny"),
		corpusAlternativeCard(t, "Lupine Harbingers"), corpusAlternativeCard(t, "Cosmos Charger"))
	hand := e.G.Zone(state.ZHand, 0)
	niko, l1, l2 := hand[0], hand[1], hand[2]
	e.G.Players[0].Pool[state.MC] = 4
	foretellIt(t, e, l1)
	foretellIt(t, e, l2)
	if p := e.G.Players[0].Pool; p[state.MC] != 0 {
		t.Fatalf("the two foretell actions left %d generic in the pool, want 0", p[state.MC])
	}
	if got := countForetoldInExile(e, 0); got != 2 {
		t.Fatalf("precondition: %d foretold cards owned in exile, want 2", got)
	}
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MW], e.G.Players[0].Pool[state.MU] = 1, 1, 1
	castMode(t, e, niko, "")
	finishCast(t, e, niko)
	e.priorityRound()
	answerQuiet(t, e, 60)
	if n := e.G.Obj(niko); n.Zone != state.ZBattlefield || n.Counter("LORE") != 1 {
		t.Fatalf("niko: zone=%s lore=%d, want battlefield/1", n.Zone, n.Counter("LORE"))
	}
	if got := e.G.Players[0].Life; got != 24 {
		t.Fatalf("life after chapter I = %d, want 24 (20 + 2 foretold x 2 life)", got)
	}
}

// TestForetoldPredicateCountsThroughAlrundPT drives Alrund's static: with
// two foretold cards owned in exile and an empty hand, its
// AddPower$ Z (Z = Count$ValidHand Card.YouOwn + Count$ValidExile
// Card.foretold+YouOwn) makes the 1/1 god read 3/3.
func TestForetoldPredicateCountsThroughAlrundPT(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Alrund, God of the Cosmos"),
		corpusAlternativeCard(t, "Lupine Harbingers"), corpusAlternativeCard(t, "Cosmos Charger"))
	hand := e.G.Zone(state.ZHand, 0)
	aid, l1, l2 := hand[0], hand[1], hand[2]
	e.G.Players[0].Pool[state.MC] = 4
	foretellIt(t, e, l1)
	foretellIt(t, e, l2)
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MU] = 3, 2
	castMode(t, e, aid, "")
	finishCast(t, e, aid)
	e.priorityRound()
	answerQuiet(t, e, 60)
	if a := e.G.Obj(aid); a.Zone != state.ZBattlefield {
		t.Fatalf("alrund in %s, want battlefield", a.Zone)
	}
	if got := countForetoldInExile(e, 0); got != 2 {
		t.Fatalf("precondition: %d foretold cards owned in exile, want 2", got)
	}
	d := e.Derived(aid)
	if d.Power != 3 || d.Toughness != 3 {
		t.Fatalf("alrund P/T = %d/%d, want 3/3 (1/1 + 2 foretold, hand empty)", d.Power, d.Toughness)
	}
}

// TestCosmosChargerReducesForetellActionOnly drives the corpus card's
// ValidSpell$ Static.Foretelling reducer: after the charger is on the
// battlefield at its FULL printed cost (no self-discount -- it was in hand),
// an ordinary spell still pays full price (the Static constraint must not
// match a Spell scope), while the foretell ACTION composes to {1}.
func TestCosmosChargerReducesForetellActionOnly(t *testing.T) {
	t.Parallel()
	bear := card(t, "Name:Vanilla Bear\nManaCost:2 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	e := handEngine(t, corpusAlternativeCard(t, "Cosmos Charger"),
		corpusAlternativeCard(t, "Haunting Voyage"), bear)
	hand := e.G.Zone(state.ZHand, 0)
	cid, vid, bid := hand[0], hand[1], hand[2]
	// The charger cast itself pays full {3}{U}: its static is battlefield-only.
	e.G.Players[0].Pool[state.MU] = 4
	castMode(t, e, cid, "")
	finishCast(t, e, cid)
	if p := e.G.Players[0].Pool; p[state.MU] != 0 {
		t.Fatalf("charger cast left %d blue in the pool, want 0 (full {3}{U} charged)", p[state.MU])
	}
	if c := e.G.Obj(cid); c.Zone != state.ZBattlefield {
		t.Fatalf("charger in %s, want battlefield", c.Zone)
	}
	// Negative branch: an ordinary spell cast pays full price with the
	// charger on the battlefield -- Static.Foretelling must not match a
	// Spell scope.
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MG] = 2, 1
	castMode(t, e, bid, "")
	finishCast(t, e, bid)
	if p := e.G.Players[0].Pool; p[state.MC] != 0 || p[state.MG] != 0 {
		t.Fatalf("ordinary cast charged pool %v, want exactly the full {2}{G}", p)
	}
	// The foretell action composes to {1}: unaffordable from an empty pool,
	// offered at one generic, and charged exactly one.
	if optionByLabel(e.legalActions(0), "Foretell Haunting Voyage") >= 0 {
		t.Error("reduced foretell offered from an empty pool")
	}
	e.G.Players[0].Pool[state.MC] = 1
	submitOption(t, e, "foretell", "Foretell Haunting Voyage")
	if p := e.G.Players[0].Pool; p[state.MC] != 0 {
		t.Fatalf("reduced foretell charged %d generic, want exactly 1", p[state.MC])
	}
	if v := e.G.Obj(vid); v.Zone != state.ZExile || v.CastFlags&state.FlagForetold == 0 {
		t.Fatalf("foretold voyage: zone=%s flags=%#x, want exile/FlagForetold", v.Zone, v.CastFlags)
	}
}

// TestRanarFirstForetellFreeThenFullPrice drives the corpus card's
// Type$ Foretell | FirstForetell$ True reducer: the first foretell ACTION of
// each turn composes to {0}, the second in the same turn pays full {2}, and
// after the next turn change the first-foretell window resets. Ranar's own
// exile trigger minting a Spirit per foretell is asserted as the drive's
// side effect (three foretells -> three Spirits).
func TestRanarFirstForetellFreeThenFullPrice(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Ranar the Ever-Watchful"),
		corpusAlternativeCard(t, "Haunting Voyage"), corpusAlternativeCard(t, "Lupine Harbingers"),
		corpusAlternativeCard(t, "Starnheim Unleashed"))
	e.G.Tokens = testutil.CorpusRegistry(t).Tokens
	hand := e.G.Zone(state.ZHand, 0)
	rid := hand[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MW], e.G.Players[0].Pool[state.MU] = 2, 1, 1
	castMode(t, e, rid, "")
	finishCast(t, e, rid)
	e.priorityRound()
	answerQuiet(t, e, 60)
	if r := e.G.Obj(rid); r.Zone != state.ZBattlefield {
		t.Fatalf("ranar in %s, want battlefield", r.Zone)
	}
	// First foretell of the turn composes to {0}: offered and free from an
	// empty pool.
	if optionByLabel(e.legalActions(0), "Foretell Haunting Voyage") < 0 {
		t.Fatal("the first foretell of the turn was not offered from an empty pool")
	}
	foretellByLabel(t, e, "Foretell Haunting Voyage")
	if p := e.G.Players[0].Pool; p[state.MC] != 0 {
		t.Fatalf("the first foretell charged %d generic, want the reduced {0}", p[state.MC])
	}
	// Second foretell in the SAME turn: the reducer no longer applies ({2}).
	if optionByLabel(e.legalActions(0), "Foretell Lupine Harbingers") >= 0 {
		t.Error("the second foretell of the turn was offered from an empty pool")
	}
	e.G.Players[0].Pool[state.MC] = 2
	foretellByLabel(t, e, "Foretell Lupine Harbingers")
	if p := e.G.Players[0].Pool; p[state.MC] != 0 {
		t.Fatalf("the second foretell charged %d generic, want the full {2}", p[state.MC])
	}
	// After the turn change the first-foretell window resets: {0} again.
	driveToTurn3Main(t, e)
	if optionByLabel(e.legalActions(0), "Foretell Starnheim Unleashed") < 0 {
		t.Fatal("the first foretell of the NEXT turn was not offered from an empty pool")
	}
	foretellByLabel(t, e, "Foretell Starnheim Unleashed")
	if p := e.G.Players[0].Pool; p[state.MC] != 0 {
		t.Fatalf("the next turn's first foretell charged %d generic, want {0}", p[state.MC])
	}
	e.priorityRound()
	answerQuiet(t, e, 60)
	if got := battlefieldNamed(t, e, "Spirit Token"); got != 3 {
		t.Fatalf("ranar minted %d Spirit tokens for three foretells, want 3", got)
	}
}

// TestForetellActionEventEncodingUnchanged pins the {2} action's two-event
// encoding byte-for-byte against the one-event marker the effect grant uses:
// CastInfo carrying FlagForetold immediately followed by a MoveZone with the
// ACTION's counter, and no marker counter anywhere in the log. The
// FirstForetell$ reader distinguishes the action from an effect grant by
// exactly this pair.
func TestForetellActionEventEncodingUnchanged(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Haunting Voyage"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC] = 2
	foretellIt(t, e, id)
	marker, pairs := 0, 0
	for i, ev := range e.L.Events {
		if ev.Kind != events.MoveZone || ev.To != state.ZExile {
			continue
		}
		switch ev.Counter {
		case "exiled_with_face_down_foretold":
			marker++
		case "exiled_with_face_down":
			if i > 0 {
				prev := e.L.Events[i-1]
				if prev.Kind == events.CastInfo && events.FlagsFrom(prev.Counter)&state.FlagForetold != 0 {
					pairs++
				}
			}
		}
	}
	if marker != 0 {
		t.Fatalf("the {2} action emitted %d marker-counter events, want 0", marker)
	}
	if pairs != 1 {
		t.Fatalf("the {2} action's CastInfo(FlagForetold)+MoveZone pair count = %d, want 1", pairs)
	}
}
