package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// synthetic pool tables: filter 1 is the enchanted permanent, filter 2 a
// Rabbit filter (subtype index 0), filter 3 "self"; override 0 sets base
// P/T 1/1, override 1 sets P/T 4/4, override 2 makes a creature a Rabbit.
func lgTables() *poolTables {
	return &poolTables{
		filters: []kop.Filter{{}, {Flags: kop.FAttachedBySrc}, {Sub: 1}, {Flags: kop.FSelf}},
		overrides: []kop.Override{
			{HasPT: true, SetP: 1, SetT: 1},
			{HasPT: true, SetP: 4, SetT: 4},
			{HasSub: true},
		},
		ovSubs: []ovSub{{}, {}, {set: []uint8{0}}},
	}
}

func lgStatic(st kop.Static) kop.Ability { return kop.Ability{Kind: kop.AbStatic, Static: st} }

func lgProg(in ...kop.Instr) kop.Ability { return kop.Ability{Kind: kop.AbActivated, Prog: in} }

func lgCheck(t *testing.T, pt *poolTables, defs ...kop.CardDef) error {
	t.Helper()
	var names []string
	var cls []layerClasses
	for i := range defs {
		names = append(names, defs[i].Name)
		cls = append(cls, classifyLayers(pt, &defs[i]))
	}
	return layerRefusal(names, cls)
}

// TestLayerGuardRowsLifted: the pairs the guard refused (two base-P/T setters,
// a subtype-reading grant next to a type grant) build: the walk
// orders them (dirac/static_layerguards_test.go and the layer scenarios).
func TestLayerGuardRowsLifted(t *testing.T) {
	pt := lgTables()
	aura := kop.CardDef{Name: "Shrinking Aura", Abilities: []kop.Ability{lgStatic(kop.Static{Kind: kop.StatOverride, Filter: 1, Override: 1})}}
	spell := kop.CardDef{Name: "Growth Spell", Abilities: []kop.Ability{lgProg(kop.Instr{Op: kop.OpOverride, Obj: kop.RefTarget0, A: kop.Const(0), C: 1})}}
	aura2 := kop.CardDef{Name: "Giant Aura", Abilities: []kop.Ability{lgStatic(kop.Static{Kind: kop.StatOverride, Filter: 1, Override: 2})}}
	lord := kop.CardDef{Name: "Rabbit Lord", Abilities: []kop.Ability{lgStatic(kop.Static{Kind: kop.StatContinuous, Filter: 2, AddSub: kop.SubMask{}.With(1)})}}
	grant := kop.CardDef{Name: "Make Rabbit", Abilities: []kop.Ability{lgProg(kop.Instr{Op: kop.OpOverride, Obj: kop.RefTarget0, A: kop.Const(0), C: 2})}}
	for _, pair := range [][2]kop.CardDef{{aura, spell}, {spell, aura}, {aura, aura2}, {lord, grant}} {
		if err := lgCheck(t, pt, pair[0], pair[1]); err != nil {
			t.Fatalf("%s + %s: refused: %v", pair[0].Name, pair[1].Name, err)
		}
	}
}

// TestLayerGuardRefusalNamesBothCards: the machinery a future row would use
// still names the pair.
func TestLayerGuardRefusalNamesBothCards(t *testing.T) {
	pt := lgTables()
	aura := kop.CardDef{Name: "Shrinking Aura", Abilities: []kop.Ability{lgStatic(kop.Static{Kind: kop.StatOverride, Filter: 1, Override: 1})}}
	aura2 := kop.CardDef{Name: "Giant Aura", Abilities: []kop.Ability{lgStatic(kop.Static{Kind: kop.StatOverride, Filter: 1, Override: 2})}}
	cls := []layerClasses{classifyLayers(pt, &aura), classifyLayers(pt, &aura2)}
	err := cardset.LayerRefusal([]string{aura.Name, aura2.Name}, cls, []layerConflict{{A: lcStatPT, B: lcStatPT, Why: "test row"}})
	if err == nil || !strings.Contains(err.Error(), aura.Name) || !strings.Contains(err.Error(), aura2.Name) {
		t.Fatalf("refusal does not name both cards: %v", err)
	}
}

func TestLayerGuardCleanPool(t *testing.T) {
	pt := lgTables()
	vanilla := kop.CardDef{Name: "Bear", Power: 2, Toughness: 2}
	// two self-only setters never meet on one permanent
	selfA := kop.CardDef{Name: "Self Animator", Abilities: []kop.Ability{lgProg(kop.Instr{Op: kop.OpOverride, Obj: kop.RefSelf, A: kop.Const(0), C: 0})}}
	selfB := kop.CardDef{Name: "Self Grower", Abilities: []kop.Ability{lgProg(kop.Instr{Op: kop.OpSetPT, Obj: kop.RefSelf, A: kop.Const(4), B: kop.Const(4)})}}
	// two records apply in timestamp order: no conflict
	recA := kop.CardDef{Name: "Shrink Spell", Abilities: []kop.Ability{lgProg(kop.Instr{Op: kop.OpOverride, Obj: kop.RefTarget0, A: kop.Const(0), C: 0})}}
	recB := kop.CardDef{Name: "Growth Spell", Abilities: []kop.Ability{lgProg(kop.Instr{Op: kop.OpSetPT, Obj: kop.RefTarget0, A: kop.Const(4), B: kop.Const(4), C: 1})}}
	// two copies of one Aura apply identical values
	aura := kop.CardDef{Name: "Shrinking Aura", Abilities: []kop.Ability{lgStatic(kop.Static{Kind: kop.StatOverride, Filter: 1, Override: 1})}}
	if err := lgCheck(t, pt, vanilla, selfA, selfB, recA, recB); err != nil {
		t.Fatalf("clean pool refused: %v", err)
	}
	if err := lgCheck(t, pt, aura, aura, vanilla); err != nil {
		t.Fatalf("one Aura twice refused: %v", err)
	}
	// an ability-stripping Aura stops the host's own setter
	strip := lgTables()
	strip.overrides[0].LoseAbilities = true
	if err := lgCheck(t, strip, aura, selfB); err != nil {
		t.Fatalf("stripping Aura next to a self-only setter refused: %v", err)
	}
}
