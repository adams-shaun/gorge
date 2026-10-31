package neutral

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

const fixturePlains = "Name:Plains\nTypes:Basic Land Plains\nA:AB$ Mana | Cost$ T | Produced$ W | SpellDescription$ Add {W}.\n"

// seatPriority is an own-turn main phase where p0 holds priority.
func seatPriority(p0 []protocol.ObjectRecord) protocol.Observation {
	obs := testObservation("p0", "precombat_main", p0, nil)
	seat := "p0"
	obs.ActiveSeat, obs.PrioritySeat = &seat, &seat
	obs.Players[0].Life, obs.Players[1].Life = 20, 20
	return obs
}

func priorityDecision(obs protocol.Observation, cands ...protocol.Semantic) protocol.SeatDecision {
	sd := protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepCount: 1},
		Context: protocol.Context{Kind: "priority"}, Observation: obs}
	sd.Candidates = append(sd.Candidates, protocol.Candidate{Semantic: protocol.Pass()})
	for _, c := range cands {
		sd.Candidates = append(sd.Candidates, protocol.Candidate{Semantic: c})
	}
	return sd
}

// TestSuspendActionIsARankedCast: gorge offers suspending a card as a cast of
// mode suspend, and the wire poses it as a special action. Left untranslated,
// the bot never suspends.
func TestSuspendActionIsARankedCast(t *testing.T) {
	s := fixtureSession(t, "Name:Fixture Comet\nManaCost:4 R\nTypes:Sorcery\nK:Suspend:3:R\n")
	src := obsRef("o-comet", "Fixture Comet", "p0")
	src.Zone = "hand"
	obs := seatPriority(nil)
	obs.Players[0].Hand = []protocol.ObjectRecord{{ObjectRef: src}}
	p := translated(t, s, priorityDecision(obs, protocol.SpecialAction(src, "suspend")))
	if op := p.Ops[1]; op.Op != "choose" {
		t.Fatalf("suspend candidate translates to %+v, want a choose", op)
	} else if o := p.Decision.Options[op.Option]; o.Kind != "cast" || o.Mode != "suspend" || o.Obj != s.B.IDs.Of("o-comet") {
		t.Fatalf("suspend option %+v, want a cast of mode suspend", o)
	}
}

// TestGrantedAbilityComesFromTheShadow: the wire names a granted ability only
// by its place in the object's inventory, past the printed ones. The rebuilt
// board holds the grant, so the shadow's offer names the option. A fixture
// Equipment grants its bearer a sacrifice ability.
func TestGrantedAbilityComesFromTheShadow(t *testing.T) {
	s := fixtureSession(t, fixturePlains,
		"Name:Fixture Bear\nManaCost:1 W\nTypes:Creature Bear\nPT:2/2\n",
		"Name:Fixture Pod\nManaCost:1\nTypes:Artifact Equipment\n"+
			"S:Mode$ Continuous | Affected$ Creature.EquippedBy | AddAbility$ Shot | Description$ Fixture grant.\n"+
			"SVar:Shot:AB$ DealDamage | Cost$ Sac<1/CARDNAME> | ValidTgts$ Any | NumDmg$ 1 | SpellDescription$ Fixture shot.\n")
	s.SetAutoPay(true)
	bear := obsRef("o-bear", "Fixture Bear", "p0")
	on := protocol.ObjectTarget(bear)
	obs := seatPriority([]protocol.ObjectRecord{
		obsPerm("o-bear", "Fixture Bear", "p0", untapped(), "creature"),
		obsPerm("o-pod", "Fixture Pod", "p0", &protocol.Permanent{AttachedTo: &on}, "artifact"),
	})
	p := translated(t, s, priorityDecision(obs, protocol.ActivateAbility(bear, 0)))
	op := p.Ops[1]
	if op.Op != "choose" {
		t.Fatalf("granted ability translates to %+v, want a choose", op)
	}
	if o := p.Decision.Options[op.Option]; o.Obj != s.B.IDs.Of("o-bear") || !strings.Contains(o.Label, "Fixture shot") {
		t.Fatalf("granted option %+v, want the Equipment's ability on the Bear", o)
	}
}

// TestLoyaltyFactsComeFromTheShadow: the bot ranks a loyalty ability by facts
// the engine works out for the option, so the option comes from the rebuilt
// board's offer. The printed face has only the cost string.
func TestLoyaltyFactsComeFromTheShadow(t *testing.T) {
	s := fixtureSession(t, fixturePlains,
		"Name:Fixture Walker\nManaCost:2 W\nTypes:Legendary Planeswalker Fixture\nLoyalty:3\n"+
			"A:AB$ Draw | Cost$ AddCounter<1/LOYALTY> | Planeswalker$ True | NumCards$ 1 | SpellDescription$ Fixture draw.\n")
	s.SetAutoPay(true)
	walker := obsRef("o-walker", "Fixture Walker", "p0")
	obs := seatPriority([]protocol.ObjectRecord{
		obsPerm("o-walker", "Fixture Walker", "p0", &protocol.Permanent{Counters: map[string]uint32{"loyalty": 3}}, "planeswalker"),
	})
	p := translated(t, s, priorityDecision(obs, protocol.ActivateAbility(walker, 0)))
	op := p.Ops[1]
	if op.Op != "choose" {
		t.Fatalf("loyalty ability translates to %+v, want a choose", op)
	}
	if l := p.Decision.Options[op.Option].Loyalty; l == nil || l.Delta != 1 {
		t.Fatalf("loyalty facts %+v, want a delta of 1", l)
	}
}
