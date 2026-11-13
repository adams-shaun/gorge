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

// initDeck flattens a fixture list into deckOf's one-name-per-card slice.
func initDeck(es []cardset.Entry) []string {
	var d []string
	for _, e := range es {
		d = append(d, deckOf(e.Count, e.Name)...)
	}
	return d
}

// withoutArenaGoad removes the goad instruction from the kernel table and
// reports how many it removed. The Undercity's Arena room goads (CR 701.15),
// and prepGoad flags every table with a goad as two-seat-only (StallTwoSeat),
// so a four-seat pod cannot start with the real dungeon. avoidArena keeps
// every venturer out of that room, so the removed instruction never runs.
func withoutArenaGoad(tbl *kop.Table) int {
	n := 0
	for i := range tbl.Cards {
		c := &tbl.Cards[i]
		for r := range c.Abilities {
			for j := range c.Abilities[r].Prog {
				if c.Abilities[r].Prog[j].Op == kop.OpGoad {
					c.Abilities[r].Prog[j] = kop.Instr{} // OpNop
					n++
				}
			}
		}
	}
	return n
}

// avoidArena wraps p so a venture's two-door room ask never enters Undercity's
// Arena: from Forge its first door is Trap!, from Lost Well its second is
// Stash, and the top room's doors are both safe. The marker's current room is
// the gorge side's; Lockstep's leader is always gorge, so c.G is set.
func avoidArena(p Policy) Policy {
	roomCode, _ := chooseCode("room")
	roomKind := ActChoose + CanonKind(roomCode)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && len(c.D.Options) > 0 {
			room := c.G.E.G.Players[c.D.Player].DungeonRoom
			for _, o := range c.D.Options {
				if o.Kind() != roomKind {
					continue
				}
				if (room == "Forge" && o.Mode() == 0) || (room == "Well" && o.Mode() == 1) ||
					(room == "Arena" && o.Mode() == 0) {
					return o
				}
			}
		}
		return p.Pick(c)
	})
}

// castCarrierFirst casts the initiative's carriers whenever they are offered
// and keeps the burn back until a holder exists: castFirstPolicy casts one
// Shock a turn instead, starving the three- and four-mana carriers and
// spreading the burn over every seat.
func castCarrierFirst(seed uint64) Policy {
	base := castFirstPolicy(seed)
	rnd := RandomPolicy(seed ^ 0x696e6974) // "init"
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return base.Pick(c)
		}
		buf = buf[:0]
		for _, o := range c.D.Options {
			if o.Kind() != ActCast && o.Kind() != ActCastFace {
				continue
			}
			switch c.G.M.InstanceCard(int(o.Src())) {
			case "Avenging Hunter", "Stirring Bard", "Dungeoneer's Pack":
				buf = append(buf, o)
			}
		}
		if len(buf) > 0 {
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if !c.G.E.G.HasInitiative {
			// No holder yet: develop mana, but keep burn (it would only spread
			// its damage before a holder exists).
			for _, o := range c.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
			for _, o := range c.D.Options {
				if o.Kind() == ActPass {
					return o
				}
			}
		}
		return base.Pick(c)
	})
}

// killHolder wraps p so targeted spells and attacks aim at the current
// initiative holder: random play rarely focuses fire, and the pod must kill
// the holder before any dungeon reaches the Throne room (whose
// UntilYourNextTurn override is two-seat-only: ovDurFl's ctrl&1).
func killHolder(p Policy) Policy {
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || !c.G.E.G.HasInitiative {
			return p.Pick(c)
		}
		h := RefPlayer + Ref(c.G.E.G.Initiative)
		switch c.D.Kind {
		case DTarget:
			for _, o := range c.D.Options {
				if o.Kind() == ActTarget && o.Tgt() == h {
					return o
				}
			}
		case DAttackers:
			for _, o := range c.D.Options {
				if o.Kind() == ActAttack && o.Tgt() == h {
					return o
				}
			}
		}
		return p.Pick(c)
	})
}

// TestFourSeatInitiativeHandoffLockstep plays four-seat pods of the initiative
// fixture pool dirac against gorge. One seat carries the initiative and three
// burn seats aim at the holder, so it leaves while two or more players remain.
// Then the active player takes the initiative, or the next player in turn order
// when the active player is the one leaving (CR 726.4, gorge
// initiativeHandoffOnDeparture), and ventures. No divergence and no stall, some
// game hands the initiative off with the game going on, and with the handoff
// dropped (dirac.DebugNoInitHandoff) some seed diverges.
//
// The pool's Bloodboil Sorcerer and Undercity's Arena room both goad, and
// prepGoad refuses any Goad table at more than two seats (StallTwoSeat), so the
// pod uses the pool's other carriers and withoutArenaGoad/avoidArena keep the
// Arena room out of play. The holder dies before its dungeon reaches the Throne
// room, whose UntilYourNextTurn override is two-seat-only (ovDurFl's ctrl&1).
func TestFourSeatInitiativeHandoffLockstep(t *testing.T) {
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
	if n := withoutArenaGoad(tbl); n == 0 {
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
		handoffs := 0
		rc.Finish = func(g *GorgeSide) { handoffs = initiativeHandoffs(g) }
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		defer CloseSide(k)
		return Lockstep(m, seed, k, killHolder(avoidArena(castCarrierFirst(seed))), rc), handoffs
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
		t.Logf("seed %d: %d decisions, over %v %s; %d handoff(s)", seed, res.Decisions, res.Over, res.Stall, n)
	}
	if total == 0 {
		t.Fatal("no game handed the initiative off: no holder left a game that went on")
	}
	t.Logf("%d handoff(s) over %d seeds", total, seeds)
	dirac.DebugNoInitHandoff = true
	defer func() { dirac.DebugNoInitHandoff = false }()
	for seed := uint64(1); seed <= seeds; seed++ {
		if res, _ := play(seed); res.Div != nil {
			t.Logf("mutation: seed %d diverges at %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("dropping the CR 726.4 handoff diverged on no seed")
}

// initiativeHandoffs counts, in a finished gorge log, the initiative's
// departures that passed the designation on while two or more players
// remained: a PlayerLost of the current InitiativeChange holder followed by an
// InitiativeChange to another seat before any other event (gorge's losePlayer
// emits it before the MonarchChange).
func initiativeHandoffs(g *GorgeSide) int {
	init, alive, n := -1, len(g.E.G.Players), 0
	departed := -1
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.PlayerLost:
			alive--
			departed = -1
			if int(e.Player) == init && alive >= 2 {
				departed = init
			}
		case events.InitiativeChange:
			if departed >= 0 && int(e.Player) != departed {
				n++
			}
			departed = -1
			init = int(e.Player)
		default:
			departed = -1
		}
	}
	return n
}
