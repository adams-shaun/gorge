package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// etbCloneCleanNames is the measured set of corpus cards carrying an
// ETBReplacement:Copy keyword whose DB$ Clone body's parameter set lies
// entirely inside etbCloneWhitelist's supported scope {Choices$, ChoiceZone$,
// ChoiceTitle$, NewName$, RememberCloneOrigin$, AddTypes$, AddKeywords$,
// PumpKeywords$, PumpDuration$, SpellDescription$, ...} AND whose values are
// inside it too (no unsupported SVar-resolved selector predicate, no
// multi-word AddKeywords$ head -- see etbCloneWhitelist). The pin is
// bidirectional: a corpus-pin
// bump or a scope change that alters membership fails here and forces a
// deliberate re-measure, so the supported boundary cannot widen (a new
// parameter accepted into etbCloneWhitelist) or narrow silently.
var etbCloneCleanNames = []string{
	"Activated Sleeper", "Altered Ego", "Auton Soldier", "Body Double", "Callidus Assassin", "Chameleon, Master of Disguise", "Clever Impersonator", "Clone", "Colossal Clone Master", "Copy Artifact", "Copy Enchantment", "Copy Land",
	"Copycrook", "Cursed Mirror", "Dack's Duplicate", "Deceptive Frostkite", "Dominion Saboteur", "Echoing Deeps", "Estrid's Invocation", "Evil Twin", "Ezri Dax, Newly Joined", "Flesh Duplicate", "Gigantoplasm",
	"Glasspool Mimic", "Hulking Metamorph", "Imposter Mech", "Jwari Shapeshifter", "Lazotep Convert", "Machine God's Effigy", "Malleable Impostor", "Masterwork of Ingenuity", "Mercurial Pretender", "Mirror Image", "Mirrorhall Mimic",
	"Mirrormade", "Mocking Doppelganger", "Mockingbird", "Moritte of the Frost", "Naga Fleshcrafter", "Omni-Changeling",
	"Phantasmal Image", "Phyrexian Metamorph", "Pirated Copy", "Progenitor Mimic", "Quicksilver Gargantuan", "Sakashima of a Thousand Faces", "Sakashima the Impostor", "Sakashima's Protege", "Sakashima's Student", "Sculpting Steel", "Spark Double",
	"Stunt Double", "Superior Spider-Man", "Surgical Metamorph", "Synth Infiltrator", "The Fourteenth Doctor", "The Master, Formed Anew", "Undercover Operative", "Vesuva", "Vesuvan Doppelganger", "Vesuvan Shapeshifter", "Visage Bandit", "Vizier of Many Faces", "Voq, Torchbearer", "Wall of Stolen Identity", "Waxen Shapethief",
}

// TestETBCloneWhitelistCensus walks every ETBReplacement:Copy Repl in the
// corpus and asserts etbCloneWhitelist classifies each body exactly as the
// scope says: offered iff every parameter is inside the supported key set and
// every value passes its check (see etbCloneWhitelist). A card is never
// classified both ways.
func TestETBCloneWhitelistCensus(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	var clean []string
	verdicts := map[string]bool{}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for i := range f.Repls {
				r := &f.Repls[i]
				if r.Params["Keyword"] != "ETBReplacement" || r.With == nil || r.With.API != "Clone" {
					continue
				}
				v := etbCloneWhitelist(r.With, f.SVars)
				if prev, ok := verdicts[f.Name]; ok && prev != v {
					t.Errorf("%s carries ETB Clone bodies classified both offered and not", f.Name)
				}
				verdicts[f.Name] = v
				if v {
					clean = append(clean, f.Name)
				}
			}
		}
	}
	slices.Sort(clean)
	t.Logf("etb clone carriers: %d total, %d inside the whitelist scope", len(verdicts), len(clean))
	if !slices.Equal(clean, etbCloneCleanNames) {
		t.Errorf("whitelist-clean carrier set moved; re-measure and re-pin.\n got (%d): %v\nwant (%d): %v",
			len(clean), clean, len(etbCloneCleanNames), etbCloneCleanNames)
	}
}

// TestETBCloneWhitelistRegressionCarriers pins the classification of the
// carriers a blacklist gets wrong (IntoPlayTapped$/ChoiceTitle$-shaped bodies)
// and of the end-to-end test
// cards.
func TestETBCloneWhitelistRegressionCarriers(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	pins := []struct {
		name string
		want bool
	}{
		{"Malleable Impostor", true},
		{"Mirror Image", true},
		// Moritte's body rides SubAbility$ DBConditionEffect (the "if it's a
		// creature, it enters with two additional +1/+1 counters" condition):
		// the type-conditional counter rider (CloneETBTypedCounters), offered.
		{"Moritte of the Frost", true},
		// Mockingbird's cast-spend Y selector is supported; Flesh Duplicate's
		// AddKeywords$ IfNew Vanishing:3 is the conditional-keyword rider
		// (CR 707.9b), supported since its inner keyword is a single-word
		// head cards expands on the copy basis face (the ifnew payload).
		{"Mockingbird", true},
		{"Flesh Duplicate", true},
		// staticgoad1: Mocking Doppelganger's AddStaticAbilities$ FamilyTease
		// is a readable Goad$ static, so its ETB copy is offered and the
		// granted goad registers (engine/rules/goad_granted_test.go pins it end to
		// end). A carrier whose AddStaticAbilities$ member is NOT a readable
		// Goad line stays withheld, the whitelist's own value gate.
		{"Mocking Doppelganger", true},
		{"Vesuva", true},                // IntoPlayTapped$ True now taps the entering copy
		{"Cursed Mirror", true},         // Duration$ UntilEndOfTurn rides the copy unit
		{"Mirrorhall Mimic", true},      // ChoiceTitle$ is prompt text only
		{"Vizier of Many Faces", true},  // Embalm$-gated exceptions, decided per entering object (edh_etbclone_vizier_test.go)
		{"Vesuvan Shapeshifter", true},  // Duration$ UntilFacedown rides rules' event-driven expiry (clone_vesuvan_facedown_test.go)
		{"The Fourteenth Doctor", true}, // ChoiceZone$ Graveyard and PumpKeywords$ Haste/PumpDuration$ EOT: inside the whitelist scope
		// The CopyExcept exceptions (CR 707.9b): a literal P/T, a parsed
		// colour and NonLegendary$ True are offered; an entry-time P/T
		// reader is offered only through the bare SVar-name shape whose body
		// is exactly Count$CardPower/Count$CardToughness (Hulking Metamorph;
		// Living Lore's Remembered$CardManaCost stays out).
		{"Quicksilver Gargantuan", true},
		{"Vesuvan Doppelganger", true},
		{"Auton Soldier", true},
		{"Hulking Metamorph", true},
		// Living Lore and Thunderbond Vanguard are the corpus's other
		// non-literal P/T writers, but neither is an ETB Clone carrier the
		// whitelist classifies (Living Lore's ETBReplacement:Copy body is a
		// ChangeZone; Thunderbond's EssenceClone names no SetPower$), so
		// neither can join the census however their SVar reads.
	}
	for _, p := range pins {
		c, ok := reg.Lookup(p.name)
		if !ok {
			t.Fatalf("corpus missing %s", p.name)
		}
		found := false
		for _, f := range c.Faces {
			for i := range f.Repls {
				r := &f.Repls[i]
				if r.Params["Keyword"] != "ETBReplacement" || r.With == nil || r.With.API != "Clone" {
					continue
				}
				found = true
				if got := etbCloneWhitelist(r.With, f.SVars); got != p.want {
					t.Errorf("%s: etbCloneWhitelist = %v, want %v (body: %s)", p.name, got, p.want, r.With.Line)
				}
			}
		}
		if !found {
			t.Fatalf("%s carries no ETBReplacement:Copy DB$ Clone body", p.name)
		}
	}
}
