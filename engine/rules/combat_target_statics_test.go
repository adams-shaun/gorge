package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Pins for the combat and targeting statics: BlockTapped (Masako the
// Humorless), BlockRestrict (Mirri, Weatherlight Duelist), AssignNoCombat
// Damage (Ophidian), the two-blocker requirement (Gorm the Great), api:Block
// with UnblockCreaturesBlockedOnlyBy$ (Balduvian Warlord) and MustTarget
// (Coalition Honor Guard, Standard Bearer).

const statBearSrc = "Name:Plain Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// TestStaticCardsAreSupported: every carrier passes the coverage check.
func TestStaticCardsAreSupported(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	for _, name := range []string{"Coalition Honor Guard", "Standard Bearer", "Masako the Humorless",
		"Mirri, Weatherlight Duelist", "Ophidian", "Gorm the Great", "Balduvian Warlord"} {
		c := lookup(t, reg, name)
		if miss := reg.Unsupported(c, sup); len(miss) != 0 {
			t.Errorf("%s: unsupported %v", name, miss)
		}
	}
}

// TestBlockTappedLetsTappedCreaturesBlock: under Masako, a tapped creature
// its controller controls is offered as a blocker and its declaration is
// legal (CR 509.1a's untapped requirement lifted); the opponent's tapped
// creatures are not, and without Masako the tapped creature is not offered.
func TestBlockTappedLetsTappedCreaturesBlock(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	attacker := onBoard(t, e, 1, statBearSrc)
	tapped := onBoard(t, e, 0, statBearSrc)
	e.G.Obj(tapped).Tapped = true
	attackSeat0(t, e, attacker)
	if d := askBlockersFresh(t, e); d != nil && findBlockOption(d, tapped, attacker) != nil {
		t.Fatal("precondition: a tapped creature is offered without Masako")
	}
	// The attacking player's Masako does not help the defender.
	onBoardCard(t, e, 1, corpusCard(t, "Masako the Humorless"))
	if d := askBlockersFresh(t, e); d != nil && findBlockOption(d, tapped, attacker) != nil {
		t.Fatal("the attacker's Masako let the defender's tapped creature block")
	}
	onBoardCard(t, e, 0, corpusCard(t, "Masako the Humorless"))
	d := askBlockersFresh(t, e)
	o := findBlockOption(d, tapped, attacker)
	if o == nil {
		t.Fatalf("Masako's controller's tapped creature is not offered: %+v", d.Options)
	}
	if err := e.validateBlockers(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
		t.Fatalf("tapped blocker rejected: %v", err)
	}
}

// TestBlockRestrictFromMirrisTrigger: the Effect Mirri's attack trigger
// registers lets each opponent block with at most one creature this combat;
// the decision publishes the ceiling and the validator enforces it.
func TestBlockRestrictFromMirrisTrigger(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	mirri := onBoardCard(t, e, 1, corpusCard(t, "Mirri, Weatherlight Duelist"))
	other := onBoard(t, e, 1, statBearSrc)
	b0 := onBoard(t, e, 0, statBearSrc)
	b1 := onBoard(t, e, 0, statBearSrc)
	attackSeat0(t, e, mirri, other)
	d := askBlockersFresh(t, e)
	if d.Max < 2 {
		t.Fatalf("precondition: Max %d before the trigger resolves", d.Max)
	}
	f := e.G.Obj(mirri).Face()
	var effect *cards.SA
	for _, tr := range f.Triggers {
		if tr.Mode == "Attacks" {
			effect = cards.ResolveSVar(f.SVars, tr.ParamStr(cards.PKExecute))
		}
	}
	if effect == nil {
		t.Fatal("precondition: Mirri's attack trigger body not found")
	}
	effects.Resolve(e, &effects.Ctx{Source: mirri, Controller: 1, SVars: f.SVars}, effect)
	d = askBlockersFresh(t, e)
	if d.Max != 1 {
		t.Fatalf("Max = %d, want 1 under Mirri's restriction", d.Max)
	}
	two := []int{findBlockOption(d, b0, mirri).Index, findBlockOption(d, b1, other).Index}
	if err := e.validateBlockers(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: two}); err == nil {
		t.Fatal("two blockers accepted under Mirri's restriction")
	}
	if err := e.validateBlockers(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: two[:1]}); err != nil {
		t.Fatalf("one blocker rejected: %v", err)
	}
}

// TestOphidianAssignsNoCombatDamageAfterDrawing: resolving Ophidian's
// unblocked trigger draws a card and leaves Ophidian assigning no combat
// damage this turn (CR 510.1).
func TestOphidianAssignsNoCombatDamageAfterDrawing(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	ophidian := onBoardCard(t, e, 1, corpusCard(t, "Ophidian"))
	if got := e.combatDamageAmount(ophidian); got != 1 {
		t.Fatalf("precondition: Ophidian assigns %d, want 1", got)
	}
	f := e.G.Obj(ophidian).Face()
	body := cards.ResolveSVar(f.SVars, f.Triggers[0].ParamStr(cards.PKExecute))
	hand := len(e.G.Zone(state.ZHand, 1))
	effects.Resolve(e, &effects.Ctx{Source: ophidian, Controller: 1, SVars: f.SVars}, body)
	if got := len(e.G.Zone(state.ZHand, 1)); got != hand+1 {
		t.Fatalf("hand %d -> %d, want one card drawn", hand, got)
	}
	if got := e.combatDamageAmount(ophidian); got != 0 {
		t.Fatalf("Ophidian assigns %d combat damage after drawing, want 0", got)
	}
	other := onBoard(t, e, 1, statBearSrc)
	if got := e.combatDamageAmount(other); got != 2 {
		t.Fatalf("an unrelated creature assigns %d, want 2", got)
	}
}

// TestGormMustBeBlockedByTwo: CR 509.1c counts "must be blocked if able"
// and "must be blocked by two or more creatures if able" as two
// requirements. With three able blockers the defender must put two on Gorm;
// with one, that one must block it.
func TestGormMustBeBlockedByTwo(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	gorm := onBoardCard(t, e, 1, corpusCard(t, "Gorm the Great"))
	plain := onBoard(t, e, 1, statBearSrc)
	b := [3]state.ObjID{onBoard(t, e, 0, statBearSrc), onBoard(t, e, 0, statBearSrc), onBoard(t, e, 0, statBearSrc)}
	attackSeat0(t, e, gorm, plain)
	d := askBlockersFresh(t, e)
	if got := d.RequiredQuota(); got != 2 {
		t.Fatalf("RequiredQuota = %d, want 2", got)
	}
	opt := func(blocker, attacker state.ObjID) int { return findBlockOption(d, blocker, attacker).Index }
	check := func(want bool, choices ...int) {
		t.Helper()
		err := e.validateBlockers(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices})
		if (err == nil) != want {
			t.Fatalf("choices %v: accepted=%v, want %v (%v)", choices, err == nil, want, err)
		}
	}
	check(false)
	check(false, opt(b[0], gorm))
	check(false, opt(b[0], gorm), opt(b[1], plain), opt(b[2], plain))
	check(true, opt(b[0], gorm), opt(b[1], gorm))
	check(true, opt(b[0], gorm), opt(b[1], gorm), opt(b[2], plain))
	if err := e.validateBlockers(d, newTestBot(3).answer(e, d)); err != nil {
		t.Fatalf("bot answer rejected: %v", err)
	}

	// One able blocker: it must block Gorm, and that is the whole duty.
	e2 := threeSeatEngine(t)
	gorm2 := onBoardCard(t, e2, 1, corpusCard(t, "Gorm the Great"))
	only := onBoard(t, e2, 0, statBearSrc)
	attackSeat0(t, e2, gorm2)
	d2 := askBlockersFresh(t, e2)
	if got := d2.RequiredQuota(); got != 1 {
		t.Fatalf("one blocker: RequiredQuota = %d, want 1", got)
	}
	if err := e2.validateBlockers(d2, decision.Intent{Seq: d2.Seq, Player: d2.Player, Choices: []int{findBlockOption(d2, only, gorm2).Index}}); err != nil {
		t.Fatalf("one blocker on Gorm rejected: %v", err)
	}
}

// TestBalduvianWarlordReblocks: the Warlord removes a blocker from combat,
// the attacker it alone blocked becomes unblocked, and the removed creature
// then blocks the attacker its controller chooses -- a block made by an
// effect, which fires "becomes blocked" only for an attacker not already
// blocked.
func TestBalduvianWarlordReblocks(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	a1 := onBoard(t, e, 1, statBearSrc)
	a2 := onBoard(t, e, 1, statBearSrc)
	blocker := onBoard(t, e, 0, statBearSrc)
	warlord := onBoardCard(t, e, 0, corpusCard(t, "Balduvian Warlord"))
	e.G.Obj(warlord).SummonSick = false
	attackSeat0(t, e, a1, a2)
	d := askBlockersFresh(t, e)
	submitChoices(t, e, findBlockOption(d, blocker, a1).Index)
	if got := e.G.Obj(a1).BlockedBy; len(got) != 1 || got[0] != blocker {
		t.Fatalf("precondition: a1 blocked by %v", got)
	}
	td := activateObjAbility(t, e, 0, warlord)
	idx := indexOfObjOption(td, blocker)
	if idx < 0 {
		t.Fatalf("the blocking creature is not a target: %+v", td.Options)
	}
	submitChoices(t, e, idx)
	for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			e.priorityRound()
			continue
		}
		switch d.Kind {
		case decision.KPriority:
			submitChoices(t, e, passPriorityOption(t, d))
		default:
			pick := indexOfObjOption(d, a2)
			if pick < 0 {
				t.Fatalf("unexpected %v decision: %+v", d.Kind, d)
			}
			submitChoices(t, e, pick)
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatal("the Warlord's ability did not resolve")
	}
	if got := e.G.Obj(a1).BlockedBy; len(got) != 0 {
		t.Fatalf("a1 should be unblocked, BlockedBy %v", got)
	}
	if got := e.G.Obj(a2).BlockedBy; len(got) != 1 || got[0] != blocker {
		t.Fatalf("a2 should be blocked by the removed creature, BlockedBy %v", got)
	}
	if b := e.G.Obj(blocker); !b.IsBlocking {
		t.Fatal("the removed creature is not blocking")
	}
	last := -1
	for i, ev := range e.L.Events {
		if ev.Kind == events.DeclareBlockers {
			last = i
		}
	}
	if ev := e.L.Events[last]; ev.Obj != warlord || len(ev.Pairs) != 1 || ev.Pairs[0] != [2]state.ObjID{a2, blocker} {
		t.Fatalf("the effect's block event = %+v", ev)
	}
}

// TestFlagbearerMustBeTargeted: an opponent of a Flagbearer's controller
// casting a spell or activating an ability must choose a Flagbearer if able;
// the Flagbearer's controller is free; a multi-target ask keeps every option
// and publishes the floor.
func TestFlagbearerMustBeTargeted(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	guard := lookup(t, reg, "Coalition Honor Guard")
	bears := lookup(t, reg, "Grizzly Bears")
	bolt := lookup(t, reg, "Lightning Bolt")

	// An opponent's spell: only the Flagbearer is offered.
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{guard, bears}, []*cards.Card{bolt})
	g := moveByName(t, e, 0, "Coalition Honor Guard", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	moveByName(t, e, 1, "Lightning Bolt", state.ZHand)
	addMana(t, e, 1, "R")
	e.askPriority(1)
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Lightning Bolt's target decision, got %+v", d)
	}
	if len(d.Options) != 1 || d.Options[0].Obj != g {
		t.Fatalf("an opponent's Bolt must target the Flagbearer only: %+v", d.Options)
	}
	submitChoices(t, e, 0)
	replayCheck(t, e, cfg)

	// The Flagbearer's controller casting the same spell is not bound.
	own, _ := corpusEngineCfg(t, reg, []*cards.Card{guard, bears, bolt}, nil)
	moveByName(t, own, 0, "Coalition Honor Guard", state.ZBattlefield)
	ownBear := moveByName(t, own, 0, "Grizzly Bears", state.ZBattlefield)
	moveByName(t, own, 0, "Lightning Bolt", state.ZHand)
	addMana(t, own, 0, "R")
	own.askPriority(0)
	castFirst(t, own, "cast")
	if d := own.Pending(); d == nil || indexOfObjOption(d, ownBear) < 0 {
		t.Fatalf("the Flagbearer's controller was bound by it: %+v", d)
	}

	// An opponent's activated ability is bound the same way.
	ae, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Standard Bearer"), bears}, []*cards.Card{lookup(t, reg, "Cunning Sparkmage")})
	sb := moveByName(t, ae, 0, "Standard Bearer", state.ZBattlefield)
	moveByName(t, ae, 0, "Grizzly Bears", state.ZBattlefield)
	spark := moveByName(t, ae, 1, "Cunning Sparkmage", state.ZBattlefield)
	d = activateObjAbility(t, ae, 1, spark)
	if len(d.Options) != 1 || d.Options[0].Obj != sb {
		t.Fatalf("an opponent's activation must target the Flagbearer only: %+v", d.Options)
	}

	// A multi-target ask keeps every option; an answer must include the
	// Flagbearer.
	md := &decision.Decision{Player: 1, Kind: decision.KTarget, Min: 0, Max: 2,
		Options: []decision.Option{{Index: 0, Kind: "obj", Obj: g}, {Index: 1, Kind: "obj", Obj: bear}}}
	e.applyMustTarget(md, &pendingCast{player: 1, ability: -1})
	if len(md.Options) != 2 || md.MinSum != 1 || md.Min != 1 || md.Options[0].Value != 1 || md.Options[1].Value != 0 {
		t.Fatalf("multi-target floor not published: %+v", md)
	}
	if err := md.Validate(decision.Intent{Seq: md.Seq, Player: 1, Choices: []int{1}}); err == nil {
		t.Fatal("a multi-target answer without the Flagbearer was accepted")
	}
	if err := md.Validate(decision.Intent{Seq: md.Seq, Player: 1, Choices: []int{0, 1}}); err != nil {
		t.Fatalf("an answer with the Flagbearer was rejected: %v", err)
	}
}
