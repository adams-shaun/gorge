package cards

import "testing"

func expanded(t *testing.T, src string) *Face {
	t.Helper()
	c, diags := ParseBytes("k.txt", []byte(src))
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	if d := c.Link(); len(d) > 0 {
		t.Fatal(d)
	}
	return c.Faces[0]
}

// contains reports whether s holds needle. A small local helper: no other
// test in this package needed a generic string-slice membership check
// before this one, so it lives here rather than in a shared file.
func contains(s []string, needle string) bool {
	for _, v := range s {
		if v == needle {
			return true
		}
	}
	return false
}

func TestEtbCounterExpandsToAReplacement(t *testing.T) {
	f := expanded(t, "Name:Endless\nManaCost:X\nTypes:Creature Eldrazi\nPT:0/0\nK:etbCounter:P1P1:X\nSVar:X:Count$xPaid\nOracle:x\n")
	if len(f.Repls) != 1 || f.Repls[0].Event != "Moved" || f.Repls[0].Params["Keyword"] != "etbCounter" || f.Repls[0].With == nil {
		t.Fatalf("%+v", f.Repls)
	}
	w := f.Repls[0].With
	if w.API != "PutCounter" || w.Params["CounterType"] != "P1P1" || w.Params["CounterNum"] != "X" || w.Params["Defined"] != "Self" {
		t.Fatalf("%+v", w)
	}
}

func TestTriggerKeywordsExpandWithLinkedEffects(t *testing.T) {
	cases := map[string]struct{ src, mode, api string }{
		"Undying":       {"K:Undying", "ChangesZone", "ChangeZone"},
		"Evolve":        {"K:Evolve", "ChangesZone", "PutCounter"},
		"Exalted":       {"K:Exalted", "Attacks", "Pump"},
		"Prowess":       {"K:Prowess", "SpellCast", "Pump"},
		"Storm":         {"K:Storm", "SpellCast", "CopySpellAbility"},
		"Living Weapon": {"K:Living Weapon", "ChangesZone", "Token"},
	}
	for kw, tc := range cases {
		f := expanded(t, "Name:C\nManaCost:1\nTypes:Creature\nPT:1/1\n"+tc.src+"\nOracle:x\n")
		if len(f.Triggers) != 1 {
			t.Errorf("%s: %d triggers", kw, len(f.Triggers))
			continue
		}
		tr := f.Triggers[0]
		if tr.Mode != tc.mode || tr.Params["Keyword"] != kw || tr.Effect == nil || tr.Effect.API != tc.api {
			t.Errorf("%s: %+v effect %+v", kw, tr.Params, tr.Effect)
		}
	}
	lw := expanded(t, "Name:B\nManaCost:5\nTypes:Artifact Equipment\nK:Living Weapon\nOracle:x\n")
	if sub := lw.Triggers[0].Effect.Sub; sub == nil || sub.API != "Attach" || sub.Params["Defined"] != "Remembered" {
		t.Fatalf("living weapon sub-ability %+v", sub)
	}
}

func TestEquipAndEnchantExpandToAbilities(t *testing.T) {
	eq := expanded(t, "Name:Sword\nManaCost:3\nTypes:Artifact Equipment\nK:Equip:2\nOracle:x\n")
	if len(eq.Abilities) != 1 || eq.Abilities[0].Kind != "AB" || eq.Abilities[0].API != "Attach" || eq.Abilities[0].Params["Cost"] != "2" || eq.Abilities[0].Params["SorcerySpeed"] != "True" || eq.Abilities[0].Params["ValidTgts"] != "Creature.YouCtrl" {
		t.Fatalf("%+v", eq.Abilities)
	}
	aura := expanded(t, "Name:Rancor\nManaCost:G\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n")
	sp := aura.SpellAbility()
	if sp == nil || sp.API != "Attach" || sp.Params["ValidTgts"] != "Creature" || sp.Params["Object"] != "Self" {
		t.Fatalf("%+v", sp)
	}
}

func TestExpansionIsIdempotentAndTagged(t *testing.T) {
	c, _ := ParseBytes("k.txt", []byte("Name:C\nManaCost:1\nTypes:Creature\nPT:1/1\nK:Prowess\nK:Equip:1\nOracle:x\n"))
	c.Link()
	n, m := len(c.Faces[0].Triggers), len(c.Faces[0].Abilities)
	c.Link()
	if len(c.Faces[0].Triggers) != n || len(c.Faces[0].Abilities) != m {
		t.Fatal("a second Link expanded again")
	}
	prims := c.Primitives()
	for _, want := range []string{"kw:Prowess", "kw:Equip", "trig:SpellCast", "api:Pump", "api:Attach"} {
		if !contains(prims, want) {
			t.Errorf("primitives lack %s: %v", want, prims)
		}
	}
}

func TestUnexpandedKeywordsStayAlone(t *testing.T) {
	f := expanded(t, "Name:C\nManaCost:1\nTypes:Creature\nPT:1/1\nK:Flash\nK:Kicker:R\nK:Delve\nK:Protection from blue\nOracle:x\n")
	if len(f.Triggers)+len(f.Repls)+len(f.Abilities) != 0 {
		t.Fatalf("cast-time/static keywords must not expand: %d/%d/%d", len(f.Triggers), len(f.Repls), len(f.Abilities))
	}
}
