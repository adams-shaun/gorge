package kgen

import (
	"strings"
	"testing"
)

// spawnerReferentTargetCards name the immediate-trigger bodies whose
// ValidTgts$ reads ControlledBy Spawner>TriggeredTarget: the artifact,
// enchantment, creature or planeswalker is controlled by the player the
// spawning combat-damage trigger hit, at any seat count.
var spawnerReferentTargetCards = []string{
	"A-Dokuchi Silencer",
	"Incinerator of the Guilty",
	"Kappa Tech-Wrecker",
	"Leatherhead, Swamp Stalker",
}

// spawnerReferentDefendingCards name the Attacks-spawned immediate triggers
// whose ValidTgts$ reads ControlledBy Spawner>TriggeredDefendingPlayer: the
// permanent is controlled by the player the attack's attacker attacked.
var spawnerReferentDefendingCards = []string{
	"Ace, Fearless Rebel",
	"Auron, Venerated Guardian",
}

// TestSpawnerReferentsAnySeats: the seven Spawner> referent bodies lower
// with TwoSeat false, and their rewritten filters name the spawning
// trigger's seat (kgenTrigPlayerCtrl for the damaged player, the
// TriggeredDefendingPlayer role for the attacked player) at any seat count.
func TestSpawnerReferentsAnySeats(t *testing.T) {
	names := append(append([]string{}, spawnerReferentTargetCards...), spawnerReferentDefendingCards...)
	names = append(names, "Biting-Palm Ninja")
	byName := map[string]*lCard{}
	for _, c := range lowerNamed(t, names) {
		byName[c.Name] = c
	}
	for _, name := range append(append([]string{}, spawnerReferentTargetCards...), spawnerReferentDefendingCards...) {
		c := byName[name]
		if c == nil {
			t.Fatalf("%s: not in the corpus", name)
		}
		if len(c.Refusals) != 0 {
			t.Errorf("%s: refused: %v", name, c.Refusals)
			continue
		}
		if c.TwoSeat {
			t.Errorf("%s: two-seat only: %v", name, c.TwoSeatWhy)
		}
		for _, why := range c.TwoSeatWhy {
			if strings.Contains(why, "Spawner>") {
				t.Errorf("%s: still marks the spawner referent two-seat: %s", name, why)
			}
		}
		lat := reflexiveLatent(c)
		if lat == nil {
			t.Errorf("%s: no latent ReflexiveTrigger ability", name)
			continue
		}
		want := func(cl lClause) bool { return cl.CmpStat == cmpTrigPlayerCtrl }
		if strings.HasPrefix(name, "Ace") || strings.HasPrefix(name, "Auron") {
			want = func(cl lClause) bool { return cl.Ctrl == whoDefending }
		}
		if !latentFilterMatches(lat, want) {
			t.Errorf("%s: the reflexive target filter does not name the spawner's seat", name)
		}
	}
	// Biting-Palm Ninja reads the damaged player through Defined$ and
	// DefinedPlayer$ Spawner>TriggeredTarget, not a filter.
	c := byName["Biting-Palm Ninja"]
	if c == nil {
		t.Fatal("Biting-Palm Ninja: not in the corpus")
	}
	if lat := reflexiveLatent(c); lat == nil {
		t.Fatal("Biting-Palm Ninja: no latent ReflexiveTrigger ability")
	} else if !latentUsesRef(lat, rTriggeredPlayer) || latentUsesRef(lat, rEachOpp) {
		t.Errorf("Biting-Palm Ninja: the reflexive body does not read the damaged seat alone")
	}
}

func reflexiveLatent(c *lCard) *lAbility {
	for _, a := range c.Abilities {
		if a.Latent && a.Name == "ReflexiveTrigger" {
			return a
		}
	}
	return nil
}

func latentFilterMatches(a *lAbility, want func(lClause) bool) bool {
	for _, t := range a.Targets {
		if filterMatches(t.F, want) {
			return true
		}
	}
	for _, in := range a.Prog {
		if filterMatches(in.F, want) || filterMatches(in.F2, want) {
			return true
		}
	}
	return false
}

func filterMatches(f *lFilter, want func(lClause) bool) bool {
	if f == nil {
		return false
	}
	for _, cl := range f.Clauses {
		if want(cl) {
			return true
		}
	}
	return false
}

func latentUsesRef(a *lAbility, kind refKind) bool {
	for _, in := range a.Prog {
		if in.Obj.Kind == kind || in.W.Kind == kind || in.Obj2.Kind == kind {
			return true
		}
	}
	return false
}
