package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// EachDamage grammar leaves, driven directly (fakeHost + synthetic SAs, the
// effects-package double): the per-damager amount binding, the ToEachOther
// set, the DefinedDamagers$ ParentTarget shape, the fail-closed damager spec
// and the per-damager lifelink rider. The engine-level end-to-end pins on
// real corpus cards live in rules/each_damage_test.go.

// putCreature adds a creature with the given P/T and keywords controlled by
// p, ON the battlefield, and returns its id.
func putCreature(t *testing.T, h *fakeHost, p state.PlayerID, name, pt, keywords string) state.ObjID {
	t.Helper()
	src := "Name:" + name + "\nTypes:Creature\nPT:" + pt + "\n"
	if keywords != "" {
		src += "K:" + keywords + "\n"
	}
	src += "Oracle:x\n"
	o := h.g.AddObject(mkCard(t, src), p)
	h.g.Obj(o.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, p, append(h.g.Zone(state.ZBattlefield, p), o.ID))
	return o.ID
}

// librarySource adds a 9/9 creature object left in the LIBRARY (the
// resolution's source): on the battlefield the ValidCards sweep would pick
// it up as a damager, and a wrong per-resolution count binding would read
// its power 9.
func librarySource(t *testing.T, h *fakeHost) state.ObjID {
	t.Helper()
	return h.g.AddObject(mkCard(t, "Name:SpellSource\nTypes:Creature\nPT:9/9\nOracle:x\n"), 0).ID
}

// damageEvents collects the log's Damage events against objects.
func damageEvents(h *fakeHost) []events.Event {
	var out []events.Event
	for _, ev := range h.log {
		if ev.Kind == events.Damage && ev.Obj != 0 {
			out = append(out, ev)
		}
	}
	return out
}

// TestEachDamagePerDamagerAmountIsBoundToEachDamager is the leaf that makes
// EachDamage different from DamageAll: NumDmg$ Count$CardPower evaluates
// PER DAMAGER. The resolving source is a 9/9 sitting in the library -- an
// engine that resolved the count once against the spell's source would hit
// every creature for 9; the correct reading hits each for its own power.
func TestEachDamagePerDamagerAmountIsBoundToEachDamager(t *testing.T) {
	h := newHost(t, 2)
	src := librarySource(t, h)
	// A 2/2 (dies to its own hit) and a 1/4 (survives with 1 marked).
	bear := putCreature(t, h, 0, "Bear", "2/2", "")
	ox := putCreature(t, h, 0, "Ox", "1/4", "")
	c := &Ctx{Source: src, Controller: 0}
	Resolve(h, c, sa(t, "DB$ EachDamage | ValidCards$ Creature | EachToItself$ True | NumDmg$ Count$CardPower"))

	dmg := damageEvents(h)
	if len(dmg) != 2 {
		t.Fatalf("damage events = %d (%+v), want one per damager (2)", len(dmg), dmg)
	}
	want := map[state.ObjID]int32{bear: 2, ox: 1}
	for _, ev := range dmg {
		if want[ev.Obj] != ev.Amount {
			t.Fatalf("damage to obj %d = %d, want %d (per-damager power, never the 9/9 source's power)",
				ev.Obj, ev.Amount, want[ev.Obj])
		}
	}
}

// TestEachDamageToEachOtherDealsBothWays pins the ToEachOther$ shape: the
// set named by ToEachOther$ (the parent's target joined with the sub's own
// answered ask) is BOTH damagers and recipients, each member dealing to the
// OTHERS -- and NumDmg$ Count$CardToughness evaluates per damager, so a 1/4
// deals 4 and a 3/2 deals 2.
func TestEachDamageToEachOtherDealsBothWays(t *testing.T) {
	h := newHost(t, 2)
	src := librarySource(t, h)
	mine := putCreature(t, h, 0, "Mine", "1/4", "")
	theirs := putCreature(t, h, 1, "Theirs", "3/2", "")
	c := &Ctx{Source: src, Controller: 0,
		Targets: []state.Target{{Obj: mine}}, // the parent's ask
		// this SA's own answered pre-ask (mvts1)
		PickedTargets: []state.Target{{Obj: theirs}},
	}
	Resolve(h, c, sa(t, "DB$ EachDamage | ToEachOther$ Targeted | NumDmg$ Count$CardToughness"))

	dmg := damageEvents(h)
	if len(dmg) != 2 {
		t.Fatalf("damage events = %d (%+v), want one per directed pair (2)", len(dmg), dmg)
	}
	want := map[state.ObjID]int32{mine: 2, theirs: 4}
	for _, ev := range dmg {
		if want[ev.Obj] != ev.Amount {
			t.Fatalf("damage to obj %d = %d, want %d (the OTHER member's toughness)", ev.Obj, ev.Amount, want[ev.Obj])
		}
	}
}

// TestEachDamageDefinedDamagersParentTargetPinsTheFightShape pins the
// fight family's shape (Polukranos, Living Inferno): DefinedDamagers$
// ParentTarget makes the PARENT's target the damager, Defined$ Self makes
// the resolving source the recipient, and the amount is the damager's own
// power.
func TestEachDamageDefinedDamagersParentTargetPinsTheFightShape(t *testing.T) {
	h := newHost(t, 2)
	src := putCreature(t, h, 0, "Polukranos", "5/5", "")
	attacker := putCreature(t, h, 1, "Bear", "2/2", "")
	c := &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: attacker}}}
	Resolve(h, c, sa(t, "DB$ EachDamage | DefinedDamagers$ ParentTarget | Defined$ Self | NumDmg$ Count$CardPower"))

	dmg := damageEvents(h)
	if len(dmg) != 1 {
		t.Fatalf("damage events = %d (%+v), want 1", len(dmg), dmg)
	}
	if dmg[0].Obj != src || dmg[0].Amount != 2 {
		t.Fatalf("damage = obj %d amount %d, want the SOURCE hit for the damager's own power 2", dmg[0].Obj, dmg[0].Amount)
	}
}

// TestEachDamageUnresolvableDamagerSpecFailsClosed: an unresolvable damager
// predicate fails closed with a replay-visible Note and no damage.
func TestEachDamageUnresolvableDamagerSpecFailsClosed(t *testing.T) {
	h := newHost(t, 2)
	src := librarySource(t, h)
	putCreature(t, h, 0, "Bear", "2/2", "")
	c := &Ctx{Source: src, Controller: 0}
	Resolve(h, c, sa(t, "DB$ EachDamage | DefinedDamagers$ Valid Creature.Wumpus+YouCtrl | Defined$ Self | NumDmg$ Count$CardPower"))
	if len(damageEvents(h)) != 0 {
		t.Fatalf("damage events = %+v, want none for the unknown predicate", damageEvents(h))
	}
	if !eachDamageHasNote(h) {
		t.Fatalf("log = %+v, want a Note for the unknown damager predicate", h.log)
	}
}

// TestEachDamagePerDamagerLifelink pays each damager's controller: two
// lifelink creatures dealing to themselves pay life to EACH DAMAGER'S OWN
// controller -- seat 0's creature pays seat 0, seat 1's pays seat 1.
func TestEachDamagePerDamagerLifelink(t *testing.T) {
	h := newHost(t, 2)
	src := librarySource(t, h)
	putCreature(t, h, 0, "LifelinkA", "2/5", "Lifelink")
	putCreature(t, h, 1, "LifelinkB", "3/5", "Lifelink")
	c := &Ctx{Source: src, Controller: 0}
	Resolve(h, c, sa(t, "DB$ EachDamage | ValidCards$ Creature | EachToItself$ True | NumDmg$ Count$CardPower"))

	var life [2]int32
	for _, ev := range h.log {
		if ev.Kind == events.LifeChange && ev.Amount > 0 {
			life[ev.Player] += ev.Amount
		}
	}
	if life[0] != 2 || life[1] != 3 {
		t.Fatalf("life gains = seat0 %d / seat1 %d, want 2 / 3 (one lifelink rider per damager, to ITS controller)", life[0], life[1])
	}
}

// TestEachDamageOwnTargetRecipients pins the DB-sub shape whose recipients
// are the SA's OWN answered ValidTgts$ ask (coordinated_clobbering's
// family): the parent's target is the damager (DefinedDamagers$
// ParentTarget), the pre-ask's answer is the recipient, and a sub with no
// eligible candidates at all leaves the resolution a no-op rather than
// falling through to the parent's targets.
func TestEachDamageOwnTargetRecipients(t *testing.T) {
	h := newHost(t, 2)
	src := putCreature(t, h, 0, "Clobberer", "3/3", "")
	mine := putCreature(t, h, 0, "Mine", "1/1", "")
	theirs := putCreature(t, h, 1, "Theirs", "1/1", "")

	t.Run("answered ask hits the chosen recipient", func(t *testing.T) {
		cc := &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: mine}},
			PickedTargets: []state.Target{{Obj: theirs}}}
		Resolve(h, cc, sa(t, "DB$ EachDamage | DefinedDamagers$ ParentTarget | ValidTgts$ Creature.OppCtrl | NumDmg$ Count$CardPower"))
		dmg := damageEvents(h)
		if len(dmg) != 1 || dmg[0].Obj != theirs || dmg[0].Amount != 1 {
			t.Fatalf("damage = %+v, want the chosen recipient hit for the damager's own power 1", dmg)
		}
	})

	t.Run("no eligible candidates fails closed to a no-op", func(t *testing.T) {
		h2 := &fakeHost{g: h.g}
		cc := &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: mine}}}
		// ValidTgts$ naming an unmatched spec: the pre-ask finds no eligible
		// candidates, poses nothing, and the resolution must NOT fall
		// through to the parent's targets (the damagers damaging themselves).
		Resolve(h2, cc, sa(t, "DB$ EachDamage | DefinedDamagers$ ParentTarget | ValidTgts$ Creature.Wumpus | NumDmg$ Count$CardPower"))
		if len(damageEvents(h2)) != 0 {
			t.Fatalf("damage events = %+v, want none when there is no answered recipient", damageEvents(h2))
		}
		if !eachDamageHasNote(h2) {
			t.Fatalf("log = %+v, want a Note for the missing recipient answer", h2.log)
		}
	})
}
