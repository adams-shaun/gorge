package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The lockstep: target-set constraints of a
// multi-object slot, one multi-select decision in gorge.
//
//   - Scout for Survivors, Joshua, Phoenix's Dominant (chapter III): up to N
//     creature cards with a total mana value cap (MaxTotalTargetCMC$);
//   - Behold the Sinister Six!: up to six cards with different names;
//   - Unbury (mode 2): two creature cards that share a creature type;
//   - Driver of the Dead: a creature card with mana value 2 or less in the
//     dying card's controller's graveyard (TargetsWithDefinedController$
//     TriggeredCardController).
//
// Wrap in Flames (damage, then "can't block", to each of up to three
// creatures) is a multi-target slot with no mana value cap, beside Scout's
// capped one: it keeps the live per-object legality recheck of execEachTarget.
//
// Secret Tunnel (two creatures you control that share a creature type, on the
// battlefield, "can't be blocked this turn") is the battlefield shared-type
// carrier: its ExileOnMoved$ Battlefield effect goes on for the survivor when
// the other creature leaves (CR 611.2c).
var ktgtsetDeckA = []cardset.Entry{
	{Name: "Scout for Survivors", Count: 4}, {Name: "Behold the Sinister Six!", Count: 3}, {Name: "Unbury", Count: 4},
	{Name: "Driver of the Dead", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
	{Name: "Elvish Mystic", Count: 3}, {Name: "Plains", Count: 5}, {Name: "Swamp", Count: 5}, {Name: "Forest", Count: 4},
}

// Joshua is legendary: with two of them in a graveyard a chapter III return
// puts three on the battlefield, and the legend rule's two losers reach the
// graveyard in battlefield order (as gorge does), so deck B holds eight.
var ktgtsetDeckB = []cardset.Entry{
	{Name: "Joshua, Phoenix's Dominant", Count: 8}, {Name: "Raging Goblin", Count: 2},
	{Name: "Glory Seeker", Count: 2}, {Name: "Savannah Lions", Count: 2}, {Name: "Lightning Bolt", Count: 4},
	{Name: "Shock", Count: 2}, {Name: "Wrap in Flames", Count: 4}, {Name: "Secret Tunnel", Count: 4},
	{Name: "Mountain", Count: 6}, {Name: "Plains", Count: 6},
}

// Agility Bobblehead ({3},{T}: up to X target creatures you control gain
// haste, and can't be blocked this turn except by creatures with haste, X the
// Bobbleheads you control) is the variable-maximum grant with a
// ValidBlocker$ Creature.withoutHaste filter (kernel ExtExHaste). Deck C holds
// four of them beside Raging Goblin (a hasty blocker), haste-less creatures and
// burn that kills one of the creatures named (the others stay unblockable).
var ktgtsetDeckC = []cardset.Entry{
	{Name: "Agility Bobblehead", Count: 6}, {Name: "Raging Goblin", Count: 2}, {Name: "Glory Seeker", Count: 4},
	{Name: "Savannah Lions", Count: 4}, {Name: "Lightning Bolt", Count: 8}, {Name: "Shock", Count: 4},
	{Name: "Mountain", Count: 9}, {Name: "Plains", Count: 3},
}

var ktgtsetMatchups = [][2][]cardset.Entry{
	{ktgtsetDeckA, ktgtsetDeckB}, {ktgtsetDeckA, ktgtsetDeckA}, {ktgtsetDeckB, ktgtsetDeckB},
	{ktgtsetDeckC, ktgtsetDeckC}, {ktgtsetDeckC, ktgtsetDeckB}, {ktgtsetDeckC, ktgtsetDeckC}, // matchup 5: burn at one's own creatures (ktgtsetFriendlyFire)
}

// ktgtsetPolicy attacks when it can, casts and activates when offered, else
// plays at random.
func ktgtsetPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActAttack {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// ktgtsetFriendlyFire is ktgtsetPolicy aiming every target ask at a creature
// of its own seat when it can (a deck's instances are 40*seat..): burn then
// kills one of the creatures Agility Bobblehead named between the activation
// and the blockers, which random aim reaches too rarely to lockstep.
func ktgtsetFriendlyFire(seed uint64) Policy {
	base, rnd := ktgtsetPolicy(seed), RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		lo := Ref(c.D.Player) * 40
		for _, a := range c.D.Options {
			if a.Kind() == ActTarget && a.Tgt() >= lo && a.Tgt() < lo+40 {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return base.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// ktgtsetTally counts gorge's target asks that carry a set-property
// constraint (setAsks) and a mana value budget (budgetAsks).
type ktgtsetTally struct {
	seq                  uint64
	setAsks, budgetAsks  int
	sharedAsks, distinct int
}

func (p *ktgtsetTally) check(ref *GorgeSide, _ Side) *Divergence {
	if d := ref.Pending(); d != nil && d.Kind == decision.KTarget && uint64(d.Seq) != p.seq {
		p.seq = uint64(d.Seq)
		if d.SetPropMode != "" {
			p.setAsks++
			if d.SetPropMode == decision.SetPropShared {
				p.sharedAsks++
			} else {
				p.distinct++
			}
		}
		if d.HasBudget() {
			p.budgetAsks++
		}
	}
	return nil
}

// ktgtsetRun plays seeds 1..seeds of every matchup; stopAtDiv ends it at the
// first divergence.
func ktgtsetRun(t *testing.T, seeds uint64, stopAtDiv bool) (tally *ktgtsetTally, games, divs, stalls int, first string) {
	return ktgtsetRunFrom(t, seeds, stopAtDiv, 0, len(ktgtsetMatchups))
}

// ktgtsetRunFrom plays the matchups from index from up to (not including) to.
func ktgtsetRunFrom(t *testing.T, seeds uint64, stopAtDiv bool, from, to int) (tally *ktgtsetTally, games, divs, stalls int, first string) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tgtset")
	if err != nil {
		t.Fatal(err)
	}
	tally = &ktgtsetTally{}
	for mi, md := range ktgtsetMatchups {
		if mi < from || mi >= to {
			continue
		}
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%4 == 0}
			m, err := NewMatchup(reg, [2]string{"ktgtset-a", "ktgtset-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			tally.seq = 0
			pol := ktgtsetPolicy(seed)
			if mi == 5 {
				pol = ktgtsetFriendlyFire(seed)
			}
			if seed%8 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: tally.check})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				t.Errorf("matchup %d seed %d: kernel stall %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				if first == "" {
					first = "matchup " + strconv.Itoa(mi) + " seed " + strconv.FormatUint(seed, 10) + ": " + d.What + " " + d.Field
				}
				lo := max(d.At-60, 0)
				t.Logf("matchup %d seed %d %s %s at %d last actions %v\ndiff %v %s\nref %v\nker %v", mi, seed, d.What, d.Field, d.At, actionStrings(res.Actions[max(lo, d.At-8):d.At]), d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				if stopAtDiv {
					return
				}
			}
		}
	}
	return
}

func TestTgtsetLockstep(t *testing.T) {
	t.Parallel()
	tally, games, divs, stalls, first := ktgtsetRun(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls; set-property asks %d (shared %d, distinct %d), budget asks %d",
		games, divs, stalls, tally.setAsks, tally.sharedAsks, tally.distinct, tally.budgetAsks)
	if divs != 0 {
		t.Fatalf("%d divergences, first %s", divs, first)
	}
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	if tally.setAsks < 20 {
		t.Errorf("set-property asks %d, want at least 20", tally.setAsks)
	}
	if tally.budgetAsks < 20 {
		t.Errorf("budget asks %d, want at least 20", tally.budgetAsks)
	}
}

// TestTgtsetExHasteMutation: with the record never read, the attacker
// Agility Bobblehead made unblockable-except-by-haste is blockable by any
// creature, and some seed diverges.
func TestTgtsetExHasteMutation(t *testing.T) {
	dirac.DebugExHasteOff = true
	defer func() { dirac.DebugExHasteOff = false }()
	_, games, divs, _, first := ktgtsetRunFrom(t, 1024, true, 3, 4)
	if divs == 0 {
		t.Fatalf("no divergence in %d games with DebugExHasteOff", games)
	}
	t.Logf("exhaste mutation diverged after %d games: %s", games, first)
}

func TestTgtsetMutation(t *testing.T) {
	dirac.DebugTgtSetOff = true
	defer func() { dirac.DebugTgtSetOff = false }()
	_, games, divs, _, first := ktgtsetRun(t, 64, true)
	if divs == 0 {
		t.Fatalf("no divergence in %d games with DebugTgtSetOff", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, first)
}
