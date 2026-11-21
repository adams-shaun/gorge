package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Throwaway diagnostic: does gorge's priority walk offer the plain cast of
// Bedlam Reveler whose {4}{R}{R} (graveyard reduction 2) exceeds the 5 mana
// of {Plains, Island, Mountain, Mountain} unless Untaidake's
// legendary-restricted {C}{C} is counted?
func TestZZBedlamUntaidakeOffer(t *testing.T) {
	for _, withUntaidake := range []bool{false, true} {
		reg := testutil.CorpusRegistry(t)
		extras := []*cards.Card{lookup(t, reg, "Bedlam Reveler"), lookup(t, reg, "Devastate"),
			lookup(t, reg, "Wanderer's Intervention"), lookup(t, reg, "Island"), lookup(t, reg, "Forest"), lookup(t, reg, "Shadowblood Ridge")}
		if withUntaidake {
			extras = append(extras, lookup(t, reg, "Untaidake, the Cloud Keeper"))
		}
		e := quietBaseWith(t, reg, extras) // one untapped Plains on the battlefield
		rev := addHand(t, e, 0, lookup(t, reg, "Bedlam Reveler"))
		addZone(t, e, 0, lookup(t, reg, "Devastate"), state.ZGraveyard)
		addZone(t, e, 0, lookup(t, reg, "Wanderer's Intervention"), state.ZGraveyard)
		addZone(t, e, 0, lookup(t, reg, "Island"), state.ZBattlefield)
		addZone(t, e, 0, lookup(t, reg, "Forest"), state.ZBattlefield)
		addZone(t, e, 0, lookup(t, reg, "Shadowblood Ridge"), state.ZBattlefield)
		addZone(t, e, 0, lookup(t, reg, "Mountain"), state.ZBattlefield)
		if withUntaidake {
			addZone(t, e, 0, lookup(t, reg, "Untaidake, the Cloud Keeper"), state.ZBattlefield)
		}
		_ = rev
		e.askPriority(0)
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("pending not priority: %+v", d)
		}
		opts := d.Options
		offered := false
		for _, o := range opts {
			if o.Kind == "cast" && o.Obj == rev {
				offered = true
			}
		}
		t.Logf("withUntaidake=%v: cast offered=%v options=%v", withUntaidake, offered, optKinds(opts))
		for _, pp := range e.PotentialPaymentPlans(0) {
			nm := ""
			if o := e.G.Obj(pp.Action.Obj); o != nil && o.Face() != nil {
				nm = o.Face().Name
			}
			if nm == "Bedlam Reveler" || nm == "Avenging Huntbonder" {
				t.Logf("  plan %s(%s): reason=%q script=%d hasPlan=%v restricted=%v",
					pp.Action.Kind, nm, pp.Reason, len(pp.Script), pp.Plan != nil, e.potentialRestricted)
				for si, st := range pp.Script {
					t.Logf("   script[%d] picks=%v", si, st.Picks)
				}
			}
		}
		if withUntaidake {
			t.Logf("MatchesSpellFrom Legendary = %v, Card.Legendary = %v",
				asPayer(e).MatchesSpellFrom("Legendary", rev, 0, 0), asPayer(e).MatchesSpellFrom("Card.Legendary", rev, 0, 0))
		}
		if withUntaidake && !offered {
			t.Fatal("with Untaidake the cast was not offered")
		}
		if !withUntaidake && offered {
			t.Fatal("without Untaidake the cast was offered")
		}
	}
}
