package oracle

import (
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// The sacrifice-only mana lockstep (spawnmana): constructed matchups over
// the generated spawnmana pool, whose carriers create "Sacrifice this
// creature: Add {C}" Eldrazi Spawn tokens (CR 605.1a: the ability is a mana
// ability; CR 605.3: it resolves immediately) and whose {C}-pip casts (the
// Devoid carriers' {1}{C} part, Old Stickfingers's {X}{B}, Sowing
// Mycospawn's and Hatchery Spider's {1}{C} kickers) have no other source of
// {C}. gorge's planner lists a sacrifice-only source as last-resort fixed
// production in battlefield order beside the choice-shaped ones
// (paymentPlanManaUnitsOnlyCompute's appended group) and its ward / unless
// window offers it while tapped, too (mana_activation.go gates only tap and
// untap costs on the source's tap state). Deck B's Zul Ashur, Lich Lord has
// ward--pay 2 life, not a mana ward, so it never opens that window; the
// ward counter stays 0 and is logged for the record.
var kspawnmanaDecks = [2][]cardset.Entry{
	{
		{Name: "Eldrazi Repurposer", Count: 4}, {Name: "Writhing Chrysalis", Count: 3},
		{Name: "Warped Tusker", Count: 4}, {Name: "Drownyard Lurker", Count: 2},
		{Name: "Sowing Mycospawn", Count: 3}, {Name: "Hatchery Spider", Count: 2},
		{Name: "Old Stickfingers", Count: 2}, {Name: "Mind Stone", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Mountain", Count: 3}, {Name: "Plains", Count: 3},
		{Name: "Island", Count: 2},
	},
	{
		{Name: "Thief of Existence", Count: 4}, {Name: "Titans' Vanguard", Count: 3},
		{Name: "Zul Ashur, Lich Lord", Count: 2}, {Name: "Old Stickfingers", Count: 2},
		{Name: "Dash Hopes", Count: 2}, {Name: "Phantasmagorian", Count: 2},
		{Name: "Mind Stone", Count: 4}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Swamp", Count: 6}, {Name: "Island", Count: 5}, {Name: "Forest", Count: 5},
		{Name: "Mountain", Count: 2},
	},
}

// spawnCounter counts, through RunConfig.Check, every distinct Eldrazi
// Spawn token that showed up on a battlefield or in a graveyard, by seat of
// the zone it showed up in, plus gorge's ward asks by seat.
type spawnCounter struct {
	seenBF   [2]map[state.ObjID]bool
	seenGY   [2]map[state.ObjID]bool
	seenGone [2]map[state.ObjID]bool
	nBF      [2]int
	nGY      [2]int
	nGone    [2]int
	ward     [2]int
	last     *GorgeSide
	seqs     map[uint64]bool
}

func newSpawnCounter() *spawnCounter {
	c := &spawnCounter{seqs: map[uint64]bool{}}
	for p := 0; p < 2; p++ {
		c.seenBF[p] = map[state.ObjID]bool{}
		c.seenGY[p] = map[state.ObjID]bool{}
		c.seenGone[p] = map[state.ObjID]bool{}
	}
	return c
}

func (c *spawnCounter) checkSpawn(z state.Zone, p state.PlayerID, id state.ObjID) {
	o := c.last.E.G.Obj(id)
	if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
		return
	}
	if o.Card.Faces[0].Name != "Eldrazi Spawn Token" {
		return
	}
	if int(p) > 1 {
		return
	}
	if z == state.ZBattlefield {
		if !c.seenBF[p][id] {
			c.seenBF[p][id] = true
			c.nBF[p]++
		}
	} else if z == state.ZGraveyard && !c.seenGY[p][id] {
		c.seenGY[p][id] = true
		c.nGY[p]++
	}
}

func (c *spawnCounter) check(ref *GorgeSide, _ Side) *Divergence {
	c.last = ref
	g := ref.E.G
	for p := state.PlayerID(0); p < 2; p++ {
		onBF := map[state.ObjID]bool{}
		for _, id := range g.Zone(state.ZBattlefield, p) {
			onBF[id] = true
			c.checkSpawn(state.ZBattlefield, p, id)
		}
		for _, id := range g.Zone(state.ZGraveyard, p) {
			c.checkSpawn(state.ZGraveyard, p, id)
		}
		// A sacrificed Spawn token ceases before the next decision, so its
		// graveyard visit is never observed. Count a token that was on the
		// battlefield and is no longer there: in this pool that is the
		// sacrifice (or a death in combat), the exercise the pool exists for.
		for id := range c.seenBF[p] {
			if !onBF[id] && !c.seenGone[p][id] {
				c.seenGone[p][id] = true
				c.nGone[p]++
			}
		}
	}
	if fr := ref.fr; fr != nil && fr.d != nil && !c.seqs[fr.seq] {
		c.seqs[fr.seq] = true
		if len(fr.d.ResumeKind) > 4 && fr.d.ResumeKind[:4] == "ward" && int(fr.d.Player) < 2 {
			c.ward[fr.d.Player]++
		}
	}
	return nil
}

func TestSpawnmanaLockstep(t *testing.T) {
	t.Parallel()
	cardsettest.Require(t, "spawnmana")

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("spawnmana")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kspawnmanaDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kspawnmanaDecks[0], kspawnmanaDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(1024)
	if s := os.Getenv("KSPAWNMANA_SEEDS"); s != "" {
		seeds = 0
		for _, ch := range s {
			seeds = seeds*10 + uint64(ch-'0')
		}
	}
	holdKick := map[Ref]bool{} // deck A's kicked casts, by card index
	for i, c := range decks[0] {
		n := c.Faces[0].Name
		if n == "Sowing Mycospawn" || n == "Hatchery Spider" {
			holdKick[Ref(i)] = true
		}
	}
	cnt := newSpawnCounter()
	var sumBF, sumGY, sumGone, sumWard [2]int
	total := 0
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"kspawnmana-a", "kspawnmana-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%4 == 1 {
			pol = kickHoldPolicy(pol, holdKick)
		}
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		*cnt = *newSpawnCounter()
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: cnt.check})
		for p := 0; p < 2; p++ {
			sumBF[p] += cnt.nBF[p]
			sumGY[p] += cnt.nGY[p]
			sumGone[p] += cnt.nGone[p]
			sumWard[p] += cnt.ward[p]
		}
		CloseSide(k)
		if os.Getenv("KSPAWNMANA_TRACE") == strconv.FormatUint(seed, 10) {
			t.Logf("seed %d actions %v", seed, actionStrings(res.Actions))
		}
		if d := res.Div; d != nil {
			div++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			if div > 8 {
				t.Fatalf("%d seeds diverged, stopping", div)
			}
		}
	}
	total = sumBF[0] + sumBF[1]
	t.Logf("%d seeds, %d diverged; Spawn tokens on battlefields %v (%d), left the battlefield %v (%d), in graveyards %v (%d); ward asks %v",
		seeds, div, sumBF, total, sumGone, sumGone[0]+sumGone[1], sumGY, sumGY[0]+sumGY[1], sumWard)
	if seeds >= 1024 {
		// coverage bars for the full run; a short seed slice can't reach
		// them (the slices are for bisecting a divergence)
		if total < 200 {
			t.Errorf("Spawn tokens created: %d (want >= 200)", total)
		}
		if g := sumGone[0] + sumGone[1]; g < 100 {
			t.Errorf("Spawn tokens sacrificed/died: %d (want >= 100)", g)
		}
	}
}

// TestSpawnmanaMutation: with the sacrifice-only mana source turned off
// (DebugIgnoreSpawnMana), the same seeds diverge -- the ability's
// classification and use are what carries the lockstep.
func TestSpawnmanaMutation(t *testing.T) {
	cardsettest.Require(t, "spawnmana")
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("spawnmana")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kspawnmanaDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	// the mutation flips the flag between matchups, and prepare runs once
	// per registered table, so each seed builds its own table
	dirac.DebugIgnoreSpawnMana = true
	defer func() { dirac.DebugIgnoreSpawnMana = false }()
	div := 0
	for seed := uint64(1); seed <= 32; seed++ {
		tbl, _, _, err := set.Matchup(kspawnmanaDecks[0], kspawnmanaDecks[1])
		if err != nil {
			t.Fatal(err)
		}
		m, err := NewMatchup(reg, [2]string{"kspawnmana-a", "kspawnmana-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%4 == 1 {
			pol = kickHoldPolicy(pol, holdKspawnKicks(decks))
		}
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			div++
		}
	}
	if div == 0 {
		t.Errorf("mutation: 0 of 32 seeds diverged with DebugIgnoreSpawnMana (want some)")
	}
}

func holdKspawnKicks(decks [2][]*cards.Card) map[Ref]bool {
	hold := map[Ref]bool{}
	for i, c := range decks[0] {
		n := c.Faces[0].Name
		if n == "Sowing Mycospawn" || n == "Hatchery Spider" {
			hold[Ref(i)] = true
		}
	}
	return hold
}

func debugKickReplay(t *testing.T, e *rules.Engine, obj state.ObjID) {
	t.Helper()
	var plan *decision.PaymentPlan
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Obj == obj && pp.Action.Mode == "" && pp.Plan != nil {
			p := *pp.Plan
			plan = &p
		}
	}
	if plan == nil {
		t.Logf("replay: no plain plan")
		return
	}
	c := e.Clone()
	step := func(src state.ObjID) bool {
		d := c.Pending()
		if d == nil {
			t.Logf("replay: pending nil before src %d", src)
			return false
		}
		if d.Kind != decision.KPriority {
			t.Logf("replay: pending kind %v player %d before src %d", d.Kind, d.Player, src)
			return false
		}
		for _, opt := range d.Options {
			if opt.Kind == "activate" && opt.Obj == src {
				err := c.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}})
				t.Logf("replay: activate src %d err %v pool %v stack %d pending %v", src, err, c.G.Players[0].Pool, len(c.G.Stack), c.Pending() != nil)
				return err == nil
			}
		}
		t.Logf("replay: no activate src %d among %d options", src, len(d.Options))
		return false
	}
	for _, a := range plan.Activations {
		if !step(a.Source) {
			return
		}
	}
	// add the two extra Spawn sacrifices the kicked cost needs
	extra := 0
	for _, id := range c.G.Zone(state.ZBattlefield, 0) {
		o := c.G.Obj(id)
		if o != nil && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Eldrazi Spawn Token" && !o.Tapped {
			if extra == 2 {
				break
			}
			if step(id) {
				extra++
			}
		}
	}
	d := c.Pending()
	if d == nil {
		t.Logf("replay: pending nil at end, pool %v stack %d", c.G.Players[0].Pool, len(c.G.Stack))
		return
	}
	t.Logf("replay: end kind %v player %d pool %v stack %d", d.Kind, d.Player, c.G.Players[0].Pool, len(c.G.Stack))
	for _, opt := range d.Options {
		t.Logf("replay: opt kind=%s obj=%d mode=%q ability=%d alt=%d", opt.Kind, opt.Obj, opt.Mode, opt.Ability, opt.AltCostIndex)
	}
}
