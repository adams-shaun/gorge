package rules

// TargetUnique$ ("every target this resolution chooses must be different")
// is read at the shared mid-resolution target pre-ask, so a root/SubAbility
// pair cannot aim "another target player/creature" at the parent's own target
// and a chain of TargetUnique asks accumulates its picks. The charms' cross-
// mode family (Shadrix Silverquill and the duo cycle) is a separate combined
// ask covered by charm_cross_mode_test.go.
//
// The carriers here inline the REAL corpus scripts (never a .cards file, per
// the licensing rule): biomantic_mastery.txt for the root/sub "another target
// player" shape, and know_evil.txt's three-rider shape for intra-chain
// accumulation. Only ManaCost/Types are simplified so the fixtures are cheap
// to cast; every target-bearing SA line and every parameter spelling is the
// corpus card's.

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// biomanticScript is biomantic_mastery.txt's script, verbatim except the
// Oracle line and the ManaCost (7 generic instead of the hybrid 4 GU GU GU,
// so the fixture funds with one colour). The root Draw declares "target
// player" and the SubAbility carries TargetUnique$ True — the corpus's own
// encoding of the Oracle's "another target player".
const biomanticScript = "Name:Biomantic Mastery\nManaCost:7\nTypes:Sorcery\n" +
	"A:SP$ Draw | Defined$ You | ValidTgts$ Player | NumCards$ X | SubAbility$ DBDraw | SpellDescription$ Draw a card for each creature target player controls, then draw a card for each creature another target player controls.\n" +
	"SVar:DBDraw:DB$ Draw | Defined$ You | ValidTgts$ Player | TargetUnique$ True | NumCards$ X\n" +
	"SVar:X:ThisTargetedPlayer$Valid Creature.YouCtrl\n" +
	"Oracle:x\n"

// pendingPlayerIDs returns the pending decision's player-kind option ids as a
// set, so a test can assert which players an ask offers.
func pendingPlayerIDs(t *testing.T, e *Engine) map[int]bool {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	out := map[int]bool{}
	for _, o := range d.Options {
		if o.Kind == "player" {
			out[int(o.Player)] = true
		}
	}
	return out
}

// TestBiomanticMasterySubTargetExcludesParentTarget pins the root/sub
// TargetUnique$ exclusion on the real corpus card: Biomantic Mastery's root
// targets a player, and the SubAbility's "another target player" ask must not
// offer that same player again. Before the shared constraint the sub pre-ask
// offered BOTH players and the deterministic first-option answer re-picked the
// parent's own target, so both draws counted the same player's creatures.
func TestBiomanticMasterySubTargetExcludesParentTarget(t *testing.T) {
	bear := "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 7001, biomanticScript, bear)
	// A creature for each player, so each Draw has a non-zero X and the two
	// halves are observable.
	putCreature(t, e, 0, bear)
	putToken(t, e, 1, bear, state.ZBattlefield) // seat 1's deck is mountains only

	addMana(t, e, 0, "GGGGGGG") // 7 generic (the simplified ManaCost)
	castFirst(t, e, "cast")

	// CR 601.2c: the root's own target ask. Choose the OPPONENT (seat 1).
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("root target ask = %+v, want KTarget", d)
	}
	if !pendingPlayerIDs(t, e)[1] {
		t.Fatalf("root target ask does not offer seat 1: %+v", d.Options)
	}
	rootChoice := -1
	for _, o := range d.Options {
		if o.Kind == "player" && int(o.Player) == 1 {
			rootChoice = o.Index
		}
	}
	if rootChoice < 0 {
		t.Fatalf("no seat-1 option in root ask: %+v", d.Options)
	}
	submitChoices(t, e, rootChoice)

	// Pass priority until the spell resolves to the SubAbility's pre-ask.
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while awaiting the sub target ask")
		}
		if d.Kind != decision.KPriority {
			break
		}
		for _, o := range d.Options {
			if o.Kind == "pass" {
				submitChoices(t, e, o.Index)
				break
			}
		}
	}

	// The SubAbility's own pre-ask (KChoose, resume kind "tgts"). TargetUnique$
	// must exclude seat 1, the root's already-chosen target.
	sub := e.Pending()
	if sub == nil || sub.Kind != decision.KChoose || sub.ResumeKind != "tgts" {
		t.Fatalf("sub pre-ask = %+v, want a KChoose tgts ask", sub)
	}
	offered := pendingPlayerIDs(t, e)
	if offered[1] {
		t.Fatalf("sub TargetUnique$ ask re-offers the parent's own target (seat 1): %+v", sub.Options)
	}
	if !offered[0] {
		t.Fatalf("sub ask offers no legal different player: %+v", sub.Options)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 30)
	replayCheck(t, e, cfg)
}

// chainUniqueScript is a synthetic carrier of know_evil.txt's three-rider
// shape: a root with no targeting followed by chained `ValidTgts$ Player |
// TargetUnique$ True | TargetMin$ 0 | TargetMax$ 1` riders.
// The target-bearing lines are the corpus card's exact parameter spellings;
// only the Effect riders are replaced by Draw so the fixture needs no
// Scheme/static machinery.
func chainUniqueScript() string {
	return "Name:Chain Unique\nManaCost:1\nTypes:Sorcery\n" +
		"A:SP$ Draw | NumCards$ 1 | SubAbility$ R1\n" +
		"SVar:R1:DB$ Draw | NumCards$ 0 | ValidTgts$ Player | TargetUnique$ True | TargetMin$ 0 | TargetMax$ 1 | SubAbility$ R2\n" +
		"SVar:R2:DB$ Draw | NumCards$ 0 | ValidTgts$ Player | TargetUnique$ True | TargetMin$ 0 | TargetMax$ 1\n" +
		"Oracle:x\n"
}

// TestTargetUniqueAccumulatesAcrossTheChain pins the intra-chain half of the
// shared constraint: the second TargetUnique$ rider in one resolution excludes
// not only the placement targets (there are none here — the root is
// untargeted) but also the target an EARLIER TargetUnique$ rider in the same
// chain chose. Without Ctx.TargetsUnique accumulation the second rider offers
// both players again.
func TestTargetUniqueAccumulatesAcrossTheChain(t *testing.T) {
	e, cfg, _ := newFixtureDeck(t, 7002, chainUniqueScript())
	addMana(t, e, 0, "C")
	castFirst(t, e, "cast")

	// Pass priority until the spell resolves to the first rider's pre-ask.
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while awaiting the first rider ask")
		}
		if d.Kind != decision.KPriority {
			break
		}
		for _, o := range d.Options {
			if o.Kind == "pass" {
				submitChoices(t, e, o.Index)
				break
			}
		}
	}

	// First rider's ask: both players.
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "tgts" {
		t.Fatalf("first rider ask = %+v, want KChoose tgts", d)
	}
	first := pendingPlayerIDs(t, e)
	if !first[0] || !first[1] {
		t.Fatalf("first rider should offer both players: %+v", d.Options)
	}
	pick0 := -1
	for _, o := range d.Options {
		if o.Kind == "player" && int(o.Player) == 0 {
			pick0 = o.Index
		}
	}
	if pick0 < 0 {
		t.Fatalf("no seat-0 option in first rider ask: %+v", d.Options)
	}
	submitChoices(t, e, pick0)

	// Second rider's ask must exclude seat 0, chosen by the FIRST rider.
	d2 := e.Pending()
	if d2 == nil || d2.Kind != decision.KChoose || d2.ResumeKind != "tgts" {
		t.Fatalf("second rider ask = %+v, want KChoose tgts", d2)
	}
	second := pendingPlayerIDs(t, e)
	if second[0] {
		t.Fatalf("second rider re-offers the first rider's target (seat 0): %+v", d2.Options)
	}
	if !second[1] {
		t.Fatalf("second rider offers no legal different player: %+v", d2.Options)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 30)
	replayCheck(t, e, cfg)
}

// cyberneticaScript is cybernetica_datasmith.txt's ability shape, verbatim
// except the ManaCost/Type lines: the root Draw declares "target player" and
// the SubAbility Token carries TargetUnique$ True — the corpus's encoding of
// the Oracle's "Another target player creates ...". This is the brief's
// api:Token instance (the api:Draw instance is Biomantic Mastery above).
const cyberneticaScript = "Name:Cybernetica Datasmith\nManaCost:1\nTypes:Artifact Creature Human Artificer\nPT:0/1\n" +
	"A:SP$ Draw | NumCards$ 1 | ValidTgts$ Player | SubAbility$ DBToken | SpellDescription$ Target player draws a card. Another target player creates a 4/4 colorless Robot artifact creature token with \"This creature can't block.\"\n" +
	"SVar:DBToken:DB$ Token | TokenScript$ c_4_4_a_robot_noblock | ValidTgts$ Player | TargetUnique$ True | TokenOwner$ ThisTargetedPlayer\n" +
	"Oracle:x\n"

const robotScript = "Name:Robot\nManaCost:\nTypes:Artifact Creature Robot\nPT:4/4\nOracle:x\n"

// TestCyberneticaTokenSubTargetExcludesParentTarget pins the api:Token half of
// the brief: the Token SubAbility's "another target player" must not create the
// token under the root's own target. It asserts the exclusion ask AND that the
// token actually lands under the DIFFERENT player (TokenOwner$
// ThisTargetedPlayer reads the sub's answered target).
func TestCyberneticaTokenSubTargetExcludesParentTarget(t *testing.T) {
	e, cfg, _ := newFixtureDeck(t, 7003, cyberneticaScript)
	e.G.Tokens["c_4_4_a_robot_noblock"] = card(t, robotScript)
	addMana(t, e, 0, "C")
	castFirst(t, e, "cast")

	// Root target ask: choose seat 1.
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("root target ask = %+v, want KTarget", d)
	}
	pick1 := -1
	for _, o := range d.Options {
		if o.Kind == "player" && int(o.Player) == 1 {
			pick1 = o.Index
		}
	}
	if pick1 < 0 {
		t.Fatalf("root ask does not offer seat 1: %+v", d.Options)
	}
	submitChoices(t, e, pick1)

	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while awaiting the Token sub ask")
		}
		if d.Kind != decision.KPriority {
			break
		}
		for _, o := range d.Options {
			if o.Kind == "pass" {
				submitChoices(t, e, o.Index)
				break
			}
		}
	}

	sub := e.Pending()
	if sub == nil || sub.Kind != decision.KChoose || sub.ResumeKind != "tgts" {
		t.Fatalf("token sub ask = %+v, want KChoose tgts", sub)
	}
	offered := pendingPlayerIDs(t, e)
	if offered[1] {
		t.Fatalf("token TargetUnique$ ask re-offers the parent's target (seat 1): %+v", sub.Options)
	}
	if !offered[0] {
		t.Fatalf("token ask offers no legal different player: %+v", sub.Options)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 30)

	// The token must exist under seat 0 (the DIFFERENT player), not seat 1.
	found := false
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Face() != nil && o.Face().Name == "Robot" && o.Zone == state.ZBattlefield {
			found = true
			if o.Controller != 0 {
				t.Fatalf("Robot token controller = %d, want seat 0 (the different target)", o.Controller)
			}
		}
	}
	if !found {
		t.Fatal("no Robot token on the battlefield after resolution")
	}
	replayCheck(t, e, cfg)
}
