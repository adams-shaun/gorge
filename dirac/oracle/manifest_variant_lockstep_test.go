package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Manifest's variant shapes in lockstep (dirac/kgen/lower_manifest.go
// manifestValidLibrary / manifestChoices; kernel face_manifest.go): Thieving
// Amalgam manifests the top card of each opponent's library under its
// controller at that opponent's upkeep (Defined$ ValidLibrary
// Card.TopLibrary+ActivePlayerCtrl), and Scroll of Fate's activation asks the
// controller for a hand card to manifest (Choices$ Card.YouCtrl, ChoiceZone$
// Hand: gorge's face_down_choice ask). Moxen make the seven-mana Amalgam
// castable early; the policy casts Amalgam and Scroll and activates the
// Scroll whenever offered, else random.
func kmanifestVariantDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Thieving Amalgam", Count: 4},
		{Name: "Scroll of Fate", Count: 4},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Mox Emerald", Count: 2}, {Name: "Mox Pearl", Count: 2},
		{Name: "Mox Jet", Count: 2}, {Name: "Mox Ruby", Count: 2},
		{Name: "Mox Sapphire", Count: 2},
		{Name: "Swamp", Count: 14},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// kmanifestVariantCounts: Amalgam top-card manifests (a face-down permanent
// owned by one seat and controlled by the other) and Scroll activations that
// reached a face_down_choice ask.
type kmanifestVariantCounts struct{ stolen, scrollAsks int }

func kmanifestVariantPolicy(counts *kmanifestVariantCounts) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		seen := map[state.ObjID]bool{}
		asked := map[int]bool{}
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				g := c.G.E.G
				for p := state.PlayerID(0); p < 2; p++ {
					for _, id := range g.Zone(state.ZBattlefield, p) {
						if o := g.Obj(id); o != nil && o.FaceDown && o.Manifested && o.Owner != o.Controller && !seen[id] {
							seen[id] = true
							counts.stolen++
						}
					}
				}
				for _, a := range c.D.Options {
					if o, ok := c.G.GorgeOption(a); ok && o.Kind == "face_down_choice" && !asked[c.Seq] {
						asked[c.Seq] = true
						counts.scrollAsks++
					}
				}
			}
			var pref []Canon
			for _, a := range c.D.Options {
				n := optionFaceName(c, a)
				switch {
				case a.Kind() == ActCast && (isMoxName(n) || n == "Thieving Amalgam" || n == "Scroll of Fate"):
					pref = append(pref, a)
				case a.Kind() == ActActivate && n == "Scroll of Fate":
					pref = append(pref, a)
				}
			}
			if len(pref) == 0 {
				return rnd.Pick(c)
			}
			d := *c.D
			d.Options = pref
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		})
	}
}

func TestManifestVariantLockstep(t *testing.T) {
	t.Parallel()
	var counts kmanifestVariantCounts
	lockstepRunPolicy(t, "manifestvariants", kmanifestVariantDecks(), 120, true, nil, nil, kmanifestVariantPolicy(&counts))
	if counts.stolen == 0 {
		t.Fatal("Thieving Amalgam never manifested an opponent's top card; the lockstep proves nothing")
	}
	if counts.scrollAsks == 0 {
		t.Fatal("Scroll of Fate's hand pick was never asked; the lockstep proves nothing")
	}
	t.Logf("Amalgam manifests %d, Scroll asks %d", counts.stolen, counts.scrollAsks)
}
