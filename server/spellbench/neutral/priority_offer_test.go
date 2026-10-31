package neutral

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

const fixturePlains = "Name:Plains\nTypes:Basic Land Plains\nA:AB$ Mana | Cost$ T | Produced$ W | SpellDescription$ Fixture mana.\n"

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

// TestAutopayCensusHoldsOnlyOfferedCasts: the planner declines a multikicker
// card, and only tapping by hand could cast it, which an engine_autopay host
// does not offer. It must leave the census, or the bot reads it as a stranded
// cast, answers with the manual policy and passes with a planned spell in hand.
func TestAutopayCensusHoldsOnlyOfferedCasts(t *testing.T) {
	s := fixtureSession(t, fixturePlains,
		"Name:Fixture Note\nManaCost:W\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1 | SpellDescription$ Fixture draw.\n",
		"Name:Fixture Rider\nManaCost:W\nTypes:Creature Human\nPT:1/1\nK:Multikicker:1 W\n")
	s.SetAutoPay(true)
	hand := func(id, name string) protocol.ObjectRecord {
		r := obsRef(id, name, "p0")
		r.Zone = "hand"
		return protocol.ObjectRecord{ObjectRef: r}
	}
	obs := seatPriority([]protocol.ObjectRecord{obsPerm("o-plains", "Plains", "p0", untapped(), "land")})
	obs.Players[0].Hand = []protocol.ObjectRecord{hand("o-note", "Fixture Note"), hand("o-rider", "Fixture Rider")}
	p := translated(t, s, priorityDecision(obs, protocol.CastSpell(obs.Players[0].Hand[0].ObjectRef, "normal")))
	if !p.Decision.PotentialCastsKnown || !slices.Contains(p.Decision.PotentialCasts, s.B.IDs.Of("o-note")) {
		t.Fatalf("the offered cast is not in the census: %v", p.Decision.PotentialCasts)
	}
	if slices.Contains(p.Decision.PotentialCasts, s.B.IDs.Of("o-rider")) {
		t.Fatalf("the unoffered cast is in the census: %v", p.Decision.PotentialCasts)
	}
}
