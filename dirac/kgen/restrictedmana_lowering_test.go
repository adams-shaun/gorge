package kgen

import (
	"strings"
	"testing"
)

// TestRestrictedManaCostRefusalNamesThePart pins the gap4-07 carriers'
// refusals: a spend-restricted mana ability whose cost goes beyond a plain {T}
// refuses by naming the cost part (a mana fee, a life
// payment, a counter cost, a sacrifice of another permanent) instead of
// the old two-class reason, and the old exact two classes are gone. No card is
// lowered until the kernel's restricted-cost gap has lockstep evidence
// (AGENTS.md's lockstep rule; the follow-up ticket). "A tap of another
// permanent" no longer has a carrier: Grand Architect lowers (see
// TestRestrictedTapXSourceLowers); the refusal path in lower_restrictedmana.go
// stays.
func TestRestrictedManaCostRefusalNamesThePart(t *testing.T) {
	const prefix = "mana restriction on a source whose cost goes beyond {T}: "
	cases := []struct{ name, part string }{
		// Castle Garenbrig lowers now (TestRestrictedManaCostUnit): its fee
		// alternative is a fixed colour of a const amount, the shape the
		// kernel's restricted pair unit plans
		{"Cormela, Glamour Thief", "a mana fee"},
		{"Great Hall of the Citadel", "a mana fee"},
		{"Hydro-Channeler", "a mana fee"},
		{"Interplanar Beacon", "a mana fee"},
		{"Orb of Dragonkind", "a mana fee"},
		{"Crucible of the Spirit Dragon", "a counter cost"},
		{"Cryptic Trilobite", "a counter cost"},
		{"Throne of Makindi", "a counter cost"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lc := lowerCorpusCard(t, c.name)
			var got []string
			for _, rf := range lc.Refusals {
				got = append(got, rf.Reason)
			}
			if len(got) != 1 || !strings.HasPrefix(got[0], prefix) || !strings.Contains(got[0], c.part) {
				t.Fatalf("%s: refusals = %v, want exactly one %q + %q", c.name, got, prefix, c.part)
			}
			for _, old := range []string{
				"mana restriction on a PayLife source: gorge plans it last-resort",
				"mana restriction on a source whose cost goes beyond {T}",
			} {
				if strings.TrimSpace(got[0]) == old {
					t.Fatalf("%s: the old two-class reason %q is back", c.name, old)
				}
			}
		})
	}
}

// TestRestrictedLifeSourcesLower: a sole restricted {T}, Pay N life ability of
// one fixed colour (Gnarlroot Trapper's Elf-only {G}, Untaidake's
// legendary-only {C}{C}) or of any one colour at one or two mana (Thieving
// Varmint's two mana, "spells you don't own") lowers: the kernel plans it as a
// last-resort unit (dirac mana_restrictedcost.go); no refusal names its cost.
func TestRestrictedLifeSourcesLower(t *testing.T) {
	for _, name := range []string{"Gnarlroot Trapper", "Untaidake, the Cloud Keeper", "Thieving Varmint"} {
		for _, rf := range lowerCorpusCard(t, name).Refusals {
			t.Errorf("%s: refused: %s", name, rf.Reason)
		}
	}
}

// TestRestrictedSacSelfSourceLowers: a sole restricted "{T}, Sacrifice this:
// Add" ability (Jeweled Lotus, any one colour x3) lowers; the kernel plans it
// as a last-resort sacrificing unit (dirac mana_restrictedsource.go).
func TestRestrictedSacSelfSourceLowers(t *testing.T) {
	for _, rf := range lowerCorpusCard(t, "Jeweled Lotus").Refusals {
		t.Errorf("Jeweled Lotus: refused: %s", rf.Reason)
	}
}

// TestRestrictedTapXSourceLowers: a sole restricted "Tap an untapped creature
// you control: Add" ability (Grand Architect, {C}{C} for artifact spells and
// artifact abilities) lowers; kop.RestrictedTapXOneShape names its shape and
// the kernel prices it in dirac mana_tapanother.go.
func TestRestrictedTapXSourceLowers(t *testing.T) {
	for _, rf := range lowerCorpusCard(t, "Grand Architect").Refusals {
		t.Errorf("Grand Architect: refused: %s", rf.Reason)
	}
}

// TestRestrictedLifePairRefusesBesideItsSecondAbility pins the narrowed
// refusal of a restricted "{T}, Pay N life" ability that is not the card's
// only mana ability (Great Hall of the Biblioplex, Voldaren Estate): the cost
// part is no longer the reason named, the pair is
// (refuseMultiMana; the restricted-life-pair follow-up ticket).
func TestRestrictedLifePairRefusesBesideItsSecondAbility(t *testing.T) {
	for _, name := range []string{"Great Hall of the Biblioplex", "Voldaren Estate"} {
		lc := lowerCorpusCard(t, name)
		var got []string
		for _, rf := range lc.Refusals {
			got = append(got, rf.Reason)
		}
		const want = "2 mana abilities: mana ability 1 (cost none, produces WUBRG) is not modelled beside the first: a cost beyond {T} the kernel's mana walk cannot activate"
		if len(got) != 1 || got[0] != want {
			t.Fatalf("%s: refusals = %v, want exactly %q", name, got, want)
		}
	}
}
