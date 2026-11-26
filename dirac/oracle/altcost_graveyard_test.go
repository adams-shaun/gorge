package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The graveyard self alternative-cost lockstep: a card's own S:Mode$
// AlternativeCost | ValidSA$ Spell.Self | EffectZone$ Graveyard static (the
// ticket T18b-B four: Raffine's Guidance, Scourge of Nel Toth, Squee, Dubious
// Monarch, Worldheart Phoenix) is offered from the graveyard at the static's
// cost (gorge legal_walk_grave_altcost.go, the AltCostIndex 1 cast) and is
// NOT offered from the hand — the static names the graveyard explicitly
// (engine/rules statics_altcost.go graveyardSelfAltCost; gorge pins this in
// TestGraveyardAltCostHandStillUsesPrintedCost). The expected answer at every
// decision is gorge's. A creature deck keeps an enchant target and the
// sacrifice fodder on the battlefield; mana producers keep pools floating at
// priority so the offer appears (the planner-surface rule of the altcost
// lockstep, altcost_test.go).
var kaltgravDecks = [2][]cardset.Entry{
	{
		{Name: "Raffine's Guidance", Count: 4}, {Name: "Scourge of Nel Toth", Count: 3},
		{Name: "Squee, Dubious Monarch", Count: 3}, {Name: "Worldheart Phoenix", Count: 2},
		{Name: "Baleful Mastery", Count: 2}, {Name: "Fervent Mastery", Count: 1}, {Name: "Ingenious Mastery", Count: 1},
		{Name: "Llanowar Elves", Count: 2}, {Name: "Seething Song", Count: 1},
		{Name: "Dark Ritual", Count: 3}, {Name: "Channel the Suns", Count: 3},
		{Name: "Plains", Count: 3}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Mind Rot", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Goblin Piker", Count: 4}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Raffine's Guidance", Count: 2}, {Name: "Scourge of Nel Toth", Count: 2},
		{Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 4}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4},
	},
}

// kaltgravNames are the four cards whose own static names the graveyard.
var kaltgravNames = map[string]bool{
	"Raffine's Guidance": true, "Scourge of Nel Toth": true,
	"Squee, Dubious Monarch": true, "Worldheart Phoenix": true,
}

// kaltgravProbe tallies, at the decisions gorge leads, the casts of the four
// cards offered from the graveyard (the alternative cast) and from the hand
// (never an alternative: mode 0 only). The zone each option's source sits in
// is the test's precondition: a lockstep that never offered a graveyard cast
// would pass vacuously.
type kaltgravProbe struct {
	mu            sync.Mutex
	grave         map[string]int
	hand, handAlt int
	// priority decisions with one of the four in a graveyard, and of them
	// with a ritual castable
	heldAt, ritual int
	mastery        map[string]int // the Masteries' hand alternative casts offered, by card
}

// kaltgravRituals float mana at priority (the oracle surface offers no mana
// ability, so a pool only floats after a spell adds it): Dark Ritual's {B}{B}{B}
// pays Scourge of Nel Toth's {B}{B}, Channel the Suns' {W}{U}{B}{R}{G} pays
// Raffine's Guidance, Squee, Worldheart Phoenix and Ingenious Mastery's
// {2}{U}, Seething Song's {R}{R}{R}{R}{R} pays Fervent Mastery's {2}{R}{R}.
var kaltgravRituals = map[string]bool{"Dark Ritual": true, "Channel the Suns": true, "Seething Song": true}

// kaltgravMasteries are the Count$AltCost.1.0 cards (SVar:AltCostPaid) whose
// hand alternative the lockstep takes when offered: the ExAltCost read at the
// spell's own resolution is the evidence for lowering them.
var kaltgravMasteries = map[string]bool{"Baleful Mastery": true, "Fervent Mastery": true, "Ingenious Mastery": true}

// policy is castFirstPolicy, except that while one of the four sits in a
// graveyard at a priority decision it casts a ritual first (so a pool floats)
// and then takes the graveyard cast when offered. Deaths in combat and Mind
// Rot's discards (the opposing deck) put the cards there.
func (p *kaltgravProbe) policy(seed uint64) Policy {
	inner := castFirstPolicy(seed)
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.D.Kind != DPriority {
			return inner.Pick(c)
		}
		g := c.G.E.G
		held := false
		for seat := state.PlayerID(0); seat < 2; seat++ {
			for _, id := range g.Zone(state.ZGraveyard, seat) {
				if o := g.Obj(id); o != nil && o.Face() != nil && kaltgravNames[o.Face().Name] {
					held = true
				}
			}
		}
		var casts, rituals []Canon
		p.mu.Lock()
		for _, a := range c.D.Options {
			ob := g.Obj(state.ObjID(a.Src()) + 1) // canon source refs are the instance index; gorge ids start at 1
			switch {
			case a.Kind() == ActCast && ob != nil && ob.Face() != nil && kaltgravRituals[ob.Face().Name]:
				rituals = append(rituals, a)
			case a.Kind() == ActCast && ob != nil && ob.Face() != nil && kaltgravMasteries[ob.Face().Name] && ob.Zone == state.ZHand:
				// the hand alternative ({1}{B}, Count$AltCost.1.0 on its branch,
				// the ExAltCost read at the spell's own resolution): taken when
				// offered, and a ritual is cast first while one is in hand
				held = true
				if a.Mode() != 0 {
					p.mastery[ob.Face().Name]++
					casts = append(casts, a)
				}
			case a.Kind() != ActCast || ob == nil || ob.Face() == nil || !kaltgravNames[ob.Face().Name]:
			case ob.Zone == state.ZGraveyard:
				p.grave[ob.Face().Name]++
				casts = append(casts, a)
			case ob.Zone == state.ZHand:
				p.hand++
				if a.Mode() != 0 {
					p.handAlt++
				}
			}
		}
		p.mu.Unlock()
		if held {
			p.mu.Lock()
			p.heldAt++
			if len(rituals) > 0 {
				p.ritual++
			}
			p.mu.Unlock()
		}
		pick := func(opts []Canon) Canon {
			d := *c.D
			d.Options = opts
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		switch {
		case len(casts) > 0:
			return pick(casts)
		case held && len(rituals) > 0:
			return pick(rituals)
		}
		return inner.Pick(c)
	})
}

func TestAltCostGraveyardSelfLockstep(t *testing.T) {
	t.Parallel()
	reg, set, gd := lockstepPool(t, "altgrav", kaltgravDecks)
	probe := &kaltgravProbe{grave: map[string]int{}, mastery: map[string]int{}}
	var clean, casts int
	var div []lockstepDivergence
	seedChunks(t, 1, 1000, lockstepChunk, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		c, n, d := lockstepSeedsMode(t, reg, set, "altgrav", kaltgravDecks, gd, lo, hi, lockstepQuiet, nil, nil, probe.policy)
		mu.Lock()
		clean, casts, div = clean+c, casts+n, append(div, d...)
		mu.Unlock()
	})
	t.Logf("altgrav: %d clean, %d casts; graveyard casts offered %v, hand casts %d (alternative %d); priority decisions with one in a graveyard %d, with a ritual castable %d; Mastery alternative casts offered %v", clean, casts, probe.grave, probe.hand, probe.handAlt, probe.heldAt, probe.ritual, probe.mastery)
	if len(div) > 0 {
		t.Error(divergenceReport("altgrav", 1000, div))
	}
	for _, n := range []string{"Raffine's Guidance", "Scourge of Nel Toth", "Squee, Dubious Monarch", "Worldheart Phoenix"} {
		if probe.grave[n] == 0 {
			t.Errorf("%s: never offered from the graveyard in 1000 seeds (precondition)", n)
		}
	}
	for n := range kaltgravMasteries {
		if probe.mastery[n] == 0 {
			t.Errorf("%s: its hand alternative was never offered in 1000 seeds (precondition of the Count$AltCost branch read)", n)
		}
	}
	if probe.hand == 0 || probe.handAlt != 0 {
		t.Errorf("hand casts of the four %d, of them alternative %d: want some, none alternative", probe.hand, probe.handAlt)
	}
}
