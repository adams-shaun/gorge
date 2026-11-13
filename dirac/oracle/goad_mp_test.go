//go:build dirac_mp

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
)

// goadInstrs counts the goad instructions a kernel table carries, without
// rewriting it: the same walk withoutArenaGoad makes, read-only (the
// Undercity's Arena room's DB$ Goad and a pool carrier's goad).
func goadInstrs(tbl *kop.Table) int {
	n := 0
	for i := range tbl.Cards {
		c := &tbl.Cards[i]
		for r := range c.Abilities {
			for j := range c.Abilities[r].Prog {
				if c.Abilities[r].Prog[j].Op == kop.OpGoad {
					n++
				}
			}
		}
	}
	return n
}

// enterArena wraps p so a venture's two-door ask follows the Arena-facing
// NextRoom$ arrow: Forge's second door is the Arena (NextRoom$ Trap,Arena),
// Lost Well's first (NextRoom$ Arena,Stash), and the Arena's own first door
// is the Archives (NextRoom$ Archives,Catacombs), whose room asks nothing.
// The marker's current room is the gorge side's; Lockstep's leader is always
// gorge, so c.G is set.
func enterArena(p Policy) Policy {
	roomCode, _ := chooseCode("room")
	roomKind := ActChoose + CanonKind(roomCode)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && len(c.D.Options) > 0 {
			room := c.G.E.G.Players[c.D.Player].DungeonRoom
			mode := -1
			switch room {
			case "Forge":
				mode = 1
			case "Well":
				mode = 0
			case "Arena":
				mode = 0
			}
			if mode >= 0 {
				for _, o := range c.D.Options {
					if o.Kind() == roomKind && o.Mode() == uint8(mode) {
						return o
					}
				}
			}
		}
		return p.Pick(c)
	})
}

// undercityGoads counts, in a finished gorge log, the Goad events whose
// source is the Undercity dungeon card (a room ability's source is the
// dungeon card itself, engine/rules/dungeon_room.go), so an Arena room's
// goad reads apart from a carrier's.
func undercityGoads(g *GorgeSide) int {
	n := 0
	for _, e := range g.E.L.Events {
		if e.Kind != events.Goad || len(e.IDs) == 0 {
			continue
		}
		o := g.E.G.Obj(e.IDs[0])
		if o != nil && o.Face() != nil && o.Face().Name == "Undercity" {
			n++
		}
	}
	return n
}

// TestFourSeatInitiativeArenaGoadLockstep plays the four-seat initiative
// pods of dirac/oracle/fourseat_initiative_mp_test.go against gorge with the
// Undercity's Arena room in play and its goad unresolved: the kernel no
// longer refuses a program-goad table at more than two seats (StallTwoSeat,
// prepGoad's marks are per seat now), so the pod needs no withoutArenaGoad
// and enterArena steers every venture into the Arena. No divergence and no
// stall, some seed resolves an Arena goad (a gorge events.Goad whose source
// is the Undercity), and with the goads ignored (dirac.DebugIgnoreGoad) some
// seed diverges: the goaded creature stops attacking each combat if able and
// stops losing its goader as a defender.
func TestFourSeatInitiativeArenaGoadLockstep(t *testing.T) {
	reg := synthReg(t)
	set, err := cardset.RawByName("init")
	if err != nil {
		t.Fatal(err)
	}
	holder := initDeck([]cardset.Entry{
		{Name: "Avenging Hunter", Count: 4}, {Name: "Stirring Bard", Count: 4},
		{Name: "Dungeoneer's Pack", Count: 4},
		{Name: "Forest", Count: 16}, {Name: "Mountain", Count: 12},
	})
	burn := initDeck([]cardset.Entry{
		{Name: "Shock", Count: 12}, {Name: "Mountain", Count: 28},
	})
	decks := [4][]string{holder, burn, burn, burn}
	var lists [][]cardset.Entry
	for i, d := range decks {
		if len(d) != 40 {
			t.Fatalf("deck %d: %d cards", i, len(d))
		}
		es := make([]cardset.Entry, len(d))
		for j, n := range d {
			es[j] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	if n := goadInstrs(tbl); n == 0 {
		t.Fatal("the kernel table carries no goad instruction: the Undercity fixture changed")
	}
	if !gorgeDigWithCounters() {
		throneAsGorge(tbl) // the pin's Throne gap (init_test.go)
	}
	play := func(seed uint64) (Result, int) {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 8000, 40
		goads := 0
		rc.Finish = func(g *GorgeSide) { goads = undercityGoads(g) }
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		defer CloseSide(k)
		return Lockstep(m, seed, k, killHolder(enterArena(castCarrierFirst(seed))), rc), goads
	}
	const seeds = 16
	total := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		res, n := play(seed)
		if res.Div != nil {
			lo := max(res.Div.At-12, 0)
			t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		total += n
		t.Logf("seed %d: %d decisions, over %v %s; %d Arena goad(s)", seed, res.Decisions, res.Over, res.Stall, n)
	}
	if total == 0 {
		t.Fatal("no game resolved an Arena goad: the pod never entered the Arena room")
	}
	t.Logf("%d Arena goad(s) over %d seeds", total, seeds)
	dirac.DebugIgnoreGoad = true
	defer func() { dirac.DebugIgnoreGoad = false }()
	for seed := uint64(1); seed <= seeds; seed++ {
		if res, _ := play(seed); res.Div != nil {
			t.Logf("mutation: seed %d diverges at %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("ignoring the goads diverged on no seed: no goaded creature reached a combat")
}
