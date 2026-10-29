package neutral

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func s45Ref(name string) protocol.ObjectRef {
	n := name
	return protocol.ObjectRef{ObjectID: "o-" + name, CardName: &n, OwnerSeat: "p0", ControllerSeat: "p0", Zone: "hand"}
}

// TestAltCostIgnoresAbilityScopedStatic: Heart of Kiran's AlternativeCost is
// scoped to ValidSA$ Activated.Crew+Vehicle, the crew cost, so it is not a
// cast alternative. Land Grant's bare Spell scope is a real one and must
// survive the gate.
func TestAltCostIgnoresAbilityScopedStatic(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	s := NewSession(reg)
	sd := &protocol.SeatDecision{ActingSeat: "p0", Observation: protocol.Observation{
		Players: [2]protocol.PlayerObs{{Seat: "p0"}, {Seat: "p1"}},
	}}
	heart, grant := s45Ref("Heart of Kiran"), s45Ref("Land Grant")
	for _, tc := range []struct {
		name string
		src  protocol.ObjectRef
		vs   string
	}{{"Heart of Kiran", heart, "Activated."}, {"Land Grant", grant, "Spell"}} {
		if f := s.B.face(tc.src.CardName, false, nil); f == nil {
			t.Fatalf("precondition: %s is not in the corpus", tc.name)
		}
		var found bool
		for _, st := range s.B.face(tc.src.CardName, false, nil).Statics {
			if st.Mode == "AlternativeCost" && st.Params["ValidCard"] == "Card.Self" {
				found = true
			}
		}
		if !found {
			t.Fatalf("precondition: %s has no self AlternativeCost static", tc.name)
		}
	}
	if s.altCost(sd, heart) {
		t.Fatal("the crew-scoped AlternativeCost was offered as a cast alternative")
	}
	if !s.altCost(sd, grant) {
		t.Fatal("Land Grant's Spell-scoped alternative cost with an empty hand was not offered")
	}
}

// TestAttackRestrictedBlanketAura: Cessation's CantAttack has no Target$ and
// restricts every defender; a Vow's Target$ You restricts only its
// controller.
func TestAttackRestrictedBlanketAura(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	s := NewSession(reg)
	att := obsRef("o-creature", "Grizzly Bears", "p0")
	p0, p1 := protocol.PlayerTarget("p0"), protocol.PlayerTarget("p1")
	obsWith := func(aura string) protocol.Observation {
		link := protocol.ObjectTarget(att)
		rec := obsPerm("o-aura", aura, "p0", &protocol.Permanent{AttachedTo: &link}, "enchantment")
		cs := obsPerm("o-creature", "Grizzly Bears", "p0", untapped(), "creature")
		return testObservation("p0", "declare_attackers", []protocol.ObjectRecord{cs, rec}, nil)
	}
	cessation := obsWith("Cessation")
	cf := s.B.face(obsRef("o-aura", "Cessation", "p0").CardName, false, nil)
	if cf == nil {
		t.Fatal("precondition: Cessation is not in the corpus")
	}
	found := false
	for _, st := range cf.Statics {
		if st.Mode == "CantAttack" && st.Params["Target"] == "" {
			found = true
		}
	}
	if !found {
		t.Fatal("precondition: Cessation has no blanket CantAttack static")
	}
	if !s.attackRestricted(&cessation, att, p0) || !s.attackRestricted(&cessation, att, p1) {
		t.Fatal("a blanket enchanted-creature CantAttack must forbid every defender")
	}
	vow := obsWith("Vow of Duty")
	if !s.attackRestricted(&vow, att, p0) {
		t.Fatal("the Vow must forbid attacking its controller")
	}
	if s.attackRestricted(&vow, att, p1) {
		t.Fatal("the Vow must not forbid attacking the other seat")
	}
}

// TestAttackersMarkStaticGoadRequired: a continuous Goad$ True static on an
// attached Coercive Impetus never becomes a "goaded" status, so the required
// attacker must be rebuilt from the static.
func TestAttackersMarkStaticGoadRequired(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	s := NewSession(reg)
	att := obsRef("o-creature", "Drowner Initiate", "p0")
	link := protocol.ObjectTarget(att)
	creature := obsPerm("o-creature", "Drowner Initiate", "p0", untapped(), "creature")
	impetus := obsPerm("o-impetus", "Coercive Impetus", "p0", &protocol.Permanent{AttachedTo: &link}, "enchantment")
	if s.B.face(impetus.CardName, false, nil) == nil {
		t.Fatal("precondition: Coercive Impetus is not in the corpus")
	}
	obs := testObservation("p0", "declare_attackers", []protocol.ObjectRecord{creature, impetus}, nil)
	def := protocol.PlayerTarget("p1")
	sd := protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 1},
		Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{{Semantic: protocol.DeclareAttack(att, &def)}}}
	p := translated(t, s, sd)
	if p.Decision.Kind != decision.KAttackers {
		t.Fatalf("decision kind %q, want attackers", p.Decision.Kind)
	}
	want := s.B.IDs.Of(att.ObjectID)
	for _, o := range p.Decision.Options {
		if o.Kind == "attacker" && o.Obj == want {
			if !o.Required {
				t.Fatalf("the statically goaded attacker is not Required: %+v", o)
			}
			return
		}
	}
	t.Fatalf("the statically goaded creature has no attacker option: %+v", p.Decision.Options)
}

// TestTargetEffectReadsEffectTriggerAmount: The Sea Devils' third chapter
// creates a delayed trigger held in an SVar, and its DealDamage$ X is the
// triggering combat damage. The Execute$ root must be found and the entry's
// XValue must become the amount.
func TestTargetEffectReadsEffectTriggerAmount(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	s := NewSession(reg)
	src := s45Ref("The Sea Devils")
	src.Zone = "stack"
	f := s.B.face(src.CardName, false, nil)
	if f == nil {
		t.Fatal("precondition: The Sea Devils is not in the corpus")
	}
	if _, ok := f.SVars["TrigDamage"]; !ok {
		t.Fatal("precondition: the Effect-created trigger SVar is missing")
	}
	obs := func(x *uint32) *protocol.Observation {
		return &protocol.Observation{Stack: []protocol.StackEntry{{
			ObjectRef: src, StackKind: "triggered_ability", XValue: x,
		}}}
	}
	x := uint32(2)
	effect := s.targetEffect(obs(&x), src, 0)
	if effect == nil || effect.API != "DealDamage" || effect.Damage == nil || effect.Damage.Amount == nil || *effect.Damage.Amount != 2 {
		t.Fatalf("the trigger's XValue did not become the damage amount: %+v", effect)
	}
	effect = s.targetEffect(obs(nil), src, 0)
	if effect == nil || effect.API != "DealDamage" {
		t.Fatalf("the Effect-created trigger root was not found without an amount: %+v", effect)
	}
	if effect.Damage == nil || effect.Damage.Amount != nil {
		t.Fatalf("an unknown TriggerCount$ amount must stay nil: %+v", effect.Damage)
	}
}
