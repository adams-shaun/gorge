package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// This file pins CR 702.15a's lifelink rider for NON-COMBAT damage
// (effects/damage.go): a spell or ability whose SOURCE has lifelink -- printed
// or granted -- gains its controller that much life with the damage, on every
// recipient shape (player, creature, planeswalker) and through both damage
// primitives (DealDamage, DamageAll). Combat has its own rider in
// rules/combat.go and is out of scope here. Real corpus cards carry the shapes
// the defect was reported on (Brion Stoutarm's throw, Piru's death sweep,
// Basilisk Collar's grant); inline fixtures carry the recipient variants those
// cards do not reach. A prevented hit (protection -> Note) must pay nothing,
// so the last test pins the zero.

// linkBoard is walkerBoard generalised to arbitrary named corpus cards on
// either seat: every named card is seeded into that seat's deck, put onto the
// battlefield with a LOGGED MoveZone (so entry grants such as the CR 306.5b
// loyalty grant run for real and every paid activation replays), and the
// clock is parked at turn 2 seat 0 Main1. A name given twice seeds two copies.
func linkBoard(t *testing.T, reg *cards.Registry, p0, p1 []string) (*Engine, Config) {
	t.Helper()
	var d0, d1 []*cards.Card
	for _, name := range p0 {
		d0 = append(d0, mustCorpusCard(t, reg, name))
	}
	for _, name := range p1 {
		d1 = append(d1, mustCorpusCard(t, reg, name))
	}
	cfg := Config{Seed: 47, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append(d0, mountainDeck(t, 40-len(d0))...),
			append(d1, mountainDeck(t, 40-len(d1))...),
		},
	}
	e := New(cfg)
	onBattlefield := func(seat state.PlayerID, named []*cards.Card) {
		for i := range e.G.Objs {
			o := &e.G.Objs[i]
			if o.Owner != seat {
				continue
			}
			for _, c := range named {
				if o.Card == c {
					e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: state.ZBattlefield})
					break
				}
			}
		}
	}
	onBattlefield(0, d0)
	onBattlefield(1, d1)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	return e, cfg
}

// activateAnswers the decisions activating an ability with a sacrifice cost
// poses, in whatever order they arrive: a KChoose sacrifice cost (answered
// with sacObj) and/or the KTarget announcement (answered with targetObj, or
// targetPlayer when the subject targets players). Returns after the target is
// answered, with the ability sitting on the stack.
func activateAnswers(t *testing.T, e *Engine, sacObj, targetObj state.ObjID, targetPlayer state.PlayerID) {
	t.Helper()
	for i := 0; i < 4; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no pending decision while activating (sac %d, target %d)", sacObj, targetObj)
		}
		switch d.Kind {
		case decision.KChoose:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "sacrifice" && o.Obj == sacObj {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("no sacrifice option for %d: %+v", sacObj, d.Options)
			}
			submitChoices(t, e, idx)
		case decision.KTarget:
			if targetObj != 0 {
				targetObject(t, e, targetObj)
				return
			}
			idx := indexOfPlayerOption(d, targetPlayer)
			if idx < 0 {
				t.Fatalf("no target option for player %d: %+v", targetPlayer, d.Options)
			}
			submitChoices(t, e, idx)
			return
		default:
			t.Fatalf("unexpected decision kind %q while activating: %+v", d.Kind, d.Options)
		}
	}
	t.Fatal("the cost and target decisions never settled")
}

// countLifeChanges counts LifeChange events for (player, amount).
func countLifeChanges(t *testing.T, e *Engine, p state.PlayerID, amount int32) int {
	t.Helper()
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.LifeChange && ev.Player == p && ev.Amount == amount {
			n++
		}
	}
	return n
}

// TestBrionStoutarmThrowGainsLife pins the reported defect on the real
// corpus card: Brion Stoutarm's throw ({R}, {T}, sacrifice another creature)
// deals damage equal to the sacrificed creature's power to a player, and
// Brion has printed lifelink -- so his controller gains that much life with
// the damage. Before the fix the damage landed and nobody gained any.
func TestBrionStoutarmThrowGainsLife(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := linkBoard(t, reg, []string{"Brion Stoutarm", "Hill Giant"}, nil)
	var fodder, brion state.ObjID
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Zone != state.ZBattlefield || o.Owner != 0 {
			continue
		}
		if o.Face() != nil && o.Face().Name == "Hill Giant" {
			fodder = o.ID
		}
		if o.Face() != nil && o.Face().Name == "Brion Stoutarm" {
			brion = o.ID
		}
	}
	if brion == 0 || fodder == 0 {
		t.Fatalf("board missing Brion or fodder: %d %d", brion, fodder)
	}
	addMana(t, e, 0, "R")
	opt := abilityOption(t, e, brion, 0)
	submitChoices(t, e, opt.Index)
	activateAnswers(t, e, fodder, 0, 1)
	passUntilStackEmpty(t, e, 30)

	// Hill Giant's power is 3: the throw dealt 3 to player 1 and Brion's
	// lifelink gained player 0 exactly 3 life, riding the damage.
	if got := e.G.Players[0].Life; got != 23 {
		t.Fatalf("Brion's controller life = %d, want 23 (20 + 3 lifelink)", got)
	}
	if got := e.G.Players[1].Life; got != 17 {
		t.Fatalf("defender life = %d, want 17", got)
	}
	if n := countLifeChanges(t, e, 0, 3); n != 1 {
		t.Fatalf("logged %d lifelink LifeChange(0, +3) events, want 1", n)
	}
	replayCheck(t, e, cfg)
}

// TestBrionStoutarmThrowAtPlaneswalkerGainsLife pins the planeswalker arm:
// the same throw at a planeswalker removes loyalty counters (CR 306.8, the
// damage is never marked as card damage) and the lifelink rider still pays.
// Teferi, Hero of Dominaria enters at 5 loyalty (per its corpus script), so
// the 3-power throw leaves it alive at 2.
func TestBrionStoutarmThrowAtPlaneswalkerGainsLife(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := linkBoard(t, reg, []string{"Brion Stoutarm", "Hill Giant"},
		[]string{"Teferi, Hero of Dominaria"})
	var fodder, brion, walker state.ObjID
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Zone != state.ZBattlefield {
			continue
		}
		switch o.Face().Name {
		case "Hill Giant":
			fodder = o.ID
		case "Brion Stoutarm":
			brion = o.ID
		case "Teferi, Hero of Dominaria":
			walker = o.ID
		}
	}
	if brion == 0 || fodder == 0 || walker == 0 {
		t.Fatalf("board missing a fixture: brion %d fodder %d walker %d", brion, fodder, walker)
	}
	addMana(t, e, 0, "R")
	opt := abilityOption(t, e, brion, 0)
	submitChoices(t, e, opt.Index)
	activateAnswers(t, e, fodder, walker, 0)
	passUntilStackEmpty(t, e, 30)

	if o := e.G.Obj(walker); o.Zone != state.ZBattlefield {
		t.Fatalf("walker left the battlefield (%s) -- 3 damage on 4 loyalty cannot", o.Zone)
	}
	if got := e.G.Obj(walker).Counter("LOYALTY"); got != 1 {
		t.Fatalf("walker loyalty = %d, want 1 (4 - 3)", got)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Damage && ev.Obj == walker {
			t.Fatalf("walker damage was marked as card damage: %+v", ev)
		}
	}
	if got := e.G.Players[0].Life; got != 23 {
		t.Fatalf("Brion's controller life = %d, want 23 (20 + 3 lifelink off the walker hit)", got)
	}
	if n := countLifeChanges(t, e, 0, 3); n != 1 {
		t.Fatalf("logged %d lifelink LifeChange(0, +3) events, want 1", n)
	}
	replayCheck(t, e, cfg)
}

// TestLifelinkAbilityDamageToCreatureGainsLife covers the plain creature arm
// on an inline fixture (DealDamage at a creature from a printed-lifelink
// source): the damage is marked and the controller gains it back.
func TestLifelinkAbilityDamageToCreatureGainsLife(t *testing.T) {
	src := "Name:Archer\nManaCost:1 W\nTypes:Creature Archer\nPT:1/2\nK:Lifelink\n" +
		"A:AB$ DealDamage | Cost$ T | ValidTgts$ Creature | NumDmg$ 2 | SpellDescription$ deals 2.\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 91, src)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZBattlefield})
	// Summoning sickness (CR 302.6): the {T} cost is only payable from the
	// controller's next turn, so park the clock at turn 2 like linkBoard does.
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	ox := putToken(t, e, 1, "Name:Ox\nManaCost:1 G\nTypes:Creature Ox\nPT:3/3\nOracle:x\n", state.ZBattlefield)
	addMana(t, e, 0, "W") // pool irrelevant (cost is {T} only) but keeps the ask fresh
	e.Advance()
	opt := abilityOption(t, e, id, 0)
	submitChoices(t, e, opt.Index)
	targetObject(t, e, ox)
	passUntilStackEmpty(t, e, 20)

	if got := e.G.Obj(ox).Damage; got != 2 {
		t.Fatalf("Ox damage = %d, want 2", got)
	}
	if o := e.G.Obj(ox); o.Zone != state.ZBattlefield {
		t.Fatalf("Ox left the battlefield (%s) -- 2 damage on a 3/3 cannot", o.Zone)
	}
	if got := e.G.Players[0].Life; got != 22 {
		t.Fatalf("Archer's controller life = %d, want 22 (20 + 2 lifelink)", got)
	}
	if n := countLifeChanges(t, e, 0, 2); n != 1 {
		t.Fatalf("logged %d lifelink LifeChange(0, +2) events, want 1", n)
	}
	// The rider is not a trigger: the LifeChange is logged immediately after
	// the Damage it rides, in the same resolution.
	lastDamage, lastLife := -1, -1
	for i, ev := range e.L.Events {
		if ev.Kind == events.Damage && ev.Obj == ox {
			lastDamage = i
		}
		if ev.Kind == events.LifeChange && ev.Player == 0 && ev.Amount == 2 {
			lastLife = i
		}
	}
	if lastDamage < 0 || lastLife < 0 || lastLife != lastDamage+1 {
		t.Fatalf("LifeChange at %d is not immediately after the Damage at %d", lastLife, lastDamage)
	}
	replayCheck(t, e, cfg)
}

// TestLifelinkDeathSweepSumsLife pins DamageAll on the real corpus card the
// defect was found beside: Piru, the Volatile has printed lifelink, and when
// it dies its sweep deals 7 to each nonlegendary creature -- two Grizzly
// Bears means two landed 7s, and the controller gains the SUM (14), not one
// rider for the resolution.
func TestLifelinkDeathSweepSumsLife(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := linkBoard(t, reg, []string{"Piru, the Volatile"},
		[]string{"Grizzly Bears", "Grizzly Bears"})
	var piru state.ObjID
	var bears []state.ObjID
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Zone != state.ZBattlefield {
			continue
		}
		switch o.Face().Name {
		case "Piru, the Volatile":
			piru = o.ID
		case "Grizzly Bears":
			bears = append(bears, o.ID)
		}
	}
	if piru == 0 || len(bears) != 2 {
		t.Fatalf("board missing Piru or the two bears: piru %d bears %v", piru, bears)
	}
	// Kill Piru through a real (logged) damage event: the 7/7 dies to the
	// lethal-damage SBA, its CR 603.10 death trigger fires, and the sweep
	// resolves through the ordinary trigger drain.
	e.emit(events.Event{Kind: events.Damage, Obj: piru, Amount: 7})
	e.checkStateBased()
	e.Advance() // re-ask priority: the stale snapshot predates the death trigger
	passUntilStackEmpty(t, e, 40)

	for _, b := range bears {
		if o := e.G.Obj(b); o.Zone != state.ZGraveyard {
			t.Fatalf("bear zone = %s, want graveyard (7 on a 2/2)", o.Zone)
		}
	}
	if o := e.G.Obj(piru); o.Zone != state.ZGraveyard {
		t.Fatalf("Piru zone = %s, want graveyard", o.Zone)
	}
	if got := e.G.Players[0].Life; got != 34 {
		t.Fatalf("Piru's controller life = %d, want 34 (20 + 7 + 7 lifelink)", got)
	}
	if n := countLifeChanges(t, e, 0, 7); n != 2 {
		t.Fatalf("logged %d lifelink LifeChange(0, +7) events, want 2 (one per landed hit)", n)
	}
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("opponent life = %d, want 20 (the sweep hurts creatures, not players)", got)
	}
	replayCheck(t, e, cfg)
}

// TestGrantedLifelinkEquipmentDamageGainsLife pins the GRANTED keyword half:
// Basilisk Collar (real corpus card, AddKeyword$ Deathtouch & Lifelink over
// Creature.EquippedBy) equips a pinger whose activated DealDamage is otherwise
// keywordless. The rider must read the granted keyword off the ability's
// source permanent -- an ability stack object carries no Face, so the
// wrapper's own Derived is empty -- and the same resolution makes the
// collateral deathtouch rider fire on an ability source too.
func TestGrantedLifelinkEquipmentDamageGainsLife(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := linkBoard(t, reg, []string{"Basilisk Collar", "Prodigal Pyromancer"},
		[]string{"Grizzly Bears"})
	var collar, pyro, bear state.ObjID
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Zone != state.ZBattlefield || o.Owner != 0 && o.Face().Name != "Grizzly Bears" {
			continue
		}
		switch o.Face().Name {
		case "Basilisk Collar":
			collar = o.ID
		case "Prodigal Pyromancer":
			pyro = o.ID
		case "Grizzly Bears":
			bear = o.ID
		}
	}
	if collar == 0 || pyro == 0 || bear == 0 {
		t.Fatalf("board missing a fixture: collar %d pyro %d bear %d", collar, pyro, bear)
	}
	addMana(t, e, 0, "CC")
	equipOpt := abilityOption(t, e, collar, 0)
	submitChoices(t, e, equipOpt.Index)
	targetObject(t, e, pyro)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(collar).AttachedTo != pyro {
		t.Fatalf("collar attached to %d, want the pyromancer %d", e.G.Obj(collar).AttachedTo, pyro)
	}
	if !e.HasKeyword(pyro, "Lifelink") || !e.HasKeyword(pyro, "Deathtouch") {
		t.Fatalf("bearer lacks the granted keywords: lifelink %v deathtouch %v",
			e.HasKeyword(pyro, "Lifelink"), e.HasKeyword(pyro, "Deathtouch"))
	}

	// Ping the bear: 1 damage, lethal only through the now-granted
	// deathtouch, and lifelink gains 1.
	opt := abilityOption(t, e, pyro, 0)
	submitChoices(t, e, opt.Index)
	targetObject(t, e, bear)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(bear); o.Zone != state.ZGraveyard {
		t.Fatalf("bear zone = %s, want graveyard (1 damage + granted deathtouch)", o.Zone)
	}
	sawDeathtouch := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == bear && ev.Counter == "Deathtouched" {
			sawDeathtouch = true
		}
	}
	if !sawDeathtouch {
		t.Fatal("no Deathtouched counter was minted for the granted-deathtouch ping")
	}
	if got := e.G.Players[0].Life; got != 21 {
		t.Fatalf("bearer's controller life = %d, want 21 (20 + 1 granted lifelink)", got)
	}
	if n := countLifeChanges(t, e, 0, 1); n != 1 {
		t.Fatalf("logged %d lifelink LifeChange(0, +1) events, want 1", n)
	}
	replayCheck(t, e, cfg)
}

// TestPreventedLifelinkDamageGainsNothing pins the zero: a lifelink source's
// DamageAll sweeps a creature with protection from the source's colour. The
// protection prevention path (emit replaces the Damage with a Note) must pay
// no life for the prevented hit while the unprotected self-hit still pays.
func TestPreventedLifelinkDamageGainsNothing(t *testing.T) {
	sweep := "Name:LinkSweep\nManaCost:2 W\nTypes:Creature Knight\nPT:2/4\nK:Lifelink\n" +
		"A:AB$ DamageAll | Cost$ 2 | ValidCards$ Creature | NumDmg$ 2 | SpellDescription$ x\nOracle:x\n"
	shield := "Name:Warded Ox\nManaCost:1 W\nTypes:Creature Ox\nPT:2/2\nK:Protection from white\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 97, sweep)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZBattlefield})
	ox := putToken(t, e, 1, shield, state.ZBattlefield)
	addMana(t, e, 0, "CC")
	e.Advance()
	opt := abilityOption(t, e, id, 0)
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)

	if got := e.G.Obj(ox).Damage; got != 0 {
		t.Fatalf("protected Ox damage = %d, want 0 (prevention replaced the hit)", got)
	}
	if o := e.G.Obj(ox); o.Zone != state.ZBattlefield {
		t.Fatalf("protected Ox left the battlefield (%s)", o.Zone)
	}
	sawPrevented := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Obj == ox && ev.Text == "prevented: protection" {
			sawPrevented = true
		}
	}
	if !sawPrevented {
		t.Fatal("no prevented: protection Note was recorded for the warded Ox")
	}
	// The sweep's own unprotected self-hit landed (2 on the 2/4 Knight): the
	// rider pays exactly once, for it, and nothing for the prevented hit.
	if got := e.G.Players[0].Life; got != 22 {
		t.Fatalf("sweep source's controller life = %d, want 22 (one landed +2 hit, prevented hit pays nothing)", got)
	}
	if n := countLifeChanges(t, e, 0, 2); n != 1 {
		t.Fatalf("logged %d lifelink LifeChange(0, +2) events, want 1", n)
	}
	replayCheck(t, e, cfg)
}
