package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestSetsKeywordGrantsLower: the Continuous-static keyword grants the
// CMM/MH3/MH2/CM2 tracking lowered, in lockstep, one subtest per grant.
//
//   - horsemanship: Sun Quan, Lord of Wu's "creatures you control have
//     horsemanship" and Herald of Hoofbeats' "other Knights you control have
//     horsemanship" (CR 702.31b) are Static.Evasion EvHorsemanship, read by
//     evasionGranted beside the printed bit (The Dilu Horse rides along: its
//     printed horsemanship plays, its commander grant matches nothing in a
//     duel). The probe counts attackers declared that print no
//     horsemanship while their controller has a granting permanent out, and
//     fails when none is reached.
//   - afterlife: Indebted Spirit's bestowed "enchanted creature has afterlife
//     1" (CR 702.135a) is a latent dies trigger granted through the static
//     (scriptfacts.GrantedKeywordTrigger's body). The probe counts creatures
//     that die while an Indebted Spirit is attached to them, and fails when
//     none does.
//   - battlecry: Reckless Pyrosurfer's landfall Pump KW$ Battle cry (CR
//     702.91a) and annihilator: Eldrazi Conscription's enchanted AddKeyword$
//     Annihilator:2 (CR 702.86a) are
//     latent Attacks Self triggers granted from gorge's synthesizer
//     (scriptfacts.GrantedKeywordTrigger). The probe counts the granted
//     triggers gorge pushes (GrantTriggerPush with the __kwBattleCryGranted /
//     __kwAnnihilatorGranted: payload), and fails when too few are reached.
func TestSetsKeywordGrantsLower(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		decks  [2][]cardset.Entry
		seeds  uint64
		horse  int // the least granted-horsemanship attackers the games must declare
		deaths int // the least deaths of a creature an Indebted Spirit enchants
		cry    int // the least granted battle cry triggers pushed
		ann    int // the least granted annihilator triggers pushed
	}{
		{"horsemanship", setsKwHorseDecks, 120, 20, 0, 0, 0},
		{"afterlife", setsKwAfterlifeDecks, 120, 0, 5, 0, 0},
		{"battlecry", setsKwBattleCryDecks, 120, 0, 0, 10, 0},
		{"annihilator", setsKwAnnihilatorDecks, 120, 0, 0, 0, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var st setsKwStats
			pol := func(seed uint64) Policy {
				return setsKwProbe(castFirstPolicy(seed), &st)
			}
			if div := lockstepRunPolicy(t, "setskeywords", tc.decks, tc.seeds, true, nil, nil, pol); div != 0 {
				return // each divergence is already a test error
			}
			t.Logf("%s: %+v", tc.name, st)
			if st.horse < tc.horse || st.deaths < tc.deaths || st.cry < tc.cry || st.ann < tc.ann {
				t.Fatalf("%s: %+v over %d seeds, want horse %d deaths %d cry %d ann %d: the pool does not exercise it",
					tc.name, st, tc.seeds, tc.horse, tc.deaths, tc.cry, tc.ann)
			}
		})
	}
}

// setsKwStats is what setsKwProbe counts.
type setsKwStats struct{ horse, deaths, cry, ann int }

// setsKwProbe wraps base and counts, when gorge leads, the attackers declared
// under a horsemanship grant and the deaths of Indebted Spirit's bearers.
func setsKwProbe(base Policy, st *setsKwStats) Policy {
	seen := 0
	bearers := map[state.ObjID]bool{}
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil {
			return a
		}
		g := c.G.E.G
		granter := map[state.PlayerID]bool{}
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Zone != state.ZBattlefield || o.Face() == nil {
				continue
			}
			switch o.Face().Name {
			case "Sun Quan, Lord of Wu", "Herald of Hoofbeats":
				granter[o.Controller] = true
			}
		}
		evs := c.G.E.L.Events
		for ; seen < len(evs); seen++ {
			switch e := evs[seen]; {
			case e.Kind == events.DeclareAttackers:
				for _, id := range e.IDs {
					o := g.Obj(id)
					if o == nil || o.Face() == nil || !granter[o.Controller] {
						continue
					}
					if n := o.Face().Name; n != "Herald of Hoofbeats" && n != "The Dilu Horse" {
						st.horse++
					}
				}
			case e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard && bearers[e.Obj]:
				st.deaths++
			case e.Kind == events.GrantTriggerPush && e.Counter == "__kwBattleCryGranted":
				st.cry++
			case e.Kind == events.GrantTriggerPush && strings.HasPrefix(e.Counter, "__kwAnnihilatorGranted:"):
				st.ann++
			}
		}
		clear(bearers)
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Zone == state.ZBattlefield && o.AttachedTo != 0 && o.Face() != nil && o.Face().Name == "Indebted Spirit" {
				bearers[o.AttachedTo] = true
			}
		}
		return a
	})
}

// setsKwHorseDecks: the two horsemanship granters, Knights and Bears to
// attack under them.
var setsKwHorseDecks = [2][]cardset.Entry{
	{
		{Name: "Sun Quan, Lord of Wu", Count: 6}, {Name: "Herald of Hoofbeats", Count: 5}, {Name: "Silver Knight", Count: 4},
		{Name: "The Dilu Horse", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Sun Quan, Lord of Wu", Count: 4}, {Name: "Herald of Hoofbeats", Count: 4}, {Name: "White Knight", Count: 4},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Island", Count: 10}, {Name: "Plains", Count: 5}, {Name: "Forest", Count: 5},
	},
}

// setsKwAfterlifeDecks: Indebted Spirit to bestow, Bears to wear it and
// Bolts to kill them.
var setsKwAfterlifeDecks = [2][]cardset.Entry{
	{
		{Name: "Indebted Spirit", Count: 10}, {Name: "Grizzly Bears", Count: 6}, {Name: "Lightning Bolt", Count: 6},
		{Name: "Plains", Count: 9}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Indebted Spirit", Count: 6}, {Name: "Grizzly Bears", Count: 8}, {Name: "Lightning Bolt", Count: 8},
		{Name: "Plains", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	},
}

// setsKwBattleCryDecks: Reckless Pyrosurfer gains battle cry on each land
// drop; Bears attack beside it.
var setsKwBattleCryDecks = [2][]cardset.Entry{
	{
		{Name: "Reckless Pyrosurfer", Count: 12}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Reckless Pyrosurfer", Count: 8}, {Name: "Grizzly Bears", Count: 10}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
}

// setsKwAnnihilatorDecks: Eldrazi Conscription to enchant the Bears that
// then attack with annihilator 2. (Nazgûl Battle-Mace's grant is not lowered:
// its static grants two abilities, a trigger beside the annihilator, and a
// kop.Static carries one.)
var setsKwAnnihilatorDecks = [2][]cardset.Entry{
	{
		{Name: "Eldrazi Conscription", Count: 10}, {Name: "Grizzly Bears", Count: 10},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Eldrazi Conscription", Count: 8}, {Name: "Grizzly Bears", Count: 12},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10},
	},
}
