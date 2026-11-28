package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The SetRefBase / SetBattlefield lockstep (kgen lower_definedset.go). The
// pool is dirac/cardset/pools/fixtures/setbases: the corpus carriers, which
// still refuse for other reasons (Enslave's DamageSource$ amount, Earthbind's
// other lines, Missy's DamageSource$ sweep), the vanilla creatures, a burn
// spell that takes a bearer away under a trigger, and basics. The set reads
// therefore come from inline carriers built into the same table.

// setBaseProbes are the inline carriers, one per base shape: battlefield
// sweeps that damage, tap and untap several creatures at once, the enchanted
// creature by filter (damaged and destroyed), its owner through RefAttached,
// and the complements of RefEnchantedCtrl and RefDefendingPlayer.
var setBaseProbes = []string{
	"Name:Setbase Sweep\nManaCost:2\nTypes:Sorcery\n" +
		"A:SP$ DealDamage | Defined$ Valid Creature.YouDontCtrl | NumDmg$ 1\n",
	"Name:Setbase Hush\nManaCost:1\nTypes:Sorcery\n" +
		"A:SP$ Tap | Defined$ Valid Creature.YouDontCtrl\n",
	"Name:Setbase Wake\nManaCost:1\nTypes:Sorcery\n" +
		"A:SP$ Untap | Defined$ Valid Creature.YouCtrl\n",
	"Name:Setbase Doom\nManaCost:1\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ SetbaseKill\n" +
		"SVar:SetbaseKill:DB$ Destroy | Defined$ Valid Card.EnchantedBy\n",
	// the watchers' triggers queue in the order the sweeps walk
	"Name:Setbase Enrage\nManaCost:1\nTypes:Creature Bear\nPT:1/4\n" +
		"T:Mode$ DamageDone | ValidTarget$ Card.Self | TriggerZones$ Battlefield | Execute$ SetbaseGain\n" +
		"SVar:SetbaseGain:DB$ GainLife | Defined$ You | LifeAmount$ 1\n",
	"Name:Setbase Wary\nManaCost:1\nTypes:Creature Bear\nPT:2/2\n" +
		"T:Mode$ Taps | ValidCard$ Card.Self | TriggerZones$ Battlefield | Execute$ SetbaseGain\n" +
		"SVar:SetbaseGain:DB$ GainLife | Defined$ You | LifeAmount$ 1\n",
	"Name:Setbase Lash\nManaCost:1\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ SetbaseDmg\n" +
		"SVar:SetbaseDmg:DB$ DealDamage | Defined$ EnchantedOwner | NumDmg$ 1\n",
	"Name:Setbase Bite\nManaCost:1\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ SetbaseDmg\n" +
		"SVar:SetbaseDmg:DB$ DealDamage | Defined$ Valid Card.EnchantedBy | NumDmg$ 1\n",
	"Name:Setbase Tithe\nManaCost:1\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ SetbaseLose\n" +
		"SVar:SetbaseLose:DB$ LoseLife | Defined$ NonEnchantedController | LifeAmount$ 1\n",
	"Name:Setbase Herald\nManaCost:1\nTypes:Creature Bear\nPT:1/3\n" +
		"T:Mode$ Attacks | ValidCard$ Card.Self | TriggerZones$ Battlefield | Execute$ SetbaseLose\n" +
		"SVar:SetbaseLose:DB$ LoseLife | Defined$ NonTriggeredDefendingPlayer | LifeAmount$ 1\n",
}

// setBaseDecks are the two forty-card decks: names of the pool and of the
// inline carriers.
var setBaseDecks = [2][]cardset.Entry{
	{
		{Name: "Setbase Sweep", Count: 3}, {Name: "Setbase Lash", Count: 4}, {Name: "Setbase Bite", Count: 3},
		{Name: "Setbase Herald", Count: 3}, {Name: "Setbase Wary", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Glory Seeker", Count: 2}, {Name: "Savannah Lions", Count: 1},
		{Name: "Mountain", Count: 7}, {Name: "Plains", Count: 7},
	},
	{
		{Name: "Setbase Hush", Count: 2}, {Name: "Setbase Wake", Count: 2}, {Name: "Setbase Doom", Count: 2},
		{Name: "Setbase Lash", Count: 3}, {Name: "Setbase Tithe", Count: 3}, {Name: "Setbase Herald", Count: 3},
		{Name: "Setbase Enrage", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Llanowar Elves", Count: 1}, {Name: "Ornithopter", Count: 1},
		{Name: "Forest", Count: 7}, {Name: "Mountain", Count: 7},
	},
}

// setBaseTable is the fixture pool's table with the inline carriers lowered
// beside it, and the two gorge decks.
func setBaseTable(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	list, err := os.ReadFile(filepath.Join("..", "cardset", "pools", "fixtures", "setbases", "cards.txt"))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*cards.Card{}
	var all []*cards.Card
	for _, line := range strings.Split(string(list), "\n") {
		if line = strings.TrimSpace(line); line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		c, ok := reg.Lookup(line)
		if !ok {
			t.Fatalf("%s not in the corpus", line)
		}
		byName[line] = c
		all = append(all, c)
	}
	for _, src := range setBaseProbes {
		c, diags := cards.ParseBytes("setbases_probe.txt", []byte(src))
		if c == nil || len(diags) != 0 {
			t.Fatalf("parse: %v", diags)
		}
		if d := c.Link(); len(d) != 0 {
			t.Fatalf("link: %v", d)
		}
		byName[c.Faces[0].Name] = c
		all = append(all, c)
	}
	pd, err := kgen.TableFromCards(reg, all, "setbases-fixture")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range setBaseDecks {
		for _, e := range d {
			c := byName[e.Name]
			if c == nil {
				t.Fatalf("%s is neither in the pool nor an inline carrier", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	return reg, cardset.Raw(cardset.NewDataSet(pd)), gd
}

// setBaseGames plays seeds 1..hi and returns the clean games, the divergent
// ones and the kernel's set counts. mutate edits the matchup's table first.
func setBaseGames(t *testing.T, hi uint64, mode lockstepMode, mutate func(*kop.Table)) (clean int, divs []lockstepDivergence, st *dirac.SetStats) {
	t.Helper()
	reg, set, gd := setBaseTable(t)
	st = &dirac.SetStats{}
	dirac.DebugSetStats = st
	defer func() { dirac.DebugSetStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, "setbases", setBaseDecks, gd, 1, hi, mode, mutate, nil, castFirstPolicy)
	return c, d, st
}

// TestSetBasesLockstep: every inline carrier's ability reads a SetRefBase or
// SetBattlefield set of the table, and over 64 seeds no game diverges or
// stalls while those sets run.
func TestSetBasesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	bases := map[kop.SetOpKind]int{}
	refs := map[kop.Ref]bool{}
	readers := 0
	clean, divs, st := setBaseGames(t, 64, lockstepFailEach, func(tbl *kop.Table) {
		for _, c := range tbl.Cards {
			if slices.ContainsFunc(c.Abilities, func(a kop.Ability) bool { return len(a.SetRefs) > 0 }) {
				readers++
			}
		}
		for _, e := range tbl.Sets {
			bases[e[0].Kind]++
			if e[0].Kind == kop.SetRefBase {
				refs[kop.Ref(e[0].Arg)] = true
			}
		}
	})
	// the sweep, tap, untap, destroy, owner, filter and two complement
	// carriers; their sweeps intern to three distinct battlefield sets
	if readers != 8 || bases[kop.SetBattlefield] != 3 || !refs[kop.RefAttached] || !refs[kop.RefEnchantedCtrl] || !refs[kop.RefDefendingPlayer] {
		t.Fatalf("precondition: %d cards read sets, bases %v over refs %v", readers, bases, refs)
	}
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if st.Runs == 0 || st.Members == 0 {
		t.Errorf("no game ran a RefSet instruction over a member: %+v", *st)
	}
	t.Logf("%d clean games; %+v", clean, *st)
}

// TestSetBasesMutation: with every SetBattlefield filter replaced by
// Card.Creature (no controller, power or attachment word), some game
// diverges.
func TestSetBasesMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	rewritten := 0
	_, divs, st := setBaseGames(t, 64, lockstepQuiet, func(tbl *kop.Table) {
		creature := slices.IndexFunc(tbl.Filters, func(f kop.Filter) bool { return f == kop.Filter{Types: kop.TCreature} })
		if creature < 0 {
			creature = len(tbl.Filters)
			tbl.Filters = append(slices.Clone(tbl.Filters), kop.Filter{Types: kop.TCreature})
		}
		sets := slices.Clone(tbl.Sets)
		for i, e := range sets {
			if len(e) > 1 && e[0].Kind == kop.SetBattlefield && e[1].Kind == kop.SetAndFilter && int(e[1].Arg) != creature {
				sets[i] = slices.Clone(e)
				sets[i][1].Arg = uint16(creature)
				rewritten++
			}
		}
		tbl.Sets = sets
	})
	if rewritten == 0 {
		t.Fatal("no SetBattlefield filter to replace: the mutation changed nothing")
	}
	if st.Runs == 0 {
		t.Fatal("no game ran a RefSet instruction")
	}
	if len(divs) == 0 {
		t.Fatal("every battlefield sweep read as every creature: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games (%d filters replaced)", len(divs), rewritten)
}
