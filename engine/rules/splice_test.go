package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const spliceOxSrc = "Name:Splice Ox\nManaCost:4\nTypes:Creature Ox\nPT:4/4\n"
const spliceBearSrc = "Name:Splice Bear\nManaCost:2\nTypes:Creature Bear\nPT:2/2\n"

// spliceEngine is a corpus engine with the named splice cards in seat 0's hand
// (plus the extras) and two creatures: seat 0's Bear and seat 1's Ox.
func spliceEngine(t *testing.T, names ...string) (e *Engine, bear, ox state.ObjID) {
	t.Helper()
	return spliceEngineVs(t, nil, names...)
}

// spliceEngineVs is spliceEngine with extra corpus cards in seat 1's deck.
func spliceEngineVs(t *testing.T, theirNames []string, names ...string) (e *Engine, bear, ox state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var mine []*cards.Card
	for _, n := range names {
		mine = append(mine, lookup(t, reg, n))
	}
	mine = append(mine, card(t, spliceBearSrc))
	theirs := []*cards.Card{card(t, spliceOxSrc)}
	for _, n := range theirNames {
		theirs = append(theirs, lookup(t, reg, n))
	}
	e = corpusEngine(t, reg, mine, theirs)
	for _, n := range names {
		moveByName(t, e, 0, n, state.ZHand)
	}
	bear = moveByName(t, e, 0, "Splice Bear", state.ZBattlefield)
	ox = moveByName(t, e, 1, "Splice Ox", state.ZBattlefield)
	return e, bear, ox
}

// spliceSelect answers the pending splice KChoose with the named cards in the
// given order, failing when it is not a splice decision.
func spliceSelect(t *testing.T, e *Engine, names ...string) *decision.Decision {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "splice" {
		t.Fatalf("not a splice decision: %+v", d)
	}
	var picks []int
	for _, n := range names {
		found := false
		for _, o := range d.Options {
			if so := e.G.Obj(o.Obj); so != nil && so.Face().Name == n {
				picks = append(picks, o.Index)
				found = true
			}
		}
		if !found {
			t.Fatalf("%q not offered for splice: %+v", n, d.Options)
		}
	}
	submitChoices(t, e, picks...)
	return d
}

// spliceTarget answers the pending target ask with the object (or, with obj 0,
// the first option), failing when it is not offered.
func spliceTarget(t *testing.T, e *Engine, obj state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("not a target decision: %+v", d)
	}
	for _, o := range d.Options {
		if o.Obj == obj {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("object %d not offered as a target: %+v", obj, d.Options)
}

// spliceResolveAll passes priority until the stack is empty.
func spliceResolveAll(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision while resolving: %+v", d)
		}
		submitChoices(t, e, passPriorityOption(t, d))
	}
	if len(e.G.Stack) > 0 {
		t.Fatal("stack did not empty")
	}
}

func spliceRevealNotes(e *Engine, id state.ObjID) int {
	return countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.Note && ev.Obj == id && ev.Text == "revealed for splice"
	})
}

func TestSpliceOntoArcaneRevealAndCost(t *testing.T) {
	e, bear, ox := spliceEngine(t, "Glacial Ray", "Kodama's Might")
	addMana(t, e, 0, "RRGGG")
	ray := findInHand(e, 0, "Glacial Ray")
	might := findInHand(e, 0, "Kodama's Might")
	if ray == 0 || might == 0 {
		t.Fatalf("precondition: Glacial Ray %d and Kodama's Might %d must be in hand", ray, might)
	}
	if got := e.G.Players[0].Pool.Total(); got != 5 {
		t.Fatalf("precondition: pool %d, want 5", got)
	}
	castNamed(t, e, "Glacial Ray")
	d := spliceSelect(t, e, "Kodama's Might")
	if len(d.Options) != 1 || d.Min != 0 || d.Max != 1 || !d.AllowNone {
		t.Fatalf("splice decision shape: %+v", d)
	}
	spliceTarget(t, e, ox)   // Glacial Ray's own text
	spliceTarget(t, e, bear) // the spliced Kodama's Might text
	if n := spliceRevealNotes(e, might); n != 1 {
		t.Fatalf("reveal notes for Kodama's Might = %d, want 1", n)
	}
	// {1}{R} + {G}: three of the five mana are spent.
	if got := e.G.Players[0].Pool.Total(); got != 2 {
		t.Fatalf("pool after the spliced cast = %d, want 2", got)
	}
	spliceResolveAll(t, e)
	if z := e.G.Obj(might).Zone; z != state.ZHand {
		t.Fatalf("Kodama's Might is in %v, want hand", z)
	}
	if z := e.G.Obj(ray).Zone; z != state.ZGraveyard {
		t.Fatalf("Glacial Ray is in %v, want graveyard", z)
	}
	if got := e.G.Obj(ox).Damage; got != 2 {
		t.Fatalf("Ox damage %d, want 2", got)
	}
	if got := e.Power(bear); got != 4 {
		t.Fatalf("Bear power %d, want 4 (+2/+2 from the spliced text)", got)
	}
	if got := e.Power(ox); got != 4 {
		t.Fatalf("Ox power %d, want 4 (untouched by the spliced pump)", got)
	}
}

func TestSpliceAddedTextTargetsAreItsOwn(t *testing.T) {
	e, bear, ox := spliceEngine(t, "Glacial Ray", "Kodama's Might")
	addMana(t, e, 0, "RRG")
	castNamed(t, e, "Glacial Ray")
	spliceSelect(t, e, "Kodama's Might")
	// Both texts accept either creature, and the two picks differ: the damage
	// must land on the spell's own pick and the pump on the spliced one.
	spliceTarget(t, e, bear)
	spliceTarget(t, e, ox)
	if bear == ox {
		t.Fatal("precondition: the two targets must differ")
	}
	spliceResolveAll(t, e)
	// Glacial Ray's own 2 damage kills the 2/2 Bear; the spliced pump went to Ox.
	if z := e.G.Obj(bear).Zone; z != state.ZGraveyard {
		t.Fatalf("Bear is in %v, want graveyard (Glacial Ray's own target)", z)
	}
	if got := e.G.Obj(ox).Damage; got != 0 {
		t.Fatalf("Ox damage %d, want 0", got)
	}
	if got := e.Power(ox); got != 6 {
		t.Fatalf("Ox power %d, want 6 (the spliced pump's target)", got)
	}
}

func TestSpliceNonManaCostSacrifices(t *testing.T) {
	e, bear, ox := spliceEngine(t, "Glacial Ray", "Torrent of Stone")
	m1 := moveByName(t, e, 0, "Mountain", state.ZBattlefield)
	m2 := moveByName(t, e, 0, "Mountain", state.ZBattlefield)
	addMana(t, e, 0, "RR")
	torrent := findInHand(e, 0, "Torrent of Stone")
	if torrent == 0 || e.G.Obj(m1).Zone != state.ZBattlefield || e.G.Obj(m2).Zone != state.ZBattlefield {
		t.Fatal("precondition: Torrent in hand and both Mountains on the battlefield")
	}
	castNamed(t, e, "Glacial Ray")
	spliceSelect(t, e, "Torrent of Stone")
	spliceTarget(t, e, bear) // Glacial Ray: 2 damage
	spliceTarget(t, e, ox)   // Torrent of Stone: 4 damage
	for i := 0; i < 6; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "sacrifice" {
			break
		}
		var picks []int
		for j := 0; j < d.Max && j < len(d.Options); j++ {
			picks = append(picks, d.Options[j].Index)
		}
		submitChoices(t, e, picks...)
	}
	for _, m := range []state.ObjID{m1, m2} {
		if z := e.G.Obj(m).Zone; z != state.ZGraveyard {
			t.Fatalf("Mountain %d is in %v, want graveyard (sacrificed to the splice cost)", m, z)
		}
	}
	spliceResolveAll(t, e)
	if z := e.G.Obj(torrent).Zone; z != state.ZHand {
		t.Fatalf("Torrent of Stone is in %v, want hand", z)
	}
	dmg := func(obj state.ObjID, n int32) int {
		return countEvents(e, func(ev events.Event) bool {
			return ev.Kind == events.Damage && ev.Obj == obj && ev.Amount == n
		})
	}
	if dmg(bear, 2) != 1 || dmg(ox, 4) != 1 {
		t.Fatalf("damage events: 2 to Bear = %d, 4 to Ox = %d; want one each", dmg(bear, 2), dmg(ox, 4))
	}
}

func TestSpliceCounteredLeavesTheCardsInHand(t *testing.T) {
	e, bear, ox := spliceEngineVs(t, []string{"Counterspell"}, "Glacial Ray", "Kodama's Might")
	addMana(t, e, 0, "RRG")
	addMana(t, e, 1, "UU")
	ray := findInHand(e, 0, "Glacial Ray")
	might := findInHand(e, 0, "Kodama's Might")
	moveByName(t, e, 1, "Counterspell", state.ZHand)
	castNamed(t, e, "Glacial Ray")
	spliceSelect(t, e, "Kodama's Might")
	spliceTarget(t, e, ox)
	spliceTarget(t, e, bear)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("not at priority: %+v", d)
	}
	submitChoices(t, e, passPriorityOption(t, d)) // seat 0 passes; seat 1 answers
	if d = e.Pending(); d == nil || d.Player != 1 {
		t.Fatalf("seat 1 does not hold priority: %+v", d)
	}
	castNamed(t, e, "Counterspell")
	spliceTarget(t, e, ray)
	spliceResolveAll(t, e)
	if z := e.G.Obj(ray).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: Glacial Ray is in %v, want graveyard (countered)", z)
	}
	if z := e.G.Obj(might).Zone; z != state.ZHand {
		t.Fatalf("Kodama's Might is in %v, want hand", z)
	}
	if e.G.Obj(ox).Damage != 0 || e.Power(bear) != 2 {
		t.Fatalf("a countered spell's text took effect: Ox damage %d, Bear power %d", e.G.Obj(ox).Damage, e.Power(bear))
	}
}

func TestSpliceQualityAndPayabilityGateTheOffer(t *testing.T) {
	const zapSrc = "Name:Plain Zap\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\n"
	offered := func(e *Engine) []string {
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "splice" {
			return nil
		}
		var names []string
		for _, o := range d.Options {
			names = append(names, e.G.Obj(o.Obj).Face().Name)
		}
		return names
	}
	t.Run("quality", func(t *testing.T) {
		reg := testutil.CorpusRegistry(t)
		e := corpusEngine(t, reg, []*cards.Card{card(t, zapSrc), lookup(t, reg, "Kodama's Might"),
			lookup(t, reg, "Everdream"), card(t, spliceBearSrc)}, nil)
		moveByName(t, e, 0, "Splice Bear", state.ZBattlefield)
		for _, n := range []string{"Plain Zap", "Kodama's Might", "Everdream"} {
			moveByName(t, e, 0, n, state.ZHand)
		}
		addMana(t, e, 0, "RGUUU")
		castNamed(t, e, "Plain Zap")
		// Kodama's Might (Arcane quality) must not match a plain instant;
		// Everdream (Instant,Sorcery) must.
		if got := offered(e); len(got) != 1 || got[0] != "Everdream" {
			t.Fatalf("offered %v, want only Everdream", got)
		}
	})
	t.Run("payability", func(t *testing.T) {
		e, _, _ := spliceEngine(t, "Glacial Ray", "Kodama's Might")
		addMana(t, e, 0, "RR") // no {G} anywhere for the splice cost
		castNamed(t, e, "Glacial Ray")
		if got := offered(e); got != nil {
			t.Fatalf("an unpayable splice cost was offered: %v", got)
		}
		if d := e.Pending(); d == nil || d.Kind != decision.KTarget {
			t.Fatalf("precondition: the cast should go on to its target ask, got %+v", d)
		}
	})
	t.Run("target", func(t *testing.T) {
		// Kodama's Might needs a creature target: with none, it is not
		// offered (CR 702.47b); with one it is.
		reg := testutil.CorpusRegistry(t)
		bare := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Glacial Ray"), lookup(t, reg, "Kodama's Might")}, nil)
		moveByName(t, bare, 0, "Glacial Ray", state.ZHand)
		moveByName(t, bare, 0, "Kodama's Might", state.ZHand)
		addMana(t, bare, 0, "RRG")
		castNamed(t, bare, "Glacial Ray")
		if got := offered(bare); got != nil {
			t.Fatalf("a splice card with no legal target was offered: %v", got)
		}
		e, _, _ := spliceEngine(t, "Glacial Ray", "Kodama's Might")
		addMana(t, e, 0, "RRG")
		castNamed(t, e, "Glacial Ray")
		if got := offered(e); len(got) != 1 || got[0] != "Kodama's Might" {
			t.Fatalf("control: offered %v, want Kodama's Might", got)
		}
	})
}
