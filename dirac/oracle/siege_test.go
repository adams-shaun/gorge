package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Tarkir Dragonstorm Sieges' lockstep: the as-enters
// "choose Abzan or Mardu" (api:GenericChoice, SetChosenMode$ True) and the
// two ChosenMode-gated self grants. Barrensteppe (a +1/+1 counter on each of
// your creatures, or the end-step edict after a creature died) and
// Glacierwood (the instant-or-sorcery mill trigger, or lands from the
// graveyard) play against creatures and spells that exercise each mode.
// Frostcliff (gorge fires its granted combat-damage batch trigger per damage
// event) and Windcrag (an attack Panharmonicon) stay refused. Hollowmurk
// (any-kind CounterAdded, ActivationLimit$ 1, granted by its Sultai mode) has
// its own decks, khollowDecks, capped at two copies: see the note there.
var ksiegeDecks = [2][]cardset.Entry{
	{
		{Name: "Barrensteppe Siege", Count: 6}, {Name: "Glacierwood Siege", Count: 6},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Plains", Count: 4}, {Name: "Swamp", Count: 4}, {Name: "Island", Count: 4},
		{Name: "Mountain", Count: 2}, {Name: "Forest", Count: 2},
	},
	{
		{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Giant Growth", Count: 4},
		{Name: "Divination", Count: 4}, {Name: "Opt", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 10},
	},
}

func TestSiegeLockstep(t *testing.T) {
	t.Parallel()
	before := dirac.SiegeModeAnswered.Load()
	lockstepSet(t, "siege", ksiegeDecks, 1000, "KSIEGE_SEEDS")
	if n := dirac.SiegeModeAnswered.Load() - before; n == 0 {
		t.Fatal("no Siege mode was answered: the pool does not exercise the as-enters choice")
	} else {
		t.Logf("%d Siege mode answers", n)
	}
}

// ksiegeExtraDecks plays the other cards the same lowering accepts: Citadel,
// Palace and Outpost Siege, Battle of Hoover Dam (two triggered grants each,
// some targeted or on a zone change) and The Marvelous Scientist (a creature
// with the as-enters choice and a spell-cast grant that animates it).
var ksiegeExtraDecks = [2][]cardset.Entry{
	{
		{Name: "Citadel Siege", Count: 4}, {Name: "Palace Siege", Count: 4}, {Name: "Outpost Siege", Count: 4},
		{Name: "Battle of Hoover Dam", Count: 4}, {Name: "The Marvelous Scientist", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Raging Goblin", Count: 3},
		{Name: "Plains", Count: 3}, {Name: "Swamp", Count: 3}, {Name: "Island", Count: 3},
		{Name: "Mountain", Count: 3}, {Name: "Forest", Count: 2},
	},
	ksiegeDecks[1],
}

func TestSiegeExtraLockstep(t *testing.T) {
	t.Parallel()
	before := dirac.SiegeModeAnswered.Load()
	lockstepSet(t, "siege", ksiegeExtraDecks, 1000, "KSIEGE_SEEDS")
	if dirac.SiegeModeAnswered.Load() == before {
		t.Fatal("no mode was answered: the pool does not exercise the as-enters choice")
	}
}

// TestSiegeLockstepMutation proves the answer is stored as chosen: stored
// as the other mode, some seed diverges (the other mode's trigger fires).
func TestSiegeLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSiegeModeFlip = true
	defer func() { dirac.DebugSiegeModeFlip = false }()
	if !lockstepDiverges(t, "siege", ksiegeDecks, 1000) {
		t.Fatal("no seed diverged with the mode flipped: the pool does not exercise the gated grants")
	}
}

// khollowDecks plays Hollowmurk Siege: its Abzan mode (a +1/+1 counter on a
// target attacker whenever you attack) feeds the Sultai mode's granted "whenever a counter is put on a creature you
// control, draw a card; only once each turn" (Mode$ CounterAdded, no
// CounterType$, ActivationLimit$ 1).
//
// At most TWO Hollowmurk Sieges and no Barrensteppe (its Abzan mode puts a
// counter on each creature, several in one turn): gorge does not enforce ActivationLimit$ on an
// AddTrigger$-granted trigger (pin b4e6762a27f9, engine/rules/trigger_granted.go
// checkGrantedStaticTriggersUsing never calls triggerActivationLimitAllows /
// reserveTriggerLimits; Forge's Trigger.checkActivationLimit does), so with
// four or more copies a second counter in one turn queues a second draw in
// gorge only (about half of 1000 seeds diverge, e.g. seed 1 at a
// trigger_order decision, ref two triggers, kernel none). The kernel keys the
// limit per (object, ability), which is the card text. This is a
// gorge finding; the kernel does not emulate it.
var khollowDecks = [2][]cardset.Entry{
	{
		{Name: "Hollowmurk Siege", Count: 2},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Raging Goblin", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Plains", Count: 5}, {Name: "Swamp", Count: 5}, {Name: "Island", Count: 4},
		{Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
	ksiegeDecks[1],
}

func TestSiegeHollowmurkLockstep(t *testing.T) {
	t.Parallel()
	before := dirac.SiegeModeAnswered.Load()
	lockstepSet(t, "siege", khollowDecks, 1000, "KSIEGE_SEEDS")
	if dirac.SiegeModeAnswered.Load() == before {
		t.Fatal("no Siege mode was answered: the pool does not exercise the as-enters choice")
	}
}

// TestSiegeHollowmurkMutation narrows the Sultai draw from any counter kind
// to -1/-1, which no pooled card puts: the +1/+1 counters stop drawing, so
// some seed must diverge. A green mutation means the lockstep never reaches
// the granted any-kind trigger.
func TestSiegeHollowmurkMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "siege", khollowDecks, 1000, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				a := &tbl.Cards[i].Abilities[j]
				if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigCounterAddedOnce && a.Trig.N == kop.TrigNAnyCounter {
					a.Trig.N = int16(kop.CtrM1M1)
				}
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the any-kind draw narrowed to -1/-1: the pool never draws off a counter")
	}
}
