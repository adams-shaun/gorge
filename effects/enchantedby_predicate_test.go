package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// enchantedByObject is corpusObject plus the zone-slice registration the
// attached-object scan (hasAttachmentOfKind's AliveFrom/Zone walk) reads;
// corpusObject alone leaves the zone slices empty.
func enchantedByObject(t *testing.T, reg *cards.Registry, g *state.Game, name string) *state.Object {
	o := corpusObject(t, reg, g, name)
	g.SetZone(state.ZBattlefield, o.Owner, append(g.Zone(state.ZBattlefield, o.Owner), o.ID))
	return o
}

// TestEnchantedByTwoTokenPredicate is the effects leaf for the two-token
// space grammar "EnchantedBy <Type>.<qual>": the candidate bears an attached
// permanent of the named type whose qualifier holds against THAT attached
// object. Before this change the whole token survived the spec splitter as
// one unknown predicate (a space is not a delimiter), so Daybreak Coronet's
// `K:Enchant:Creature.EnchantedBy Aura.Other` matched nothing -- zero legal
// targets, and the cast was withheld permanently (CR 601.2c). The bare
// "EnchantedBy" token keeps its map-predicate meaning (attachedBy: the
// candidate is the permanent the RESOLVING SOURCE is attached to) and is
// guarded separately below.
func TestEnchantedByTwoTokenPredicate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := enchantedByObject(t, reg, g, "Grizzly Bears") // a Creature
	mountain := enchantedByObject(t, reg, g, "Mountain")  // a Land
	auraOnBear := enchantedByObject(t, reg, g, "Wild Growth")
	foreignAura := enchantedByObject(t, reg, g, "Wild Growth")
	bearPlain := enchantedByObject(t, reg, g, "Grizzly Bears")

	// Mutations re-fetch through g.Obj: AddObject returns a pointer into
	// g.Objs, which later appends (more objects below) reallocate away.
	attach := func(o *state.Object, to state.ObjID) { g.Obj(o.ID).AttachedTo = to }
	ctrl := func(o *state.Object, p state.PlayerID) { g.Obj(o.ID).Controller = p }
	attach(auraOnBear, bear.ID)
	attach(foreignAura, mountain.ID)
	ctrl(foreignAura, 1) // an Aura neither YOU control

	sc := SpecContext{You: 0, Source: 0}

	// Creature.EnchantedBy Aura.Other: exactly the creature bearing an Aura
	// (during a cast the source is not yet attached, so any current Aura
	// qualifies -- the oracle's "another Aura attached to it").
	if !MatchesObjectCtx(g, "Creature.EnchantedBy Aura.Other", g.Obj(bear.ID), sc) {
		t.Errorf("Creature.EnchantedBy Aura.Other must match a creature bearing an Aura")
	}
	if MatchesObjectCtx(g, "Creature.EnchantedBy Aura.Other", g.Obj(bearPlain.ID), sc) {
		t.Errorf("Creature.EnchantedBy Aura.Other must not match a bare creature")
	}

	// Creature.EnchantedBy Aura.YouCtrl: the attached Aura must be controlled
	// by the spec's you; the opposing Aura does not qualify.
	if !MatchesObjectCtx(g, "Creature.EnchantedBy Aura.YouCtrl", g.Obj(bear.ID), sc) {
		t.Errorf("Creature.EnchantedBy Aura.YouCtrl must match a creature bearing your Aura")
	}
	if MatchesObjectCtx(g, "Creature.EnchantedBy Aura.YouCtrl", g.Obj(bear.ID), SpecContext{You: 1}) {
		t.Errorf("Creature.EnchantedBy Aura.YouCtrl must not match a creature bearing an opposing Aura")
	}

	// Other excludes the resolving source itself: Face of Divinity's static
	// (`Affected$ Creature.EnchantedBy+EnchantedBy Aura.Other`) asks whether
	// ANOTHER Aura is attached -- Face itself is the source and must not
	// satisfy its own predicate. A second Aura does. A dedicated creature
	// carrying ONLY the source Aura keeps the two checks disjoint.
	lone := enchantedByObject(t, reg, g, "Grizzly Bears")
	face := enchantedByObject(t, reg, g, "Wild Growth")
	attach(face, lone.ID)
	scFace := SpecContext{You: 0, Source: face.ID}
	if MatchesObjectCtx(g, "Creature.EnchantedBy Aura.Other", g.Obj(lone.ID), scFace) {
		t.Errorf("with only the source Aura attached, Creature.EnchantedBy Aura.Other must not match (the source is not ANOTHER Aura)")
	}
	attach(foreignAura, lone.ID)
	if !MatchesObjectCtx(g, "Creature.EnchantedBy Aura.Other", g.Obj(lone.ID), scFace) {
		t.Errorf("with a second Aura attached, Creature.EnchantedBy Aura.Other must match")
	}

	// The leading-'!' negation works through the same classifier (the corpus
	// carries no !EnchantedBy <Type>.<qual>, but the recognition agreement
	// must hold).
	if !MatchesObjectCtx(g, "Creature.!EnchantedBy Aura.Other", g.Obj(bearPlain.ID), sc) {
		t.Errorf("Creature.!EnchantedBy Aura.Other must match a bare creature")
	}
	if MatchesObjectCtx(g, "Creature.!EnchantedBy Aura.Other", g.Obj(bear.ID), sc) {
		t.Errorf("Creature.!EnchantedBy Aura.Other must not match an Aura-bearing creature")
	}

	// The matcher and UnknownPredicates agree: both corpus shapes (and their
	// negation) are recognised, so the census reports nothing for them.
	for _, spec := range []string{
		"Creature.EnchantedBy Aura.Other", "Creature.EnchantedBy Aura.YouCtrl",
		"Creature.!EnchantedBy Aura.Other",
	} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty (recognised two-token EnchantedBy)", spec, un)
		}
	}
}

// TestEnchantedByNarrowArgsStillUnknown keeps the scope honest: any argument
// beyond the validated `<corpus type word>.Other|YouCtrl` shape -- a
// resolution-time referent, a nested predicate, a qualifier outside the
// allowlist, an unrecognised type word, or no dot at all -- stays unknown,
// fails closed, and is still reported by UnknownPredicates.
func TestEnchantedByNarrowArgsStillUnknown(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := enchantedByObject(t, reg, g, "Grizzly Bears")
	aura := enchantedByObject(t, reg, g, "Wild Growth")
	g.Obj(aura.ID).AttachedTo = bear.ID

	for _, spec := range []string{
		"Creature.EnchantedBy Aura.Targeted",
		"Creature.EnchantedBy Aura.Permanent.YouCtrl",
		"Creature.EnchantedBy Aura.Self",
		"Creature.EnchantedBy Thingling.YouCtrl",
		"Creature.EnchantedBy Aura",
	} {
		if MatchesObjectCtx(g, spec, g.Obj(bear.ID), SpecContext{You: 0}) {
			t.Errorf("%s must match nothing (the argument shape stays unrecognised)", spec)
		}
	}
	for spec, want := range map[string]string{
		"Creature.EnchantedBy Aura.Targeted":          "EnchantedBy Aura.Targeted",
		"Creature.EnchantedBy Aura.Permanent.YouCtrl": "EnchantedBy Aura.Permanent.YouCtrl",
		"Creature.EnchantedBy Aura.Self":              "EnchantedBy Aura.Self",
		"Creature.EnchantedBy Thingling.YouCtrl":      "EnchantedBy Thingling.YouCtrl",
		"Creature.EnchantedBy Aura":                   "EnchantedBy Aura",
	} {
		un := UnknownPredicates(spec)
		if len(un) != 1 || un[0] != want {
			t.Errorf("UnknownPredicates(%q) = %v, want [%s]", spec, un, want)
		}
	}
}

// TestEnchantedByBareFormUnchanged pins the guard the brief names: the BARE
// "EnchantedBy" map predicate (attachedBy) keeps its existing meaning -- the
// candidate is the permanent the resolving SOURCE is currently attached to.
// Several other open issues depend on that shape; the two-token work must
// not have touched it.
func TestEnchantedByBareFormUnchanged(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := enchantedByObject(t, reg, g, "Grizzly Bears")
	aura := enchantedByObject(t, reg, g, "Wild Growth")
	bearPlain := enchantedByObject(t, reg, g, "Grizzly Bears")
	g.Obj(aura.ID).AttachedTo = bear.ID

	// The source (the Aura) is attached to the bear: the bearer matches, a
	// bare creature does not.
	if !MatchesObjectCtx(g, "Creature.EnchantedBy", g.Obj(bear.ID), SpecContext{Source: aura.ID}) {
		t.Errorf("Creature.EnchantedBy (bare) must match the permanent the source Aura is attached to")
	}
	if MatchesObjectCtx(g, "Creature.EnchantedBy", g.Obj(bearPlain.ID), SpecContext{Source: aura.ID}) {
		t.Errorf("Creature.EnchantedBy (bare) must not match a creature the source is not attached to")
	}
	// A source with no attachment matches nothing.
	if MatchesObjectCtx(g, "Creature.EnchantedBy", g.Obj(bear.ID), SpecContext{Source: bearPlain.ID}) {
		t.Errorf("Creature.EnchantedBy (bare) must not match when the source is not an attached Aura")
	}
	if un := UnknownPredicates("Creature.EnchantedBy"); len(un) != 0 {
		t.Errorf("UnknownPredicates(\"Creature.EnchantedBy\") = %v, want empty (the bare form stays a recognised map predicate)", un)
	}
}

// TestEnchantedByCompiledFallsBackToTextual verifies the compiled-predicate
// side needs no change and cannot disagree: a word the compiler does not
// model compiles to a `maybe` term, Evaluate returns PredicateMaybe, and
// MatchesObjectCtx falls through to the textual oracle -- the same behaviour
// wordAttachedTo/wordSharesCardType already have. The program and the bare
// textual path must answer identically for every object here.
func TestEnchantedByCompiledFallsBackToTextual(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := enchantedByObject(t, reg, g, "Grizzly Bears")
	mountain := enchantedByObject(t, reg, g, "Mountain")
	auraOnBear := enchantedByObject(t, reg, g, "Wild Growth")
	foreignAura := enchantedByObject(t, reg, g, "Wild Growth")
	bearPlain := enchantedByObject(t, reg, g, "Grizzly Bears")
	g.Obj(auraOnBear.ID).AttachedTo = bear.ID
	g.Obj(foreignAura.ID).AttachedTo = mountain.ID
	g.Obj(foreignAura.ID).Controller = 1

	ps := CompilePredicatePrograms([]string{
		"Creature.EnchantedBy Aura.Other", "Creature.EnchantedBy Aura.YouCtrl",
	})
	for _, tc := range []struct {
		spec string
		sc   SpecContext
	}{
		{"Creature.EnchantedBy Aura.Other", SpecContext{You: 0}},
		{"Creature.EnchantedBy Aura.YouCtrl", SpecContext{You: 0}},
		{"Creature.EnchantedBy Aura.YouCtrl", SpecContext{You: 1}},
	} {
		for name, o := range map[string]*state.Object{
			"aura-bearing bear": g.Obj(bear.ID), "bare bear": g.Obj(bearPlain.ID),
		} {
			with := MatchesObjectCtx(g, tc.spec, o, func() SpecContext { s := tc.sc; s.PredicatePrograms = ps; return s }())
			without := MatchesObjectCtx(g, tc.spec, o, tc.sc)
			if with != without {
				t.Errorf("%s under %s: compiled program (%v) disagrees with the textual oracle (%v)", tc.spec, name, with, without)
			}
		}
	}
}
