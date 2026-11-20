package rules

// Combat restriction keywords (CR 508.1c-d, 509.1b-c, 702.28): the
// "can block shadow / as though reach" statics, "must be blocked by all /
// exactly one", and the attack/block companion restrictions ("can only attack
// alone", "can't attack unless ... also attacks"). Each test drives the real
// declare-attackers / declare-blockers decision and reads the three places a
// whole-declaration rule must reach: the wire validator, the engine's own
// validator and the bot repair.

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	shadowAttackerSrc = "Name:Shade Attacker\nTypes:Creature Shade\nPT:1/1\nK:Shadow\nOracle:x\n"
	dragonAttackerSrc = "Name:Test Dragon\nTypes:Creature Dragon\nPT:3/3\nK:Flying\nOracle:x\n"
	flyerAttackerSrc  = "Name:Test Flyer\nTypes:Creature Bird\nPT:2/2\nK:Flying\nOracle:x\n"
	wallSrc           = "Name:Test Wall\nTypes:Creature Wall\nPT:0/4\nOracle:x\n"
)

// combatKeywordPair reports whether the declare-blockers ask offers the pair.
func combatKeywordPair(d *decision.Decision, blocker, attacker state.ObjID) bool {
	return d != nil && findBlockOption(d, blocker, attacker) != nil
}

// TestAetherWebBlocksShadowCreature: a creature enchanted with Aether Web
// blocks a shadow attacker (CR 702.28b's "as though they didn't have
// shadow"); the same creature without the Aura cannot.
func TestAetherWebBlocksShadowCreature(t *testing.T) {
	t.Parallel()
	web := corpusCard(t, "Aether Web")

	control := threeSeatEngine(t)
	shade0 := onBoard(t, control, 1, shadowAttackerSrc)
	bear0 := onBoard(t, control, 0, statBearSrc)
	attackSeat0(t, control, shade0)
	if combatKeywordPair(askBlockersFresh(t, control), bear0, shade0) {
		t.Fatal("precondition: a plain bear blocks a shadow creature without Aether Web")
	}

	e := threeSeatEngine(t)
	shade := onBoard(t, e, 1, shadowAttackerSrc)
	bear := onBoard(t, e, 0, statBearSrc)
	aura := onBoardCard(t, e, 0, web)
	e.G.Obj(aura).AttachedTo = bear
	attackSeat0(t, e, shade)
	d := askBlockersFresh(t, e)
	if !combatKeywordPair(d, bear, shade) {
		t.Fatalf("an Aether Web creature does not block the shadow attacker: %+v", d)
	}
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{findBlockOption(d, bear, shade).Index}}
	if err := e.validateBlockers(d, in); err != nil {
		t.Fatalf("validateBlockers rejected the Aether Web block: %v", err)
	}
}

// TestDragonHunterBlocksFlyerAsThoughReach: Dragon Hunter blocks a flying
// Dragon, and only a Dragon (the static's ValidAttacker$).
func TestDragonHunterBlocksFlyerAsThoughReach(t *testing.T) {
	t.Parallel()
	hunterCard := corpusCard(t, "Dragon Hunter")

	e := threeSeatEngine(t)
	dragon := onBoard(t, e, 1, dragonAttackerSrc)
	hunter := onBoardCard(t, e, 0, hunterCard)
	attackSeat0(t, e, dragon)
	d := askBlockersFresh(t, e)
	if !combatKeywordPair(d, hunter, dragon) {
		t.Fatalf("Dragon Hunter does not block the flying Dragon: %+v", d)
	}

	other := threeSeatEngine(t)
	bird := onBoard(t, other, 1, flyerAttackerSrc)
	hunter2 := onBoardCard(t, other, 0, hunterCard)
	attackSeat0(t, other, bird)
	if combatKeywordPair(askBlockersFresh(t, other), hunter2, bird) {
		t.Fatal("Dragon Hunter blocks a flying non-Dragon")
	}
}

// TestMarblePriestMustBeBlockedByAllWalls: every Wall able to block Marble
// Priest must (CR 509.1c); a non-Wall blocker is not bound.
func TestMarblePriestMustBeBlockedByAllWalls(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	priest := onBoardCard(t, e, 1, corpusCard(t, "Marble Priest"))
	w1 := onBoard(t, e, 0, wallSrc)
	w2 := onBoard(t, e, 0, wallSrc)
	bear := onBoard(t, e, 0, statBearSrc)
	attackSeat0(t, e, priest)
	d := askBlockersFresh(t, e)
	if d == nil {
		t.Fatal("no declare-blockers decision")
	}
	for _, o := range []state.ObjID{w1, w2} {
		if opt := findBlockOption(d, o, priest); opt == nil || !opt.BlockMust {
			t.Fatalf("Wall %d is not offered as a required block on Marble Priest: %+v", o, opt)
		}
	}
	if opt := findBlockOption(d, bear, priest); opt == nil || opt.BlockMust {
		t.Fatalf("the non-Wall bear must be offered, unbound: %+v", opt)
	}
	check := func(want bool, choices ...int) {
		t.Helper()
		in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}
		err := e.validateBlockers(d, in)
		if (err == nil) != want {
			t.Fatalf("choices %v: accepted=%v, want %v (%v)", choices, err == nil, want, err)
		}
	}
	o := func(b state.ObjID) int { return findBlockOption(d, b, priest).Index }
	check(false)
	check(false, o(w1))
	check(false, o(w1), o(bear))
	check(true, o(w1), o(w2))
	check(true, o(w1), o(w2), o(bear))
	if err := e.validateBlockers(d, newTestBot(5).answer(e, d)); err != nil {
		t.Fatalf("bot answer rejected: %v", err)
	}
}

// TestNacatlWarPrideBlockedByExactlyOne: with two able blockers the defender
// must declare exactly one on Nacatl War-Pride; two are refused, and so is
// none while one could block.
func TestNacatlWarPrideBlockedByExactlyOne(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	nacatl := onBoardCard(t, e, 1, corpusCard(t, "Nacatl War-Pride"))
	b1 := onBoard(t, e, 0, statBearSrc)
	b2 := onBoard(t, e, 0, statBearSrc)
	attackSeat0(t, e, nacatl)
	d := askBlockersFresh(t, e)
	if d == nil {
		t.Fatal("no declare-blockers decision")
	}
	if got := d.RequiredQuota(); got != 1 {
		t.Fatalf("RequiredQuota = %d, want 1", got)
	}
	o := func(b state.ObjID) int { return findBlockOption(d, b, nacatl).Index }
	check := func(want bool, choices ...int) {
		t.Helper()
		in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}
		err := e.validateBlockers(d, in)
		if (err == nil) != want {
			t.Fatalf("choices %v: accepted=%v, want %v (%v)", choices, err == nil, want, err)
		}
		// The required quota (the empty answer) is the engine's rule; the wire
		// validator carries the [1,1] blocker bound the options publish.
		if len(choices) == 0 {
			return
		}
		if derr := d.Validate(in); (derr == nil) != want {
			t.Fatalf("choices %v: Decision.Validate accepted=%v, want %v (%v)", choices, derr == nil, want, derr)
		}
	}
	check(false)
	check(true, o(b1))
	check(true, o(b2))
	check(false, o(b1), o(b2))
	if err := e.validateBlockers(d, newTestBot(7).answer(e, d)); err != nil {
		t.Fatalf("bot answer rejected: %v", err)
	}
}

// TestMasterOfCrueltiesAttacksAlone: "can only attack alone" refuses any
// declaration naming another attacker beside it.
func TestMasterOfCrueltiesAttacksAlone(t *testing.T) {
	t.Parallel()
	e, d, idx := aloneAttackersTable(t, "Master of Cruelties", "Elite Vanguard")
	master, other := idx["Master of Cruelties"], idx["Elite Vanguard"]
	check := func(want bool, choices ...int) {
		t.Helper()
		in := decision.Intent{Seq: d.Seq, Player: 0, Choices: choices}
		if err := e.validateAttackers(d, in); (err == nil) != want {
			t.Fatalf("choices %v: validateAttackers accepted=%v, want %v (%v)", choices, err == nil, want, err)
		}
		if err := d.Validate(in); (err == nil) != want {
			t.Fatalf("choices %v: Decision.Validate accepted=%v, want %v (%v)", choices, err == nil, want, err)
		}
	}
	check(true, master)
	check(true, other)
	check(false, master, other)
	// The bot repair keeps its answer legal: a rules-ignorant "attack with
	// everything" answer is repaired, not refused.
	in := botpolicy.Clamp(d, decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{master, other}})
	if err := e.validateAttackers(d, in); err != nil {
		t.Fatalf("the repaired bot answer %v is rejected: %v", in.Choices, err)
	}
}

// TestOkkNeedsGreaterPowerAttacker: Okk alone is refused; beside a creature
// with greater power it is legal; beside a smaller one it is refused.
func TestOkkNeedsGreaterPowerAttacker(t *testing.T) {
	t.Parallel()
	e, d, idx := aloneAttackersTable(t, "Okk", "Craw Wurm", "Elite Vanguard")
	okk, big, small := idx["Okk"], idx["Craw Wurm"], idx["Elite Vanguard"]
	check := func(want bool, choices ...int) {
		t.Helper()
		in := decision.Intent{Seq: d.Seq, Player: 0, Choices: choices}
		if err := e.validateAttackers(d, in); (err == nil) != want {
			t.Fatalf("choices %v: validateAttackers accepted=%v, want %v (%v)", choices, err == nil, want, err)
		}
		if err := d.Validate(in); (err == nil) != want {
			t.Fatalf("choices %v: Decision.Validate accepted=%v, want %v (%v)", choices, err == nil, want, err)
		}
	}
	check(false, okk)
	check(false, okk, small)
	check(true, okk, big)
	check(true, okk, big, small)
	check(true, big, small)
	in := botpolicy.Clamp(d, decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{okk, small}})
	if err := e.validateAttackers(d, in); err != nil {
		t.Fatalf("the repaired bot answer %v is rejected: %v", in.Choices, err)
	}
}

// TestOkkAndKinCantBlockUnlessCompanion: the block-side sentence of Okk and
// Orcish Conscripts needs a creature with greater power / two others
// blocking too, and Scarred Puma needs a black or green attacker.
func TestOkkAndKinCantBlockUnlessCompanion(t *testing.T) {
	t.Parallel()
	_, d, idx := aloneBlockersTable(t, "Okk", "Craw Wurm")
	okk, big := idx["Okk"], idx["Craw Wurm"]
	for _, tc := range []struct {
		want    bool
		choices []int
	}{{false, []int{okk}}, {true, []int{okk, big}}, {true, []int{big}}} {
		if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: tc.choices}); (err == nil) != tc.want {
			t.Fatalf("block choices %v: accepted=%v, want %v (%v)", tc.choices, err == nil, tc.want, err)
		}
	}

	ce, cd, cidx := aloneAttackersTable(t, "Orcish Conscripts", "Elite Vanguard", "Llanowar Elves")
	cons, v1, v2 := cidx["Orcish Conscripts"], cidx["Elite Vanguard"], cidx["Llanowar Elves"]
	for _, tc := range []struct {
		want    bool
		choices []int
	}{{false, []int{cons}}, {false, []int{cons, v1}}, {true, []int{cons, v1, v2}}} {
		in := decision.Intent{Seq: cd.Seq, Player: 0, Choices: tc.choices}
		if err := ce.validateAttackers(cd, in); (err == nil) != tc.want {
			t.Fatalf("Orcish Conscripts %v: accepted=%v, want %v (%v)", tc.choices, err == nil, tc.want, err)
		}
	}

	e, pd, pidx := aloneAttackersTable(t, "Scarred Puma", "Elite Vanguard", "Llanowar Elves")
	puma, white, green := pidx["Scarred Puma"], pidx["Elite Vanguard"], pidx["Llanowar Elves"]
	for _, tc := range []struct {
		want    bool
		choices []int
	}{{false, []int{puma}}, {false, []int{puma, white}}, {true, []int{puma, green}}} {
		in := decision.Intent{Seq: pd.Seq, Player: 0, Choices: tc.choices}
		if err := e.validateAttackers(pd, in); (err == nil) != tc.want {
			t.Fatalf("Scarred Puma %v: accepted=%v, want %v (%v)", tc.choices, err == nil, tc.want, err)
		}
	}
}

// TestEveryCombatRestrictionCardIsPlayable: the 13 cards of the combat
// restriction keyword class link and leave Registry.Unsupported.
func TestEveryCombatRestrictionCardIsPlayable(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	supported := effects.Supported()
	names := []string{
		"Aether Web", "Aetherflame Wall", "Heartwood Dryad", "Wall of Diffusion",
		"Dragon Hunter", "Marble Priest", "Talruum Piper", "Nacatl War-Pride",
		"Master of Cruelties", "Okk", "Orcish Conscripts", "Scarred Puma", "Johan",
	}
	var missing []string
	for _, n := range names {
		c := searchCorpusCard(t, reg, n)
		if d := c.Link(); len(d) != 0 {
			t.Errorf("link %s: %v", n, d)
		}
		if m := reg.Unsupported(c, supported); len(m) != 0 {
			missing = append(missing, n+": "+joinStrings(m))
		}
	}
	if len(missing) != 0 {
		t.Fatalf("combat restriction cards still unsupported: %v", missing)
	}
	_ = cards.KeywordHead
}

func joinStrings(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}

// TestJohanAttackVigilance: Johan's AttackVigilance Effect keeps the
// attackers it covers untapped while Johan is untapped, and lapses with it.
func TestJohanAttackVigilance(t *testing.T) {
	t.Parallel()
	run := func(tapJohan bool) bool {
		e, d, idx := aloneAttackersTable(t, "Johan", "Elite Vanguard")
		johan := d.Options[idx["Johan"]].Obj
		f := e.G.Obj(johan).Face()
		body := cards.ResolveSVar(f.SVars, "DBEffect")
		effects.Resolve(e, &effects.Ctx{Source: johan, Controller: 0, SVars: f.SVars}, body)
		if tapJohan {
			e.G.Obj(johan).Tapped = true
		}
		vanguard := d.Options[idx["Elite Vanguard"]].Obj
		submitChoices(t, e, idx["Elite Vanguard"])
		return e.G.Obj(vanguard).Tapped
	}
	if run(false) {
		t.Fatal("an attacker tapped although Johan's AttackVigilance effect holds")
	}
	if !run(true) {
		t.Fatal("precondition: an attacker stayed untapped with Johan tapped")
	}
}
