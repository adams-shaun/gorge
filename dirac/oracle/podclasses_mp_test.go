package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Four-seat pods of the card classes the Commander pod census
// (TestCommanderPodTwoSeatCensus) once held to two seats for reading "the
// opponent" as one player. Each class plays N four-seat games of one deck in
// every seat in lockstep with gorge, watching for the multi-opponent case at
// gorge's own decisions, and has a mutation that reads the class the old
// two-seat way and must diverge.

// podClassStats counts the games and the watched multi-opponent decisions.
type podClassStats struct {
	games, divs, stalls, hits int
}

// podClassSteer, when a test sets it for its run, picks first: its action
// when it reports one, else podClassPolicy's usual preference.
var podClassSteer func(c *PickCtx) (Canon, bool)

// podClassPolicy casts, plays lands, attacks and takes targets whenever
// offered, else random; watch reports whether gorge's posed decision is the
// class's multi-opponent case.
func podClassPolicy(seed uint64, watch func(c *PickCtx) bool, st *podClassStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.G.fr != nil && watch(c) {
			st.hits++
		}
		if podClassSteer != nil {
			if a, ok := podClassSteer(c); ok {
				return a
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActAttack, ActPlayLand, ActTarget:
				buf = append(buf, a)
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

// runPodClass plays seeds four-seat games of deck in every seat, lowered in
// process (kgen.TableFromCards), with mut set to on for the run.
func runPodClass(t *testing.T, deck []cardset.Entry, seeds int, watch func(c *PickCtx) bool, mut *bool, on bool) podClassStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var pool []*cards.Card
	for _, e := range deck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		pool = append(pool, c)
	}
	pd, err := kgen.TableFromCards(reg, pool, "podclass-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	tbl, ids, err := set.MatchupN([][]cardset.Entry{deck, deck, deck, deck})
	if err != nil {
		t.Fatal(err)
	}
	if dirac.TwoSeatOnly(tbl) {
		t.Fatalf("the table is still two-seat only: %v", dirac.TwoSeatCauses(tbl))
	}
	old := *mut
	*mut = on
	defer func() { *mut = old }()
	var st podClassStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		m, err := KernelMatchupN(reg, tbl, []string{"pod-a", "pod-b", "pod-c", "pod-d"}, ids, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{MaxDecisions: 4000, MaxTurns: 40}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, podClassPolicy(seed, watch, &st), rc)
		CloseSide(k)
		st.games++
		if d := res.Div; d != nil {
			st.divs++
			if !on {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			st.stalls++
			if !on {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
	}
	return st
}

// podClassLockstep runs the class clean (0 divergences, 0 stalls, at least
// minHits watched decisions) and under its mutation (some game diverges).
func podClassLockstep(t *testing.T, deck []cardset.Entry, seeds, minHits int, watch func(c *PickCtx) bool, mut *bool) {
	t.Run("clean", func(t *testing.T) {
		st := runPodClass(t, deck, seeds, watch, mut, false)
		t.Logf("%d games, %d divergences, %d stalls, %d watched decisions", st.games, st.divs, st.stalls, st.hits)
		if st.hits < minHits {
			t.Errorf("only %d watched decisions, want >= %d", st.hits, minHits)
		}
	})
	t.Run("mutation", func(t *testing.T) {
		st := runPodClass(t, deck, seeds, watch, mut, true)
		if st.divs == 0 {
			t.Fatalf("mutation left %d games with no divergence", st.games)
		}
		t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
	})
}

// One target for each opponent (TargetsForEachPlayer$ True | TargetMax$
// OneEach over Permanent.OppCtrl: Grasp of Fate, Dismantling Wave, Mass
// Mutiny). gorge asks one target slot whose maximum is the number of
// distinct controllers among the candidates, one pick per controller
// (oneEachTargetBounds, Option.Group; CR 601.2c); dirac's
// different-controllers slot (TgfDiffController) is bounded the same way
// (diffCtrlCap) and offers no second pick from a controller already picked
// (multiCands). The mutation (DebugOneEachOne) reads the controllers as
// one, the old two-seat reading.
var oneEachPodDeck = []cardset.Entry{
	{Name: "Grasp of Fate", Count: 2}, {Name: "Mass Mutiny", Count: 5},
	{Name: "Dismantling Wave", Count: 5},
	{Name: "Savannah Lions", Count: 4}, {Name: "Goblin Piker", Count: 4},
	{Name: "Ornithopter", Count: 3},
	{Name: "Plains", Count: 9}, {Name: "Mountain", Count: 8},
}

// oneEachWatch: gorge's posed target ask has a one-per-controller group
// spanning two or more opponents, and a pick of it is already made (the
// pick of a second opponent's permanent, or done).
func oneEachWatch(c *PickCtx) bool {
	d := c.G.fr.d
	if d == nil || d.Kind != decision.KTarget || d.Max < 2 || len(c.G.fr.chosen) == 0 {
		return false
	}
	groups := map[string]bool{}
	for _, o := range d.Options {
		if strings.HasPrefix(o.Group, "target-controller-") {
			groups[o.Group] = true
		}
	}
	return len(groups) >= 2
}

// TestFourSeatOneEachOpponentLockstep: four-seat "for each opponent, up to
// one target" games play in lockstep, with the ask spanning several
// opponents.
func TestFourSeatOneEachOpponentLockstep(t *testing.T) {
	podClassLockstep(t, oneEachPodDeck, 24, 10, oneEachWatch, &dirac.DebugOneEachOne)
}

// attackingYou (Creature.attackingYou: Arachnogenesis's and Sentry Bot's
// count, Everett K. Ross's trigger, Firemane Commando's condition). gorge
// reads an attacker whose defending seat is the evaluating player
// (attackingYou: o.Attacking == you; CR 506.2); dirac reads the attacker's
// defending seat (kop.PredAttackingYou, attackingYou). The mutation
// (DebugAttackingYouOpp) reads any attacker an opponent controls, the old
// two-seat reading.
var attackingYouPodDeck = []cardset.Entry{
	{Name: "Arachnogenesis", Count: 5}, {Name: "Sentry Bot", Count: 4},
	{Name: "Firemane Commando", Count: 4}, {Name: "Everett K. Ross, Hapless Attaché", Count: 2},
	{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4},
	{Name: "Grizzly Bears", Count: 3},
	{Name: "Plains", Count: 8}, {Name: "Forest", Count: 6},
}

var attackingYouCards = []string{"Arachnogenesis", "Sentry Bot", "Firemane Commando", "Everett K. Ross, Hapless Attaché"}

// attackingYouWatch: a player gorge asks holds or controls a watched card
// while an attacker is attacking another player, so "attacking you" and
// "attacking an opponent of yours" differ.
func attackingYouWatch(c *PickCtx) bool {
	g := c.G.E.G
	p := state.PlayerID(c.G.fr.d.Player)
	if p == g.Active {
		return false
	}
	other := false
	for q := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(q)) {
			if o := g.Obj(id); o != nil && o.IsAttacking && o.Attacking != p {
				other = true
			}
		}
	}
	if !other {
		return false
	}
	for _, z := range []state.Zone{state.ZHand, state.ZBattlefield} {
		for _, id := range g.Zone(z, p) {
			if o := g.Obj(id); o != nil && o.Face() != nil && slices.Contains(attackingYouCards, o.Face().Name) {
				return true
			}
		}
	}
	return false
}

// TestFourSeatAttackingYouLockstep: four-seat attackingYou games play in
// lockstep, with watched cards in the hands and on the battlefields of
// players another player's attackers pass by.
func TestFourSeatAttackingYouLockstep(t *testing.T) {
	podClassLockstep(t, attackingYouPodDeck, 24, 10, attackingYouWatch, &dirac.DebugAttackingYouOpp)
}

// OppNonTriggeredTarget (Hydra Omnivore: "whenever it deals combat damage
// to an opponent, it deals that much damage to each other opponent"). gorge
// damages every living opponent but the trigger's damaged player, from seat
// 0 (validPlayers; CR 102.2); dirac's OpDamageAll bit 32 does the same
// (damageOtherOpps). The mutation (DebugOtherOppsAll) damages the damaged
// opponent again too.
var otherOppsPodDeck = []cardset.Entry{
	{Name: "Hydra Omnivore", Count: 8}, {Name: "Llanowar Elves", Count: 6},
	{Name: "Elvish Mystic", Count: 4}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Forest", Count: 18},
}

// otherOppsWatch: a Hydra Omnivore is attacking with three or more players
// alive, so its trigger reaches another opponent.
func otherOppsWatch(c *PickCtx) bool {
	g := c.G.E.G
	alive := 0
	for p := range g.Players {
		if !g.Players[p].Lost {
			alive++
		}
	}
	if alive < 3 {
		return false
	}
	for _, id := range g.Zone(state.ZBattlefield, g.Active) {
		if o := g.Obj(id); o != nil && o.IsAttacking && o.Face() != nil && o.Face().Name == "Hydra Omnivore" {
			return true
		}
	}
	return false
}

// TestFourSeatOtherOpponentsLockstep: four-seat Hydra Omnivore games play
// in lockstep, with Hydras connecting while three or more players live.
func TestFourSeatOtherOpponentsLockstep(t *testing.T) {
	podClassLockstep(t, otherOppsPodDeck, 24, 10, otherOppsWatch, &dirac.DebugOtherOppsAll)
}

// TriggeredSourceController on a SpellAbility.OppCtrl watcher (Thunderbreak
// Regent: "whenever a Dragon you control becomes the target of a spell or
// ability an opponent controls, it deals 3 damage to that player"). gorge
// damages the targeting spell or ability's controller (TriggerSource's
// controller, CR 603.2); dirac's trigger player carries that controller,
// read as the trigger is queued while the targeting entry is on the stack
// (kop.TrigNSrcCtrl). The mutation (DebugSrcCtrlListener) keeps the
// listener's controller.
var thunderbreakPodDeck = []cardset.Entry{
	{Name: "Thunderbreak Regent", Count: 8}, {Name: "Shock", Count: 6},
	{Name: "Prodigal Pyromancer", Count: 4}, {Name: "Goblin Piker", Count: 4},
	{Name: "Mountain", Count: 18},
}

// thunderbreakWatch: gorge asks a target for a spell or ability with another
// player's Thunderbreak Regent among the options, three or more players
// alive.
func thunderbreakWatch(c *PickCtx) bool {
	d := c.G.fr.d
	if d == nil || d.Kind != decision.KTarget {
		return false
	}
	g := c.G.E.G
	alive := 0
	for p := range g.Players {
		if !g.Players[p].Lost {
			alive++
		}
	}
	if alive < 3 {
		return false
	}
	for _, o := range d.Options {
		if ob := g.Obj(o.Obj); o.Obj != 0 && ob != nil && ob.Controller != d.Player && ob.Face() != nil && ob.Face().Name == "Thunderbreak Regent" {
			return true
		}
	}
	return false
}

// TestFourSeatThunderbreakLockstep: four-seat Thunderbreak Regent games
// play in lockstep, with Regents targeted by other players' spells and
// abilities.
func TestFourSeatThunderbreakLockstep(t *testing.T) {
	podClassLockstep(t, thunderbreakPodDeck, 24, 10, thunderbreakWatch, &dirac.DebugSrcCtrlListener)
}

// The Regent's "that player" once the targeting spell or ability has left
// the stack before the trigger resolves. gorge reads TriggeredSourceController
// at resolution (effects.TriggeredSourceController): the live controller of
// the targeting spell, or of an ability's source object, and its owner once
// that object has left the stack and the battlefield (a spell countered in
// response, a Mogg Fanatic sacrificed as the cost). It is not last-known
// information (CR 608.2h). dirac reads the same object again as the trigger
// resolves (trig_srcctrllive.go). The mutation (DebugBtSrcCtrlGone) names the
// Regent's controller for a departed source and must diverge.
var thunderbreakCounterPodDeck = []cardset.Entry{
	{Name: "Thunderbreak Regent", Count: 7}, {Name: "Mogg Fanatic", Count: 6},
	{Name: "Shock", Count: 3}, {Name: "Counterspell", Count: 5},
	{Name: "Act of Treason", Count: 3},
	{Name: "Mountain", Count: 10}, {Name: "Island", Count: 6},
}

// TestFourSeatThunderbreakCounteredLockstep: four-seat games where Regent
// triggers resolve after their targeting spell was countered or their
// targeting ability's source was sacrificed.
func TestFourSeatThunderbreakCounteredLockstep(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		var bt dirac.BtStats
		dirac.DebugBtStats = &bt
		defer func() { dirac.DebugBtStats = nil }()
		st := runPodClass(t, thunderbreakCounterPodDeck, 24, thunderbreakWatch, &dirac.DebugBtSrcCtrlGone, false)
		t.Logf("%d games, %d divergences, %d stalls, %d watched decisions; %d live reads, %d departed sources",
			st.games, st.divs, st.stalls, st.hits, bt.SrcCtrlReads, bt.SrcGone)
		if bt.SrcGone < 10 {
			t.Errorf("the games reached %d departed sources, want >= 10", bt.SrcGone)
		}
	})
	t.Run("mutation", func(t *testing.T) {
		st := runPodClass(t, thunderbreakCounterPodDeck, 24, thunderbreakWatch, &dirac.DebugBtSrcCtrlGone, true)
		if st.divs == 0 {
			t.Fatalf("mutation left %d games with no divergence", st.games)
		}
		t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
	})
}

// A bare MustAttack effect (Bident of Thassa: "creatures your opponents
// control attack this turn if able"). gorge marks every attack pair of such
// a creature Required and leaves the defender to its controller (CR 508.1d);
// dirac asks the same at more than two seats (mustAttackAsked), binding every
// opponent's creatures, whichever seat holds the effect (anyEmb reads every
// seat's emblems). The mutation (DebugMustAttackOneOpp) binds only the next
// live seat's creatures, the two-seat reading.
var mustAttackPodDeck = []cardset.Entry{
	{Name: "Bident of Thassa", Count: 8}, {Name: "Savannah Lions", Count: 6},
	{Name: "Goblin Piker", Count: 6}, {Name: "Coral Merfolk", Count: 4},
	{Name: "Island", Count: 9}, {Name: "Plains", Count: 4}, {Name: "Mountain", Count: 3},
}

// mustAttackWatch: gorge asks the active player's attackers with no Done (a
// creature must still attack), three or more players alive, while another
// player controls a tapped Bident of Thassa.
func mustAttackWatch(c *PickCtx) bool {
	d := c.G.fr.d
	if d == nil || d.Kind != decision.KAttackers || slices.ContainsFunc(c.D.Options, func(a Canon) bool { return a.Kind() == ActDone }) {
		return false
	}
	g := c.G.E.G
	alive := 0
	for p := range g.Players {
		if !g.Players[p].Lost {
			alive++
		}
	}
	if alive < 3 {
		return false
	}
	for q := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(q)) {
			if o := g.Obj(id); o != nil && o.Controller != d.Player && o.Tapped && o.Face() != nil && o.Face().Name == "Bident of Thassa" {
				return true
			}
		}
	}
	return false
}

// TestFourSeatMustAttackLockstep: four-seat Bident of Thassa games play in
// lockstep, with creatures that must attack choosing among three defenders.
func TestFourSeatMustAttackLockstep(t *testing.T) {
	podClassSteer = mustAttackSteer
	defer func() { podClassSteer = nil }()
	podClassLockstep(t, mustAttackPodDeck, 24, 10, mustAttackWatch, &dirac.DebugMustAttackOneOpp)
}

// mustAttackSteer keeps a Bident of Thassa untapped on its controller's and
// the next live seat's turns and activates it before combat on a later
// seat's turn (the multi-opponent case), so its effect binds that player's attackers, and declares
// no attacker the requirement does not force (Done whenever offered), so a
// requirement only one side holds shows.
func mustAttackSteer(c *PickCtx) (Canon, bool) {
	if slices.ContainsFunc(c.D.Options, func(a Canon) bool { return a.Kind() == ActAttack }) {
		return findAction(c, func(a Canon) bool { return a.Kind() == ActDone })
	}
	if c.G == nil || c.G.fr == nil || c.G.fr.d == nil {
		return 0, false
	}
	g, me := c.G.E.G, c.G.fr.d.Player
	bident := func(a Canon) bool {
		if a.Kind() != ActActivate {
			return false
		}
		for _, id := range g.Zone(state.ZBattlefield, me) {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Bident of Thassa" && c.G.InstRef(id) == a.Src() {
				return true
			}
		}
		return false
	}
	if g.Active != me && g.Active != nextLiveSeat(g, me) {
		if g.Step > state.StepBeginCombat {
			return 0, false
		}
		return findAction(c, bident)
	}
	if !slices.ContainsFunc(c.D.Options, bident) {
		return 0, false
	}
	// on its controller's turn and the next seat's (the one opponent of a
	// two-seat reading) a Bident stays untapped for a later seat's turn
	if a, ok := findAction(c, func(a Canon) bool {
		k := a.Kind()
		return k == ActCast || k == ActPlayLand
	}); ok {
		return a, true
	}
	return findAction(c, func(a Canon) bool { return a.Kind() == ActPass })
}

// nextLiveSeat is the first seat after p in turn order still in the game.
func nextLiveSeat(g *state.Game, p state.PlayerID) state.PlayerID {
	n := len(g.Players)
	for i := 1; i < n; i++ {
		if q := state.PlayerID((int(p) + i) % n); !g.Players[q].Lost {
			return q
		}
	}
	return p
}

// Crackling Doom: "each opponent sacrifices a creature with the greatest
// power among creatures that player controls". gorge repeats over the
// opponents and each chooses among its own greatest-power creatures
// (greatestPowerControlledByRemembered); dirac reads that player's greatest
// power inside the RepeatEach body (kop.CtrlEachPl). The mutation
// (DebugEachPlOppCtrl) takes the greatest power across every opponent, the
// two-seat reading, so an opponent whose creatures are all smaller
// sacrifices nothing.
var greatestPowerPodDeck = []cardset.Entry{
	{Name: "Crackling Doom", Count: 8}, {Name: "Savannah Lions", Count: 5},
	{Name: "Goblin Piker", Count: 4}, {Name: "Hill Giant", Count: 4},
	{Name: "Plains", Count: 6}, {Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 6},
}

// greatestPowerWatch: gorge asks a player to choose a creature to sacrifice
// while three or more players are alive.
func greatestPowerWatch(c *PickCtx) bool {
	d := c.G.fr.d
	if d == nil || d.Kind == decision.KPriority || d.Kind == decision.KAttackers || d.Kind == decision.KBlockers {
		return false
	}
	g := c.G.E.G
	alive := 0
	for p := range g.Players {
		if !g.Players[p].Lost {
			alive++
		}
	}
	if alive < 3 {
		return false
	}
	for _, id := range g.Zone(state.ZStack, 0) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Crackling Doom" {
			return true
		}
	}
	return false
}

// TestFourSeatGreatestPowerLockstep: four-seat Crackling Doom games play in
// lockstep, each opponent choosing among its own greatest-power creatures.
func TestFourSeatGreatestPowerLockstep(t *testing.T) {
	podClassLockstep(t, greatestPowerPodDeck, 24, 10, greatestPowerWatch, &dirac.DebugEachPlOppCtrl)
}
