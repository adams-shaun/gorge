package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
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
//   - undying / persist: Mikaeus, the Unhallowed's AddKeyword$ Undying (CR 702.93a)
//     and Rattleblaze Scarecrow's conditional AddKeyword$ Persist (CR 702.79a) are latent
//     look-back dies triggers granted from the same synthesizer. The probe
//     counts the granted triggers gorge pushes (__kwUndyingGranted /
//     __kwPersistGranted), and fails when too few of either are reached.
//
// The Pump-granted persist and undying (Cauldron of Souls, Cauldron Haze,
// Undying Evil) beside Mikaeus's static undying are
// TestSetsKeywordPumpReturnLowers'.
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
		und    int // the least granted undying triggers pushed
		per    int // the least granted persist triggers pushed
	}{
		{"horsemanship", setsKwHorseDecks, 120, 20, 0, 0, 0, 0, 0},
		{"afterlife", setsKwAfterlifeDecks, 120, 0, 5, 0, 0, 0, 0},
		{"battlecry", setsKwBattleCryDecks, 120, 0, 0, 10, 0, 0, 0},
		{"annihilator", setsKwAnnihilatorDecks, 120, 0, 0, 0, 5, 0, 0},
		{"undying", setsKwUndyingDecks, 120, 0, 0, 0, 0, 5, 0},
		{"persist", setsKwPersistDecks, 120, 0, 0, 0, 0, 3, 3},
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
			if st.horse < tc.horse || st.deaths < tc.deaths || st.cry < tc.cry || st.ann < tc.ann || st.und < tc.und || st.per < tc.per {
				t.Fatalf("%s: %+v over %d seeds, want horse %d deaths %d cry %d ann %d und %d per %d: the pool does not exercise it",
					tc.name, st, tc.seeds, tc.horse, tc.deaths, tc.cry, tc.ann, tc.und, tc.per)
			}
		})
	}
}

// setsKwStats is what setsKwProbe counts; both is the deaths that pushed a
// granted undying and a granted persist trigger for one creature together.
type setsKwStats struct{ horse, deaths, cry, ann, und, per, both int }

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
			if e := evs[seen]; seen > 0 && e.Kind == events.GrantTriggerPush {
				if p := evs[seen-1]; p.Kind == events.GrantTriggerPush && p.Obj == e.Obj && returnPair(p.Counter, e.Counter) {
					st.both++
				}
			}
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
			case e.Kind == events.GrantTriggerPush && e.Counter == "__kwUndyingGranted":
				st.und++
			case e.Kind == events.GrantTriggerPush && e.Counter == "__kwPersistGranted":
				st.per++
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

// returnPair reports whether a and b are a granted undying and a granted
// persist trigger, in either order.
func returnPair(a, b string) bool {
	const u, p = "__kwUndyingGranted", "__kwPersistGranted"
	return a == u && b == p || a == p && b == u
}

// TestSetsKeywordPumpReturnLowers: a Pump's KW$ Persist / KW$ Undying grant
// (Cauldron of Souls' tap, Cauldron Haze, Undying Evil; Endling's and Antler
// Skulkin's activations, Dusk Legion Sergeant's PumpAll; CR 702.79a, 702.93a)
// is a layer-6 effect gorge lists among a dying creature's granted keyword
// triggers by timestamp beside a static's AddKeyword$ grant (Mikaeus, the
// Unhallowed's undying): the controller's trigger-order ask (CR 603.3b)
// offers them in that order. Cauldron of Souls taps at the first priority of
// each turn, before Mikaeus is cast in a main phase, so the Pump grant is
// often the older. The probe counts deaths that push a granted undying and a
// granted persist for one creature together, and fails when too few do (the
// activated pool's grants rarely meet Mikaeus's on one death: it proves the
// three activated grants alone).
func TestSetsKeywordPumpReturnLowers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		decks     [2][]cardset.Entry
		both, per int
	}{
		{"cauldron", setsKwPumpReturnDecks, 5, 10},
		{"activated", setsKwPumpReturnActDecks, 0, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var st setsKwStats
			pol := func(seed uint64) Policy { return setsKwProbe(castFirstPolicy(seed), &st) }
			if div := lockstepRunPolicy(t, "setskeywords", tc.decks, 100, true, nil, nil, pol); div != 0 {
				return // each divergence is already a test error
			}
			t.Logf("%s: %+v", tc.name, st)
			if st.both < tc.both || st.per < tc.per || st.und < 10 {
				t.Fatalf("%s: %+v over 100 seeds, want both %d per %d und 10: the pool does not exercise it", tc.name, st, tc.both, tc.per)
			}
		})
	}
}

// TestSetsKeywordPumpReturnMutation: with the Pump-granted triggers listed
// after the static grants (GrantedKw cleared on the three Pump cards' latent
// abilities, the kernel's order before the fix), some seed diverges from
// gorge on the trigger-order ask.
func TestSetsKeywordPumpReturnMutation(t *testing.T) {
	t.Parallel()
	div := lockstepRunPolicy(t, "setskeywords", setsKwPumpReturnDecks, 100, false, func(tb *kop.Table) {
		for i := range tb.Cards {
			switch tb.Cards[i].Name {
			case "Cauldron of Souls", "Cauldron Haze", "Undying Evil":
				for j := range tb.Cards[i].Abilities {
					tb.Cards[i].Abilities[j].GrantedKw = false
				}
			}
		}
	}, nil, castFirstPolicy)
	if div == 0 {
		t.Fatal("no game diverged with the Pump grants listed after the static's: the pool does not reach the order")
	}
}

// setsKwPumpReturnDecks: Cauldron of Souls, Cauldron Haze and Undying Evil
// grant the Bears persist and undying beside Mikaeus, the Unhallowed's
// undying; Bolts kill them.
var setsKwPumpReturnDecks = [2][]cardset.Entry{
	{
		{Name: "Cauldron of Souls", Count: 5}, {Name: "Cauldron Haze", Count: 3}, {Name: "Undying Evil", Count: 3},
		{Name: "Mikaeus, the Unhallowed", Count: 6}, {Name: "Grizzly Bears", Count: 7},
		{Name: "Swamp", Count: 12}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Lightning Bolt", Count: 10},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

// setsKwPumpReturnActDecks: Endling grants itself undying, Antler Skulkin
// grants Savannah Lions persist and Dusk Legion Sergeant grants the
// Vampires persist, beside Mikaeus, the Unhallowed's undying; Bolts kill
// them.
var setsKwPumpReturnActDecks = [2][]cardset.Entry{
	{
		{Name: "Endling", Count: 3}, {Name: "Antler Skulkin", Count: 5}, {Name: "Dusk Legion Sergeant", Count: 4},
		{Name: "Child of Night", Count: 3}, {Name: "Savannah Lions", Count: 5}, {Name: "Mikaeus, the Unhallowed", Count: 4},
		{Name: "Swamp", Count: 12}, {Name: "Plains", Count: 4},
	},
	setsKwPumpReturnDecks[1],
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

// setsKwUndyingDecks: Mikaeus, the Unhallowed grants the Bears undying;
// Bolts kill them.
var setsKwUndyingDecks = [2][]cardset.Entry{
	{
		{Name: "Mikaeus, the Unhallowed", Count: 10}, {Name: "Grizzly Bears", Count: 10},
		{Name: "Swamp", Count: 14}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Lightning Bolt", Count: 10},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

// setsKwPersistDecks: Rattleblaze Scarecrow has persist while its controller
// controls a black creature (Mikaeus, the Unhallowed, which also grants the
// non-Human Scarecrow undying: a creature granted both by two statics);
// Bolts kill them.
var setsKwPersistDecks = [2][]cardset.Entry{
	{
		{Name: "Rattleblaze Scarecrow", Count: 10}, {Name: "Mikaeus, the Unhallowed", Count: 8},
		{Name: "Swamp", Count: 22},
	},
	{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Lightning Bolt", Count: 10},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}
