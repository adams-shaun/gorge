package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// isTargetingFixture builds the board the `IsTargeting` predicate tests read:
// two creatures split across the seats, a land, a source spell (the object
// `Other`/`~Other` exclusions are relative to) and a candidate spell ON THE
// STACK whose target list the tests assign per case.
func isTargetingFixture(t *testing.T) (*state.Game, map[string]state.ObjID, *state.Object) {
	t.Helper()
	g := state.NewGame([]string{"you", "them"})
	mk := func(owner state.PlayerID, zone state.Zone, src string) state.ObjID {
		c, d := cards.ParseBytes("t.txt", []byte(src))
		if len(d) != 0 {
			t.Fatalf("diags: %v", d)
		}
		c.Link()
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		o := g.AddObject(c, owner)
		o.Zone = zone
		g.SetZone(zone, owner, append(g.Zone(zone, owner), o.ID))
		return o.ID
	}
	ids := map[string]state.ObjID{
		"myBear":    mk(0, state.ZBattlefield, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"),
		"myLand":    mk(0, state.ZBattlefield, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"),
		"theirBear": mk(1, state.ZBattlefield, "Name:Elk\nManaCost:1 G\nTypes:Creature Elk\nPT:2/2\nOracle:x\n"),
		// The perspective source: a battlefield permanent whose ID the
		// `~Other` rewrite (+Other) excludes targets by. The exclusion is the
		// filter's own source-relative `Other` predicate, which reads the
		// object identity -- so the source must be a battlefield object for a
		// `Valid Permanent` target spec to be able to coincide with it.
		"sourceSpell": mk(0, state.ZBattlefield, "Name:Orvarish\nManaCost:2 U\nTypes:Creature Wizard\nPT:1/1\nOracle:x\n"),
	}
	// The candidate under test.
	spell := mk(0, state.ZStack, "Name:Bolt\nManaCost:R\nTypes:Instant\nOracle:x\n")
	// Preconditions every case below leans on, asserted once here so a
	// vacuous setup fails loudly instead of passing silently.
	if g.Obj(spell).Zone != state.ZStack {
		t.Fatalf("candidate spell is in zone %v, want the stack", g.Obj(spell).Zone)
	}
	if g.Obj(spell).Controller != 0 {
		t.Fatalf("candidate spell controller = %d, want 0", g.Obj(spell).Controller)
	}
	if g.Obj(ids["myBear"]).Controller != 0 || g.Obj(ids["theirBear"]).Controller != 1 {
		t.Fatalf("fixture creatures not split across the seats: %d / %d",
			g.Obj(ids["myBear"]).Controller, g.Obj(ids["theirBear"]).Controller)
	}
	if g.Obj(ids["myLand"]).Zone != state.ZBattlefield {
		t.Fatalf("fixture land is in zone %v, want the battlefield", g.Obj(ids["myLand"]).Zone)
	}
	if ids["sourceSpell"] == ids["myBear"] {
		t.Fatalf("source and target must be different objects for the ~Other case")
	}
	return g, ids, g.Obj(spell)
}

// TestSpellIsTargetingMatchesRecordedTargets pins the shared matcher's
// `Spell.IsTargeting <target-spec>` predicate: the candidate stack spell's
// own recorded target list (state.Object.Targets) decides, object targets
// through MatchesSpecCtx and player targets through MatchesPlayerSpecCtx,
// `Valid ` stripped and `~Other` rewritten to the source-relative +Other.
func TestSpellIsTargetingMatchesRecordedTargets(t *testing.T) {
	g, id, spell := isTargetingFixture(t)
	sc := SpecContext{You: 0, Source: id["sourceSpell"]}
	for _, tc := range []struct {
		name string
		tgts []state.Target
		spec string
		want bool
	}{
		{"object target, Valid Creature", []state.Target{{Obj: id["myBear"]}}, "Spell.IsTargeting Valid Creature", true},
		{"object target, Creature.YouCtrl", []state.Target{{Obj: id["myBear"]}}, "Spell.IsTargeting Valid Creature.YouCtrl", true},
		{"bare form without Valid", []state.Target{{Obj: id["myBear"]}}, "Spell.IsTargeting Creature.YouCtrl", true},
		{"target is not a land", []state.Target{{Obj: id["myBear"]}}, "Spell.IsTargeting Valid Land.YouCtrl", false},
		// The same YouCtrl spec against a target the OTHER seat controls: the
		// precondition (asserted below) is that the two controllers differ.
		{"target is the opponent's creature", []state.Target{{Obj: id["theirBear"]}}, "Spell.IsTargeting Valid Creature.YouCtrl", false},
		{"~Other admits a non-source permanent", []state.Target{{Obj: id["myBear"]}}, "Spell.IsTargeting Valid Permanent.YouCtrl~Other", true},
		{"plus conjunction with a candidate predicate", []state.Target{{Obj: id["myBear"]}}, "Spell.IsTargeting Valid Creature.YouCtrl+YouCtrl", true},
		{"plus conjunction rejects an unmatched candidate predicate", []state.Target{{Obj: id["myBear"]}}, "Spell.IsTargeting Valid Creature.YouCtrl+!YouCtrl", false},
		{"negation of a matching target", []state.Target{{Obj: id["myBear"]}}, "Spell.!IsTargeting Valid Creature.YouCtrl", false},
		{"negation holds on a non-matching target", []state.Target{{Obj: id["theirBear"]}}, "Spell.!IsTargeting Valid Creature.YouCtrl", true},
		{"comma OR reaching a non-matching alternative", []state.Target{{Obj: id["myBear"]}}, "Spell.IsTargeting Valid Land.YouCtrl,Spell.IsTargeting Valid Creature", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Each case's own precondition: the targets are really bound and
			// the YouCtrl negative really is a cross-seat target.
			if len(tc.tgts) == 0 || tc.tgts[0].Obj == 0 {
				t.Fatalf("case binds no target")
			}
			if tc.name == "target is the opponent's creature" && g.Obj(tc.tgts[0].Obj).Controller == sc.You {
				t.Fatalf("precondition: target controller == You, the negative proves nothing")
			}
			spell.Targets = tc.tgts
			if got := MatchesSpecCtx(g, tc.spec, spell.ID, sc); got != tc.want {
				t.Fatalf("MatchesSpecCtx(%q) = %v, want %v", tc.spec, got, tc.want)
			}
			// The textual oracle must agree with the compiled path -- the two
			// share the predicate through matchPositive, but the parity is
			// the point of keeping it there.
			if got := matchesObjectText(g, tc.spec, spell, sc); got != tc.want {
				t.Fatalf("matchesObjectText(%q) = %v, want %v", tc.spec, got, tc.want)
			}
		})
	}
}

// TestSpellIsTargetingExcludesItsOwnSource pins the `~Other` suffix against
// the target that must be excluded BY IT: the spell targets the perspective
// source itself, so the rewritten +Other is what fails the match. The
// precondition asserts the coincidence the exclusion leans on.
func TestSpellIsTargetingExcludesItsOwnSource(t *testing.T) {
	g, id, spell := isTargetingFixture(t)
	sc := SpecContext{You: 0, Source: id["sourceSpell"]}
	spell.Targets = []state.Target{{Obj: id["sourceSpell"]}}
	if spell.Targets[0].Obj != sc.Source {
		t.Fatalf("precondition: target %d is not the source %d the exclusion excludes",
			spell.Targets[0].Obj, sc.Source)
	}
	if MatchesSpecCtx(g, "Spell.IsTargeting Valid Permanent.YouCtrl~Other", spell.ID, sc) {
		t.Fatal("~Other must exclude the source itself")
	}
	// Without the exclusion the same target is an ordinary match -- so the
	// negative above is the exclusion's work, not the base spec's.
	if !MatchesSpecCtx(g, "Spell.IsTargeting Valid Permanent", spell.ID, sc) {
		t.Fatal("the same target must match without the ~Other exclusion")
	}
}

// TestSpellIsTargetingPlayerTargets pins the player half of the predicate: a
// player target is answered by MatchesPlayerSpecCtx (never handed to the
// object matcher as a playerless object), with the caller's You binding.
func TestSpellIsTargetingPlayerTargets(t *testing.T) {
	g, id, spell := isTargetingFixture(t)
	sc := SpecContext{You: 0, Source: 0}
	spell.Targets = []state.Target{{Player: 1, IsPlayer: true}}
	if !spell.Targets[0].IsPlayer || spell.Targets[0].Player == sc.You {
		t.Fatalf("precondition: target is not a player other than you (%v)", spell.Targets[0])
	}
	for _, tc := range []struct {
		name string
		spec string
		want bool
	}{
		{"bare Player admits any player target", "Spell.IsTargeting Player", true},
		{"Valid Player", "Spell.IsTargeting Valid Player", true},
		{"Opponent", "Spell.IsTargeting Opponent", true},
		{"You rejects the opponent", "Spell.IsTargeting Player.You", false},
		{"negated You", "Spell.!IsTargeting Player.You", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchesSpecCtx(g, tc.spec, spell.ID, sc); got != tc.want {
				t.Fatalf("MatchesSpecCtx(%q) = %v, want %v", tc.spec, got, tc.want)
			}
		})
	}
	// A player-typed spec must not match an OBJECT target through the same
	// predicate: the player grammar and the object grammar stay separate.
	spell.Targets = []state.Target{{Obj: id["myBear"]}}
	if MatchesSpecCtx(g, "Spell.IsTargeting Player", spell.ID, sc) {
		t.Fatal("a player spec must not admit an object target")
	}
}

// TestSpellIsTargetingUntargetedSpellFails pins the missing-target case: an
// empty target list is a RESOLVED non-match (never an unresolved gate), and a
// target record with no object behind it is skipped, not a match.
func TestSpellIsTargetingUntargetedSpellFails(t *testing.T) {
	g, _, spell := isTargetingFixture(t)
	sc := SpecContext{You: 0, Source: 0}
	if len(spell.Targets) != 0 {
		t.Fatalf("precondition: fresh spell carries %d targets, want none", len(spell.Targets))
	}
	for _, spec := range []string{
		"Spell.IsTargeting Valid Creature",
		"Spell.IsTargeting Player",
	} {
		if MatchesSpecCtx(g, spec, spell.ID, sc) {
			t.Fatalf("%s matched an untargeted spell", spec)
		}
	}
	// The negated spelling is the exception BY MEANING, not by omission: an
	// untargeted spell targets nothing at all, so "targets nothing matching
	// X" holds.
	if !MatchesSpecCtx(g, "Spell.!IsTargeting Valid Creature", spell.ID, sc) {
		t.Fatal("!IsTargeting must hold for a spell with no targets")
	}
	// A placeholder target with no object and no player never matches.
	spell.Targets = []state.Target{{}}
	if MatchesSpecCtx(g, "Spell.IsTargeting Valid Creature", spell.ID, sc) {
		t.Fatal("an object-less target record must be skipped, not matched")
	}
}

// TestSpellTargetingMatchesSplitsAlternatives pins the ConditionPresent$ gate's
// wrapper: OR over the spec's comma alternatives, each cut at its Spell./
// SpellAbility. base, evaluated through the shared matcher's normalisation --
// so the condition gate and the ordinary filter callers cannot drift.
func TestSpellTargetingMatchesSplitsAlternatives(t *testing.T) {
	g, id, spell := isTargetingFixture(t)
	sc := SpecContext{You: 0, Source: id["sourceSpell"]}
	// Orvar's exact shape: an object target OR a player target.
	spell.Targets = []state.Target{{Player: 1, IsPlayer: true}}
	spec := "Spell.IsTargeting Valid Permanent.YouCtrl~Other,Spell.IsTargeting Player"
	if !spellTargetingMatches(g, spec, spell, sc) {
		t.Fatal("the player alternative of Orvar's OR list must carry the match")
	}
	// The same OR answered when only the object alternative can match.
	spell.Targets = []state.Target{{Obj: id["myBear"]}}
	if !spellTargetingMatches(g, spec, spell, sc) {
		t.Fatal("the ~Other alternative of Orvar's OR list must carry the match")
	}
	// A SpellAbility spelling resolves the same way.
	if !spellTargetingMatches(g, "SpellAbility.IsTargeting Valid Creature", spell, sc) {
		t.Fatal("the SpellAbility spelling must read the same target list")
	}
	// A spec whose every alternative misses resolves to a plain false.
	if spellTargetingMatches(g, "Spell.IsTargeting Valid Land", spell, sc) {
		t.Fatal("a creature target must not satisfy a land spec")
	}
}

// TestSpellIsTargetingCensusAgreement pins the UnknownPredicates agreement:
// the supported complete forms are known to the census (so a ConditionPresent$
// or Count$ caller stops failing them closed), while the malformed and
// incomplete shapes -- no argument, a '+' the token grammar would truncate,
// a malformed ValidX, an unknown inner predicate -- stay unknown, which is
// the fail-closed direction every other predicate family takes.
func TestSpellIsTargetingCensusAgreement(t *testing.T) {
	for _, spec := range []string{
		"Spell.IsTargeting Valid Creature",
		"Spell.IsTargeting Valid Creature.YouCtrl",
		"Spell.IsTargeting Valid Permanent.YouCtrl~Other",
		"Spell.IsTargeting Player",
		"Spell.IsTargeting Creature.YouCtrl",
		"Spell.!IsTargeting Valid Creature",
		"SpellAbility.IsTargeting Valid Permanent,Spell.IsTargeting Player",
	} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty", spec, un)
		}
	}
	for _, spec := range []string{
		"Spell.IsTargeting",        // no argument
		"Spell.IsTargeting ",       // empty argument
		"Spell.IsTargeting ValidX", // malformed Valid prefix
		"Spell.IsTargeting Valid",  // Valid with no spec
		"Spell.IsTargeting Valid Creature.someMechanicWeDoNotModel", // unknown inner predicate
	} {
		if un := UnknownPredicates(spec); len(un) != 1 || un[0] != restAfterBase(spec) {
			t.Errorf("UnknownPredicates(%q) = %v, want the whole token reported unknown", spec, un)
		}
	}
	// A '+'-joined argument is split by the token grammar into separate
	// candidate predicates: the IsTargeting half is recognised on its own
	// truncation and the other half is an ordinary candidate predicate. The
	// census therefore reports exactly the non-IsTargeting remainder, and the
	// corpus convention for the common conjunction is `~Other`, which stays
	// whole (asserted above).
	if un := UnknownPredicates("Spell.IsTargeting Creature.YouCtrl+someMechanicWeDoNotModel"); len(un) != 1 || un[0] != "someMechanicWeDoNotModel" {
		t.Errorf("UnknownPredicates of a +joined argument = %v, want only the other predicate", un)
	}
	// An unknown form must not match either: the matcher and the census agree.
	g, id, spell := isTargetingFixture(t)
	spell.Targets = []state.Target{{Obj: id["myBear"]}}
	for _, spec := range []string{"Spell.IsTargeting", "Spell.IsTargeting ValidX", "Spell.IsTargeting Valid Creature.someMechanicWeDoNotModel"} {
		if MatchesSpecCtx(g, spec, spell.ID, SpecContext{You: 0}) {
			t.Fatalf("%s matched an unknown argument", spec)
		}
	}
}

// restAfterBase strips the leading base of a census-unknown spell spec for
// the assertion above (the census reports the token after the base cut).
func restAfterBase(spec string) string {
	for i := 0; i < len(spec); i++ {
		if spec[i] == '.' {
			return strings.TrimSpace(spec[i+1:])
		}
	}
	return strings.TrimSpace(spec)
}
