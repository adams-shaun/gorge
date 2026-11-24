package kgen

import (
	"testing"
)

// The stay-tapped mana lands print a plain {T} ability beside a {T} ability
// whose DBStayTapped rider keeps the source tapped through the controller's
// next untap step (Cinder Marsh's A1: Add {C}; A2: Add {B} or {R}, then the
// Effect Duration$ UntilTheEndOfYourNextUntap rider). The pair is Grove of
// the Burnwillows' shape with a different bounded rider: both abilities lower
// as mana abilities, the second carrying riderStayTapped and its lowered
// [SkipUntap Self] program.
func TestStayTappedRiderLowersTheManaLandPair(t *testing.T) {
	want := map[string]string{
		"Cinder Marsh":          "BR",
		"Cloudcrest Lake":       "WU",
		"Lantern-Lit Graveyard": "BR",
		"Mogg Hollows":          "RG",
		"Pinecrest Ridge":       "RG",
		"Rootwater Depths":      "UB",
		"Thalakos Lowlands":     "WU",
		"Tranquil Garden":       "GW",
		"Vec Townships":         "GW",
		"Waterveil Cavern":      "UB",
	}
	names := make([]string, 0, len(want))
	for n := range want {
		names = append(names, n)
	}
	byName := map[string]*lCard{}
	for _, c := range lowerNamedPool(t, names).cards {
		byName[c.Name] = c
	}
	for _, name := range names {
		c := byName[name]
		if c == nil {
			t.Errorf("%s: not lowered", name)
			continue
		}
		if len(c.Refusals) > 0 {
			t.Errorf("%s refused: %v", name, c.Refusals)
			continue
		}
		var plain, rider *lAbility
		for _, a := range c.Abilities {
			if a.Kind != akMana || a.Mana == nil || a.Latent {
				continue
			}
			switch a.Mana.Rider {
			case riderNone:
				plain = a
			case riderStayTapped:
				rider = a
			default:
				t.Errorf("%s: mana ability carries rider %d", name, a.Mana.Rider)
			}
		}
		if plain == nil || rider == nil {
			t.Errorf("%s: plain %v, stay-tapped rider %v; want both mana abilities", name, plain != nil, rider != nil)
			continue
		}
		if plain.Mana.Colors != "C" {
			t.Errorf("%s: plain ability produces %q, want C", name, plain.Mana.Colors)
		}
		if rider.Mana.Colors != want[name] {
			t.Errorf("%s: rider ability produces %q, want %q", name, rider.Mana.Colors, want[name])
		}
		if !riderTapMana(rider) {
			t.Errorf("%s: the stay-tapped ability is not the second-ability shape refuseMultiMana admits", name)
		}
		if len(rider.Prog) != 1 || rider.Prog[0].Op != "SkipUntap" || rider.Prog[0].Obj.Kind != rSelf {
			t.Errorf("%s: rider program %v, want the single [SkipUntap Self]", name, rider.Prog)
		}
	}
}
