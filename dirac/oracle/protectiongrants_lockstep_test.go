package oracle

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The protection-forms lockstep (gap4-05): a Protection effect with a fixed
// Gains$ (Crimson Acolyte, Obsidian Acolyte, Tel-Jilad Defiance, Keeper of
// Kookus, Grotesque Hybrid's list, Tower of the Magistrate's Artifact), a
// granted Protection:<spec> with an exception field (the Ward Auras, Tattoo
// Ward, Spectra Ward), a grant from a creature host (Feline Sovereign, Hungry
// Lynx, Masked Gorgon, Pristine Angel), a PumpAll's quality (Spare from Evil)
// and the multicolored, instant and sorcery qualities (Guildscorn Ward,
// Hexdrinker, Devoted Caretaker), against coloured, colourless, artifact and
// subtype creatures, burn and removal. The expected answer at every decision
// is gorge's.
var protectionGrantsDecks = [2][]cardset.Entry{
	{
		{Name: "Black Ward", Count: 1}, {Name: "Blue Ward", Count: 1}, {Name: "Green Ward", Count: 1},
		{Name: "Red Ward", Count: 1}, {Name: "White Ward", Count: 1}, {Name: "Tattoo Ward", Count: 1},
		{Name: "Spectra Ward", Count: 1}, {Name: "Guildscorn Ward", Count: 1}, {Name: "Crimson Acolyte", Count: 1},
		{Name: "Obsidian Acolyte", Count: 1}, {Name: "Devoted Caretaker", Count: 1}, {Name: "Spare from Evil", Count: 1},
		{Name: "Tel-Jilad Defiance", Count: 1}, {Name: "Feline Sovereign", Count: 1}, {Name: "Hungry Lynx", Count: 1},
		{Name: "Hexdrinker", Count: 1}, {Name: "Prowling Caracal", Count: 1}, {Name: "Sanctuary Cat", Count: 1},
		{Name: "Silver Knight", Count: 1}, {Name: "Pristine Angel", Count: 1}, {Name: "Willow Priestess", Count: 1},
		// the Choices$ asks: the colorless (Giver of Runes), the artifact
		// (Apostle's Blessing, Razor Barrier, Jeweled Spirit) and the
		// card-type (Pippin) offers
		{Name: "Giver of Runes", Count: 1}, {Name: "Apostle's Blessing", Count: 2},
		{Name: "Razor Barrier", Count: 1}, {Name: "Pippin, Guard of the Citadel", Count: 1}, {Name: "Jeweled Spirit", Count: 1},
		{Name: "Plains", Count: 8}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Keeper of Kookus", Count: 2}, {Name: "Goblin Wizard", Count: 1}, {Name: "Hell-Bent Raider", Count: 2},
		{Name: "Grotesque Hybrid", Count: 1}, {Name: "Masked Gorgon", Count: 1}, {Name: "Muck Rats", Count: 2},
		{Name: "Gutter Skulk", Count: 2}, {Name: "Ornithopter", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Terror", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Tower of the Magistrate", Count: 1}, {Name: "Mountain", Count: 9}, {Name: "Swamp", Count: 9},
	},
}

func TestProtectionFormsTwoSeatLockstep(t *testing.T) {
	t.Parallel()
	skipWithoutRepoCorpus(t)
	lockstepSet(t, "protectiongrants", protectionGrantsDecks, 150, "KPROTECTIONGRANTS_SEEDS")
}

// protectionGrantsPod is the fixture's four decks: the two above, twice, so
// each pairing of a granting seat and a shooting seat recurs.
func protectionGrantsPod(t *testing.T) venturePod {
	t.Helper()
	d0, d1 := protectionGrantsDecks[0], protectionGrantsDecks[1]
	p := newVenturePodSet(t, "protectiongrants", [4][]cardset.Entry{d0, d1, d1, d0})
	return p
}

// newVenturePodSet is newVenturePodOf over the named fixture pool.
func newVenturePodSet(t *testing.T, pool string, decks [4][]cardset.Entry) venturePod {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName(pool)
	if err != nil {
		t.Fatal(err)
	}
	lists := make([][]cardset.Entry, len(decks))
	for i, d := range decks {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
		lists[i] = d
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	return venturePod{reg: reg, set: set, tbl: tbl, decks: decks}
}

// protectionGrantsSeen is what a pod's gorge side showed at the decisions
// both sides agreed on: a fixed Protection grant on a creature (a "Protection
// from" keyword of a colour or of artifacts that the creature does not print),
// and a Ward Aura still attached to a creature that has its protection.
type protectionGrantsSeen struct{ fixed, ward int }

// protectionChoiceTally is the pod's policy with the protection choices'
// answered qualities recorded: the offer label the gorge side's own option
// carries for the picked canonical action, so a run can be required to have
// actually chosen colorless and artifacts.
type protectionChoiceTally struct {
	inner Policy
	seen  map[string]int
}

func (t protectionChoiceTally) Pick(c *PickCtx) Canon {
	a := t.inner.Pick(c)
	if c.G == nil {
		return a
	}
	if d := c.G.Pending(); d != nil && d.Kind == decision.KChoose && d.ResumeKind == "protectionchoice" {
		t.seen["posed"]++
		matched := false
		for i := range d.Options {
			if ca, ok := c.G.optionAction(d, &d.Options[i]); ok {
				t.seen["canon:"+d.Options[i].Label+"#"+strconv.FormatUint(uint64(ca), 16)]++
				if ca == a {
					t.seen[d.Options[i].Label]++
					matched = true
					break
				}
			}
		}
		if !matched {
			t.seen["unmatched:"+strconv.FormatUint(uint64(a), 16)]++
		}
	}
	return a
}

// play plays one seed of the pod like venturePod.play, tallying what it saw.
func (p venturePod) playSeen(t *testing.T, seed uint64, seen *protectionGrantsSeen, tally protectionChoiceTally) Result {
	t.Helper()
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	decks := p.decks
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(decks[0]), expand(decks[1])},
		MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{expand(decks[2]), expand(decks[3])}, Seed: seed, PassElision: true, Surface: "full"}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, 40
	counted := map[state.ObjID]bool{}
	rc.Check = func(g *GorgeSide, _ Side) *Divergence {
		for pl := 0; pl < 4; pl++ {
			for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(pl)) {
				o := g.E.G.Obj(id)
				if o == nil || o.Face() == nil || counted[id] {
					continue
				}
				for _, kw := range g.E.Keywords(id) {
					if (kw == "Protection from Artifact" || kw == "Protection from white" && !slices.Contains(o.Face().Keywords, kw)) && o.Face().IsCreature() {
						seen.fixed++
						counted[id] = true
					}
				}
				if o.AttachedTo != 0 && strings.HasSuffix(o.Face().Name, " Ward") {
					seen.ward++
					counted[id] = true
				}
			}
		}
		return nil
	}
	k, err := SetSide(p.set, p.tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	tally.inner = castFirstPolicy(seed)
	return LockstepLed(m, seed, lead, k, tally, rc)
}

func TestProtectionFormsFourSeatLockstep(t *testing.T) {
	skipWithoutRepoCorpus(t)
	p := protectionGrantsPod(t)
	var seen protectionGrantsSeen
	tally := protectionChoiceTally{seen: map[string]int{}}
	for _, seed := range []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12} {
		res := p.playSeen(t, seed, &seen, tally)
		if res.Div != nil {
			ventureFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	t.Logf("fixed grants seen %d, Ward Auras kept attached %d, protection choices answered %v", seen.fixed, seen.ward, tally.seen)
	if seen.fixed == 0 || seen.ward == 0 {
		t.Fatalf("the pod never showed a fixed grant (%d) and a Ward Aura attached (%d)", seen.fixed, seen.ward)
	}
	if tally.seen["colorless"] == 0 || tally.seen["Artifact"] == 0 {
		t.Fatalf("the seeds never answered a colorless (%d) or an artifact (%d) protection choice", tally.seen["colorless"], tally.seen["Artifact"])
	}
}

// TestProtectionWardExceptionMutation: with the granted protection's exception
// dropped (DebugProtectionNoWardException), a Ward Aura's own protection
// removes it at the state-based check, so a pod that casts one diverges.
func TestProtectionWardExceptionMutation(t *testing.T) {
	skipWithoutRepoCorpus(t)
	dirac.DebugProtectionNoWardException = true
	defer func() { dirac.DebugProtectionNoWardException = false }()
	p := protectionGrantsPod(t)
	for seed := uint64(1); seed <= 12; seed++ {
		if res, _, _ := p.play(t, seed, 40); res.Div != nil {
			t.Logf("mutation diverged at seed %d decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("dropping the exception diverged on no seed: no Ward Aura stayed attached")
}
