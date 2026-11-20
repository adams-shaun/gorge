package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The mana-rider class's lowered shapes (dirac/cardset/pools/fixtures/
// manariders): the production-reflected mana over the controller's OWN
// permanents (AB$ ManaReflected, ReflectProperty$ Produce, every Valid$
// alternative YouCtrl: Star Compass's "a basic land you control", Gond
// Gate's "a Gate you control", Reflecting Pool's "any type that a land you
// control could produce"). The decks hold few lands of their own colours and
// lean on the reflected sources, so the read moves as lands are played; the
// Pool decks carry Wastes and Command Tower (the {C} a Type read adds, which
// gorge's census and dirac's reflected view keep) and Cackling Prowler's ward
// so a payment window runs.
var manaRidersDecks = []fixtureDecks{{
	{
		{Name: "Star Compass", Count: 6}, {Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Gond Gate", Count: 6}, {Name: "Savannah Lions", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Shock", Count: 8}, {Name: "Command Tower", Count: 2}, {Name: "Island", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 4},
	},
}, {
	{
		{Name: "Reflecting Pool", Count: 8}, {Name: "Cackling Prowler", Count: 6}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 4}, {Name: "Plains", Count: 3}, {Name: "Mountain", Count: 3}, {Name: "Wastes", Count: 6},
	},
	{
		{Name: "Reflecting Pool", Count: 8}, {Name: "Cackling Prowler", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Shock", Count: 8}, {Name: "Command Tower", Count: 4}, {Name: "Island", Count: 3}, {Name: "Forest", Count: 3}, {Name: "Wastes", Count: 6},
	},
}, {
	// AddsNoCounter$: Cavern of Souls' restricted colour pays Goblins (the
	// type the bot names); the opponent's counterspells meet a spell paid with
	// that mana and must not counter it
	{
		{Name: "Cavern of Souls", Count: 5}, {Name: "Goblin Piker", Count: 8}, {Name: "Goblin Roughrider", Count: 6},
		{Name: "Mogg Fanatic", Count: 6}, {Name: "Lightning Bolt", Count: 4}, {Name: "Mountain", Count: 11},
	},
	manaRidersCounters,
}, {
	manaRidersCounters,
	{
		{Name: "Cavern of Souls", Count: 5}, {Name: "Goblin Piker", Count: 8}, {Name: "Goblin Roughrider", Count: 6},
		{Name: "Mogg Fanatic", Count: 6}, {Name: "Lightning Bolt", Count: 4}, {Name: "Mountain", Count: 11},
	},
}, {
	// Delighted Halfling's any-colour mana pays Isamaru (legendary, white)
	// with Plains scarce
	{
		{Name: "Delighted Halfling", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 5}, {Name: "Savannah Lions", Count: 4},
		{Name: "Grizzly Bears", Count: 10}, {Name: "Plains", Count: 3}, {Name: "Forest", Count: 14},
	},
	manaRidersCounters,
}, {
	manaRidersCounters,
	{
		{Name: "Delighted Halfling", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 5}, {Name: "Savannah Lions", Count: 4},
		{Name: "Grizzly Bears", Count: 10}, {Name: "Plains", Count: 3}, {Name: "Forest", Count: 14},
	},
}, {
	// AddsNoCounter$ !Permanent: Boseiju's {C} (a prefix script or the mana
	// window taps it, paying 2 life) pays the generic part of burn spells,
	// which the counterspell deck must not counter; Grizzly Bears (a permanent
	// spell) paid with it stay counterable
	{
		{Name: "Boseiju, Who Shelters All", Count: 6}, {Name: "Searing Spear", Count: 10}, {Name: "Lava Axe", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 12}, {Name: "Plains", Count: 4},
	},
	manaRidersCounters,
}, {
	manaRidersCounters,
	{
		{Name: "Boseiju, Who Shelters All", Count: 6}, {Name: "Searing Spear", Count: 10}, {Name: "Lava Axe", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 12}, {Name: "Plains", Count: 4},
	},
}, {
	// Utopia Sprawl: the enchanted Forest taps for its Forest mana and the
	// chosen colour (red: Goblins and burn beside a few Mountains)
	{
		{Name: "Utopia Sprawl", Count: 6}, {Name: "Forest", Count: 12}, {Name: "Mountain", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Goblin Roughrider", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
	},
	manaRidersCounters,
}, {
	manaRidersCounters,
	{
		{Name: "Utopia Sprawl", Count: 6}, {Name: "Forest", Count: 12}, {Name: "Mountain", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Goblin Roughrider", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
	},
}, {
	// Horizon of Progress: the {T} + pay 1 life reflected Produce read (the
	// pain-gated lone shape). Its mana is the planner's last resort
	// (Consequence{Life:1}), so the payer taps it under a cost it cannot fund
	// otherwise, at 1 life a pop
	{
		{Name: "Horizon of Progress", Count: 5}, {Name: "Grizzly Bears", Count: 6}, {Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 3}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Wastes", Count: 6},
	},
	manaRidersCounters,
}, {
	manaRidersCounters,
	{
		{Name: "Horizon of Progress", Count: 5}, {Name: "Grizzly Bears", Count: 6}, {Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 3}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Wastes", Count: 6},
	},
}, {
	// The Grey Havens: the reflected Is read over the graveyard ("any color
	// among legendary creature cards in your graveyard") beside {T}: Add {C}.
	// The vanilla legendaries (Isamaru white, Happy Hogan red, Kalakscion
	// black) die to the burn deck, so the read's candidates move as they fall
	{
		{Name: "The Grey Havens", Count: 5}, {Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Happy Hogan, Dauntless Driver", Count: 4}, {Name: "Kalakscion, Hunger Tyrant", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 5},
	},
	{
		{Name: "Goblin Piker", Count: 10}, {Name: "Shock", Count: 8}, {Name: "Lightning Bolt", Count: 8}, {Name: "Mountain", Count: 14},
	},
}, {
	{
		{Name: "Goblin Piker", Count: 10}, {Name: "Shock", Count: 8}, {Name: "Lightning Bolt", Count: 8}, {Name: "Mountain", Count: 14},
	},
	{
		{Name: "The Grey Havens", Count: 5}, {Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Happy Hogan, Dauntless Driver", Count: 4}, {Name: "Kalakscion, Hunger Tyrant", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 5},
	},
}, {
	// Incubation Druid: the reflected Produce read with an Amount$
	// (Count$Compare over its +1/+1 counters). gorge's planner defers the
	// amount, so its census leaves the source a gap and the one prefix search
	// evaluates it at the activation; the Adapt 3 activation puts the counters
	// on, so both amounts run. The table holds no filter, no tap-another
	// source, no granted mana ability, no Lion's Eye Diamond and no
	// reflected-view source beside it (StallComboAnyMix refuses the mix).
	{
		{Name: "Incubation Druid", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 4}, {Name: "Giant Growth", Count: 4},
		{Name: "Forest", Count: 10}, {Name: "Plains", Count: 4}, {Name: "Mountain", Count: 6},
	},
	manaRidersCounters,
}, {
	manaRidersCounters,
	{
		{Name: "Incubation Druid", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 4}, {Name: "Giant Growth", Count: 4},
		{Name: "Forest", Count: 10}, {Name: "Plains", Count: 4}, {Name: "Mountain", Count: 6},
	},
}}

// manaRidersCounters is the counterspell deck the AddsNoCounter$ matchups face.
var manaRidersCounters = []cardset.Entry{
	{Name: "Counterspell", Count: 5}, {Name: "Essence Scatter", Count: 4}, {Name: "Mana Leak", Count: 4}, {Name: "Cancel", Count: 4},
	{Name: "Wind Drake", Count: 8}, {Name: "Island", Count: 15},
}

var manaRidersWatch = map[string]bool{
	"Star Compass": true, "Reflecting Pool": true, "Gond Gate": true, "Shock": true, "Lightning Bolt": true, "Isamaru, Hound of Konda": true,
	"Goblin Piker": true, "Utopia Sprawl": true, "Goblin Roughrider": true, "Mogg Fanatic": true, "Delighted Halfling": true, "Savannah Lions": true,
	"Horizon of Progress": true, "Incubation Druid": true, "The Grey Havens": true, "Happy Hogan, Dauntless Driver": true, "Kalakscion, Hunger Tyrant": true,
	"Boseiju, Who Shelters All": true, "Searing Spear": true, "Lava Axe": true,
}

func TestManaRidersLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugUncounterStats = true
	dirac.NoCounterHeld, dirac.DeferNoCounterStamped = 0, 0
	defer func() { dirac.DebugUncounterStats = false }()
	run := runFixtureLockstep(t, "manariders", manaRidersDecks, manaRidersWatch, 900, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v, counters stopped by AddsNoCounter$ mana %d", run.games, run.divs, run.stalls, run.casts, dirac.NoCounterHeld)
	if run.casts["Shock"] < 40 {
		t.Errorf("only %d Shock casts: the mana windows never ran", run.casts["Shock"])
	}
	if run.casts["Utopia Sprawl"] < 20 {
		t.Errorf("only %d Utopia Sprawl casts: the enchanted Forest's chosen-colour extra never ran", run.casts["Utopia Sprawl"])
	}
	if dirac.DeferNoCounterStamped < 10 {
		t.Errorf("only %d spells were paid with Boseiju's AddsNoCounter$ !Permanent mana: its deferred source never paid an instant or sorcery", dirac.DeferNoCounterStamped)
	}
	if dirac.NoCounterHeld < 10 {
		t.Errorf("only %d Counters met a spell paid with AddsNoCounter$ mana: the Cavern / Halfling matchups never exercised the stamp", dirac.NoCounterHeld)
	}
}

// TestManaRidersLockstepNoCounterMutation: the stamp is dropped, so a spell
// paid with Cavern of Souls' or Delighted Halfling's mana is countered by the
// kernel where gorge stops the Counter. It must diverge.
func TestManaRidersLockstepNoCounterMutation(t *testing.T) {
	dirac.DebugNoCounterOff = true
	defer func() { dirac.DebugNoCounterOff = false }()
	run := runFixtureLockstep(t, "manariders", manaRidersDecks, manaRidersWatch, 400, false, nil)
	run.requireDivergence(t, "an AddsNoCounter$ spell is countered")
}

// TestManaRidersLockstepBoseijuMutation: Boseiju's deferred source stamps no
// spell, so the kernel counters a burn spell paid with its {C} where gorge
// stops the Counter. It must diverge.
func TestManaRidersLockstepBoseijuMutation(t *testing.T) {
	dirac.DebugNoCounterDeferOff = true
	defer func() { dirac.DebugNoCounterDeferOff = false }()
	run := runFixtureLockstep(t, "manariders", manaRidersDecks[6:8], manaRidersWatch, 400, false, nil)
	run.requireDivergence(t, "a Boseiju-paid instant is countered")
}

// TestManaRidersLockstepMutation: the switch reads no colours, so the
// reflected sources add none (gorge taps one for a land's colour). It must
// diverge.
func TestManaRidersLockstepMutation(t *testing.T) {
	dirac.SetDebugReflMutate(1)
	defer dirac.SetDebugReflMutate(0)
	run := runFixtureLockstep(t, "manariders", manaRidersDecks, manaRidersWatch, 200, false, nil)
	run.requireDivergence(t, "the reflected sources read no colours")
}

// TestManaRidersLockstepSprawlMutation: Utopia Sprawl's enchanted Forest adds
// no chosen-colour mana in the kernel, where gorge's tap floats it. It must
// diverge.
func TestManaRidersLockstepSprawlMutation(t *testing.T) {
	dirac.DebugTapChosenOff = true
	defer func() { dirac.DebugTapChosenOff = false }()
	run := runFixtureLockstep(t, "manariders", manaRidersDecks, manaRidersWatch, 400, false, nil)
	run.requireDivergence(t, "the chosen-colour extra is dropped")
}
