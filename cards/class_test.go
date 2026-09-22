package cards

import "testing"

// TestClassExpandsEntryCounterActivatorsAndGrants pins the kw:Class expansion
// on Fortune Teller's Talent's real script shape: one entry counter (a Class
// enters at level 1, CR 702.118a), one sorcery-speed level-up activator per
// level gated on the Class's level being BELOW that level (CR 702.118b), and
// the level's granted static appended with a counters_GE<N>_LEVEL gate so it
// is live from level N on.
func TestClassExpandsEntryCounterActivatorsAndGrants(t *testing.T) {
	f := expanded(t, "Name:Fortune Teller's Talent\nManaCost:U\nTypes:Enchantment Class\n"+
		"K:Class:2:3 U:AddStaticAbility$ SFutureSight\n"+
		"K:Class:3:2 U:AddStaticAbility$ SReduceCost\n"+
		"SVar:SFutureSight:Mode$ Continuous | Affected$ Card.TopLibrary+YouCtrl | CheckSVar$ X | AffectedZone$ Library | MayPlay$ True\n"+
		"SVar:SReduceCost:Mode$ ReduceCost | ValidCard$ Card.!wasCastFromYourHand | Type$ Spell | Activator$ You | Amount$ 2\n"+
		"SVar:X:Count$ThisTurnCast_Card.YouCtrl\nOracle:x\n")

	// 1. The entry counter, once.
	if len(f.Repls) != 1 || f.Repls[0].Event != "Moved" || f.Repls[0].With == nil {
		t.Fatalf("want one entry replacement, got %+v", f.Repls)
	}
	if w := f.Repls[0].With; w.API != "PutCounter" || w.Params["CounterType"] != "LEVEL" || w.Params["CounterNum"] != "1" {
		t.Fatalf("entry counter wrong: %+v", w.Params)
	}

	// 2. One activator per level, at the named cost, the right level gate.
	if len(f.Abilities) != 2 {
		t.Fatalf("want 2 level-up activators, got %d: %+v", len(f.Abilities), f.Abilities)
	}
	byCost := map[string]*SA{}
	for _, a := range f.Abilities {
		if a.API != "PutCounter" {
			t.Fatalf("level-up activator is not a PutCounter: %+v", a)
		}
		byCost[a.Params["Cost"]] = a
	}
	l2, l3 := byCost["3 U"], byCost["2 U"]
	if l2 == nil || l3 == nil {
		t.Fatalf("activator costs wrong: %+v", byCost)
	}
	if l2.Params["IsPresent"] != "Card.Self+counters_LT2_LEVEL" {
		t.Fatalf("level-2 activator gate: %q", l2.Params["IsPresent"])
	}
	if l3.Params["IsPresent"] != "Card.Self+counters_LT3_LEVEL" {
		t.Fatalf("level-3 activator gate: %q", l3.Params["IsPresent"])
	}
	for _, a := range f.Abilities {
		if a.Params["SorcerySpeed"] != "True" {
			t.Fatalf("level-up must be sorcery speed: %+v", a.Params)
		}
	}

	// 3. The granted statics, gated at their own level.
	var sf, sr *Static
	for i := range f.Statics {
		switch f.Statics[i].Params["IsPresent"] {
		case "Card.Self+counters_GE2_LEVEL":
			sf = &f.Statics[i]
		case "Card.Self+counters_GE3_LEVEL":
			sr = &f.Statics[i]
		}
	}
	if sf == nil || sf.Mode != "Continuous" || sf.Params["MayPlay"] != "True" || sf.Params["CheckSVar"] != "X" {
		t.Fatalf("level-2 grant wrong: %+v", sf)
	}
	if sr == nil || sr.Mode != "ReduceCost" || sr.Params["Amount"] != "2" {
		t.Fatalf("level-3 grant wrong: %+v", sr)
	}
}

// TestClassExpandsTriggerAndAmpersandGrants pins the other two grant kinds:
// AddTrigger$ appends a trigger with the level gate (Warlock Class's
// TriggerClassLevel shape) and an "A & B" AddStaticAbility$ value yields one
// static per named body (with_two_of_everything? -> SMayLook & SMayPlay).
func TestClassExpandsTriggerAndAmpersandGrants(t *testing.T) {
	f := expanded(t, "Name:Warlock Class\nManaCost:B\nTypes:Enchantment Class\n"+
		"K:Class:2:1 B:AddTrigger$ TriggerClassLevel\n"+
		"K:Class:3:2 U:AddStaticAbility$ SMayLook & SMayPlay\n"+
		"SVar:TriggerClassLevel:Mode$ ClassLevelGained | ClassLevel$ 2 | ValidCard$ Card.Self | Execute$ TrigDig | TriggerZones$ Battlefield\n"+
		"SVar:TrigDig:DB$ Dig | DigNum$ 3\n"+
		"SVar:SMayLook:Mode$ Continuous | Affected$ Card.TopLibrary+YouCtrl | AffectedZone$ Library | MayLookAt$ You\n"+
		"SVar:SMayPlay:Mode$ Continuous | Affected$ Card.TopLibrary+YouCtrl | AffectedZone$ Library | MayPlay$ True\n"+
		"Oracle:x\n")

	if len(f.Triggers) != 1 || f.Triggers[0].Mode != "ClassLevelGained" {
		t.Fatalf("want one ClassLevelGained trigger, got %+v", f.Triggers)
	}
	if f.Triggers[0].Params["IsPresent"] != "Card.Self+counters_GE2_LEVEL" {
		t.Fatalf("trigger level gate: %q", f.Triggers[0].Params["IsPresent"])
	}
	if f.Triggers[0].Effect == nil || f.Triggers[0].Effect.API != "Dig" {
		t.Fatalf("trigger Execute$ did not link: %+v", f.Triggers[0].Effect)
	}
	n := 0
	for _, s := range f.Statics {
		if s.Params["IsPresent"] == "Card.Self+counters_GE3_LEVEL" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("want 2 statics for an 'A & B' grant, got %d: %+v", n, f.Statics)
	}
}

// TestClassExpansionIsIdempotent pins the KeywordLine idempotence across a
// second Link() of the same face -- the cached-face re-link path. Without the
// static arm of the `has` check a second Link double-appends every level's
// granted static and activator.
func TestClassExpansionIsIdempotent(t *testing.T) {
	c, diags := ParseBytes("k.txt", []byte("Name:Warlock Class\nManaCost:B\nTypes:Enchantment Class\n"+
		"K:Class:2:1 B:AddStaticAbility$ SBoost\n"+
		"SVar:SBoost:Mode$ Continuous | Affected$ Creature.YouCtrl | AddPower$ 1\n"+
		"Oracle:x\n"))
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	if d := c.Link(); len(d) > 0 {
		t.Fatal(d)
	}
	f := c.Faces[0]
	repls, abilities, statics := len(f.Repls), len(f.Abilities), len(f.Statics)
	if repls != 1 || abilities != 1 || statics != 1 {
		t.Fatalf("first Link: repls=%d abilities=%d statics=%d", repls, abilities, statics)
	}
	if d := c.Link(); len(d) > 0 {
		t.Fatal(d)
	}
	if len(f.Repls) != repls || len(f.Abilities) != abilities || len(f.Statics) != statics {
		t.Fatalf("second Link re-expanded: repls=%d abilities=%d statics=%d", len(f.Repls), len(f.Abilities), len(f.Statics))
	}
}
