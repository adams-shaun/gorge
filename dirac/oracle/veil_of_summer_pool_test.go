package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Veil of Summer and Domri, Anarch of Bolas against blue and black spells,
// played decision by decision against gorge (kop.ExtTurnRepl's uncounter,
// player hexproof and hexproof-from-blue-and-black kinds): "spells you control
// can't be countered this turn" and "you and permanents you control gain
// hexproof from blue and from black until end of turn" (Veil), "creature
// spells you cast this turn can't be countered" (Domri's mana ability). The
// green seat casts a Veil or activates Domri whenever offered one (half the
// time), the blue-black seat casts counterspells and targeting spells at
// random. The kernel agrees with gorge at every decision of every game it
// models to the end, and the pool reaches each part: a counter stopped, a
// player targeting refused, a marked permanent asked about.

func veilPoolDecks() [2][]string {
	cat := func(ds ...[]string) []string {
		var o []string
		for _, d := range ds {
			o = append(o, d...)
		}
		return o
	}
	return [2][]string{
		cat(deckOf(4, "Veil of Summer"), deckOf(3, "Domri, Anarch of Bolas"), deckOf(8, "Grizzly Bears"), deckOf(3, "Llanowar Elves"),
			deckOf(8, "Forest"), deckOf(4, "Mountain")),
		cat(deckOf(3, "Counterspell"), deckOf(3, "Essence Scatter"), deckOf(4, "Disfigure"), deckOf(3, "Duress"), deckOf(4, "Thought Scour"),
			deckOf(3, "Unsummon"), deckOf(2, "Lightning Bolt"), deckOf(6, "Island"), deckOf(6, "Swamp"), deckOf(2, "Mountain")),
	}
}

// veilResolvedThisTurn: a Veil of Summer left the stack for the graveyard after
// the last turn change.
func veilResolvedThisTurn(g *GorgeSide) bool {
	done := false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			done = false
		case e.Kind == events.MoveZone && e.From == state.ZStack && e.To == state.ZGraveyard && sideObjName(g, e.Obj) == "Veil of Summer":
			done = true
		}
	}
	return done
}

// veilTargeter reports the second seat's spells that target and are blue or black
// (or, Lightning Bolt, neither: Veil's hexproof does not stop it).
func veilTargeter(name string) bool {
	switch name {
	case "Disfigure", "Duress", "Thought Scour", "Unsummon", "Lightning Bolt":
		return true
	}
	return false
}

// veilPolicy: the first seat takes a Veil cast or a Domri activation when
// offered one on a coin (seeded, so the games differ), and passes on a coin
// once a Veil has resolved this turn and the stack is empty; else it casts at
// random. The second seat holds its targeting spells (Disfigure, Duress,
// Thought Scour, Unsummon, Lightning Bolt) until a Veil has resolved this turn
// and then casts one when it can, so the games reach a targeting after the
// grant; counterspells are cast at random.
func veilPolicy(m *Matchup, seed uint64) Policy {
	base := castFirstPolicy(seed)
	x := seed*2862933555777941757 + 3037000493
	coin := func() bool {
		x = x*6364136223846793005 + 1442695040888963407
		return x>>63 != 0
	}
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind != DPriority || p.G == nil {
			return base.Pick(p)
		}
		window := veilResolvedThisTurn(p.G)
		switch p.D.Player {
		case 1:
			var held []Canon
			for _, o := range p.D.Options {
				if o.Kind() != ActCast || !veilTargeter(m.InstanceCard(int(o.Src()))) {
					held = append(held, o)
				} else if window {
					return o
				}
			}
			d := *p.D
			d.Options = held
			return base.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
		case 0:
			for _, o := range p.D.Options {
				name := m.InstanceCard(int(o.Src()))
				if o.Kind() == ActCast && name == "Veil of Summer" || o.Kind() == ActActivate && name == "Domri, Anarch of Bolas" {
					if coin() {
						return o
					}
				}
			}
			if window && len(p.G.E.G.Stack) == 0 && coin() {
				for _, o := range p.D.Options {
					if o.Kind() == ActPass {
						return o
					}
				}
			}
		}
		return base.Pick(p)
	})
}

func TestVeilOfSummerPoolLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	decks := veilPoolDecks()
	c := effectLongCase{name: "veil_pool", decks: decks, surface: "autopay", reached: func(*GorgeSide) bool { return false }}
	set := printedDelveSet(t, decks)
	play := func(seed uint64) Result {
		r := &Repro{Format: reproFormat, Decks: [2]string{"veil_0", "veil_1"}, Cards: decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, _ := effectLongPlay(t, reg, c, r, set, veilPolicy(m, seed))
		return res
	}
	const games = 120
	stalled, agreed := 0, 0
	var unc, hexP, hexO int64
	for seed := uint64(1); seed <= games; seed++ {
		u0, p0, o0 := dirac.TurnReplStats.Unc.Load(), dirac.TurnReplStats.HexP.Load(), dirac.TurnReplStats.HexO.Load()
		res := play(seed)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.StallUnmodelled {
			// two Veils' records on one spell: gorge asks the CR 616.1 order, the kernel declares it
			if res.Stall != "kernel uncounterorder" {
				t.Fatalf("seed %d: the kernel stalled (%s): the game is not an agreed one", seed, res.Stall)
			}
			stalled++
			continue
		}
		agreed++
		unc += dirac.TurnReplStats.Unc.Load() - u0
		hexP += dirac.TurnReplStats.HexP.Load() - p0
		hexO += dirac.TurnReplStats.HexO.Load() - o0
	}
	t.Logf("%d agreed games, %d ended on the two-record uncounter-order stall; spells protected %d, player targetings refused %d, marked permanents asked %d", agreed, stalled, unc, hexP, hexO)
	if agreed < games*2/3 {
		t.Errorf("only %d of %d games were modelled to their end", agreed, games)
	}
	if unc == 0 {
		t.Error("no agreed game had a spell protected from a counter: the pool does not exercise the uncounter scope")
	}
	if hexP == 0 {
		t.Error("no agreed game had a targeting of the player refused: the pool does not exercise the player's hexproof")
	}
	if hexO == 0 {
		t.Error("no agreed game asked about a permanent holding Veil's grant: the pool does not exercise the permanent plane")
	}

	// With the kernel ignoring the records, some game diverges: the pool leans on them.
	dirac.DebugTurnReplOff = true
	defer func() { dirac.DebugTurnReplOff = false }()
	diverged := 0
	for seed := uint64(1); seed <= games; seed++ {
		if play(seed).Div != nil {
			diverged++
		}
	}
	if diverged == 0 {
		t.Error("no game diverged with the scope records ignored: the pool does not exercise them")
	}
	t.Logf("%d of %d games diverge with the records ignored", diverged, games)
}
