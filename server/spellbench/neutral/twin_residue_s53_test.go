package neutral

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// withSubtypes sets the observation's subtype list (the protocol splits
// subtypes out of Characteristics.Types).
func withSubtypes(r protocol.ObjectRecord, subs ...string) protocol.ObjectRecord {
	if r.Characteristics != nil {
		r.Characteristics.Subtypes = subs
	}
	return r
}

// TestConditionalCantAttackReadsObservableGates pins the seed-59 attack
// residue class: a conditional cant-attack static the observation can answer
// (IsPresent$ over the board) must be evaluated, while one it cannot
// (CheckSVar$ over this turn's cast history) must be treated as not proven
// absent and kept out of the derived declaration. Goblin Cohort and War
// Falcon are the measured shapes.
func TestConditionalCantAttackReadsObservableGates(t *testing.T) {
	s := fixtureSession(t,
		"Name:Fixture Bears\nTypes:Creature Bear\nPT:2/2\n",
		"Name:Fixture Knight\nTypes:Creature Human Knight\nPT:2/2\n",
		"Name:Fixture Falcon\nTypes:Creature Bird\nPT:2/1\n"+
			"S:Mode$ CantAttack | ValidCard$ Card.Self | IsPresent$ Knight.YouCtrl | PresentCompare$ EQ0 | Description$ CARDNAME can't attack unless you control a Knight.\n",
		"Name:Fixture Cohort\nTypes:Creature Goblin Warrior\nPT:2/2\n"+
			"S:Mode$ CantAttack | ValidCard$ Card.Self | CheckSVar$ X | SVarCompare$ EQ0 | Description$ CARDNAME can't attack unless you've cast a creature spell this turn.\n"+
			"SVar:X:Count$ThisTurnCast_Creature.YouCtrl\n")
	cur := obsRef("o-bears", "Fixture Bears", "p0")
	player := protocol.PlayerTarget("p1")
	knight := obsPerm("o-knight", "Fixture Knight", "p0", untapped(), "creature")
	knight.Characteristics.Subtypes = []string{"Human", "Knight"}
	obs := testObservation("p0", "declare_attackers",
		[]protocol.ObjectRecord{
			obsPerm("o-bears", "Fixture Bears", "p0", untapped(), "creature"),
			knight,
			obsPerm("o-falcon", "Fixture Falcon", "p0", untapped(), "creature"),
			obsPerm("o-cohort", "Fixture Cohort", "p0", untapped(), "creature"),
		}, nil)
	sd := protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 4},
		Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{{Semantic: protocol.DeclareAttack(cur, &player)}}}
	p := translated(t, s, sd)
	ids := map[string]bool{}
	for _, o := range p.Decision.Options {
		if o.Kind == "attacker" {
			ids[s.B.IDs.V2(o.Obj)] = true
		}
	}
	if !ids["o-bears"] {
		t.Fatal("precondition: the current substep's attacker option is missing")
	}
	if !ids["o-falcon"] {
		t.Error("the observable conditional CantAttack (Knight present, PresentCompare EQ0) dropped a legal attacker")
	}
	if ids["o-cohort"] {
		t.Error("a CheckSVar$ CantAttack the observation cannot evaluate was read as absent and offered as a derived attacker")
	}
}

// TestUnlessDefenderUsesTheDefendingBoard pins the Deep-Sea Serpent and
// Godhunter Octopus shapes: UnlessDefender$ is a board fact the observation
// carries, not an unknowable gate.
func TestUnlessDefenderUsesTheDefendingBoard(t *testing.T) {
	s := fixtureSession(t,
		"Name:Fixture Bears\nTypes:Creature Bear\nPT:2/2\n",
		"Name:Fixture Serpent\nTypes:Creature Serpent\nPT:5/5\n"+
			"S:Mode$ CantAttack | ValidCard$ Card.Self | UnlessDefender$ controlsIsland | Description$ CARDNAME can't attack unless defending player controls an Island.\n",
		"Name:Fixture Forest\nTypes:Basic Land Forest\n",
		"Name:Fixture Island\nTypes:Basic Land Island\n")
	cur := obsRef("o-bears", "Fixture Bears", "p0")
	player := protocol.PlayerTarget("p1")
	for _, tc := range []struct {
		name    string
		defLand string
		want    bool
	}{
		{"no island", "Fixture Forest", false},
		{"island", "Fixture Island", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obs := testObservation("p0", "declare_attackers",
				[]protocol.ObjectRecord{
					obsPerm("o-bears", "Fixture Bears", "p0", untapped(), "creature"),
					obsPerm("o-serpent", "Fixture Serpent", "p0", untapped(), "creature"),
				},
				[]protocol.ObjectRecord{withSubtypes(obsPerm("o-land", tc.defLand, "p1", untapped(), "land"), tc.defLand[len("Fixture "):])})
			sd := protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 2},
				Context: protocol.Context{Kind: "choice"}, Observation: obs,
				Candidates: []protocol.Candidate{{Semantic: protocol.DeclareAttack(cur, &player)}}}
			p := translated(t, s, sd)
			found := false
			for _, o := range p.Decision.Options {
				if o.Kind == "attacker" && o.Obj == s.B.IDs.Of("o-serpent") {
					found = true
				}
			}
			if found != tc.want {
				t.Fatalf("serpent option present = %v, want %v (UnlessDefender$ %s)", found, tc.want, tc.defLand)
			}
		})
	}
}

// TestWalkingWallPumpGrantAttacks pins the seed-59 Walking Wall shape: a
// defender whose activated Pump leads to a DB$ Effect with a self-scoped
// CanAttackDefender static is a legal derived attacker for the whole
// declaration, not only in its own substep's candidate list.
func TestWalkingWallPumpGrantAttacks(t *testing.T) {
	s := fixtureSession(t,
		"Name:Fixture Wall\nTypes:Artifact Creature Wall\nPT:0/6\nK:Defender\n"+
			"A:AB$ Pump | Cost$ 3 | NumAtt$ +3 | NumDef$ -1 | SubAbility$ DBEffect | SpellDescription$ CARDNAME can attack this turn as though it didn't have defender.\n"+
			"SVar:DBEffect:DB$ Effect | StaticAbilities$ CanAttack | Duration$ UntilHostLeavesPlayOrEOT\n"+
			"SVar:CanAttack:Mode$ CanAttackDefender | ValidCard$ Card.EffectSource | Description$ EFFECTSOURCE can attack this turn as though it didn't have defender.\n",
		"Name:Fixture Bears\nTypes:Creature Bear\nPT:2/2\n")
	cur := obsRef("o-bears", "Fixture Bears", "p0")
	player := protocol.PlayerTarget("p1")
	obs := testObservation("p0", "declare_attackers",
		[]protocol.ObjectRecord{
			obsPerm("o-bears", "Fixture Bears", "p0", untapped(), "creature"),
			obsPerm("o-wall", "Fixture Wall", "p0", untapped(), "artifact", "creature"),
		}, nil)
	// The census facts the session would have recorded from the activation:
	// one activation this turn, and the activated ability is the Pump.
	s.usesTurn = obs.Turn
	s.uses = map[string]int32{"o-wall": 1}
	s.activated = map[string]int{"o-wall": 0}
	sd := protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 2},
		Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{{Semantic: protocol.DeclareAttack(cur, &player)}}}
	p := translated(t, s, sd)
	wall := s.B.IDs.Of("o-wall")
	found := false
	for _, o := range p.Decision.Options {
		if o.Kind == "attacker" && o.Obj == wall {
			found = true
		}
	}
	if !found {
		t.Fatalf("the activated CanAttackDefender grant was not read: %+v", p.Decision.Options)
	}
}

// TestMustAttackStaticMarksRequired pins the CR 508.1d requirement read: a
// self-scoped printed MustAttack marks the declaration option Required.
func TestMustAttackStaticMarksRequired(t *testing.T) {
	s := fixtureSession(t,
		"Name:Fixture Rig\nTypes:Artifact Creature Construct\nPT:4/4\n"+
			"S:Mode$ MustAttack | ValidCard$ Card.Self | Description$ CARDNAME attacks each combat if able.\n",
		"Name:Fixture Bears\nTypes:Creature Bear\nPT:2/2\n")
	cur := obsRef("o-bears", "Fixture Bears", "p0")
	player := protocol.PlayerTarget("p1")
	obs := testObservation("p0", "declare_attackers",
		[]protocol.ObjectRecord{
			obsPerm("o-bears", "Fixture Bears", "p0", untapped(), "creature"),
			obsPerm("o-rig", "Fixture Rig", "p0", untapped(), "artifact", "creature"),
		}, nil)
	sd := protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 2},
		Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{{Semantic: protocol.DeclareAttack(cur, &player)}}}
	p := translated(t, s, sd)
	rig := s.B.IDs.Of("o-rig")
	for _, o := range p.Decision.Options {
		if o.Kind == "attacker" && o.Obj == rig {
			if !o.Required {
				t.Fatal("a self-scoped printed MustAttack did not mark the option Required")
			}
			return
		}
	}
	t.Fatalf("the must-attack creature got no derived attacker option: %+v", p.Decision.Options)
}

// TestViewTypeLineMirrorsNativeLayer4Gate pins the seed-59 Reconfigure view
// gap: native's ViewTypes table is empty until some LType effect is live, so
// an attached Reconfigure Equipment shows its printed type line (creature),
// not the observation's derived artifact line. Once another permanent proves
// an LType effect exists, the derived line wins.
func TestViewTypeLineMirrorsNativeLayer4Gate(t *testing.T) {
	s := fixtureSession(t,
		"Name:Fixture Ripper\nTypes:Artifact Creature Equipment Rhino\nPT:3/3\nK:Reconfigure:2\n",
		"Name:Fixture Host\nTypes:Artifact Creature Leviathan\nPT:9/8\n",
		"Name:Fixture Wilds\nTypes:Land\n")
	host := obsRef("o-host", "Fixture Host", "p0")
	ripperP := untapped()
	ripperP.AttachedTo = &protocol.TargetRef{Object: &host}
	ripper := withSubtypes(obsPerm("o-ripper", "Fixture Ripper", "p0", ripperP, "artifact"), "Equipment", "Rhino")
	hostRec := withSubtypes(obsPerm("o-host", "Fixture Host", "p0", untapped(), "artifact", "creature"), "Leviathan")
	wilds := obsPerm("o-wilds", "Fixture Wilds", "p0", untapped(), "creature") // observed animated line
	mk := func(records ...protocol.ObjectRecord) protocol.Observation {
		return protocol.Observation{Viewer: "p0", Turn: 3, PhaseStep: "precombat_main",
			Players: [2]protocol.PlayerObs{{Seat: "p0", Battlefield: records}, {Seat: "p1"}}}
	}
	viewOf := func(obs protocol.Observation) (string, int32, int32, bool) {
		v, err := s.B.View(&obs)
		if err != nil {
			t.Fatal(err)
		}
		for _, cv := range v.Players[0].Battlefield {
			if cv.Name == "Fixture Ripper" {
				return cv.Types, cv.Power, cv.Toughness, true
			}
		}
		return "", 0, 0, false
	}
	got, gotPow, gotTough, ok := viewOf(mk(ripper, hostRec))
	if !ok {
		t.Fatal("precondition: the attached Ripper is not in the projected view")
	}
	if got != "Artifact Creature Equipment Rhino" {
		t.Fatalf("attached Reconfigure type line = %q, want native's printed line (no LType table)", got)
	}
	if gotPow != 3 || gotTough != 3 {
		t.Fatalf("attached Reconfigure P/T = %d/%d, want the printed 3/3 native's view carries", gotPow, gotTough)
	}
	// A second, animated land proves a live LType effect: native's table now
	// exists for every candidate, so the Ripper shows its derived line.
	got, _, _, ok = viewOf(mk(ripper, hostRec, wilds))
	if !ok {
		t.Fatal("precondition: the Ripper is missing from the animated board")
	}
	if got != "Artifact Equipment Rhino" {
		t.Fatalf("attached Reconfigure type line with an LType effect = %q, want the derived line", got)
	}
}

// TestPreferTokensSelectsTheDeckScript pins the token-script preference: the
// observation names a token only by face name, and the deck's TokenScript$
// stems select which same-named script's statics the neutral reads.
func TestPreferTokensSelectsTheDeckScript(t *testing.T) {
	reg := cards.NewRegistry()
	addToken := func(stem, body string) {
		c, diags := cards.ParseBytes(stem+".txt", []byte(body))
		if len(diags) > 0 {
			t.Fatalf("parse token %s: %v", stem, diags)
		}
		c.Link()
		reg.Tokens[stem] = c
	}
	addToken("a_rat_block", "Name:Rat\nTypes:Creature Rat\nColors:B\nPT:1/1\nOracle:x\n")
	addToken("z_rat_noblock", "Name:Rat\nTypes:Creature Rat\nColors:B\nPT:1/1\n"+
		"S:Mode$ CantBlock | ValidCard$ Card.Self | Description$ CARDNAME can't block.\nOracle:x\n")
	s := NewSession(reg)
	name := "Rat"
	obs := protocol.Observation{Viewer: "p0", Turn: 3, PhaseStep: "declare_blockers",
		Players: [2]protocol.PlayerObs{{Seat: "p0", Battlefield: []protocol.ObjectRecord{{
			ObjectRef:       protocol.ObjectRef{ObjectID: "o-rat", CardName: &name, ControllerSeat: "p0", OwnerSeat: "p0", Zone: "battlefield"},
			Token:           true,
			Characteristics: &protocol.Characteristics{Types: []string{"creature"}, Colors: []string{"B"}},
			Permanent:       &protocol.Permanent{},
		}}}, {Seat: "p1"}}}
	blockable := func() bool {
		v, err := s.B.View(&obs)
		if err != nil {
			t.Fatal(err)
		}
		for _, cv := range v.Players[0].Battlefield {
			if cv.Name == "Rat" {
				return !s.cantBlock(cv, nil)
			}
		}
		t.Fatal("precondition: the Rat token is not in the projected view")
		return false
	}
	if !blockable() {
		t.Fatal("precondition: the alphabetically first same-named script is expected to be blockable")
	}
	s.B.PreferTokens([]string{"z_rat_noblock"})
	if blockable() {
		t.Fatal("TokenScript$ preference did not select the no-block Rat script")
	}
}

// TestFaceDownKeywordOpensNativeTypeTable pins the other face-down signal:
// an Earthbend-hasted face-down land proves a live LType effect, so native's
// table shows the masked line for every face-down object.
func TestFaceDownKeywordOpensNativeTypeTable(t *testing.T) {
	s := fixtureSession(t,
		"Name:Fixture Forest\nTypes:Basic Land Forest\n",
		"Name:Fixture Beast\nTypes:Creature Elemental\nPT:4/4\n")
	forest := obsPerm("o-forest", "Fixture Forest", "p0", untapped(), "creature")
	forest.FaceDown = true
	forest.Characteristics.Keywords = []string{"haste"}
	beast := obsPerm("o-beast", "Fixture Beast", "p0", untapped(), "creature")
	beast.FaceDown = true
	obs := protocol.Observation{Viewer: "p0", Turn: 3, PhaseStep: "precombat_main",
		Players: [2]protocol.PlayerObs{{Seat: "p0", Battlefield: []protocol.ObjectRecord{forest, beast}}, {Seat: "p1"}}}
	v, err := s.B.View(&obs)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, cv := range v.Players[0].Battlefield {
		got[cv.Name] = cv.Types
	}
	if got["Fixture Forest"] != "Creature" {
		t.Fatalf("hasted face-down land type line = %q, want native's masked Creature", got["Fixture Forest"])
	}
	if got["Fixture Beast"] != "Creature" {
		t.Fatalf("face-down Elemental type line = %q, want native's masked Creature", got["Fixture Beast"])
	}
}
