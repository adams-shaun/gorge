package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Power-up lockstep (CR 702.193): constructed matchups
// over the generated powerup pool. Every carrier puts at least one +1/+1
// counter on itself, so each activation is the lifted mechanic resolving.
// Cloudshift blinks a carrier, proving CR 400.7 (a new object may power up
// again) and re-testing the entry-turn reduction.
//
// White Tiger, Ava Ayala is refused by kgen because The Tiger God token's
// MinMaxBlocker static is not lowered; it is not in the pool. Viv Vision,
// Teen Synthezoid is refused because her attack trigger gates on
// ConditionDefined$ TriggeredAttackerLKICopy, which gorge's condition census
// classifies as unmodelled and fails CLOSED; modelling the draw would diverge
// for a reason unrelated to Power-up (CR 702.193), so she is not in the pool.

var powerupCarriers = []string{
	"Abomination, Terrifying Titan", "Brave Brawler", "Extremis Elite",
	"Gamma Grotesque", "Goliath, Mass Manipulator", "Hercules, Prince of Power",
	"Human Torch, Johnny Storm", "Molly Hayes, Runaway", "Pet Avengers",
	"Serpent Specialist", "Shang-Chi, Martial Mentor",
	"Ultron Drone", "Volcanic Villain",
}

var powerupDecks = [][2][]cardset.Entry{{
	{
		{Name: "Serpent Specialist", Count: 2}, {Name: "Goliath, Mass Manipulator", Count: 4},
		{Name: "Forest", Count: 34},
	},
	{
		{Name: "Hercules, Prince of Power", Count: 4}, {Name: "Gamma Grotesque", Count: 4},
		{Name: "Forest", Count: 32},
	},
}, {
	{
		{Name: "Shang-Chi, Martial Mentor", Count: 4}, {Name: "Goliath, Mass Manipulator", Count: 4},
		{Name: "Forest", Count: 32},
	},
	{
		{Name: "Pet Avengers", Count: 4}, {Name: "Hercules, Prince of Power", Count: 4},
		{Name: "Forest", Count: 32},
	},
}, {
	{
		{Name: "Volcanic Villain", Count: 4}, {Name: "Human Torch, Johnny Storm", Count: 4},
		{Name: "Abomination, Terrifying Titan", Count: 2}, {Name: "Mountain", Count: 30},
	},
	{
		{Name: "Molly Hayes, Runaway", Count: 4}, {Name: "Extremis Elite", Count: 4},
		{Name: "Abomination, Terrifying Titan", Count: 2}, {Name: "Mountain", Count: 30},
	},
}, {
	{
		{Name: "Brave Brawler", Count: 4}, {Name: "Cloudshift", Count: 4},
		{Name: "Plains", Count: 32},
	},
	{
		{Name: "Ultron Drone", Count: 8}, {Name: "Island", Count: 32},
	},
}, {
	// Abomination alone: no other carrier competes for the pool, so a
	// freshly cast copy is powered up on its entry turn (the reduced hybrid
	// branch) and a later copy at full price.
	{
		{Name: "Abomination, Terrifying Titan", Count: 8}, {Name: "Mountain", Count: 32},
	},
	{
		{Name: "Abomination, Terrifying Titan", Count: 8}, {Name: "Mountain", Count: 32},
	},
}}

// powerupTally scans gorge's event log for the lifted mechanic. A
// Power-up activation is an AbilityPush on a carrier; it is entry-turn
// (reduced) when the carrier's latest battlefield entry was this turn.
//
// Withholds are observed from the kernel's side at each agreed decision: a
// carrier whose once-per-stint bit is set, whose controller has priority,
// and whose activation is not offered, once per (object, turn). That is the
// once gate withholding the second activation; the mana clause is covered by
// the unit tests, which pose the exact reduced cost.
type powerupTally struct {
	evAt      int
	turn      int
	entry     map[state.ObjID]int
	carriers  map[string]bool
	reduced   int
	full      int
	hybrid    int
	hybRed    int // hybrid activations on the entry turn (reduced)
	hybFull   int // hybrid activations at full price
	fights    int // Abomination's Fight resolved onto a creature
	fightPend bool
	aboms     state.ObjID
	resolved  map[string]int
	withheld  map[[2]int]bool
	nHeld     int
}

func newPowerupTally() *powerupTally {
	t := &powerupTally{
		turn:     1,
		entry:    map[state.ObjID]int{},
		carriers: map[string]bool{},
		resolved: map[string]int{},
		withheld: map[[2]int]bool{},
	}
	for _, n := range powerupCarriers {
		t.carriers[n] = true
	}
	return t
}

// reset starts a new game. reduced, full, resolved and withheld are
// cumulative across the run: they are the occurrence counts the assertions
// check, so a reset here would leave only the last game's tally.
func (t *powerupTally) reset() {
	t.evAt = 0
	t.turn = 1
	t.entry = map[state.ObjID]int{}
	t.withheld = map[[2]int]bool{}
}

func (t *powerupTally) check(ref *GorgeSide, k Side) *Divergence {
	if ref == nil || ref.E == nil {
		return nil
	}
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < t.evAt {
		t.evAt = 0
	}
	for j := t.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.TurnChange:
			t.turn++
			t.fightPend = false
		case events.StepChange:
			t.fightPend = false
		case events.Damage:
			// Abomination's DBFight: the damage lands on a creature other
			// than Abomination in the step of its activation (the target
			// fights back, so the pair's first Damage names the target).
			if t.fightPend && e.Obj != 0 && e.Obj != t.aboms {
				if o := g.Obj(e.Obj); o != nil && o.Face() != nil && slices.Contains(o.Face().Types, "Creature") {
					t.fights++
					t.fightPend = false
				}
			}
		case events.MoveZone:
			if e.To == state.ZBattlefield {
				t.entry[e.Obj] = t.turn
			}
		case events.AbilityPush:
			ob := g.Obj(e.Obj)
			if ob == nil || ob.Face() == nil || !t.carriers[ob.Face().Name] {
				continue
			}
			name := ob.Face().Name
			t.resolved[name]++
			isHyb := name == "Abomination, Terrifying Titan"
			if isHyb {
				t.hybrid++
				t.aboms, t.fightPend = e.Obj, true
			}
			if t.entry[e.Obj] == t.turn {
				t.reduced++
				if isHyb {
					t.hybRed++
				}
			} else {
				t.full++
				if isHyb {
					t.hybFull++
				}
			}
		}
	}
	t.evAt = len(evs)
	t.countWithheld(k)
	return nil
}

// countWithheld observes the kernel's once gate: a carrier with its
// once-per-stint bit set whose controller has priority and whose activation
// is not among the offered options. The definition also requires the
// mana: a used carrier that cannot pay the reduced cost is not a withhold, so
// the count is gated on the projected pool plus untapped lands covering the
// reduced cost's mana value (the decks are mono-coloured, so MV is exact).
func (t *powerupTally) countWithheld(k Side) {
	r, ok := k.(*realSide)
	if !ok || r.d.Kind != dirac.DkPriority {
		return
	}
	for i := 0; i < int(r.kp.NObj); i++ {
		o := &r.kp.Objs[i]
		if o.Zone != kop.ZoneBattlefield || o.Ctrl != r.d.Player || int(o.Card) >= len(r.tbl.Cards) {
			continue
		}
		if !t.carriers[r.tbl.Cards[o.Card].Name] {
			continue
		}
		used, _, _ := dirac.PowerUpState(&r.s, dirac.Obj(i))
		if !used {
			continue
		}
		if !t.powerUpPayable(r, uint8(i)) {
			continue
		}
		offered := false
		for j := 0; j < int(r.d.N); j++ {
			if a := r.d.Options[j]; a.Kind() == dirac.AkActivate && a.Source() == dirac.TCode(uint8(i)) {
				offered = true
				break
			}
		}
		if offered {
			continue
		}
		key := [2]int{i, int(r.kp.Turn)}
		if !t.withheld[key] {
			t.withheld[key] = true
			t.nHeld++
		}
	}
}

// powerUpPayable reports whether the carrier's controller has at least the
// reduced cost's mana value floating in the pool or untapped in lands.
func (t *powerupTally) powerUpPayable(r *realSide, o uint8) bool {
	_, cost, ok := dirac.PowerUpCost(&r.s, dirac.Obj(o))
	if !ok {
		return false
	}
	need := cost.MV()
	for _, n := range r.kp.Pool[r.d.Player] {
		need -= int(n)
	}
	if need <= 0 {
		return true
	}
	for i := 0; i < int(r.kp.NObj); i++ {
		p := &r.kp.Objs[i]
		if p.Ctrl != r.d.Player || p.Zone != kop.ZoneBattlefield || p.Flags&dirac.ProjTapped != 0 {
			continue
		}
		if r.tbl.Cards[p.Card].Types&kop.TLand != 0 {
			need--
			if need <= 0 {
				return true
			}
		}
	}
	return false
}

// powerupPolicy taps every land for mana first, then activates a power-up,
// then casts. The floating pool pays both sides' costs, so the payment is a
// pool choice, not a land-tapping order (which the kernel and gorge need not
// share). Casts wait until turn 12: by then 12 lands make the FULL printed
// power-up cost affordable after the carrier's own cost, so gorge's planner
// (which does not fold CR 702.193b) still offers the ability. The reduced
// cost is what both sides charge, which the projection checks.
func powerupPolicy(seed uint64, castTurn int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	pick := func(c *PickCtx) Canon {
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActActivate && basicLandSource(c.G, a.Src()) {
				buf = append(buf, a)
			}
		}
		if len(buf) > 0 {
			return pick(c)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) > 0 {
			return pick(c)
		}
		if c.G != nil && int(c.G.E.G.Turn) >= castTurn {
			for _, a := range c.D.Options {
				if a.Kind() == ActCast {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				return pick(c)
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) > 0 {
			return pick(c)
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActPass {
				return a
			}
		}
		return rnd.Pick(c)
	})
}

func basicLandSource(g *GorgeSide, ref Ref) bool {
	if g == nil || int(ref) >= len(g.id.obj) {
		return false
	}
	o := g.E.G.Obj(g.id.obj[ref])
	if o == nil || o.Card == nil {
		return false
	}
	switch cardName(o.Card) {
	case "Forest", "Island", "Mountain", "Plains":
		return true
	}
	return false
}

func powerupSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("powerup")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func powerupDecksFor(t *testing.T, set cardset.Set, reg *cards.Registry, md [2][]cardset.Entry) (tbl *kop.Table, decks [2][]*cards.Card) {
	t.Helper()
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	// The deck CardIDs come from the set's matchup over the same entries.
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	return tbl, decks
}

func TestPowerupLockstep(t *testing.T) {
	t.Parallel()

	reg, set := powerupSetup(t)
	tally := newPowerupTally()
	games := 0
	for mi, md := range powerupDecks {
		tbl, decks := powerupDecksFor(t, set, reg, md)
		for seed := uint64(1); seed <= 256; seed++ {
			cfg := Config{}
			m, err := NewMatchup(reg, [2]string{"powerup-a", "powerup-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			tally.reset()
			// The Abomination-only matchup waits for 11 lands (turn 22) so
			// gorge's planner, which prices the UNREDUCED {5}{R/G}{R/G},
			// still offers the entry-turn power-up.
			castTurn, maxTurns := 12, 20
			if mi == len(powerupDecks)-1 {
				castTurn, maxTurns = 24, 34
			}
			res := Lockstep(m, seed, k, powerupPolicy(seed, castTurn), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: int32(maxTurns), Check: tally.check})
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				CloseSide(k)
				t.Fatalf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nlast actions %v\nref %v\nker %v",
					mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
			}
			CloseSide(k)
		}
	}
	t.Logf("%d games, %d entry-turn (reduced), %d full-price, %d hybrid (%d reduced, %d full), %d Abomination fights, %d withholds; carriers %v",
		games, tally.reduced, tally.full, tally.hybrid, tally.hybRed, tally.hybFull, tally.fights, tally.nHeld, tally.resolved)
	if tally.reduced < 50 {
		t.Errorf("only %d entry-turn reduced power-ups, want >= 50", tally.reduced)
	}
	if tally.full < 50 {
		t.Errorf("only %d full-price power-ups, want >= 50", tally.full)
	}
	if tally.nHeld < 20 {
		t.Errorf("only %d already-activated withholds, want >= 20", tally.nHeld)
	}
	if tally.hybRed < 5 || tally.hybFull < 5 {
		t.Errorf("hybrid power-ups: %d reduced, %d full, want >= 5 each", tally.hybRed, tally.hybFull)
	}
	if tally.fights < 10 {
		t.Errorf("only %d Abomination Fight resolutions, want >= 10", tally.fights)
	}
	for _, n := range powerupCarriers {
		if tally.resolved[n] == 0 {
			t.Errorf("carrier %s never powered up", n)
		}
	}
}

// mutatePowerUpFlags copies tbl, clearing CostPowerUp (drop the CR 702.193b
// reduction) or CostOncePerGame (drop the CR 702.193a once gate) on every
// power-up ability. The mutation is a table copy, not a kernel switch: the
// kernel carries no debug state.
func mutatePowerUpFlags(tbl *kop.Table, dropReduce, dropOnce bool) *kop.Table {
	mt := *tbl
	mt.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	for i := range mt.Cards {
		abs := append([]kop.Ability(nil), tbl.Cards[i].Abilities...)
		for j := range abs {
			if abs[j].Cost.Flags&kop.CostPowerUp == 0 {
				continue
			}
			if dropReduce {
				abs[j].Cost.Flags &^= kop.CostPowerUp
			}
			if dropOnce {
				abs[j].Cost.Flags &^= kop.CostOncePerGame
			}
		}
		mt.Cards[i].Abilities = abs
	}
	return &mt
}

// TestPowerupMutation: dropping the entry-turn reduction, or the
// once-per-stint gate, makes the lockstep diverge.
func TestPowerupMutation(t *testing.T) {
	t.Parallel()

	reg, set := powerupSetup(t)
	md := powerupDecks[0]
	tbl, decks := powerupDecksFor(t, set, reg, md)
	run := func(mt *kop.Table) bool {
		for seed := uint64(1); seed <= 256; seed++ {
			cfg := Config{}
			m, err := NewMatchup(reg, [2]string{"powerup-a", "powerup-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, mt, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, powerupPolicy(seed, 12), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 20})
			CloseSide(k)
			if res.Div != nil {
				return true
			}
		}
		return false
	}
	if !run(mutatePowerUpFlags(tbl, true, false)) {
		t.Error("dropping the entry-turn reduction did not diverge in 256 seeds")
	}
	if !run(mutatePowerUpFlags(tbl, false, true)) {
		t.Error("dropping the once-per-stint gate did not diverge in 256 seeds")
	}
}
