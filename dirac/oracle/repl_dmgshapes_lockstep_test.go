package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The replace-the-damage DamageDone replacements played against gorge
// (repl_dminstead.go; lower_replinstead.go): the damage is replaced by counters,
// mill, exile or a draw, and Ghosts of the Innocent halves it. Each pool pairs
// two decks that play one such line each, so no event meets gorge's CR 616.1
// order ask; every pool has a mutation (the line dropped, or its halving
// cleared) that makes some seed diverge, so the lockstep tests the line, not
// the cards around it.

var (
	// Angel of Suffering: damage to you is prevented and you mill twice that many.
	kdmgAngelDecks = [2][]cardset.Entry{
		{{Name: "Angel of Suffering", Count: 1}, {Name: "Walking Corpse", Count: 19}, {Name: "Swamp", Count: 20}},
		kdmgBurnDeck,
	}
	// Force Bubble: damage to you becomes depletion counters on it.
	kdmgBubbleDecks = [2][]cardset.Entry{
		{{Name: "Force Bubble", Count: 1}, {Name: "Savannah Lions", Count: 21}, {Name: "Plains", Count: 18}},
		kdmgBurnDeck,
	}
	// Gloom Surgeon (combat damage to it exiles that many of your top cards)
	// against Swans of Bryn Argoll (damage to it draws its source's controller
	// that many cards).
	kdmgSelfDecks = [2][]cardset.Entry{
		{{Name: "Gloom Surgeon", Count: 8}, {Name: "Walking Corpse", Count: 14}, {Name: "Swamp", Count: 18}},
		{{Name: "Swans of Bryn Argoll", Count: 4}, {Name: "Savannah Lions", Count: 8}, {Name: "Wind Drake", Count: 8}, {Name: "Plains", Count: 10}, {Name: "Island", Count: 10}},
	}
	// Panther Habit: damage to the equipped creature becomes counters on it.
	kdmgHabitDecks = [2][]cardset.Entry{
		{{Name: "Panther Habit", Count: 1}, {Name: "Savannah Lions", Count: 17}, {Name: "Plains", Count: 22}},
		kdmgBurnDeck,
	}
	// Soul-Scar Mage: noncombat damage you deal to an opponent's creature
	// becomes -1/-1 counters on it.
	kdmgSoulScarDecks = [2][]cardset.Entry{
		{{Name: "Soul-Scar Mage", Count: 1}, {Name: "Pyroclasm", Count: 6}, {Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Raging Goblin", Count: 6}, {Name: "Mountain", Count: 15}},
		{{Name: "Hill Giant", Count: 23}, {Name: "Mountain", Count: 17}},
	}
	// Undead Alchemist (a Zombie's combat damage to a player mills them) against
	// Szadek (its combat damage to a player is counters on it and a mill).
	kdmgMillDecks = [2][]cardset.Entry{
		{{Name: "Undead Alchemist", Count: 1}, {Name: "Walking Corpse", Count: 15}, {Name: "Island", Count: 10}, {Name: "Swamp", Count: 14}},
		{{Name: "Szadek, Lord of Secrets", Count: 3}, {Name: "Walking Corpse", Count: 12}, {Name: "Island", Count: 12}, {Name: "Swamp", Count: 13}},
	}
	// Ghosts of the Innocent: every damage is half of it, rounded down.
	kdmgGhostsDecks = [2][]cardset.Entry{
		{{Name: "Ghosts of the Innocent", Count: 1}, {Name: "Savannah Lions", Count: 17}, {Name: "Plains", Count: 22}},
		kdmgBurnDeck,
	}
	kdmgBurnDeck = []cardset.Entry{{Name: "Lava Axe", Count: 6}, {Name: "Hill Giant", Count: 12}, {Name: "Shock", Count: 4}, {Name: "Mountain", Count: 18}}
)

func TestReplDmgShapesLockstep(t *testing.T) {
	// A fresh clone has no corpus: skip the whole test, not only its subtests,
	// so the body check below does not read a skipped subtest as a body that
	// never ran.
	corpusDirOrSkip(t)
	t.Parallel()
	for _, p := range []struct {
		name  string
		decks [2][]cardset.Entry
		env   string
		body  int
	}{
		{"angel", kdmgAngelDecks, "KDMGANGEL_SEEDS", kop.ReplDmgInsMill},
		{"bubble", kdmgBubbleDecks, "KDMGBUBBLE_SEEDS", kop.ReplDmgInsCtrHost},
		{"self", kdmgSelfDecks, "KDMGSELF_SEEDS", kop.ReplDmgInsExileTop | kop.ReplDmgInsDrawSrc},
		{"habit", kdmgHabitDecks, "KDMGHABIT_SEEDS", kop.ReplDmgInsCtrEquipped},
		{"soulscar", kdmgSoulScarDecks, "KDMGSOULSCAR_SEEDS", kop.ReplDmgInsCtrTarget},
		{"mill", kdmgMillDecks, "KDMGMILL_SEEDS", kop.ReplDmgInsMill},
		{"ghosts", kdmgGhostsDecks, "KDMGGHOSTS_SEEDS", 0},
	} {
		// TestReplDmgShapesReach is this assertion, not a second replay: the
		// games the lockstep plays prove every body ran (the stats are the
		// replacement's own counters; no other test touches them).
		var before [8]int64
		for b := range before {
			before[b] = dirac.DmgInsteadStats[b].Load()
		}
		t.Run(p.name, func(t *testing.T) { lockstepSet(t, "repl-dmgshapes", p.decks, 300, p.env) })
		for b := range before {
			if p.body&(1<<b) != 0 && dirac.DmgInsteadStats[b].Load() == before[b] {
				t.Errorf("%s: the body %#x never ran", p.name, 1<<b)
			}
		}
	}
}

// TestReplDmgShapesLockstepMutation: dropping the replace-the-damage lines, or
// clearing the halving of Ghosts of the Innocent, diverges some seed of each
// pool.
func TestReplDmgShapesLockstepMutation(t *testing.T) {
	t.Parallel()
	drop := func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				if a := &tbl.Cards[i].Abilities[j]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplDamageInstead {
					a.Repl.Kind = kop.ReplNone
				}
			}
		}
	}
	for _, p := range []struct {
		name  string
		decks [2][]cardset.Entry
		seeds uint64
	}{
		{"angel", kdmgAngelDecks, 300}, {"bubble", kdmgBubbleDecks, 300}, {"self", kdmgSelfDecks, 100}, {"habit", kdmgHabitDecks, 300},
		{"soulscar", kdmgSoulScarDecks, 300}, {"mill", kdmgMillDecks, 100},
	} {
		if n := lockstepRun(t, "repl-dmgshapes", p.decks, p.seeds, false, drop); n == 0 {
			t.Errorf("%s: dropping the replace-the-damage lines diverged no seed", p.name)
		}
	}
	whole := func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				if a := &tbl.Cards[i].Abilities[j]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplDamageMinus {
					a.Repl.C &^= kop.ReplDmgHalfDown
				}
			}
		}
	}
	if n := lockstepRun(t, "repl-dmgshapes", kdmgGhostsDecks, 100, false, whole); n == 0 {
		t.Error("ghosts: clearing the halving diverged no seed")
	}
}
