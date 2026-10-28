package neutral

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
)

// These cases pin the printed-restriction reading against shapes the v2
// observation makes look unconditional: an unblockable creature's own
// evasion static, and a "can't attack unless" gate.

func fixtureSession(t *testing.T, scripts ...string) *Session {
	t.Helper()
	reg := cards.NewRegistry()
	for i, script := range scripts {
		c, diags := cards.ParseBytes(fmt.Sprintf("fixture%d.txt", i), []byte(script))
		if len(diags) > 0 {
			t.Fatalf("parse fixture %d: %v", i, diags)
		}
		reg.Add(c)
	}
	return NewSession(reg)
}

func obsRef(id, name, seat string) protocol.ObjectRef {
	n := name
	return protocol.ObjectRef{ObjectID: id, CardName: &n, OwnerSeat: seat, ControllerSeat: seat, Zone: "battlefield"}
}

func obsPerm(id, name, seat string, p *protocol.Permanent, types ...string) protocol.ObjectRecord {
	r := protocol.ObjectRecord{ObjectRef: obsRef(id, name, seat), Permanent: p}
	if len(types) > 0 {
		r.Characteristics = &protocol.Characteristics{Types: types}
	}
	return r
}

func untapped() *protocol.Permanent { return &protocol.Permanent{} }

func attackingAt(seat string) *protocol.Permanent {
	target := protocol.PlayerTarget(seat)
	return &protocol.Permanent{Attacking: true, AttackTarget: &target}
}

func testObservation(viewer, step string, p0, p1 []protocol.ObjectRecord) protocol.Observation {
	active, priority := "p0", "p1"
	if viewer == "p1" {
		active, priority = "p0", "p1"
	}
	return protocol.Observation{Viewer: viewer, Turn: 3, PhaseStep: step, ActiveSeat: &active, PrioritySeat: &priority,
		Players: [2]protocol.PlayerObs{
			{Seat: "p0", Battlefield: p0},
			{Seat: "p1", Battlefield: p1},
		}}
}

// translated rawifies the decision the way the wire does, then runs the
// neutral translation over it.
func translated(t *testing.T, s *Session, sd protocol.SeatDecision) xview.Payload {
	t.Helper()
	raw, err := json.Marshal(sd)
	if err != nil {
		t.Fatal(err)
	}
	var wire protocol.SeatDecision
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	p, err := s.Payload(&wire)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestUnblockableBlockerIsNotRestrictedAsABlocker: Phantom Warrior's
// "CARDNAME can't be blocked" is its evasion as an attacker. As a
// non-current blocker it must still get its block pair; treating the static
// as a blocker restriction removes every pair.
func TestUnblockableBlockerIsNotRestrictedAsABlocker(t *testing.T) {
	s := fixtureSession(t,
		"Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\n",
		"Name:Fixture Squire\nTypes:Creature Human Soldier\nPT:1/1\n",
		"Name:Fixture Phantom\nTypes:Creature Illusion Warrior\nPT:2/2\n"+
			"S:Mode$ CantBlockBy | ValidAttacker$ Creature.Self | Description$ CARDNAME can't be blocked.\n")
	att := obsRef("o-att", "Fixture Bear", "p0")
	cur := obsRef("o-cur", "Fixture Squire", "p1")
	obs := testObservation("p1", "declare_blockers",
		[]protocol.ObjectRecord{obsPerm("o-att", "Fixture Bear", "p0", attackingAt("p1"), "creature")},
		[]protocol.ObjectRecord{
			obsPerm("o-cur", "Fixture Squire", "p1", untapped(), "creature"),
			obsPerm("o-phantom", "Fixture Phantom", "p1", untapped(), "creature"),
		})
	sd := protocol.SeatDecision{ActingSeat: "p1", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 1},
		Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{{Semantic: protocol.DeclareBlock(cur, &att)}}}
	p := translated(t, s, sd)
	if p.Decision.Kind != decision.KBlockers {
		t.Fatalf("decision kind %q, want blockers", p.Decision.Kind)
	}
	phantom := s.B.IDs.Of("o-phantom")
	foundPair, foundCurrent := false, false
	for _, o := range p.Decision.Options {
		if o.Kind != "block" {
			continue
		}
		if o.Obj == phantom {
			foundPair = true
		}
		if o.Obj == s.B.IDs.Of("o-cur") {
			foundCurrent = true
		}
	}
	if !foundCurrent {
		t.Fatal("precondition: the current blocker has no block option")
	}
	if !foundPair {
		t.Fatalf("unblockable creature as a non-current blocker got no block pair: %+v", p.Decision.Options)
	}
}

// TestConditionalCantAttackKeepsTheCreatureAttackable: War Falcon can attack
// while its controller has a Knight. Its static's IsPresent$/PresentCompare$
// gate is not observable here, so it must not be read as an unconditional
// ban and dropped from the group declaration.
func TestConditionalCantAttackKeepsTheCreatureAttackable(t *testing.T) {
	s := fixtureSession(t,
		"Name:Fixture Bears\nTypes:Creature Bear\nPT:2/2\n",
		"Name:Fixture Knight\nTypes:Creature Human Knight\nPT:2/2\n",
		"Name:Fixture Falcon\nTypes:Creature Bird\nPT:2/1\n"+
			"S:Mode$ CantAttack | ValidCard$ Card.Self | IsPresent$ Knight.YouCtrl,Soldier.YouCtrl | PresentCompare$ EQ0 | Description$ CARDNAME can't attack unless you control a Knight or Soldier.\n")
	if s.cantAttack(view.CardView{Name: "Fixture Falcon"}) {
		t.Fatal("a conditional CantAttack was read as unconditional")
	}
	player := protocol.PlayerTarget("p1")
	cur := obsRef("o-bears", "Fixture Bears", "p0")
	obs := testObservation("p0", "declare_attackers",
		[]protocol.ObjectRecord{
			obsPerm("o-bears", "Fixture Bears", "p0", untapped(), "creature"),
			obsPerm("o-knight", "Fixture Knight", "p0", untapped(), "creature"),
			obsPerm("o-falcon", "Fixture Falcon", "p0", untapped(), "creature"),
		}, nil)
	sd := protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 3},
		Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{{Semantic: protocol.DeclareAttack(cur, &player)}}}
	p := translated(t, s, sd)
	if p.Decision.Kind != decision.KAttackers {
		t.Fatalf("decision kind %q, want attackers", p.Decision.Kind)
	}
	falcon := s.B.IDs.Of("o-falcon")
	for _, o := range p.Decision.Options {
		if o.Kind == "attacker" && o.Obj == falcon {
			return
		}
	}
	t.Fatalf("conditionally-restricted attacker is missing from the native declaration: %+v", p.Decision.Options)
}

// TestOpponentBattleIsNotAnAttackDefender: CR 310.8. The acting seat's own
// battles are the attackable ones; an opponent's battle is protected by the
// acting seat and cannot be attacked.
func TestOpponentBattleIsNotAnAttackDefender(t *testing.T) {
	s := fixtureSession(t,
		"Name:Fixture Bears\nTypes:Creature Bear\nPT:2/2\n",
		"Name:Fixture Lions\nTypes:Creature Cat\nPT:2/1\n",
		"Name:Fixture Siege\nTypes:Battle Siege\nDefense:4\n",
		"Name:Fixture Walker\nTypes:Planeswalker Ajani\nLoyalty:3\n")
	player := protocol.PlayerTarget("p1")
	cur := obsRef("o-bears", "Fixture Bears", "p0")
	obs := testObservation("p0", "declare_attackers",
		[]protocol.ObjectRecord{
			obsPerm("o-bears", "Fixture Bears", "p0", untapped(), "creature"),
			obsPerm("o-lions", "Fixture Lions", "p0", untapped(), "creature"),
			obsPerm("o-own-battle", "Fixture Siege", "p0", untapped(), "battle"),
		},
		[]protocol.ObjectRecord{
			obsPerm("o-walker", "Fixture Walker", "p1", untapped(), "planeswalker"),
			obsPerm("o-foe-battle", "Fixture Siege", "p1", untapped(), "battle"),
		})
	sd := protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 2},
		Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{{Semantic: protocol.DeclareAttack(cur, &player)}}}
	p := translated(t, s, sd)
	if p.Decision.Kind != decision.KAttackers {
		t.Fatalf("decision kind %q, want attackers", p.Decision.Kind)
	}
	own, foe := s.B.IDs.Of("o-own-battle"), s.B.IDs.Of("o-foe-battle")
	foundOwnBattle, foundFoeBattle := false, false
	for _, o := range p.Decision.Options {
		if o.Kind != "attacker" {
			continue
		}
		if o.Battle == own && o.Player == 1 {
			foundOwnBattle = true
		}
		if o.Battle == foe {
			foundFoeBattle = true
		}
	}
	if !foundOwnBattle {
		t.Fatalf("the acting seat's battle has no attack option at its protector: %+v", p.Decision.Options)
	}
	if foundFoeBattle {
		t.Fatalf("an opponent's battle was offered as an attack defender: %+v", p.Decision.Options)
	}
}

// TestUnlessCostIsScopedToTheStackedAbility: an UnlessCost on a different
// printed ability must not turn this ability's optional cost into the
// unless-pay native shape.
func TestUnlessCostIsScopedToTheStackedAbility(t *testing.T) {
	s := fixtureSession(t, "Name:Fixture Relic\nTypes:Artifact\n"+
		"A:AB$ Draw | Cost$ 2 | SpellDescription$ Draw a card.\n"+
		"A:AB$ Sacrifice | UnlessCost$ 2 | UnlessPayer$ You | SpellDescription$ Sacrifice it unless you pay {2}.\n")
	perm := obsRef("o-relic", "Fixture Relic", "p0")
	perm.Zone = "battlefield"
	src := obsRef("o-ab0", "Fixture Relic", "p0")
	src.Zone = "stack"
	obs := protocol.Observation{Stack: []protocol.StackEntry{{
		ObjectRef: src, StackKind: "activated_ability", Source: &perm,
	}}}
	// The stacked ability is ability 0, whose body carries no UnlessCost.
	// Ability 1's unrelated UnlessCost must not be mistaken for it.
	s.activated["o-relic"] = 0
	if sa := s.unlessCostSA(src, &obs); sa != nil {
		t.Fatalf("resolved an unrelated ability's UnlessCost: %+v", sa.Params)
	}
	// The positive control: when the stacked ability itself carries the
	// unless cost, it is found.
	s.activated["o-relic"] = 1
	if sa := s.unlessCostSA(src, &obs); sa == nil {
		t.Fatal("stacked ability's own UnlessCost was not found")
	}
}
