package kgen

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The hosted-Commander choose shapes (dirac gap cli-20261127T065130Z-0be3787f)
// in lockstep. Each probe here is a real hosted-deck card whose shape kgen
// refused as of the 11-27 census (~/op/cmdr-tix/census-1127.tsv):
//
//   - Price of Progress (valgavoth-endless-punishment): SP$ RepeatEach over
//     every player whose lone DealDamage body reads NumDmg$ X with
//     X = Count$Valid Land.nonBasic+RememberedPlayerCtrl/Times.2, under
//     DamageMap$ True (one held damage batch for the whole loop). The census
//     reason was `RepeatEach body NumDmg$ "X"`.
//   - Cataclysmic Gearhulk (growing-threat): a ChangesZone trigger's
//     DB$ ChooseCard | Defined$ Player | Choices$ Permanent.nonLand |
//     ChooseEach$ "Artifact & Creature & Enchantment & Planeswalker" |
//     ControlledByPlayer$ Chooser, then SacrificeAll nonChosenCard. The census
//     reason was `ChooseEach shape`.
//
//   - Planetary Annihilation (hearthhull-worldseed-landfall, pro-shaper): SP$
//     ChooseCard | Defined$ Player | Choices$ Land | ControlledByPlayer$
//     Chooser | Amount$ 6 | Mandatory$ True, then SacrificeAll Land.nonChosen-
//     Card. The census reason was `mandatory ChooseCard ControlledByPlayer$
//     "Chooser"`.
//   - Dance with Calamity (tinker-time): SP$ Shuffle, then a GenericChoice
//     between DB$ Repeat | RepeatOptional$ True (body: DB$ Dig | DigNum$ 1 |
//     ChangeNum$ All | DestinationZone$ Exile | RememberChanged$ True) and the
//     bare Play of the remembered cards. The census reason was `Repeat
//     RepeatOptional$ body shape Dig$ (no change test)`.
//   - Together Forever (call-for-backup): AB$ ChooseCard | ValidTgts$
//     Creature.HasCounters carrying its card target to a DelayedTrigger
//     (RememberObjects$ Targeted, dies this turn, return that card to its
//     owner's hand). The census reason was `optional ChooseCard with
//     ValidTgts$`.
//
// The form check (TestChooseShapesHostedLowering) proves each lowering
// through the ordinary lowerCard path; the lockstep plays the real cards from
// the corpus, both seats running the same decks, gorge leading the decisions.
// The cast tracker in the policy is the precondition the compared decisions
// depend on: it fails the test outright when a seed never plays a probe (the
// shape would otherwise be covered vacuously), and a kernel that skipped or
// miscounted a per-player ask diverges from gorge's decision stream.

// gearhulkTypeMask is the ChooseEachType ask's type mask for the Gearhulk
// arm: artifact, creature, enchantment and planeswalker, the four
// nonland-permanent types its ChooseEach$ names.
var gearhulkTypeMask = int(kop.TArtifact | kop.TCreature | kop.TEnchantment | kop.TPlaneswalker)

// chooseShapesProbes are the five hosted cards the lockstep plays.
var chooseShapesProbes = []string{"Price of Progress", "Cataclysmic Gearhulk", "Planetary Annihilation", "Dance with Calamity", "Together Forever"}

// chooseShapesFillers are the deck fillers: creatures to soak the Gearhulk's
// DamageAll, and Rupture Spire so a nonbasic land sits on the battlefield and
// Price of Progress' per-player X is nonzero.
var chooseShapesFillers = []string{"Grizzly Bears", "Hill Giant", "Mountain", "Plains", "Rupture Spire"}

// chooseShapesDecks are the two seats' 40-card decks. Both play both probes.
func chooseShapesDecks() [2][]cardset.Entry {
	d0 := []cardset.Entry{
		{Name: "Price of Progress", Count: 2},
		{Name: "Cataclysmic Gearhulk", Count: 2},
		{Name: "Planetary Annihilation", Count: 3},
		{Name: "Dance with Calamity", Count: 3},
		{Name: "Together Forever", Count: 3},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 8},
		{Name: "Plains", Count: 8},
		{Name: "Rupture Spire", Count: 7},
	}
	d1 := []cardset.Entry{
		{Name: "Price of Progress", Count: 2},
		{Name: "Cataclysmic Gearhulk", Count: 2},
		{Name: "Planetary Annihilation", Count: 3},
		{Name: "Dance with Calamity", Count: 3},
		{Name: "Together Forever", Count: 3},
		{Name: "Hill Giant", Count: 4},
		{Name: "Mountain", Count: 9},
		{Name: "Plains", Count: 10},
		{Name: "Rupture Spire", Count: 4},
	}
	return [2][]cardset.Entry{slices.Clone(d0), slices.Clone(d1)}
}

// chooseShapesPool is the fixture pool: the probes and fillers only.
func chooseShapesPool(t *testing.T, cardsDir string) *pool {
	t.Helper()
	names := append(append([]string{}, chooseShapesProbes...), chooseShapesFillers...)
	p, err := loadCardNames(names, cardsDir)
	if err != nil {
		t.Fatalf("fixture pool: %v", err)
	}
	return p
}

// TestChooseShapesHostedLowering: both hosted cards lower with the shape's
// own kernel instructions and no refusals.
func TestChooseShapesHostedLowering(t *testing.T) {
	t.Parallel()
	ratchetCards(t)

	pop := lowerCorpusCard(t, "Price of Progress")
	for _, rf := range pop.Refusals {
		t.Errorf("Price of Progress refused: %s", rf.Reason)
	}
	sawRepeat, sawBatch, sawDmg := false, false, false
	for _, a := range pop.Abilities {
		for i, in := range a.Prog {
			if in.Op == "RepeatEach" && in.Obj.Kind == rEachPlayer {
				sawRepeat = true
				if in.C&kop.RepeatEachDmgBatch == 0 {
					t.Errorf("Price of Progress: RepeatEach C=%d lacks the DamageMap$ damage batch", in.C)
				}
				sawBatch = true
				if i+1 >= len(a.Prog) {
					t.Fatalf("Price of Progress: RepeatEach closes nothing")
				}
				body := a.Prog[i+1]
				if body.Op != "DealDamage" || body.Obj.Kind != rEach || body.A.Kind == vNone {
					t.Errorf("Price of Progress: body %v, want a DealDamage of the repeated player with a per-player amount", body)
				} else {
					sawDmg = true
				}
			}
		}
	}
	if !sawRepeat || !sawBatch || !sawDmg {
		t.Errorf("Price of Progress lowered no per-player batched DealDamage (repeat %v batch %v dmg %v)", sawRepeat, sawBatch, sawDmg)
	}

	pa := lowerCorpusCard(t, "Planetary Annihilation")
	for _, rf := range pa.Refusals {
		t.Errorf("Planetary Annihilation refused: %s", rf.Reason)
	}
	sawPick := false
	for _, a := range pa.Abilities {
		for _, in := range a.Prog {
			if in.Op == "ChooseCard" && in.CS == "EachAmount" {
				sawPick = true
				if in.Obj.Kind != rEachPlayer || in.A.Kind != vConst || in.A.N != 6 || in.F == nil {
					t.Errorf("Planetary Annihilation: pick %v, want every player, six, a filter", in)
				}
			}
		}
	}
	if !sawPick {
		t.Errorf("Planetary Annihilation lowered no each-player counted ChooseCard")
	}

	dc := lowerCorpusCard(t, "Dance with Calamity")
	for _, rf := range dc.Refusals {
		t.Errorf("Dance with Calamity refused: %s", rf.Reason)
	}
	sawHead, sawTail := false, false
	for _, a := range dc.Abilities {
		for _, in := range a.Prog {
			if in.Op == "RepeatOptional" {
				switch in.B.N {
				case 2:
					sawHead = true
				case 1:
					sawTail = true
				}
			}
		}
	}
	if !sawHead || !sawTail {
		t.Errorf("Dance with Calamity lowered no pure RepeatOptional bracket (head %v tail %v)", sawHead, sawTail)
	}

	tf := lowerCorpusCard(t, "Together Forever")
	for _, rf := range tf.Refusals {
		t.Errorf("Together Forever refused: %s", rf.Reason)
	}
	sawDelay := false
	for _, a := range tf.Abilities {
		for _, in := range a.Prog {
			if in.Op == "DelayedTrigger" && in.CS == "DiesThisTurn" && in.Obj.Kind == rTarget {
				sawDelay = true
			}
		}
	}
	if !sawDelay {
		t.Errorf("Together Forever lowered no dies-this-turn DelayedTrigger over its target")
	}

	gh := lowerCorpusCard(t, "Cataclysmic Gearhulk")
	for _, rf := range gh.Refusals {
		t.Errorf("Cataclysmic Gearhulk refused: %s", rf.Reason)
	}
	sawEach, sawSac := false, false
	for _, a := range gh.Abilities {
		for _, in := range a.Prog {
			switch in.Op {
			case "ChooseEachType":
				sawEach = true
				if in.Obj.Kind != rEachPlayer {
					t.Errorf("Cataclysmic Gearhulk: ChooseEachType Obj %v, want every player", in.Obj)
				}
				if in.A.Kind != vConst || in.A.N != gearhulkTypeMask {
					t.Errorf("Cataclysmic Gearhulk: ChooseEachType A=%v, want the artifact-creature-enchantment-planeswalker mask %d", in.A, gearhulkTypeMask)
				}
			case "SacrificeAll":
				sawSac = true
				if in.F == nil {
					t.Errorf("Cataclysmic Gearhulk: SacrificeAll with no filter, want the nonChosenCard pool")
				}
			}
		}
	}
	if !sawEach || !sawSac {
		t.Errorf("Cataclysmic Gearhulk lowered no ChooseEachType+SacrificeAll pair (each %v sac %v)", sawEach, sawSac)
	}
}

// TestChooseShapesHostedLoweringRefusals: the acceptances stay exact. A
// RepeatEach body whose amount modifier is not the admitted numeric ones
// keeps refusing, and a ChooseEach arm outside the two Choices$ spellings
// whose type list they print keeps refusing the shape.
func TestChooseShapesHostedLoweringRefusals(t *testing.T) {
	t.Parallel()
	ratchetCards(t)

	// the X tail the whitelist does not admit: a slash tail before Times
	if got := lowerScript(t, "Probe Progress",
		"Name:Probe Progress\nManaCost:1 R\nTypes:Instant\n"+
			"A:SP$ RepeatEach | RepeatPlayers$ Player | RepeatSubAbility$ DBDamage | DamageMap$ True\n"+
			"SVar:DBDamage:DB$ DealDamage | NumDmg$ X | Defined$ Remembered\n"+
			"SVar:X:Count$Valid Land.untapped+RememberedPlayerCtrl/Plus.1/Times.2\n"+
			"Oracle:Probe.\n"); !strings.Contains(strings.Join(got, "\n"), "RepeatEach body NumDmg$ \"X\"") {
		t.Errorf("the plus-then-times probe lowered or refused differently: %v", got)
	}

	// a ChooseEach$ list out of the kernel's type order (gorge asks in the printed order)
	if got := lowerScript(t, "Probe Gearhulk",
		"Name:Probe Gearhulk\nManaCost:3 W W\nTypes:Sorcery\n"+
			"A:SP$ ChooseCard | Defined$ Player | Choices$ Permanent.nonLand | "+
			"ChooseEach$ Creature & Artifact | ControlledByPlayer$ Chooser | Reveal$ True | Mandatory$ True\n"+
			"Oracle:Probe.\n"); !strings.Contains(strings.Join(got, "\n"), "ChooseEach shape") {
		t.Errorf("the out-of-order probe lowered or refused differently: %v", got)
	}
}

// chooseShapesSeen records the preconditions the lockstep comparison depends
// on: each probe has left its owner's library (battlefield or graveyard) at
// least once, and Planetary Annihilation resolved while a player controlled
// more than six lands, so its keep-six election and its sacrifice of the rest
// really ran in both engines.
type chooseShapesSeen struct {
	pop, gearhulk, planetary, planetaryBig, dance, danceExiled, togetherBack bool
}

// chooseShapesGame is one game's scan state: whether Planetary Annihilation
// was already in a graveyard at the previous pick, and the most lands any
// player controlled then.
type chooseShapesGame struct {
	paInGY   bool
	maxLands int
	inGY     map[state.ObjID]bool // every card seen in a graveyard this game
}

// chooseShapesScan walks gorge's zones for the probes.
func chooseShapesScan(g *state.Game, seen *chooseShapesSeen, gm *chooseShapesGame) {
	if g == nil {
		return
	}
	paGY, maxLands := false, 0
	if gm.inGY == nil {
		gm.inGY = map[state.ObjID]bool{}
	}
	for p := state.PlayerID(0); p < 2; p++ {
		lands := 0
		for _, id := range g.Zone(state.ZGraveyard, p) {
			gm.inGY[id] = true
		}
		for _, id := range g.Zone(state.ZHand, p) {
			if gm.inGY[id] {
				seen.togetherBack = true // only Together Forever returns a card to a hand in this fixture
			}
		}
		for _, z := range []state.Zone{state.ZBattlefield, state.ZGraveyard} {
			for _, id := range g.Zone(z, p) {
				o := g.Obj(id)
				if o == nil || o.Card == nil {
					continue
				}
				switch o.Card.Faces[0].Name {
				case "Price of Progress":
					seen.pop = true
				case "Cataclysmic Gearhulk":
					seen.gearhulk = true
				case "Planetary Annihilation":
					seen.planetary = true
					paGY = paGY || z == state.ZGraveyard
				case "Dance with Calamity":
					seen.dance = true
				case "Mountain", "Plains", "Rupture Spire":
					if z == state.ZBattlefield {
						lands++
					}
				}
			}
		}
		maxLands = max(maxLands, lands)
		if len(g.Zone(state.ZExile, p)) > 0 {
			seen.danceExiled = true // only Dance with Calamity exiles in this fixture
		}
	}
	if paGY && !gm.paInGY && gm.maxLands > 6 {
		seen.planetaryBig = true
	}
	gm.paInGY, gm.maxLands = paGY, maxLands
}

// chooseShapesRun plays the fixture's lockstep over seeds 1..seeds. report
// fails on each divergence (else the first stops the run).
func chooseShapesRun(t *testing.T, seeds uint64, report bool) (div int, seen chooseShapesSeen) {
	t.Helper()
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("no corpus: %v", err)
	}
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	p := chooseShapesPool(t, cardsDir)
	pd, _, _, err := tableOf(p, "choose-shapes-fixture", nil)
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	decks := chooseShapesDecks()
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s := range gd {
		for _, e := range decks[s] {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := oracle.NewMatchup(reg, [2]string{"shapes-a", "shapes-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := oracle.Config{}
		if seed%4 == 0 {
			cfg = oracle.Config{GorgeAutopaySurface: true}
		}
		k, err := oracle.SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := chooseShapesPolicy(seed, &seen)
		if seed%8 == 0 {
			pol = oracle.RandomPolicy(seed)
		}
		res := oracle.Lockstep(m, seed, k, pol, oracle.RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		oracle.CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
				return div, seen
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
	return div, seen
}

// chooseShapesPolicy casts and plays lands whenever it can and attacks and
// blocks with everything; on every pick it scans gorge's zones for the two
// probes.
func chooseShapesPolicy(seed uint64, seen *chooseShapesSeen) oracle.Policy {
	rnd := oracle.RandomPolicy(seed)
	var gm chooseShapesGame
	return oracle.PolicyFunc(func(c *oracle.PickCtx) oracle.Canon {
		if c.G != nil {
			chooseShapesScan(c.G.E.G, seen, &gm)
		}
		buf := make([]oracle.Canon, 0, 8)
		for _, a := range c.D.Options {
			switch c.D.Kind {
			case oracle.DAttackers:
				if a.Kind() == oracle.ActAttack || a.Kind() == oracle.ActDone || a.Kind() == oracle.ActPass {
					buf = append(buf, a)
				}
			case oracle.DBlockers:
				if a.Kind() == oracle.ActBlock || a.Kind() == oracle.ActDone || a.Kind() == oracle.ActPass {
					buf = append(buf, a)
				}
			default:
				switch a.Kind() {
				case oracle.ActCast, oracle.ActCastFace, oracle.ActActivate, oracle.ActPlayLand:
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 || len(buf) == len(c.D.Options) {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&oracle.PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// TestChooseShapesHostedLockstep: the five hosted choose shapes play
// decision-for-decision with gorge over the seeds, and both probes were
// genuinely played (the precondition the comparison depends on: each left
// its owner's library at least once, so its resolution ran in both engines).
func TestChooseShapesHostedLockstep(t *testing.T) {
	t.Parallel()
	div, seen := chooseShapesRun(t, 24, true)
	if !seen.pop || !seen.gearhulk || !seen.planetaryBig || !seen.danceExiled || !seen.togetherBack {
		t.Fatalf("over 24 seeds the probes were played %+v: the lockstep never reached the shapes (Planetary Annihilation needs more than six lands in play)", seen)
	}
	if div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
	t.Logf("probes played %+v over 24 seeds", seen)
}
