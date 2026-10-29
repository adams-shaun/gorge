package mapping_test

// TestGrantedAbilityIndexStableAcrossDecisions pins the v2 ability_index of a
// granted activated ability to the object, not to one decision's option list.
// A creature wearing Viridian Longbow gains its "{T}: This creature deals 1
// damage to any target" body. It also carries an earlier, sorcery-speed grant
// (the Kargan Dragonlord shape) that a later instant-speed priority window
// withholds. The Longbow body keeps the same index across both decisions.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
)

func TestGrantedAbilityIndexStableAcrossDecisions(t *testing.T) {
	g := untilPending(t, "Wildfire", 1, func(d *decision.Decision, e *rules.Engine) bool {
		if d.Kind != decision.KPriority {
			return false
		}
		for _, id := range e.G.Zone(state.ZBattlefield, d.Player) {
			if e.IsCreature(id) {
				return true
			}
		}
		return false
	})
	d0 := g.E.Pending()
	seat := d0.Player
	var recipient state.ObjID
	for _, id := range g.E.G.Zone(state.ZBattlefield, seat) {
		if g.E.IsCreature(id) {
			recipient = id
			break
		}
	}
	if recipient == 0 {
		t.Fatal("precondition: the deciding seat controls no creature to equip")
	}
	if o := g.E.G.Obj(recipient); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: recipient %d is not a battlefield object", recipient)
	}

	reg := testcorpus.Registry(t)
	longbow, ok := reg.Lookup("Viridian Longbow")
	if !ok {
		t.Fatal(`precondition: the corpus has no "Viridian Longbow"`)
	}
	lbFace := longbow.Faces[0]
	if cards.ResolveSVar(lbFace.SVars, "Damage") == nil {
		t.Fatal(`precondition: Viridian Longbow has no "Damage" SVar body`)
	}

	// Two persistent grants on the same object. The Dragonlord-shaped grant
	// is older, so the Longbow grant must follow it in the object's index
	// order even when a decision withholds the sorcery-speed body.
	const pumpBody = "AB$ Pump | Cost$ R | SorcerySpeed$ True | NumAtt$ +1 | SpellDescription$ gets +1/+0."
	g.E.AddContinuous(rules.ContinuousEffect{Source: recipient, Controller: seat, Timestamp: 1,
		Affects: "Creature.YouCtrl", AddAbilities: []string{"Pump"},
		SVars: map[string]string{"Pump": pumpBody}})
	g.E.AddContinuous(rules.ContinuousEffect{Source: recipient, Controller: seat, Timestamp: 2,
		Affects: "Creature.YouCtrl", AddAbilities: []string{"Damage"}, SVars: lbFace.SVars})

	pump := decision.Option{Kind: "ability", Obj: recipient, Label: "Pump",
		SVar: "Pump", GrantSource: recipient}
	damage := decision.Option{Kind: "ability", Obj: recipient, Label: "Damage",
		SVar: "Damage", GrantSource: recipient}

	// Decision 1 is the sorcery-speed window: both grants are offered.
	d1 := synthetic(d0, pump, damage)
	// Decision 2 is the next priority window: the sorcery-speed grant is
	// withheld, the Longbow grant persists. The option list is the only thing
	// that changed.
	d2 := synthetic(d0, damage)
	d2.Seq = d1.Seq + 1

	env := envFor(t, g)
	indexAt := func(d *decision.Decision, native int) uint32 {
		t.Helper()
		tx, err := mapping.Begin(env, d)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		p, err := tx.Pose()
		if err != nil {
			t.Fatalf("pose: %v", err)
		}
		for _, c := range p.Candidates {
			if c.Sem.Kind != "activate_ability" || c.Op.Op != "choose" || c.Op.Option != native {
				continue
			}
			idx, ok := c.Sem.Fields["ability_index"].(uint32)
			if !ok {
				t.Fatalf("ability_index %v is not a u32", c.Sem.Fields["ability_index"])
			}
			if err := c.Sem.Check(); err != nil {
				t.Fatalf("activate_ability candidate fails the protocol shape: %v", err)
			}
			return idx
		}
		t.Fatalf("no activate_ability candidate commits native option %d: %s", native, semJSON(p.Candidates))
		return 0
	}

	pump1 := indexAt(d1, 0)
	damage1 := indexAt(d1, 1)
	if damage1 != pump1+1 {
		t.Fatalf("Longbow grant index = %d, want one past the older grant %d", damage1, pump1)
	}
	// Precondition: the second decision really withholds the older grant, so
	// the old decision-local ordinal would move the Longbow body from
	// damage1 to damage1-1.
	for _, o := range d2.Options {
		if o.SVar == "Pump" {
			t.Fatalf("precondition: decision 2 still offers the withheld grant: %+v", o)
		}
	}
	damage2 := indexAt(d2, 0)
	if damage2 != damage1 {
		t.Fatalf("Longbow grant index moved across decisions: %d then %d; want %d", damage1, damage2, damage1)
	}
}

// TestGrantedAbilityIndexDistinctForIdenticalGrants covers the sibling case
// an anchor alone cannot separate: two live grants of the SAME body (same
// granting object, same SVar) are two inventory entries and two offered
// options. Each option must take its own position; resolving both to the
// first matching anchor would pose two identical activate_ability candidates
// and Finalize would fail the whole decision with ErrDuplicate.
func TestGrantedAbilityIndexDistinctForIdenticalGrants(t *testing.T) {
	g := untilPending(t, "Wildfire", 1, func(d *decision.Decision, e *rules.Engine) bool {
		if d.Kind != decision.KPriority {
			return false
		}
		for _, id := range e.G.Zone(state.ZBattlefield, d.Player) {
			if e.IsCreature(id) {
				return true
			}
		}
		return false
	})
	d0 := g.E.Pending()
	seat := d0.Player
	var recipient state.ObjID
	for _, id := range g.E.G.Zone(state.ZBattlefield, seat) {
		if g.E.IsCreature(id) {
			recipient = id
			break
		}
	}
	if recipient == 0 {
		t.Fatal("precondition: the deciding seat controls no creature to grant an ability")
	}

	reg := testcorpus.Registry(t)
	longbow, ok := reg.Lookup("Viridian Longbow")
	if !ok {
		t.Fatal(`precondition: the corpus has no "Viridian Longbow"`)
	}
	lbFace := longbow.Faces[0]
	if cards.ResolveSVar(lbFace.SVars, "Damage") == nil {
		t.Fatal(`precondition: Viridian Longbow has no "Damage" SVar body`)
	}

	// Two continuous effects grant the identical body from the identical
	// source, at distinct timestamps: two entries in the object's inventory
	// and two options in the offer walk, both with the same anchor.
	for _, stamp := range []uint32{1, 2} {
		g.E.AddContinuous(rules.ContinuousEffect{Source: recipient, Controller: seat, Timestamp: stamp,
			Affects: "Creature.YouCtrl", AddAbilities: []string{"Damage"}, SVars: lbFace.SVars})
	}

	observed := rules.ObservedAbilities(g.E, seat, recipient)
	matching := 0
	for _, a := range observed {
		if a.SVar == "Damage" && a.GrantSource == recipient {
			matching++
		}
	}
	if matching != 2 {
		t.Fatalf("precondition: observed %d identical Damage grants, want 2: %+v", matching, observed)
	}

	grant := decision.Option{Kind: "ability", Obj: recipient, Label: "Damage",
		SVar: "Damage", GrantSource: recipient}
	d := synthetic(d0, grant, grant)

	env := envFor(t, g)
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("two identical grants must pose with distinct indices, got %v", err)
	}
	var indices []uint32
	for _, c := range p.Candidates {
		if c.Sem.Kind != "activate_ability" {
			continue
		}
		if err := c.Sem.Check(); err != nil {
			t.Fatalf("activate_ability candidate %s fails the protocol shape: %v", semJSON(c.Sem), err)
		}
		idx, ok := c.Sem.Fields["ability_index"].(uint32)
		if !ok {
			t.Fatalf("ability_index %v is not a u32", c.Sem.Fields["ability_index"])
		}
		indices = append(indices, idx)
	}
	if len(indices) != 2 {
		t.Fatalf("posed %d activate_ability candidates, want 2: %s", len(indices), semJSON(p.Candidates))
	}
	if indices[0] == indices[1] {
		t.Fatalf("identical grants share ability_index %d: %s", indices[0], semJSON(p.Candidates))
	}
}
