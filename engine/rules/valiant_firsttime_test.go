package rules

// Valiant$ True on Mode$ BecomesTarget (the BLB ability word, CR 603.2's
// "for the first time each turn" family): Seedglaive Mentor
// (`.cards/cardsfolder/s/seedglaive_mentor.txt`, the measured carrier from
// a dirac report). The gate is
// trigmatch.valiantFirstTimeTargetedByYouThisTurn: the once-each-turn count
// is per CONTROLLER -- only targetings of a spell or ability the trigger's
// controller controls count, so an opponent's targeting neither fires nor
// disarms (XMage's ValiantTriggeredAbility, Forge's Card.isValiant).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestValiantFiresOncePerTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	trig := corpusTriggerMode(t, mustCorpusCard(t, reg, "Seedglaive Mentor"), "BecomesTarget")
	if trig.Params["Valiant"] != "True" {
		t.Fatalf("corpus fixture drift: Seedglaive Mentor Valiant$ = %q, want True", trig.Params["Valiant"])
	}
	if trig.Params["ValidSource"] != "SpellAbility.YouCtrl" {
		t.Fatalf("corpus fixture drift: Seedglaive Mentor ValidSource$ = %q, want SpellAbility.YouCtrl", trig.Params["ValidSource"])
	}
	e, ids := becomesTargetOnceBoard(t, reg,
		[]string{"Seedglaive Mentor", "Prodigal Pyromancer", "Prodigal Pyromancer"},
		[]string{"Prodigal Pyromancer"})
	mentor := ids["Seedglaive Mentor"]
	if o := e.G.Obj(mentor); o == nil || o.Zone != state.ZBattlefield || targetCounterN(o, "P1P1") != 0 {
		t.Fatalf("precondition: Seedglaive Mentor not a counterless battlefield permanent: %+v", o)
	}
	// The board holds the mentor and two your-side Pyromancers; seat 1 holds
	// the opponent's.
	yours, theirs := []state.ObjID{}, state.ObjID(0)
	for _, id := range append(e.G.Zone(state.ZBattlefield, 0), e.G.Zone(state.ZBattlefield, 1)...) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil || o.Face().Name != "Prodigal Pyromancer" {
			continue
		}
		if o.Controller == 0 {
			yours = append(yours, id)
		} else {
			theirs = id
		}
	}
	if len(yours) != 2 || theirs == 0 {
		t.Fatalf("precondition: %d your-side Pyromancers, theirs set: %v", len(yours), theirs)
	}
	counters := func() int { return targetCounterN(e.G.Obj(mentor), "P1P1") }
	heal := func() { e.G.Obj(mentor).Damage = 0 }
	// A ping resolves one {T}: deals-1-damage activation of the Pyromancer
	// `p` against the mentor. The activations tap their sources; each
	// harness step uses a different one, and the heal between steps is
	// setup-only state repair so the 3/2 mentor survives the assertion
	// sequence (a lethal ping would move it out from under the counters).
	ping := func(p, target state.ObjID) {
		t.Helper()
		seat := e.G.Obj(p).Controller
		if o := e.G.Obj(p); o == nil || o.Tapped || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: source %d not an untapped battlefield permanent: %+v", p, o)
		}
		e.askPriority(seat)
		submitChoices(t, e, abilityOption(t, e, p, 0).Index)
		submitChoices(t, e, targetOptionIdx(t, e, target))
		passUntilStackEmpty(t, e, 40)
	}
	newTurn := func() {
		t.Helper()
		e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: e.G.Turn + 1})
		e.G.Step = state.StepMain1
		for _, id := range append(yours, theirs) {
			e.G.Obj(id).Tapped = false
		}
	}

	// An opponent's targeting fires nothing (ValidSource$ SpellAbility.YouCtrl)
	// and must not arm the once-per-turn count.
	ping(theirs, mentor)
	if got := counters(); got != 0 {
		t.Fatalf("opponent targeting fired the Valiant trigger: counters = %d, want 0", got)
	}
	heal()
	// Your first targeting this turn fires.
	ping(yours[0], mentor)
	if got := counters(); got != 1 {
		t.Fatalf("first your-side targeting this turn did not fire: counters = %d, want 1", got)
	}
	heal()
	// Your second targeting the same turn fires again ONLY under the bug:
	// the ability word's once-each-turn clause must hold it back.
	ping(yours[1], mentor)
	if got := counters(); got != 1 {
		t.Fatalf("second your-side targeting this turn fired: counters = %d, want 1", got)
	}
	heal()
	// A new turn re-arms the count.
	newTurn()
	ping(yours[0], mentor)
	if got := counters(); got != 2 {
		t.Fatalf("first your-side targeting of the next turn did not fire: counters = %d, want 2", got)
	}
	heal()
	// The next turn, an opponent's targeting first: still nothing, and it
	// must not disarm the count, so your first targeting after it fires.
	newTurn()
	ping(theirs, mentor)
	if got := counters(); got != 2 {
		t.Fatalf("opponent targeting fired or disarmed across turns: counters = %d, want 2", got)
	}
	heal()
	ping(yours[1], mentor)
	if got := counters(); got != 3 {
		t.Fatalf("your first targeting after an opponent's targeting did not fire: counters = %d, want 3", got)
	}
}

// valiantProbeScript is the probe spell the SPELL cases below cast at the
// mentor: an instant whose one subability is a harmless targeted Pump, so a
// cast reaches the stack, asks for the mentor and resolves without moving the
// mentor out from under its counters (a damage spell would kill the 3/2).
// Hand-authored fixture, no Forge script text.
const valiantProbeScript = "Name:Valiant Probe\nManaCost:1 R\nTypes:Instant\n" +
	"A:SP$ Pump | ValidTgts$ Creature | NumAtt$ +1 | NumDef$ +1 | SpellDescription$ Target creature gets +1/+1 until end of turn.\nOracle:x\n"

// valiantProbeInHand adds the probe to seat p's hand with the given OWNER
// (p its controller) -- the gonti_youdongown_cast_test.go AddObject patch
// shape -- then re-asks priority so the new cast option is offered.
func valiantProbeInHand(t *testing.T, e *Engine, p, owner state.PlayerID) state.ObjID {
	t.Helper()
	o := e.G.AddObject(card(t, valiantProbeScript), owner)
	o.Zone = state.ZHand
	o.Controller = p
	e.G.SetZone(state.ZHand, p, append(e.G.Zone(state.ZHand, p), o.ID))
	e.pending = nil
	e.priorityRound()
	return o.ID
}

// valiantCastProbe submits seat p's cast of the probe id against the mentor
// and lets the stack drain, asserting the spell actually reached the stack
// (a cast that never happened cannot prove anything about the gate).
func valiantCastProbe(t *testing.T, e *Engine, p state.PlayerID, probe, mentor state.ObjID) {
	t.Helper()
	addMana(t, e, p, "RR")
	e.askPriority(p)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != p {
		t.Fatalf("test precondition: expected seat %d's priority before the cast, got %+v", p, d)
	}
	submitChoices(t, e, miscCastOption(t, e, probe))
	o := e.G.Obj(probe)
	if o == nil || o.Zone != state.ZStack {
		t.Fatalf("test precondition: the probe spell is in %v after the cast submission, want the stack", o.Zone)
	}
	submitChoices(t, e, targetOptionIdx(t, e, mentor))
	passUntilStackEmpty(t, e, 40)
}

// TestValiantCountsCasterNotOwner pins the controller the once-each-turn
// count reads for a SPELL: the caster, read off the cast's PutOnStack
// (stackObjControllerAt), never the card's owner. Seat 0 casts a probe OWNED
// by seat 1 (the Gonti taken-card shape, Owner 1 Controller 0); its targeting
// counts as yours and arms the gate, so a second probe cast by seat 0 the
// same turn must stay silent. The owner-proxy this replaced would skip the
// first spell's event (its object sits in the graveyard owned by seat 1) and
// fire again. The opponent's own probe spell first proves the spell case of
// the YouCtrl gate: it neither fires nor arms.
func TestValiantCountsCasterNotOwner(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	trig := corpusTriggerMode(t, mustCorpusCard(t, reg, "Seedglaive Mentor"), "BecomesTarget")
	if trig.Params["Valiant"] != "True" || trig.Params["ValidSource"] != "SpellAbility.YouCtrl" {
		t.Fatalf("corpus fixture drift: Valiant$ = %q ValidSource$ = %q, want True / SpellAbility.YouCtrl",
			trig.Params["Valiant"], trig.Params["ValidSource"])
	}
	e, _ := miscHandsEngine(t, reg, nil, nil, []string{"Seedglaive Mentor"}, nil)
	mentor := miscBoardObj(t, e, 0, "Seedglaive Mentor")
	if o := e.G.Obj(mentor); o == nil || o.Zone != state.ZBattlefield || targetCounterN(o, "P1P1") != 0 {
		t.Fatalf("precondition: Seedglaive Mentor not a counterless battlefield permanent: %+v", o)
	}
	counters := func() int { return targetCounterN(e.G.Obj(mentor), "P1P1") }

	// An opponent's SPELL targeting the mentor: rejected by ValidSource$
	// SpellAbility.YouCtrl, and it must not arm the count either.
	theirs := valiantProbeInHand(t, e, 1, 1)
	if o := e.G.Obj(theirs); o.Owner != 1 || o.Controller != 1 {
		t.Fatalf("precondition: opponent probe owner %d controller %d, want 1/1", o.Owner, o.Controller)
	}
	valiantCastProbe(t, e, 1, theirs, mentor)
	if got := counters(); got != 0 {
		t.Fatalf("an opponent's spell fired the Valiant trigger: counters = %d, want 0", got)
	}

	// Your spell cast from a card you do NOT own: the targeting is yours
	// ("a spell or ability you control" -- caster, not owner), it fires.
	yoursStolen := valiantProbeInHand(t, e, 0, 1)
	if o := e.G.Obj(yoursStolen); o.Owner != 1 || o.Controller != 0 {
		t.Fatalf("precondition: stolen probe owner %d controller %d, want owner 1 controller 0", o.Owner, o.Controller)
	}
	valiantCastProbe(t, e, 0, yoursStolen, mentor)
	if got := counters(); got != 1 {
		t.Fatalf("your cast of an opponent-owned spell did not fire: counters = %d, want 1", got)
	}

	// A second spell cast by you the same turn: the once-each-turn clause
	// holds it back even though the first spell's card was not yours.
	yours := valiantProbeInHand(t, e, 0, 0)
	if o := e.G.Obj(yours); o.Owner != 0 || o.Controller != 0 {
		t.Fatalf("precondition: own probe owner %d controller %d, want 0/0", o.Owner, o.Controller)
	}
	valiantCastProbe(t, e, 0, yours, mentor)
	if got := counters(); got != 1 {
		t.Fatalf("a second spell you cast this turn fired: counters = %d, want 1", got)
	}
}

// TestValiantDedupesSameSpellTargets pins the per-spell half of the gate: a
// spell that names the same creature in two target groups (a later appended
// group, the fuse/modal shape recordChosenTargets' two events carry) is still
// ONE targeting of it, so the Valiant trigger fires once, not once per
// TargetsChosen event. Forge fires once per spell (its distinct-target check
// per cast), and the ability word's "for the first time each turn" reads per
// targeting, not per event.
func TestValiantDedupesSameSpellTargets(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := miscHandsEngine(t, reg, nil, nil, []string{"Seedglaive Mentor"}, nil)
	mentor := miscBoardObj(t, e, 0, "Seedglaive Mentor")
	if o := e.G.Obj(mentor); o == nil || o.Zone != state.ZBattlefield || targetCounterN(o, "P1P1") != 0 {
		t.Fatalf("precondition: Seedglaive Mentor not a counterless battlefield permanent: %+v", o)
	}
	counters := func() int { return targetCounterN(e.G.Obj(mentor), "P1P1") }
	probe := valiantProbeInHand(t, e, 0, 0)
	addMana(t, e, 0, "RR")
	e.askPriority(0)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != state.PlayerID(0) {
		t.Fatalf("test precondition: expected seat 0's priority, got %+v", d)
	}
	submitChoices(t, e, miscCastOption(t, e, probe))
	submitChoices(t, e, targetOptionIdx(t, e, mentor))
	// The spell is on the stack, targets recorded, unresolvable yet: drive a
	// second target group of the SAME spell naming the mentor again (the
	// APPEND shape a later clause records).
	spell := e.G.Obj(probe)
	if spell == nil || spell.Zone != state.ZStack || len(spell.Targets) != 1 || spell.Targets[0].Obj != mentor {
		t.Fatalf("test precondition: the probe spell on the stack does not name the mentor once: %+v", spell)
	}
	trigsOnStack := func() int {
		n := 0
		for _, id := range e.G.Stack {
			if o := e.G.Obj(id); o != nil && o.StackKindKnown && o.StackKind == state.StackKindTriggered {
				n++
			}
		}
		return n
	}
	if got := trigsOnStack(); got != 1 {
		t.Fatalf("precondition: %d triggered abilities on the stack after the cast's target group, want the mentor's one Valiant trigger", got)
	}
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: probe, Amount: 2, IDs: []state.ObjID{mentor}})
	if got := trigsOnStack(); got != 1 {
		t.Fatalf("the spell's second group queued a further Valiant trigger, want none (one trigger per spell)")
	}
	passUntilStackEmpty(t, e, 40)
	if got := counters(); got != 1 {
		t.Fatalf("after resolution the mentor has %d counters, want 1", got)
	}
}
