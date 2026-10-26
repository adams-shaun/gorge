package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 509.1c "All creatures able to block CARDNAME do so." (Lure): a per-
// attacker blocking requirement binding every offered, cost-free pair.

const (
	lurePlainSrc   = "Name:Plain Attacker\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	lureHostSrc    = "Name:Lured Host\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	lureBlockerSrc = "Name:Lure Blocker\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"
)

// lureBlockers builds the shared fixture: seat 1 attacks seat 0 with the
// plain creature declared FIRST and the lured attacker second, seat 0 holds
// two untapped 1/1s. It returns the decision and the object ids.
func lureBlockers(t *testing.T, e *Engine, lured state.ObjID) (d *decision.Decision, plain state.ObjID, blockers [2]state.ObjID) {
	t.Helper()
	plain = onBoard(t, e, 1, lurePlainSrc)
	blockers[0] = onBoard(t, e, 0, lureBlockerSrc)
	blockers[1] = onBoard(t, e, 0, lureBlockerSrc)
	attackSeat0(t, e, plain, lured)
	d = askBlockersFresh(t, e)
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("precondition: no blockers decision: %#v", d)
	}
	for _, b := range blockers {
		if findBlockOption(d, b, plain) == nil || findBlockOption(d, b, lured) == nil {
			t.Fatalf("precondition: blocker %d is not offered both attackers: %+v", b, d.Options)
		}
	}
	return d, plain, blockers
}

// lureVerdicts pins what the requirement means for the declaration.
func lureVerdicts(t *testing.T, e *Engine, d *decision.Decision, lured, plain state.ObjID, blockers [2]state.ObjID) {
	t.Helper()
	if got := d.RequiredQuota(); got != 2 {
		t.Fatalf("RequiredQuota = %d, want 2 (both able creatures must block the lure)", got)
	}
	opt := func(b, a state.ObjID) int {
		o := findBlockOption(d, b, a)
		if o == nil {
			t.Fatalf("pair (%d,%d) not offered", b, a)
		}
		return o.Index
	}
	intent := func(choices ...int) decision.Intent {
		return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}
	}
	if err := e.validateBlockers(d, intent()); err == nil {
		t.Fatal("empty declaration accepted despite the lure")
	}
	if err := e.validateBlockers(d, intent(opt(blockers[0], lured), opt(blockers[1], plain))); err == nil {
		t.Fatal("one block on the lure and one on the plain attacker accepted")
	}
	if err := e.validateBlockers(d, intent(opt(blockers[0], lured), opt(blockers[1], lured))); err != nil {
		t.Fatalf("both creatures on the lure rejected: %v", err)
	}
	bot := newTestBot(7).answer(e, d)
	if err := e.validateBlockers(d, bot); err != nil {
		t.Fatalf("bot answer rejected: %v", err)
	}
}

func TestCR509_1cLurePrintedKeyword(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	unicorn := onBoardCard(t, e, 1, mustCorpusCardT(t, "Prized Unicorn"))
	d, plain, blockers := lureBlockers(t, e, unicorn)
	lureVerdicts(t, e, d, unicorn, plain, blockers)
}

func TestCR509_1cLureRuntimeGrant(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	host := onBoard(t, e, 1, lureHostSrc)
	challenge := mustCorpusCardT(t, "Taunting Challenge")
	src := onBoardCard(t, e, 1, challenge)
	f := e.G.Obj(src).Face()
	if len(f.Abilities) == 0 {
		t.Fatal("precondition: Taunting Challenge has no spell ability")
	}
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 1, SVars: f.SVars,
		Targets: []state.Target{{Obj: host}}, TargetsOffered: true}, f.Abilities[0])
	granted := false
	for _, k := range asBoard(e).Keywords(host) {
		if cards.HiddenKeywordOf(k) == cards.HiddenKeywordAllMustBlock {
			granted = true
		}
	}
	if !granted {
		t.Fatalf("precondition: the Pump did not grant the lure keyword: %v", asBoard(e).Keywords(host))
	}
	d, plain, blockers := lureBlockers(t, e, host)
	lureVerdicts(t, e, d, host, plain, blockers)
}

func TestCR509_1cLureExemptsTappedAndCostedBlockers(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	unicorn := onBoardCard(t, e, 1, mustCorpusCardT(t, "Prized Unicorn"))
	free := onBoard(t, e, 0, lureBlockerSrc)
	tapped := onBoard(t, e, 0, lureBlockerSrc)
	e.G.Obj(tapped).Tapped = true
	qal := onBoardCard(t, e, 0, mustCorpusCardT(t, "Qal Sisma Behemoth"))
	floatMana(t, e, 0, "RR")
	attackSeat0(t, e, unicorn)
	d := askBlockersFresh(t, e)
	if d == nil {
		t.Fatal("precondition: no blockers decision")
	}
	if findBlockOption(d, qal, unicorn) == nil {
		t.Fatalf("precondition: the costed Qal pair is not offered: %+v", d.Options)
	}
	if findBlockOption(d, tapped, unicorn) != nil {
		t.Fatal("precondition: the tapped creature is offered")
	}
	if o := findBlockOption(d, qal, unicorn); o.BlockMust || o.Required {
		t.Fatalf("costed pair carries the requirement: %+v", o)
	}
	if o := findBlockOption(d, free, unicorn); o == nil || !o.BlockMust || !o.Required {
		t.Fatalf("free pair lacks the requirement: %+v", o)
	}
	if got := d.RequiredQuota(); got != 1 {
		t.Fatalf("RequiredQuota = %d, want 1", got)
	}
}

// TestMustBlockBoundDutyCountsOnlyItsAttacker: a duty bound to one attacker
// is obeyed only by the pair naming it, wherever the solver meets the pair.
func TestMustBlockBoundDutyCountsOnlyItsAttacker(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	src := onBoard(t, e, 1, "Name:Duty source\nTypes:Creature\nPT:2/2\nSVar:Ordinary:DB$ MustBlock | ValidTgts$ Creature | DefinedAttacker$ TriggeredAttacker\nOracle:x\n")
	blocker := onBoard(t, e, 0, lureBlockerSrc)
	first := onBoard(t, e, 1, lurePlainSrc)
	bound := onBoard(t, e, 1, lureHostSrc)
	attackSeat0(t, e, first, bound)
	face := e.G.Obj(src).Face()
	sa := cards.ResolveSVar(face.SVars, "Ordinary")
	if sa == nil {
		t.Fatal("precondition: no Ordinary SVar")
	}
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 1, SVars: face.SVars,
		Remembered: []state.Target{{Obj: bound}}, Targets: []state.Target{{Obj: blocker}}, TargetsOffered: true}, sa)
	d := askBlockersFresh(t, e)
	if d == nil {
		t.Fatal("precondition: no blockers decision")
	}
	ob, of := findBlockOption(d, blocker, bound), findBlockOption(d, blocker, first)
	if ob == nil || of == nil || !ob.BlockMust || of.BlockMust {
		t.Fatalf("precondition: only the bound pair carries the duty: %+v", d.Options)
	}
	if got := d.RequiredQuota(); got != 1 {
		t.Fatalf("RequiredQuota = %d, want 1", got)
	}
	for name, choices := range map[string][]int{"empty": nil, "block on first": {of.Index}} {
		if err := e.validateBlockers(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err == nil {
			t.Fatalf("%s declaration accepted though the duty binds the other attacker", name)
		}
	}
	if err := e.validateBlockers(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{ob.Index}}); err != nil {
		t.Fatalf("block on the bound attacker rejected: %v", err)
	}
}

func TestLureKeywordIsRegistered(t *testing.T) {
	t.Parallel()
	sup := effects.Supported()
	if !sup["kw:AllMustBlock"] {
		t.Fatal("kw:AllMustBlock is not registered")
	}
	for _, p := range mustCorpusCardT(t, "Prized Unicorn").Primitives() {
		if !sup[p] {
			t.Errorf("Prized Unicorn primitive %q unsupported", p)
		}
	}
}
