package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// This file pins rules.PotentialActions and rules.PotentialMana — the
// server-side projection the auto-pass stop decision reads (view's
// potential_actions, rv2c): the engine's own legal-offer walk priced against
// the hypothetical pool the seat's untapped sources could produce. Every case
// here is one of the eight client-side pricing defects the projection
// replaces, so the client never re-derives castability again. The fixture
// board of the first test is the Jitte feedback snapshot's capture point
// (feedback/20260914T145022Z-64a8422c, seq 482): seat 0, main1, pool {C}{W},
// both lands tapped, Thalia on the battlefield raising the Jitte to {3}.

// jitteHandSrcs are the capture-point hand cards except Serra Avenger (whose
// cast carries a turn-4 Condition static the projection need not resolve to
// answer the Jitte question; the pool {C}{W} cannot pay her {W}{W} either).
var jitteHandSrcs = []string{
	"Name:Batterskull\nManaCost:5\nTypes:Artifact Equipment\nOracle:Living weapon\n",
	"Name:Palace Jailer\nManaCost:2 W W\nTypes:Creature Human Soldier\nPT:2/2\nOracle:When this enters, become the monarch.\n",
	"Name:Umezawa's Jitte\nManaCost:2\nTypes:Legendary Artifact Equipment\nK:Equip:2\nOracle:Whenever equipped creature deals combat damage, put two charge counters on CARDNAME.\n",
	"Name:Sanctum Prelate\nManaCost:1 W W\nTypes:Creature Human Cleric\nPT:2/2\nOracle:x\n",
	"Name:Recruiter of the Guard\nManaCost:2 W\nTypes:Creature Human Soldier\nPT:1/1\nOracle:x\n",
}

const thaliaSrc = "Name:Thalia, Guardian of Thraben\nManaCost:1 W\nTypes:Legendary Creature Human Soldier\nPT:2/1\nK:First Strike\n" +
	"S:Mode$ RaiseCost | ValidCard$ Card.nonCreature | Type$ Spell | Amount$ 1 | Description$ Noncreature spells cost {1} more to cast.\n" +
	"Oracle:First strike. Noncreature spells cost {1} more to cast.\n"

const jitteSnapshotPlains = "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n"

const jitteSnapshotWasteland = "Name:Wasteland\nTypes:Land Wasteland\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\n" +
	"Oracle:{T}: Add {C}.\n"

// jitteSnapshotBoard builds the capture-point board: the two lands already
// tapped (the reporter tapped them for the {C}{W} in the pool), Thalia
// untapped, and seat 0 in main1 with the snapshot pool {C:1, W:1}. The
// opening hand's Mountains are cleared (the reporter's hand held no lands)
// and the land drop is spent, so a play_land can never mask the cast answer.
func jitteSnapshotBoard(t *testing.T, withThalia bool) (*Engine, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	e.G.Step = state.StepMain1
	e.G.Players[0].LandsPlayed = 1
	e.G.SetZone(state.ZHand, 0, nil)
	pool := e.G.Players[0].Pool
	pool[state.MC] = 1
	pool[state.MW] = 1
	e.G.Players[0].Pool = pool
	plains := onBoard(t, e, 0, jitteSnapshotPlains)
	e.G.Obj(plains).SummonSick = false
	e.G.Obj(plains).Tapped = true
	wasteland := onBoard(t, e, 0, jitteSnapshotWasteland)
	e.G.Obj(wasteland).SummonSick = false
	e.G.Obj(wasteland).Tapped = true
	if withThalia {
		onBoard(t, e, 0, thaliaSrc)
	}
	var jitte state.ObjID
	for _, src := range jitteHandSrcs {
		id := onHand(t, e, 0, src)
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Umezawa's Jitte" {
			jitte = id
		}
	}
	return e, jitte
}

// TestPotentialActionsJitteThaliaSnapshotIsNotCastable pins the Jitte report's
// real defect on the projection: at the capture point the engine rightly
// refused the cast (Thalia raises the {2} Jitte to {3}, the pool is {C}{W}
// and BOTH lands are already tapped), so potential_actions carries no cast at
// all — the new client reads that and SKIPS the window the old client stopped
// for. The contrast half proves the RaiseCost is what the projection carries:
// without Thalia the same board makes the Jitte cast potential, because the
// pool CAN pay the printed {2}.
func TestPotentialActionsJitteThaliaSnapshotIsNotCastable(t *testing.T) {
	e, jitte := jitteSnapshotBoard(t, true)
	acts := e.PotentialActions(0)
	for _, a := range acts {
		if a.Kind == "cast" {
			t.Errorf("Jitte snapshot: seat 0 potential actions carry a cast %q (obj %d); want none — Thalia raises Jitte to {3} and the pool is {C}{W}", a.Label, a.Obj)
		}
	}
	for _, a := range acts {
		if a.Kind == "activate" || a.Kind == "pass" || a.Kind == "concede" {
			t.Errorf("potential_actions must never carry kind %q", a.Kind)
		}
	}

	// Contrast: without Thalia, the same pool pays the printed {2}, so the
	// cast IS potential — the projection carries the engine's live cost
	// rules, not the printed number.
	e2, jitte2 := jitteSnapshotBoard(t, false)
	found := false
	for _, a := range e2.PotentialActions(0) {
		if a.Kind == "cast" && a.Obj == jitte2 {
			found = true
		}
	}
	if !found {
		t.Fatal("without Thalia the Jitte cast must be a potential action (pool {C}{W} pays {2})")
	}
	if jitte == 0 || jitte2 == 0 {
		t.Fatal("fixture lost the Jitte object")
	}
}

// TestPotentialActionsReduceCostOffered pins the Medallion side: a live
// ReduceCost static on the battlefield composes into the offer cost, so the
// {1}{W} spell is castable from one untapped Plains plus the floating pool —
// even though the floating pool alone (empty) offers no cast. The old client
// priced the printed cost and passed the window; the projection stops it.
func TestPotentialActionsReduceCostOffered(t *testing.T) {
	e := layerEngine(t)
	e.G.Step = state.StepMain1
	e.G.SetZone(state.ZHand, 0, nil)
	e.G.Players[0].LandsPlayed = 1
	onBoard(t, e, 0, jitteSnapshotPlains) // untapped: PotentialMana W:1
	onBoard(t, e, 0, "Name:Pearl Medallion\nManaCost:2\nTypes:Artifact\n"+
		"S:Mode$ ReduceCost | ValidCard$ Card.White | Type$ Spell | Activator$ You | Amount$ 1 | Description$ White spells you cast cost {1} less to cast.\n"+
		"Oracle:White spells you cast cost {1} less to cast.\n")
	spell := onHand(t, e, 0, "Name:Divine Verdict\nManaCost:1 W\nTypes:Instant\nOracle:W {1}\n")

	// The REAL offer (floating pool empty) carries no cast: the
	// float-then-cast model is why the projection exists.
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" && o.Obj == spell {
			t.Fatal("empty floating pool must not offer the cast directly")
		}
	}
	found := false
	for _, a := range e.PotentialActions(0) {
		if a.Kind == "cast" && a.Obj == spell {
			found = true
		}
	}
	if !found {
		t.Fatal("Pearl Medallion reduction: the {1}{W} spell must be a potential cast from one untapped Plains")
	}
}

// TestPotentialActionsEquipAbility pins the activate-effects half: an Equip
// {2} (a SorcerySpeed$ Attach ability) on an untapped permanent with two
// untapped Plains behind it is a potential ABILITY, though the floating pool
// is empty. The old client skipped every activation without {T} plus floated
// mana and read the window as empty.
func TestPotentialActionsEquipAbility(t *testing.T) {
	e := layerEngine(t)
	e.G.Step = state.StepMain1
	e.G.SetZone(state.ZHand, 0, nil)
	e.G.Players[0].LandsPlayed = 1
	onBoard(t, e, 0, jitteSnapshotPlains)
	onBoard(t, e, 0, jitteSnapshotPlains)
	onBoard(t, e, 0, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	bf := onHand(t, e, 0, "Name:Bonehoard Dracosaur\nManaCost:0\nTypes:Artifact Equipment\nK:Equip:2\nOracle:Equip {2}\n")
	e.G.Obj(bf).Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), bf))
	found := false
	for _, a := range e.PotentialActions(0) {
		if a.Kind == "ability" && a.Obj == bf {
			found = true
		}
	}
	if !found {
		t.Fatal("Equip {2} with two untapped Plains must be a potential ability")
	}
}

// TestPotentialActionsCommandZoneCast pins the command-zone half: in a
// Commander game the commander sitting in the command zone is a potential
// cast once the seat's untapped sources cover its (taxed) cost, though the
// floating pool is empty. The old client scanned only the hand.
func TestPotentialActionsCommandZoneCast(t *testing.T) {
	e, _ := commanderGame(t, commanderDamageSeed, FormatCommander, 0, [][]string{
		{"Name:Giada, Font of Hope\nManaCost:1 W\nTypes:Legendary Creature Angel\nPT:2/2\nOracle:x\n"},
		{"Name:Beatstick\nManaCost:0\nTypes:Creature Zombie\nPT:1/1\nOracle:x\n"},
	})
	e.G.Step = state.StepMain1
	e.G.SetZone(state.ZHand, 0, nil)
	e.G.Players[0].LandsPlayed = 1
	onBoard(t, e, 0, jitteSnapshotPlains)
	onBoard(t, e, 0, jitteSnapshotPlains)
	cmd := e.G.Zone(state.ZCommand, 0)[0]
	found := false
	for _, a := range e.PotentialActions(0) {
		if a.Kind == "cast" && a.Obj == cmd {
			found = true
		}
	}
	if !found {
		t.Fatal("Giada in the command zone with two untapped Plains must be a potential cast")
	}
}

// TestPotentialActionsFlashbackCast pins the graveyard half: a Lingering
// Souls in the graveyard is a potential cast for its flashback cost {1}{B}
// once the seat's untapped sources cover it. The old client scanned only the
// hand.
func TestPotentialActionsFlashbackCast(t *testing.T) {
	e := layerEngine(t)
	e.G.Step = state.StepMain1
	e.G.SetZone(state.ZHand, 0, nil)
	e.G.Players[0].LandsPlayed = 1
	onBoard(t, e, 0, "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n")
	onBoard(t, e, 0, jitteSnapshotPlains)
	souls := onHand(t, e, 0, "Name:Lingering Souls\nManaCost:2 W\nTypes:Sorcery\nK:Flashback:1 B\n"+
		"Oracle:Create two 1/1 white Spirit creature tokens with flying.\n")
	o := e.G.Obj(souls)
	o.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, append(e.G.Zone(state.ZGraveyard, 0), souls))
	e.G.SetZone(state.ZHand, 0, nil)
	found := false
	for _, a := range e.PotentialActions(0) {
		if a.Kind == "cast" && a.Obj == souls && a.Mode == "flashback" {
			found = true
		}
	}
	if !found {
		t.Fatal("Lingering Souls flashback {1}{B} from the graveyard must be a potential cast")
	}
}

// TestPotentialActionsIndeterminateSource pins the Tron half: three untapped
// Urza lands (Amount$ UrzaAmount — an indeterminate amount the static pricing
// resolves to zero) make the {7} creature a potential cast, though the
// floating pool is empty and the fixed-colour bound would see nothing.
func TestPotentialActionsIndeterminateSource(t *testing.T) {
	e := layerEngine(t)
	e.G.Step = state.StepMain1
	e.G.SetZone(state.ZHand, 0, nil)
	e.G.Players[0].LandsPlayed = 1
	for _, src := range []string{
		"Name:Urza's Mine\nTypes:Land Urza's Mine\nA:AB$ Mana | Cost$ T | Produced$ C | Amount$ UrzaAmount | SpellDescription$ Add {C}.\nOracle:x\n",
		"Name:Urza's Power-Plant\nTypes:Land Urza's Power-Plant\nA:AB$ Mana | Cost$ T | Produced$ C | Amount$ UrzaAmount | SpellDescription$ Add {C}.\nOracle:x\n",
		"Name:Urza's Tower\nTypes:Land Urza's Tower\nA:AB$ Mana | Cost$ T | Produced$ C | Amount$ UrzaAmount | SpellDescription$ Add {C}.\nOracle:x\n",
	} {
		onBoard(t, e, 0, src)
	}
	big := onHand(t, e, 0, "Name:Wandering Archon\nManaCost:7\nTypes:Creature Archon\nPT:1/1\nOracle:x\n")
	found := false
	for _, a := range e.PotentialActions(0) {
		if a.Kind == "cast" && a.Obj == big {
			found = true
		}
	}
	if !found {
		t.Fatal("three indeterminate Urza lands must make the {7} creature a potential cast")
	}
}

// TestPotentialActionsBlazeAtXZero pins the X half: an X spell is a potential
// cast at X=0 from whatever its fixed pips need — CR 107.3b — the shape the
// old client's variable-pip refusal ate (the pin autopilot.test.ts used to
// carry). One untapped Mountain behind a floating {R} is enough.
func TestPotentialActionsBlazeAtXZero(t *testing.T) {
	e := layerEngine(t)
	e.G.Step = state.StepMain1
	e.G.SetZone(state.ZHand, 0, nil)
	e.G.Players[0].LandsPlayed = 1
	onBoard(t, e, 0, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	pool := e.G.Players[0].Pool
	pool[state.MR] = 1
	e.G.Players[0].Pool = pool
	blaze := onHand(t, e, 0, "Name:Blaze\nManaCost:X R\nTypes:Sorcery\nOracle:Blaze deals X damage to any target.\n")
	found := false
	for _, a := range e.PotentialActions(0) {
		if a.Kind == "cast" && a.Obj == blaze {
			found = true
		}
	}
	if !found {
		t.Fatal("Blaze must be a potential cast (X priced at 0, CR 107.3b)")
	}
}

// TestPotentialActionsRishadanPortRespondable pins the respondable half: an
// instant-speed activation needing floated mana (Rishadan Port's "11, T:
// tap target land", cost {1} + T) is a potential ABILITY with no floating
// mana — the fact an opponent-spell window reads to decide whether the seat
// could respond if it stopped. The old client ignored mana-only activations
// entirely, so the window passed.
func TestPotentialActionsRishadanPortRespondable(t *testing.T) {
	e := layerEngine(t)
	e.G.Step = state.StepMain1
	e.G.SetZone(state.ZHand, 0, nil)
	e.G.Players[0].LandsPlayed = 1
	port := onBoard(t, e, 0, "Name:Rishadan Port\nManaCost:3\nTypes:Land\n"+
		"A:AB$ Mana | Cost$ T | Produced$ 1 | SpellDescription$ Add {1}.\n"+
		"A:AB$ Tap | Cost$ 1 T | ValidTgts$ Land | TgtPrompt$ Select target land | SpellDescription$ Tap target land.\n"+
		"Oracle:{T}: Add {1}.\n{1}, {T}: Tap target land.\n")
	found := false
	for _, a := range e.PotentialActions(0) {
		if a.Kind == "ability" && a.Obj == port {
			found = true
		}
	}
	if !found {
		t.Fatal("Rishadan Port's tap-a-land activation must be a potential ability")
	}
	// The same shape with the Port TAPPED is not: the tap gate is real.
	e.G.Obj(port).Tapped = true
	for _, a := range e.PotentialActions(0) {
		if a.Kind == "ability" && a.Obj == port {
			t.Fatal("a tapped Port must not be a potential ability")
		}
	}
}

// TestPotentialActionsKarakasNoTargetDoesNotStop pins the Karakas half: an
// activation whose cost is a bare {T} and whose target does not exist is NOT
// a potential action (the engine's own target gate withholds the offer), so
// it must not make a window stop-worthy. The old client read every bare-T
// cost as payable and stopped every main phase.
func TestPotentialActionsKarakasNoTargetDoesNotStop(t *testing.T) {
	e := layerEngine(t)
	e.G.Step = state.StepMain1
	e.G.SetZone(state.ZHand, 0, nil)
	e.G.Players[0].LandsPlayed = 1
	onBoard(t, e, 0, "Name:Karakas\nManaCost:no cost\nTypes:Legendary Land\n"+
		"A:AB$ MoveZone | Cost$ T | ValidTgts$ Creature.Legendary | Origin$ Battlefield | DestinationZone$ Hand | SpellDescription$ Return target legendary creature to its owner's hand.\n"+
		"Oracle:{T}: Return target legendary creature to its owner's hand.\n")
	// No legendary creature anywhere (the decks are Mountains): nothing to
	// bounce. The battlefield walk must not have handed the seat a reason to
	// stop every main phase.
	if acts := e.PotentialActions(0); len(acts) != 0 {
		t.Fatalf("Karakas with no legal target must leave no potential actions, got %v", acts)
	}
	// With a legendary creature on the board the activation IS potential —
	// the gate is the target, not the bare-T cost.
	leg := onBoard(t, e, 0, "Name:Saffi Eriksdotter\nManaCost:G W\nTypes:Legendary Creature Human\nPT:2/2\nOracle:x\n")
	e.G.Obj(leg).SummonSick = false
	found := false
	for _, a := range e.PotentialActions(0) {
		if a.Kind == "ability" && a.Obj == e.G.Zone(state.ZBattlefield, 0)[0] {
			found = true
		}
	}
	if !found {
		t.Fatal("Karakas with a legendary creature on the battlefield must be a potential ability")
	}
}

// TestPotentialManaOverbound pins PotentialMana's folding rules directly:
// a fixed single-face source, a fixed multi-face source, an open (Any)
// production and an indeterminate amount.
func TestPotentialManaOverbound(t *testing.T) {
	e := layerEngine(t)
	e.G.SetZone(state.ZHand, 0, nil)
	onBoard(t, e, 0, jitteSnapshotPlains) // W:1
	if m := e.PotentialMana(0); m[state.MW] != 1 || m.Total() != 1 {
		t.Fatalf("one Plains potential = %v, want exactly {W:1}", m)
	}

	e2 := layerEngine(t)
	e2.G.SetZone(state.ZHand, 0, nil)
	onBoard(t, e2, 0, "Name:Volcanic Island\nTypes:Land Island Mountain\nOracle:x\n") // U or R: both folded
	onBoard(t, e2, 0, "Name:Falling Timbaland\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ Any | SpellDescription$ Add one mana of any colour.\nOracle:x\n")
	onBoard(t, e2, 0, "Name:Cradle Stand-in\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ G | Amount$ Count$Valid Creature.YouCtrl | SpellDescription$ Add G per creature.\nOracle:x\n")
	m := e2.PotentialMana(0)
	for _, i := range []int{state.MW, state.MU, state.MB, state.MR, state.MG, state.MC} {
		if m[i] < potentialUnbounded {
			t.Errorf("potential slot %d = %d, want >= %d (an Any source and an indeterminate amount price unbounded)", i, m[i], potentialUnbounded)
		}
	}
}

// TestPotentialActionsIsAPureRead pins the projection's side-effect contract:
// it emits no event, mutates no pool, and leaves the real offer walk (and
// therefore the engine's pending decision surface) exactly as it found it.
func TestPotentialActionsIsAPureRead(t *testing.T) {
	e := layerEngine(t)
	e.G.Step = state.StepMain1
	e.G.SetZone(state.ZHand, 0, nil)
	e.G.Players[0].LandsPlayed = 1
	before := len(e.L.Events)
	poolBefore := e.G.Players[0].Pool
	_ = e.PotentialActions(0)
	if len(e.L.Events) != before {
		t.Fatalf("PotentialActions emitted %d events", len(e.L.Events)-before)
	}
	if e.G.Players[0].Pool != poolBefore {
		t.Fatalf("PotentialActions mutated the pool: %v -> %v", poolBefore, e.G.Players[0].Pool)
	}
	// The real (floating-pool) walk is unchanged by the refactor: with the
	// hand cleared the only options are the mana taps and pass/concede.
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" || o.Kind == "ability" || o.Kind == "play_land" {
			t.Fatalf("an empty hand must not offer %q directly", o.Kind)
		}
	}
}
