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
		"Undying":           {"K:Undying", "ChangesZone", "ChangeZone"},
		"Persist":           {"K:Persist", "ChangesZone", "ChangeZone"},
		"Evolve":            {"K:Evolve", "ChangesZone", "PutCounter"},
		"Exalted":           {"K:Exalted", "Attacks", "Pump"},
		"Prowess":           {"K:Prowess", "SpellCast", "Pump"},
		"Storm":             {"K:Storm", "SpellCast", "CopySpellAbility"},
		"Living Weapon":     {"K:Living Weapon", "ChangesZone", "Token"},
		"Cumulative upkeep": {"K:Cumulative upkeep:1", "Phase", "CumulativeUpkeep"},
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

// TestEnchantUsesTrailingFieldAsPrompt covers the K:Enchant:<spec>:<prompt>
// three-field form (162 corpus lines): the third field is Forge's own
// human-readable prompt and must be used verbatim, not glued onto ValidTgts.
func TestEnchantUsesTrailingFieldAsPrompt(t *testing.T) {
	aura := expanded(t, "Name:A\nManaCost:G\nTypes:Enchantment Aura\nK:Enchant:Creature.YouCtrl:creature you control\nOracle:x\n")
	sp := aura.SpellAbility()
	if sp == nil || sp.Params["ValidTgts"] != "Creature.YouCtrl" || sp.Params["TgtPrompt"] != "Select target creature you control" {
		t.Fatalf("%+v", sp)
	}
}

// TestEquipReadsTrailingFields covers K:Equip:<cost>:<restriction>:<desc>
// and the rider forms: the cost stays field 0 (trailing text must never
// reach Cost$), the restriction spec becomes ValidTgts$ verbatim, and
// ReduceCost$/ActivationLimit$ riders ride the minted SA (eqcm1 -- the
// expansion used to drop every trailing field). Prose fields (spaces) and
// "Flavor " markers are never a spec, and a rider-then-prose line keeps the
// default Creature.YouCtrl targets.
func TestEquipReadsTrailingFields(t *testing.T) {
	eq := expanded(t, "Name:S\nManaCost:3\nTypes:Artifact Equipment\nK:Equip:3:Creature.YouCtrl+Legendary:legendary creature\nOracle:x\n")
	if len(eq.Abilities) != 1 {
		t.Fatalf("%+v", eq.Abilities)
	}
	a := eq.Abilities[0]
	if a.Params["Cost"] != "3" || a.Params["ValidTgts"] != "Creature.YouCtrl+Legendary" {
		t.Fatalf("%+v", a.Params)
	}
	crown := expanded(t, "Name:C\nManaCost:4\nTypes:Artifact Equipment\nK:Equip:4:::ReduceCost$ Monarch:This ability costs {3} less to activate if you're the monarch\nOracle:x\n")
	ca := crown.Abilities[0]
	if ca.Params["ValidTgts"] != "Creature.YouCtrl" || ca.Params["ReduceCost"] != "Monarch" {
		t.Fatalf("%+v", ca.Params)
	}
	la := expanded(t, "Name:L\nManaCost:1\nTypes:Artifact Equipment\nK:Equip:0:::ActivationLimit$ 1:Activate only once each turn\nOracle:x\n")
	laa := la.Abilities[0]
	if laa.Params["ValidTgts"] != "Creature.YouCtrl" || laa.Params["ActivationLimit"] != "1" {
		t.Fatalf("%+v", laa.Params)
	}
	fl := expanded(t, "Name:F\nManaCost:5\nTypes:Artifact Equipment\nK:Equip:5:Flavor Murasame\nOracle:x\n")
	if fl.Abilities[0].Params["ValidTgts"] != "Creature.YouCtrl" {
		t.Fatalf("Flavor field leaked into ValidTgts: %+v", fl.Abilities[0].Params)
	}
}

// TestEtbCounterDropsTrailingConditionFromCounterNum covers
// K:etbCounter:<KIND>:<N>:<CheckSVar>:<desc> (182 corpus lines): only the
// second field is <N> -- a condition or description tacked on afterward
// must never reach CounterNum$ (some contain a "|", which would otherwise
// inject a spurious param).
func TestEtbCounterDropsTrailingConditionFromCounterNum(t *testing.T) {
	f := expanded(t, "Name:C\nManaCost:1\nTypes:Creature\nPT:1/1\nK:etbCounter:P1P1:1:CheckSVar$ WasKicked:If CARDNAME was kicked\nOracle:x\n")
	if len(f.Repls) != 1 || f.Repls[0].With == nil {
		t.Fatalf("%+v", f.Repls)
	}
	w := f.Repls[0].With
	if w.Params["CounterNum"] != "1" || w.Params["CounterType"] != "P1P1" {
		t.Fatalf("%+v", w.Params)
	}
	if _, ok := w.Params["CheckSVar"]; ok {
		t.Fatalf("trailing condition field leaked into a param: %+v", w.Params)
	}
}

// TestEquipExpandsEachDistinctKeywordLine is ruling FL-13: idempotency keys
// on the full keyword line (head + params), not the head alone, so a face
// with two distinct K:Equip: lines (different costs/restrictions -- 24 such
// cards in the corpus) expands both, not just the first.
func TestEquipExpandsEachDistinctKeywordLine(t *testing.T) {
	c, diags := ParseBytes("k.txt", []byte("Name:S\nManaCost:5\nTypes:Artifact Equipment\nK:Equip:2\nK:Equip:3:Creature.YouCtrl+Legendary:legendary creature\nOracle:x\n"))
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	if d := c.Link(); len(d) > 0 {
		t.Fatal(d)
	}
	f := c.Faces[0]
	if len(f.Abilities) != 2 {
		t.Fatalf("want 2 abilities for 2 distinct K:Equip: lines, got %+v", f.Abilities)
	}
	costs := map[string]bool{}
	for _, a := range f.Abilities {
		costs[a.Params["Cost"]] = true
	}
	if !costs["2"] || !costs["3"] {
		t.Fatalf("want costs {2,3}, got %+v", f.Abilities)
	}
	n := len(f.Abilities)
	if d := c.Link(); len(d) > 0 {
		t.Fatal(d)
	}
	if len(f.Abilities) != n {
		t.Fatal("a second Link expanded a still-present line again")
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

// TestTypeCyclingExpandsToATypedLibrarySearch pins CR 702.28d: typed cycling
// is a library SEARCH for the named type (not a draw), the trailing
// description field after the cost is dropped, and a second Link does not
// expand the line again (the KeywordLine idempotence contract the Affinity
// case documents).
func TestTypeCyclingExpandsToATypedLibrarySearch(t *testing.T) {
	c, diags := ParseBytes("k.txt", []byte("Name:S\nManaCost:2\nTypes:Creature\nPT:1/1\nK:TypeCycling:Island:2\nOracle:x\n"))
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	if d := c.Link(); len(d) > 0 {
		t.Fatal(d)
	}
	f := c.Faces[0]
	if len(f.Abilities) != 1 {
		t.Fatalf("want 1 TypeCycling ability, got %+v", f.Abilities)
	}
	a := f.Abilities[0]
	if a.API != "ChangeZone" {
		t.Fatalf("API = %q, want ChangeZone (a search, not a draw)", a.API)
	}
	for k, want := range map[string]string{
		"Origin": "Library", "Destination": "Hand",
		"ChangeType": "Island", "ChangeNum": "1",
		"Cost": "2 Discard<1/CARDNAME>", "KeywordLine": "TypeCycling:Island:2",
	} {
		if a.Params[k] != want {
			t.Errorf("param %s = %q, want %q", k, a.Params[k], want)
		}
	}
	// A trailing description field (Sojourner's Companion's
	// K:TypeCycling:Land.Artifact:2:artifact land) is dropped: the type and
	// cost are fields 0 and 1 only.
	c2, _ := ParseBytes("k.txt", []byte("Name:S\nManaCost:2\nTypes:Creature\nPT:1/1\nK:TypeCycling:Land.Artifact:2:artifact land\nOracle:x\n"))
	c2.Link()
	if got := c2.Faces[0].Abilities[0].Params["ChangeType"]; got != "Land.Artifact" {
		t.Fatalf("ChangeType with trailing description = %q, want Land.Artifact", got)
	}
	// Idempotence: a second Link (cards/registry.go re-links cached faces)
	// must not append a second search.
	n := len(f.Abilities)
	c.Link()
	if len(f.Abilities) != n {
		t.Fatal("a second Link expanded K:TypeCycling again")
	}
}

func TestUnexpandedKeywordsStayAlone(t *testing.T) {
	f := expanded(t, "Name:C\nManaCost:1\nTypes:Creature\nPT:1/1\nK:Flash\nK:Kicker:R\nK:Delve\nK:Protection from blue\nOracle:x\n")
	if len(f.Triggers)+len(f.Repls)+len(f.Abilities) != 0 {
		t.Fatalf("cast-time/static keywords must not expand: %d/%d/%d", len(f.Triggers), len(f.Repls), len(f.Abilities))
	}
}
