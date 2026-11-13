package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The this-turn records' lockstep: Elanor Gardner's
// Food gate (the per-player Food tally), Blitzball's legendary-hit gate and
// Steel Hellkite's controller-hit destruction (the combat-hit records),
// Avengers Assemble!'s and Thaumaton Torpedo's attacked-this-turn count (the
// object-width attacked mark), compared with gorge through combat, Food
// sacrifices, station and removal. Seats are swapped: each deck plays both
// seats.

var kturntalliesDecks = [2][]cardset.Entry{
	{ // A: W/B, Heroes, a legend and Food
		{Name: "Tempting Witch", Count: 3}, {Name: "Bake into a Pie", Count: 3},
		{Name: "Blitzball", Count: 3}, {Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Avengers Assemble!", Count: 3},
		{Name: "Amateur Hero", Count: 3}, {Name: "Hero in Training", Count: 2}, {Name: "Savannah Lions", Count: 2},
		{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 7},
	},
	{ // B: R/G, Elanor Gardner, a Spacecraft, the Torpedo and the Hellkite
		{Name: "Elanor Gardner", Count: 2}, {Name: "Steel Hellkite", Count: 3}, {Name: "Thaumaton Torpedo", Count: 4},
		{Name: "Galvanizing Sawship", Count: 4}, {Name: "Grizzly Bears", Count: 2}, {Name: "Hill Giant", Count: 4},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 9},
	},
}

// kturntalliesStats is what the policy counts across the games, on gorge's
// state.
type kturntalliesStats struct {
	elanor    int // Elanor Gardner end-step searches offered (a Food sacrificed this turn)
	food      int // Food sacrifices (Tempting Witch's cost, a Food token's) taken
	blitzball int // Blitzball draw activations taken
	hellkite  int // Steel Hellkite DestroyAll activations taken
	hellHit   int // ... after that Hellkite dealt combat damage to a player this turn
	hellKill  int // ... after which a permanent went from the battlefield to a graveyard that turn without being sacrificed
	hellConn  int // (turn, seat)s where a Steel Hellkite dealt combat damage to that player
	hellAtk   int // Steel Hellkite attack declarations
	spaceAtk  int // (turn, seat)s where a Galvanizing Sawship was declared an attacker
	avengers  int // (turn, seat)s where a Hero that was already on the battlefield attacked with Avengers Assemble! out
	torpedo   int // Thaumaton Torpedo activations taken
	torpRed   int // ... after a Spacecraft attacked this turn
	torpDead  int // ... with that Spacecraft no longer on the battlefield
	legHit    int // (turn, seat)s where an opponent was dealt combat damage by a legendary creature
}

type kturntalliesProbe struct {
	st       *kturntalliesStats
	base     Policy
	r        *rand.Rand
	seen     map[state.ObjID]int32 // Heroes: the turn first seen on the battlefield
	avTurn   map[[2]int32]bool
	legTurn  map[[2]int32]bool
	lastTurn int32
	hellMark int // len(g.Entered) when a Hellkite destruction was taken this turn, -1 none
	hellTurn int32
}

func (p *kturntalliesProbe) observe(gs *GorgeSide) {
	g := gs.E.G
	turn := int32(g.Turn)
	for pl := state.PlayerID(0); pl < 2; pl++ {
		for _, id := range g.Zone(state.ZBattlefield, pl) {
			if n := firstFaceName(g, id); n == "Amateur Hero" || n == "Hero in Training" {
				if _, ok := p.seen[id]; !ok {
					p.seen[id] = turn
				}
			}
		}
	}
	for _, id := range gs.E.AttackersDeclaredThisTurn() {
		o := g.Obj(id)
		if o == nil {
			continue
		}
		if firstFaceName(g, id) == "Galvanizing Sawship" && !p.avTurn[[2]int32{turn, -1}] {
			p.avTurn[[2]int32{turn, -1}] = true
			p.st.spaceAtk++
		}
		if n := firstFaceName(g, id); (n == "Amateur Hero" || n == "Hero in Training") && p.seen[id] < turn && !p.avTurn[[2]int32{turn, int32(o.Controller)}] {
			for _, bf := range g.Zone(state.ZBattlefield, o.Controller) {
				if firstFaceName(g, bf) == "Avengers Assemble!" {
					p.avTurn[[2]int32{turn, int32(o.Controller)}] = true
					p.st.avengers++
					break
				}
			}
		}
	}
	for pl := state.PlayerID(0); pl < 2; pl++ {
		if p.legTurn[[2]int32{turn, int32(pl)}] {
			continue
		}
		for _, hit := range gs.E.CombatDamageToPlayersThisTurn() {
			if hit.Player != pl || hit.Card == nil || len(hit.Card.Faces) == 0 {
				continue
			}
			if hit.Card.Faces[0].Name == "Steel Hellkite" && !p.avTurn[[2]int32{turn, 10 + int32(pl)}] {
				p.avTurn[[2]int32{turn, 10 + int32(pl)}] = true
				p.st.hellConn++
			}
			if hit.Card.Faces[0].Name == "Isamaru, Hound of Konda" {
				p.legTurn[[2]int32{turn, int32(pl)}] = true
				p.st.legHit++
				break
			}
		}
	}
}

// ready reports whether activating a is worth taking now: the Hellkite's
// destruction only after it connected, the Torpedo only after a Spacecraft
// attacked (each still taken now and then, so the unreduced and the
// unconnected gates are compared too).
func (p *kturntalliesProbe) ready(gs *GorgeSide, a Canon) bool {
	g := gs.E.G
	switch refCardName(gs, a.Src()) {
	case "Steel Hellkite":
		if a.Mode() != 1 {
			return p.r.IntN(16) == 0 // the {2} pump spends the mana the destruction wants
		}
		for _, hit := range gs.E.CombatDamageToPlayersThisTurn() {
			if hit.Source == gs.id.obj[a.Src()] {
				return true
			}
		}
		return g.Step == state.StepMain2 && p.r.IntN(4) == 0 // the limit is once a turn: keep it for after the attack
	case "Thaumaton Torpedo":
		for _, id := range gs.E.AttackersDeclaredThisTurn() {
			if firstFaceName(g, id) == "Galvanizing Sawship" {
				return true
			}
		}
		return p.r.IntN(64) == 0 // it is sacrificed: mostly held for the reduction
	}
	return true
}

// holdsMana reports whether pl controls a Steel Hellkite, or a Thaumaton
// Torpedo and a Galvanizing Sawship.
func holdsMana(g *state.Game, pl state.PlayerID) bool {
	var hell, torp, ship bool
	for _, id := range g.Zone(state.ZBattlefield, pl) {
		switch firstFaceName(g, id) {
		case "Steel Hellkite":
			hell = true
		case "Thaumaton Torpedo":
			torp = true
		case "Galvanizing Sawship":
			ship = true
		}
	}
	return hell || torp && ship
}

// pass is the decision's pass option, else a itself.
func (p *kturntalliesProbe) pass(c *PickCtx, a Canon) Canon {
	for _, o := range c.D.Options {
		if o.Kind() == ActPass {
			return o
		}
	}
	return a
}

func (p *kturntalliesProbe) pick(c *PickCtx) Canon {
	gs := c.G
	if gs != nil {
		p.observe(gs)
		if g := gs.E.G; p.hellMark >= 0 {
			if int32(g.Turn) != p.hellTurn {
				p.hellMark = -1
			} else {
				for _, en := range g.Entered[min(p.hellMark, len(g.Entered)):] {
					if en.From == state.ZBattlefield && en.To == state.ZGraveyard && !en.Sacrificed {
						p.st.hellKill++
						p.hellMark = -1
						break
					}
				}
			}
		}
	}
	var pool, attacks []Canon
	hold := gs != nil && gs.E.G.Step == state.StepMain1 && gs.E.G.Active == state.PlayerID(c.D.Player) && holdsMana(gs.E.G, gs.E.G.Active)
	for _, a := range c.D.Options {
		switch a.Kind() {
		case ActActivate:
			if gs == nil || p.ready(gs, a) && (!hold || p.r.IntN(16) == 0) {
				pool = append(pool, a)
			}
		case ActSpecial:
			pool = append(pool, a)
		case ActAttack:
			attacks = append(attacks, a)
		}
	}
	a := p.base.Pick(c)
	// prefer attacking, then a carrier's ability or a station, then the random pick
	if len(attacks) > 0 && p.r.IntN(4) != 0 {
		a = attacks[p.r.IntN(len(attacks))]
	} else if len(pool) > 0 && p.r.IntN(4) != 0 {
		a = pool[p.r.IntN(len(pool))]
	}
	if gs == nil {
		return a
	}
	g := gs.E.G
	if a.Kind() == ActActivate && !p.ready(gs, a) {
		a = p.pass(c, a) // the random fallback must keep the gates too
	}
	// hold the mana before combat while the Hellkite (or the Torpedo beside a
	// Spacecraft) is out: their abilities are spent after the attack
	if a.Kind() == ActCast && g.Step == state.StepMain1 && g.Active == state.PlayerID(c.D.Player) && p.r.IntN(8) != 0 && holdsMana(g, g.Active) {
		a = p.pass(c, a)
	}
	for _, o := range c.D.Options {
		if o.Kind() == ActYes && refCardName(gs, o.Src()) == "Elanor Gardner" {
			p.st.elanor++
			break
		}
	}
	if a.Kind() == ActAttack && refCardName(gs, a.Src()) == "Steel Hellkite" {
		p.st.hellAtk++
	}
	if a.Kind() == ActActivate {
		switch refCardName(gs, a.Src()) {
		case "Tempting Witch", "Food":
			p.st.food++
		case "Blitzball":
			if a.Mode() == 1 {
				p.st.blitzball++
			}
		case "Steel Hellkite":
			if a.Mode() == 1 {
				p.st.hellkite++
				p.hellMark, p.hellTurn = len(g.Entered), int32(g.Turn)
				for _, hit := range gs.E.CombatDamageToPlayersThisTurn() {
					if hit.Source == gs.id.obj[a.Src()] {
						p.st.hellHit++
						break
					}
				}
			}
		case "Thaumaton Torpedo":
			p.st.torpedo++
			for _, id := range gs.E.AttackersDeclaredThisTurn() {
				if firstFaceName(g, id) == "Galvanizing Sawship" {
					p.st.torpRed++
					if o := g.Obj(id); o != nil && o.Zone != state.ZBattlefield {
						p.st.torpDead++
					}
					break
				}
			}
		}
	}
	return a
}

// runTurntallies plays the constructed matchups (both seat orders) and
// returns the games, the divergences, the stalls and the carrier counts.
func runTurntallies(t *testing.T, seeds int, report bool) (games, divs, stalls int, st kturntalliesStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("turntallies")
	if err != nil {
		t.Fatal(err)
	}
	for order := 0; order < 2; order++ {
		md := [2][]cardset.Entry{kturntalliesDecks[order], kturntalliesDecks[1-order]}
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
				t.Fatalf("order %d deck %d: %d cards", order, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kturntallies-a", "kturntallies-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			probe := &kturntalliesProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x7475726e)), seen: map[state.ObjID]int32{}, avTurn: map[[2]int32]bool{}, legTurn: map[[2]int32]bool{}, hellMark: -1}
			res := LockstepLed(m, seed, g, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				if report {
					t.Errorf("order %d seed %d: kernel stall %q", order, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if report {
					lo := max(d.At-12, 0)
					t.Logf("order %d seed %d last actions %v", order, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("order %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", order, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestTurntalliesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runTurntallies(t, 96, true)
	t.Logf("%d games, %d divergences, %d stalls, %+v", games, divs, stalls, st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		n    int
		what string
	}{
		{st.elanor, "Elanor Gardner end-step searches"}, {st.food, "Food sacrifices"}, {st.blitzball, "Blitzball draws"},
		{st.hellkite, "Steel Hellkite activations"}, {st.hellHit, "Steel Hellkite activations after it connected"}, {st.hellKill, "Steel Hellkite activations that destroyed a permanent"}, {st.spaceAtk, "turns a Spacecraft attacked"}, {st.avengers, "Avengers Assemble! attacker turns through Y"},
		{st.torpedo, "Thaumaton Torpedo activations"}, {st.torpRed, "Torpedo activations after a Spacecraft attacked"},
		{st.legHit, "turns an opponent was hit by a legendary creature"},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d, want at least 3", c.what, c.n)
		}
	}
}

// TestTurntalliesMutation: keeping the three records across the turn change
// must make the lockstep diverge.
func TestTurntalliesMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugTurnTalliesNoReset = true
	defer func() { dirac.DebugTurnTalliesNoReset = false }()
	games, divs, _, _ := runTurntallies(t, 32, false)
	if divs == 0 {
		t.Fatalf("DebugTurnTalliesNoReset: no divergence in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
