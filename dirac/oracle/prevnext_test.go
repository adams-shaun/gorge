package oracle

import (
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The depleting prevent-the-next-N shield lockstep (Ext PreventNext, CR 615.7):
// the corpus's one-recipient shields (a spell or an activated ability, a
// constant amount, an X, a target of any kind, a creature, "you") against burn,
// a damage sweep, lifelink, deathtouch and ordinary combat. The expected answer
// at every decision is gorge's.
var kprevnextDecks = [2][]cardset.Entry{
	{
		{Name: "Healing Salve", Count: 4}, {Name: "Alabaster Potion", Count: 3}, {Name: "Battlefield Medic", Count: 3},
		{Name: "Combat Medic", Count: 3}, {Name: "Mending Hands", Count: 2}, {Name: "Esper Battlemage", Count: 2},
		{Name: "Withstand", Count: 2}, {Name: "Hold at Bay", Count: 1}, {Name: "Alabaster Wall", Count: 1}, {Name: "Decorated Griffin", Count: 1},
		{Name: "Savannah Lions", Count: 4}, {Name: "Serra Angel", Count: 2}, {Name: "Plains", Count: 12},
	},
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Pyroclasm", Count: 2},
		{Name: "Prodigal Pyromancer", Count: 3}, {Name: "Typhoid Rats", Count: 3}, {Name: "Child of Night", Count: 3},
		{Name: "Hill Giant", Count: 3}, {Name: "Raging Goblin", Count: 3}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 7},
	},
}

// kprevnextBurnDecks is the noncombat half: instants against burn at the two
// players, so shields meet noncombat damage (a stack of shields on one
// recipient, a partial prevention).
var kprevnextBurnDecks = [2][]cardset.Entry{
	{
		{Name: "Healing Salve", Count: 8}, {Name: "Mending Hands", Count: 6}, {Name: "Alabaster Potion", Count: 6},
		{Name: "Hold at Bay", Count: 4}, {Name: "Withstand", Count: 4}, {Name: "Plains", Count: 12},
	},
	{
		{Name: "Lightning Bolt", Count: 10}, {Name: "Shock", Count: 10}, {Name: "Mountain", Count: 20},
	},
}

// kprevnextShapeDecks spreads the shapes: tap, mana, life, sacrifice, discard and
// pay-life costs, a trigger (Dawnfluke), artifacts, a land, a creature-only target
// the legendary / artifact filters and a combat-only shield.
var kprevnextShapeDecks = [2][]cardset.Entry{
	{
		{Name: "Abuna Acolyte", Count: 1}, {Name: "Amulet of Kroog", Count: 1}, {Name: "Atalya, Samite Master", Count: 1},
		{Name: "Aurora Eidolon", Count: 1}, {Name: "Balm of Restoration", Count: 1}, {Name: "Bandage", Count: 1},
		{Name: "Barrenton Medic", Count: 1}, {Name: "Benevolent Ancestor", Count: 1}, {Name: "Caregiver", Count: 1},
		{Name: "Conservator", Count: 1}, {Name: "Dawnfluke", Count: 1}, {Name: "Hallowed Healer", Count: 1},
		{Name: "Hypochondria", Count: 1}, {Name: "Ivory Charm", Count: 1}, {Name: "Kithkin Healer", Count: 1},
		{Name: "Kitsune Bonesetter", Count: 1}, {Name: "Loxodon Anchorite", Count: 1}, {Name: "Master Apothecary", Count: 1},
		{Name: "Militant Monk", Count: 1}, {Name: "Orim's Touch", Count: 1}, {Name: "Daru Healer", Count: 1},
		{Name: "Master Healer", Count: 1}, {Name: "Revered Elder", Count: 1}, {Name: "Samite Healer", Count: 1},
		{Name: "Troubled Healer", Count: 1}, {Name: "Argivian Blacksmith", Count: 1}, {Name: "Eiganjo Castle", Count: 1},
		{Name: "Oasis", Count: 1}, {Name: "Decorated Griffin", Count: 1}, {Name: "Plains", Count: 11},
	},
	kprevnextDecks[1],
}

func TestPrevNextShapesLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "prevnext", kprevnextShapeDecks, 600, "KPREVNEXTS_SEEDS")
}

// pnxPolicy scripts the noncombat half: both seats play lands, nobody attacks;
// seat 1 casts burn whenever it can, aimed at seat 0; seat 0 passes until a spell
// is on the stack, then answers with every shield it can cast, aimed at the
// player the spell on top of the stack targets (so shields stack on one
// recipient and burn meets them). Any other choice is random.
type pnxPolicy struct{ rnd Policy }

func (p pnxPolicy) Pick(c *PickCtx) Canon {
	d := c.D
	g := c.G.E.G
	switch d.Kind {
	case DMulligan:
		return pickKind(c, ActKeep)
	case DAttackers, DBlockers:
		return pickKind(c, ActDone)
	case DTarget:
		want := RefPlayer
		if n := len(g.Stack); n > 0 {
			if top := g.Obj(g.Stack[n-1]); top != nil {
				for _, t := range top.Targets {
					if t.IsPlayer {
						want = RefPlayer + Ref(t.Player)
					}
				}
			}
		}
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActTarget && a.Tgt() == want }); ok {
			return a
		}
	case DPriority:
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
			return a
		}
		if d.Player == 0 && len(g.Stack) == 0 {
			return pickKind(c, ActPass)
		}
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActCast }); ok {
			return a
		}
		return pickKind(c, ActPass)
	}
	return p.rnd.Pick(c)
}

// TestPrevNextBurnLockstep: shields answer burn at the players (noncombat damage,
// a stack of shields on one recipient asked in order, a partial prevention).
func TestPrevNextBurnLockstep(t *testing.T) {
	t.Parallel()
	seeds := uint64(500)
	if v := os.Getenv("KPREVNEXTB_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	lockstepRunPolicy(t, "prevnext", kprevnextBurnDecks, seeds, true, nil, nil, func(seed uint64) Policy { return pnxPolicy{RandomPolicy(seed)} })
}

func TestPrevNextLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "prevnext", kprevnextDecks, 600, "KPREVNEXT_SEEDS")
}

// TestPrevNextLockstepMutation proves the pool exercises the shield: with the
// shields installed but preventing nothing, some seed diverges.
func TestPrevNextLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPrevNext = true
	defer func() { dirac.DebugNoPrevNext = false }()
	if !lockstepDiverges(t, "prevnext", kprevnextDecks, 200) {
		t.Fatal("no seed diverged with the shields dropped: the pool never has damage prevented by a prevent-the-next-N shield")
	}
}

// TestPrevNextBurnLockstepMutation: the same for the noncombat half.
func TestPrevNextBurnLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPrevNext = true
	defer func() { dirac.DebugNoPrevNext = false }()
	if lockstepRunPolicy(t, "prevnext", kprevnextBurnDecks, 100, false, nil, nil, func(seed uint64) Policy { return pnxPolicy{RandomPolicy(seed)} }) == 0 {
		t.Fatal("no seed diverged with the shields dropped: burn never meets a shield")
	}
}
