package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// The CR 903.9b park suspends the instruction it interrupts. These tests pin
// the state each answer re-runs from: a library-bound sweep's already-moved
// cards, the stale-source branches that must decline before the gate, the
// dig window an answer resumes at, and the battlefield sweep's pending
// placement and shuffle.

// cmdZoneResScen starts a resolving spell for card c and runs its program.
func cmdZoneResScen(t *testing.T, tbl *kop.Table, cmd CardID, card CardID) *scen {
	t.Helper()
	sc := newCommanderScen(t, tbl, cmd, true)
	s := sc.s
	s.res = resCtx{e: stackEntry{kind: seSpell, card: card, ctrl: 0}, active: true, lastTok: NoObj, eachObj: NoObj}
	s.ctl = ctlResolve
	return sc
}

// TestCommanderSweepLibraryKeepsMoveOrder: a graveyard sweep into the
// library parks at the commander after moving the cards before it. Those
// cards must be in the library (not lost zoneless) and, on the answer, the
// whole sweep must still sit on top in move order -- the first moved on top.
func TestCommanderSweepLibraryKeepsMoveOrder(t *testing.T) {
	for _, mode := range [2]uint8{0, 1} {
		t.Run([2]string{"command zone", "library"}[mode], func(t *testing.T) {
			tbl, cmd := commanderTable(0)
			tbl.Filters = append(tbl.Filters, kop.Filter{Zone: kop.ZoneGraveyard})
			f := kop.FilterID(len(tbl.Filters) - 1)
			tbl.Cards = append(tbl.Cards, kop.CardDef{
				Name:  "Test Commander Sweep",
				Types: kop.TInstant,
				Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{
					Op: kop.OpChangeZoneAll, F: f, C: int16(kop.ZoneLibrary),
				}}}},
			})
			card := CardID(len(tbl.Cards) - 1)
			sc := cmdZoneResScen(t, tbl, cmd, card)
			s := sc.s
			first := sc.grave(v.Forest, 0, 1)[0]
			o := s.pl[0].cmd
			s.detach(o)
			s.place(o, kop.ZoneGraveyard, false)
			last := sc.grave(v.Forest, 0, 1)[0]

			s.runProg(sc.k)
			if s.res.ask != raGenCmdZone {
				t.Fatalf("ask %d, want raGenCmdZone", s.res.ask)
			}
			if z := s.objs[first].zone; z != kop.ZoneLibrary {
				t.Fatalf("the card before the commander is in zone %v at the ask, want the library (the sweep must place it, not leave it zoneless)", z)
			}
			if z := s.objs[o].zone; z != kop.ZoneGraveyard {
				t.Fatalf("the parked commander is in zone %v, want the graveyard", z)
			}
			if z := s.objs[last].zone; z != kop.ZoneGraveyard {
				t.Fatalf("the card after the commander is in zone %v before the answer, want the graveyard", z)
			}
			d := s.Step()
			if d.Kind != DkChoose || d.Sub != ChCommanderZone {
				t.Fatalf("decision kind=%d sub=%d, want DkChoose/ChCommanderZone", d.Kind, d.Sub)
			}
			s.Apply(d.Options[mode])
			if s.Stalled() {
				t.Fatalf("the answer stalled: %s", StallName(s.StallReason()))
			}
			want := []Obj{first, last}
			if mode == 1 {
				want = []Obj{first, o, last}
			}
			if got := libTopOrder(s, 0, len(want)); !slices.Equal(got, want) {
				t.Fatalf("library top %v, want move order %v", got, want)
			}
		})
	}
}

// TestCommanderStaleSelfReturnNoAsk: a resolving self-return whose source is
// a new object (CR 400.7) declines the move before the CR 903.9b gate sees
// it. The stale object must neither be asked about nor park, and no answer
// may be left behind in the resolution context.
func TestCommanderStaleSelfReturnNoAsk(t *testing.T) {
	tbl, cmd := commanderTable(0)
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Return Self",
		Types: kop.TInstant,
		Abilities: []kop.Ability{{Kind: kop.AbActivated, Prog: []kop.Instr{{
			Op: kop.OpBounce, Obj: kop.RefSelf,
		}}}},
	})
	card := CardID(len(tbl.Cards) - 1)
	sc := newCommanderScen(t, tbl, cmd, true)
	s := sc.s
	o := s.pl[0].cmd // the command zone: the moveRef "default" branch
	e := stackEntry{kind: seAbility, card: card, obj: o, srcVer: s.objs[o].ver + 1, ctrl: 0}
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	s.ctl = ctlResolve
	s.runProg(sc.k)
	if s.res.ask != raNone {
		t.Fatalf("ask %d posed for a stale source, want none", s.res.ask)
	}
	if z := s.objs[o].zone; z != kop.ZoneCommand {
		t.Fatalf("the stale source moved to zone %v, want the command zone", z)
	}
	if s.g.pre.kind != 0 || s.g.pre.state != 0 {
		t.Fatalf("parked state kind=%d state=%d left behind, want none", s.g.pre.kind, s.g.pre.state)
	}
}

// TestCommanderDigLibraryNoRetake: a dig into the library parks at the
// commander after taking the cards above it. The answer re-runs the take
// loop at the parked card, so the earlier cards are not taken a second time
// (a second zone change would make them new objects, CR 400.7).
func TestCommanderDigLibraryNoRetake(t *testing.T) {
	tbl, cmd := commanderTable(0)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	f := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Commander Dig",
		Types: kop.TInstant,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{
			Op: kop.OpDigEx, Obj: kop.RefController, A: kop.Const(2), B: kop.Const(-1),
			C: int16(kop.ZoneLibrary), F: f,
		}}}},
	})
	card := CardID(len(tbl.Cards) - 1)
	sc := cmdZoneResScen(t, tbl, cmd, card)
	s := sc.s
	bears := sc.obj(v.GrizzlyBears, 0)
	o := s.pl[0].cmd
	s.detach(o)
	s.objs[o].zone = kop.ZoneLibrary
	s.objs[o].ver++
	s.pl[0].libTop(o) // [commander, ...]
	s.place(bears, kop.ZoneLibrary, false)

	s.runProg(sc.k)
	if s.res.ask != raGenCmdZone {
		t.Fatalf("ask %d, want raGenCmdZone", s.res.ask)
	}
	if z := s.objs[bears].zone; z != kop.ZoneLibrary {
		t.Fatalf("the card before the commander is in zone %v at the ask, want the library", z)
	}
	ver := s.objs[bears].ver
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChCommanderZone {
		t.Fatalf("decision kind=%d sub=%d, want DkChoose/ChCommanderZone", d.Kind, d.Sub)
	}
	s.Apply(d.Options[1])
	if s.Stalled() {
		t.Fatalf("the answer stalled: %s", StallName(s.StallReason()))
	}
	if z := s.objs[o].zone; z != kop.ZoneLibrary {
		t.Fatalf("mode 1 left the commander in zone %v, want the library", z)
	}
	if got := s.objs[bears].ver; got != ver {
		t.Fatalf("the card before the commander moved again on the answer: ver %d -> %d (a second zone change)", ver, got)
	}
}

// TestCommanderSweepShuffleWaitsForAnswer: a battlefield sweep into the
// libraries parks at the commander. The shuffle must not run before the
// answer (gorge's park-at-the-move order), and the answer must place the
// moved cards in move order and shuffle once.
func TestCommanderSweepShuffleWaitsForAnswer(t *testing.T) {
	tbl, cmd := commanderTable(0)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	f := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Commander Sweep Library",
		Types: kop.TInstant,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{
			Op: kop.OpExt, A: kop.Const(int(kop.ExtLibSweep)), F: f,
		}}}},
	})
	card := CardID(len(tbl.Cards) - 1)
	sc := cmdZoneResScen(t, tbl, cmd, card)
	s := sc.s
	bears := sc.perm(v.GrizzlyBears, 0)
	o := s.pl[0].cmd
	s.detach(o)
	sl := s.enter(sc.k, o, 0, false, kop.ZoneCommand)
	s.perms[sl].flags &^= pfSick

	before := s.rng
	s.runProg(sc.k)
	if s.res.ask != raGenCmdZone {
		t.Fatalf("ask %d, want raGenCmdZone", s.res.ask)
	}
	if s.rng != before {
		t.Fatal("the sweep drew from the rng before the answer: the shuffle ran early")
	}
	if z := s.objs[bears].zone; z != kop.ZoneLibrary {
		t.Fatalf("the permanent before the commander is in zone %v at the ask, want the library", z)
	}
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChCommanderZone {
		t.Fatalf("decision kind=%d sub=%d, want DkChoose/ChCommanderZone", d.Kind, d.Sub)
	}
	s.Apply(d.Options[1])
	if s.Stalled() {
		t.Fatalf("the answer stalled: %s", StallName(s.StallReason()))
	}
	if z := s.objs[o].zone; z != kop.ZoneLibrary {
		t.Fatalf("mode 1 left the commander in zone %v, want the library", z)
	}
	// exactly one shuffle, of the library that received the whole sweep: an
	// early shuffle of the partial library followed by the answer's would
	// draw twice
	if got, want := s.rng.Draws-before.Draws, uint64(s.pl[0].libLen()-1); got != want {
		t.Fatalf("the sweep drew %d times, want %d (one shuffle of the %d-card library)", got, want, s.pl[0].libLen())
	}
}

// TestCommanderEachPlayerDigSuspends: an each-player Dig parks at a
// commander in the first player's library. The loop must suspend there
// instead of moving on to the next player (which would overwrite the window
// under the pending ask).
func TestCommanderEachPlayerDigSuspends(t *testing.T) {
	tbl, cmd := commanderTable(0)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	f := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Commander Each Dig",
		Types: kop.TInstant,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{
			Op: kop.OpDigEx, Obj: kop.RefEachPlayer, A: kop.Const(2), B: kop.Const(-1),
			C: int16(kop.ZoneLibrary), F: f,
		}}}},
	})
	card := CardID(len(tbl.Cards) - 1)
	sc := cmdZoneResScen(t, tbl, cmd, card)
	s := sc.s
	bears := sc.obj(v.GrizzlyBears, 0)
	o := s.pl[0].cmd
	s.detach(o)
	s.objs[o].zone = kop.ZoneLibrary
	s.objs[o].ver++
	s.pl[0].libTop(o)
	s.place(bears, kop.ZoneLibrary, false)

	s.runProg(sc.k)
	if s.res.ask != raGenCmdZone {
		t.Fatalf("ask %d, want raGenCmdZone (the each-player loop did not suspend at the park)", s.res.ask)
	}
	if s.g.pickP != 0 {
		t.Fatalf("the dig window is player %d's, want player 0's", s.g.pickP)
	}
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChCommanderZone || d.Player != 0 {
		t.Fatalf("decision kind=%d sub=%d player=%d, want ChCommanderZone to player 0", d.Kind, d.Sub, d.Player)
	}
	s.Apply(d.Options[1])
	if s.Stalled() {
		t.Fatalf("the answer stalled: %s", StallName(s.StallReason()))
	}
	if z := s.objs[o].zone; z != kop.ZoneLibrary {
		t.Fatalf("mode 1 left the commander in zone %v, want player 0's library", z)
	}
	if s.g.pre.state != 0 {
		t.Fatalf("the answer left park state %d behind", s.g.pre.state)
	}
}
