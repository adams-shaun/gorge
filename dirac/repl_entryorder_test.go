package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// entFix is a table with a printed enters-tapped land, a plain land, "lands
// you control enter untapped" and "lands you control enter tapped" carriers,
// and a program that returns the remembered card to the battlefield.
type entFix struct {
	sc                           *scen
	tapLand, plain, untapC, tapC CardID
	mover, ctrLand               CardID
	choiceLand                   CardID // enters tapped, and as it enters, choose a color (Sunken Citadel)
}

func newEntFix(t *testing.T) *entFix {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var pl CardID = 255
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			pl = CardID(i)
		}
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	f := kop.FilterID(len(tbl.Filters) - 1)
	base := CardID(len(tbl.Cards))
	untap := []kop.Instr{{Op: kop.OpExt, A: kop.Const(int(kop.ExtEnterUntapped))}}
	tap := []kop.Instr{{Op: kop.OpEnterTapped}}
	carrier := func(name string, prog []kop.Instr) kop.CardDef {
		return kop.CardDef{Name: name, Types: kop.TArtifact, Abilities: []kop.Ability{
			{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplOthersEnterProgram, Filter: f}, Prog: prog}}}
	}
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Tap Land", Types: kop.TLand, Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplEntersTapped}}}},
		kop.CardDef{Name: "Plain Land", Types: kop.TLand},
		carrier("Untap Carrier", untap),
		carrier("Tap Carrier", tap),
		kop.CardDef{Name: "Mover", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpReturnEx, Obj: kop.RefRemembered, C: int16(kop.ZoneBattlefield)}}}}},
		kop.CardDef{Name: "Counter Land", Types: kop.TLand, Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplEntersTapped}},
			{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplEntersWithCounters, A: 1, C: int16(kop.CtrP1P1)}}}},
		kop.CardDef{Name: "Choice Land", Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplEntersTapped}},
			{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplEntersProgram}, Prog: []kop.Instr{{Op: kop.OpChooseColor}}}},
		},
	)
	sc := newScenTableLib(t, tbl, pl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyEntUntap {
		t.Fatal("the fixture table has no entry-order carrier")
	}
	sc.s.g.on = true
	return &entFix{sc: sc, tapLand: base, plain: base + 1, untapC: base + 2, tapC: base + 3, mover: base + 4, ctrLand: base + 5, choiceLand: base + 6}
}

func (e *entFix) tapped(o Obj) bool { return e.sc.tapped(o) }

// play plays land o as player p from the priority window.
func (e *entFix) play(o Obj, p uint8) {
	s := e.sc.s
	s.prio = p
	s.applyPriority(e.sc.k, MakeAction(AkPlayLand, TCode(o), 0, 0))
}

// ask returns the posed entry-order decision.
func (e *entFix) ask(t *testing.T) Decision {
	t.Helper()
	d := e.sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder {
		t.Fatalf("decision kind %d sub %d, want DkChoose/ChGenReplOrder", d.Kind, d.Sub)
	}
	return d
}

// srcs is the options' sources by scan index (the action's mode).
func srcs(d Decision) []Obj {
	out := make([]Obj, 4)
	for _, a := range d.Options[:d.N] {
		out[a.Mode()] = Obj(a.Source())
	}
	return out[:d.N]
}

func pick(d Decision, src Obj) Action {
	for _, a := range d.Options[:d.N] {
		if Obj(a.Source()) == src {
			return a
		}
	}
	panic("not an option")
}

// pickScan is d's option whose mode is m (the entry ask's scan index).
func pickScan(d Decision, m uint8) Action {
	for _, a := range d.Options[:d.N] {
		if a.Mode() == m {
			return a
		}
	}
	panic("no such mode")
}

// TestReplEntryOrder: CR 616.1 for a land entering under its own enters-
// tapped body and an other-enter untap body: the ask is
// posed to the entering object's player with the card still in its zone,
// the last body applied decides the tapped bit, a lone class asks nothing, a
// third body re-asks (CR 616.1f), and a competition the ask cannot pose
// stalls.
func TestReplEntryOrder(t *testing.T) {
	t.Run("ask_before_move_both_orders", func(t *testing.T) {
		for _, tapLast := range []bool{false, true} {
			e := newEntFix(t)
			sc := e.sc
			c := sc.perm(e.untapC, 0)
			l := sc.hand(e.tapLand, 0)
			e.play(l, 0)
			if sc.s.objs[l].zone != kop.ZoneHand || sc.s.pl[0].landsPlayed != 1 {
				t.Fatalf("zone %d, landsPlayed %d: want the card still in the hand with the play counted (gorge counts it at the play)", sc.s.objs[l].zone, sc.s.pl[0].landsPlayed)
			}
			d := e.ask(t)
			if d.Player != 0 || d.N != 2 {
				t.Fatalf("player %d options %d, want player 0 with both sources", d.Player, d.N)
			}
			if got := srcs(d); got[0] != l || got[1] != c {
				t.Fatalf("options %v, want the land's own line then the carrier (scan order) [%d %d]", got, l, c)
			}
			first := l // untap last: the land's tap applies first
			if tapLast {
				first = c
			}
			sc.s.applyGen(sc.k, pick(d, first))
			if !sc.onBF(l) || e.tapped(l) != tapLast {
				t.Fatalf("tapLast=%v: on battlefield %v tapped %v", tapLast, sc.onBF(l), e.tapped(l))
			}
			if sc.s.pl[0].landsPlayed != 1 || sc.s.ctl != ctlPriority || sc.s.g.ro.n != 0 || sc.s.g.ro.pcol != 0 {
				t.Fatalf("after the answer: landsPlayed %d ctl %d ro %+v", sc.s.pl[0].landsPlayed, sc.s.ctl, sc.s.g.ro.replHdr)
			}
			if sc.s.g.stalled != 0 {
				t.Fatalf("stalled %d", sc.s.g.stalled)
			}
		}
	})
	t.Run("damage_ask_pcol_is_not_a_route", func(t *testing.T) {
		// a damage ask's pcol holds the PreventColor candidates (bit 0 =
		// entLand): it must stay a damage ask, never the entry ask
		for _, noncombat := range []bool{false, true} {
			e := newEntFix(t)
			s := e.sc.s
			s.g.ro.replHdr = replHdr{n: 2, p: 1, cand: [4]Obj{7, 9}, pcol: 1, on: !noncombat, nca: noncombat}
			var d Decision
			if !s.genAskOptions(e.sc.k, &d) || d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 1 || d.N != 2 {
				t.Fatalf("noncombat=%v: decision %+v, want the damage order ask for player 1 with both candidates", noncombat, d)
			}
			if got := srcs(d); got[0] != 7 || got[1] != 9 {
				t.Fatalf("noncombat=%v: options %v, want the damage candidates [7 9]", noncombat, got)
			}
			if s.entryAsked(e.sc.k) {
				t.Fatalf("noncombat=%v: a damage ask taken for the entry ask", noncombat)
			}
		}
	})
	t.Run("affected_player_is_the_entering_objects", func(t *testing.T) {
		e := newEntFix(t)
		sc := e.sc
		sc.perm(e.untapC, 1)
		l := sc.hand(e.tapLand, 1)
		sc.s.playLandNow(sc.k, l, 1)
		if d := e.ask(t); d.Player != 1 {
			t.Fatalf("asked player %d, want 1", d.Player)
		}
	})
	t.Run("one_class_asks_nothing", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			land       func(e *entFix) CardID
			carrier    func(e *entFix) CardID
			wantTapped bool
		}{
			{"tap_land_alone", func(e *entFix) CardID { return e.tapLand }, nil, true},
			{"untap_carrier_plain_land", func(e *entFix) CardID { return e.plain }, func(e *entFix) CardID { return e.untapC }, false},
			{"tap_land_tap_carrier", func(e *entFix) CardID { return e.tapLand }, func(e *entFix) CardID { return e.tapC }, true},
		} {
			e := newEntFix(t)
			sc := e.sc
			if tc.carrier != nil {
				sc.perm(tc.carrier(e), 0)
			}
			l := sc.hand(tc.land(e), 0)
			e.play(l, 0)
			if !sc.onBF(l) || e.tapped(l) != tc.wantTapped || sc.s.g.ro.n != 0 || sc.s.g.stalled != 0 {
				t.Fatalf("%s: on battlefield %v tapped %v ro.n %d stalled %d", tc.name, sc.onBF(l), e.tapped(l), sc.s.g.ro.n, sc.s.g.stalled)
			}
		}
	})
	t.Run("two_untaps_one_tap_reasks", func(t *testing.T) {
		// scan order: the land's own tap, then the carriers in entry order.
		// gorge emits the move at the first answer, so the second ask (CR
		// 616.1f) is posed with the land on the battlefield.
		for _, tc := range []struct {
			first, second int // scan indexes of the picks; second -1: no second ask
			wantTapped    bool
		}{
			{0, -1, false}, // the tap first: both untaps commute and follow
			{1, 0, false},  // an untap, then the tap: the other untap is applied last
			{1, 2, true},   // an untap, then the other untap: the tap is applied last
			{2, 0, false},
			{2, 1, true},
		} {
			e := newEntFix(t)
			sc := e.sc
			c1 := sc.perm(e.untapC, 0)
			c2 := sc.perm(e.untapC, 0)
			l := sc.hand(e.tapLand, 0)
			e.play(l, 0)
			d := e.ask(t)
			if d.N != 3 {
				t.Fatalf("options %d, want 3", d.N)
			}
			order := []Obj{l, c1, c2}
			sc.s.applyGen(sc.k, pick(d, order[tc.first]))
			if tc.second >= 0 {
				if !sc.onBF(l) || e.tapped(l) != (tc.first == 0) {
					t.Fatalf("pick %d: on battlefield %v tapped %v, want entered with the first body applied", tc.first, sc.onBF(l), e.tapped(l))
				}
				d2 := e.ask(t)
				if d2.N != 2 || d2.Player != 0 {
					t.Fatalf("pick %d: re-asked with %d options for player %d, want 2 (the pick is spent)", tc.first, d2.N, d2.Player)
				}
				sc.s.applyGen(sc.k, pick(d2, order[tc.second]))
				if sc.s.ctl != ctlPriority || sc.s.g.ro.n != 0 || sc.s.g.ro.pcol != 0 {
					t.Fatalf("pick %d,%d: ctl %d ro %+v after the last answer", tc.first, tc.second, sc.s.ctl, sc.s.g.ro.replHdr)
				}
			}
			if !sc.onBF(l) || e.tapped(l) != tc.wantTapped {
				t.Fatalf("picks %d,%d: on battlefield %v tapped %v, want tapped %v", tc.first, tc.second, sc.onBF(l), e.tapped(l), tc.wantTapped)
			}
		}
	})
	t.Run("last_remaining_tap_decides", func(t *testing.T) {
		e := newEntFix(t)
		sc := e.sc
		c1 := sc.perm(e.untapC, 0)
		c2 := sc.perm(e.untapC, 0)
		l := sc.hand(e.tapLand, 0)
		e.play(l, 0)
		d := e.ask(t)
		sc.s.applyGen(sc.k, pick(d, c1)) // remaining: the tap and an untap
		d2 := e.ask(t)
		sc.s.applyGen(sc.k, pick(d2, c2)) // the untap now, the tap is applied last
		if !sc.onBF(l) || !e.tapped(l) {
			t.Fatalf("on battlefield %v tapped %v, want entered tapped (the tap applied last)", sc.onBF(l), e.tapped(l))
		}
	})
	t.Run("nine_candidates_stall", func(t *testing.T) {
		e := newEntFix(t)
		sc := e.sc
		for i := 0; i < 8; i++ {
			sc.perm(e.untapC, 0)
		}
		l := sc.hand(e.tapLand, 0)
		e.play(l, 0)
		if sc.s.g.stalled != StallReplCands || sc.s.g.ro.n != 0 {
			t.Fatalf("stalled %d ro.n %d, want StallReplCands and no ask", sc.s.g.stalled, sc.s.g.ro.n)
		}
	})
	t.Run("unsupported_route_stalls", func(t *testing.T) {
		e := newEntFix(t)
		sc := e.sc
		sc.perm(e.untapC, 0)
		o := sc.obj(e.tapLand, 0)
		sc.s.enter(sc.k, o, 0, false, kop.ZoneNone) // a token / clone / spell route: no ask
		if sc.s.g.stalled != StallEntryOrder {
			t.Fatalf("stalled %d, want StallEntryOrder", sc.s.g.stalled)
		}
	})
	t.Run("self_program_stalls", func(t *testing.T) {
		e := newEntFix(t)
		sc := e.sc
		sc.perm(e.untapC, 0)
		l := sc.hand(e.ctrLand, 0)
		e.play(l, 0)
		if sc.s.g.stalled != StallEntryOrder || sc.s.g.ro.n != 0 {
			t.Fatalf("stalled %d ro.n %d, want StallEntryOrder and no ask", sc.s.g.stalled, sc.s.g.ro.n)
		}
	})
	t.Run("resolving_move_asks_with_the_card_in_hand", func(t *testing.T) {
		for _, tapLast := range []bool{false, true} {
			e := newEntFix(t)
			sc, s, k := e.sc, e.sc.s, e.sc.k
			c := sc.perm(e.untapC, 0)
			l := sc.hand(e.tapLand, 0)
			s.g.rem[0], s.g.nrem = l, 1
			s.res = resCtx{e: stackEntry{kind: seSpell, obj: sc.obj(e.mover, 0), card: e.mover, ctrl: 0, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
			s.res.e.tgt = noTargets
			s.ctl = ctlResolve
			s.runProg(k)
			if s.res.ask != raGenEntryOrder || s.objs[l].zone != kop.ZoneHand {
				t.Fatalf("ask %d zone %d, want the entry-order ask with the card in the hand", s.res.ask, s.objs[l].zone)
			}
			d := s.Step()
			if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != 0 || d.N != 2 {
				t.Fatalf("decision %d/%d player %d options %d", d.Kind, d.Sub, d.Player, d.N)
			}
			first := l
			if tapLast {
				first = c
			}
			s.applyResolveGen(k, pick(d, first))
			if !sc.onBF(l) || e.tapped(l) != tapLast || s.g.ro.pcol != 0 {
				t.Fatalf("tapLast=%v: on battlefield %v tapped %v ent %d", tapLast, sc.onBF(l), e.tapped(l), s.g.ro.pcol)
			}
		}
	})
}

// TestEntryOrderOwnChoiceBodyAsks: the entering land's own as-enters choice
// record (Sunken Citadel's "As it enters, choose a color") joins the CR
// 616.1 order ask beside its own enters-tapped body and an other-enter
// untap body (Spelunking). The choice is asked before the move (preLandAsk)
// as gorge asks it, so the record candidate competes for the order without
// touching the tapped bit: the last tap or untap applied still decides, and
// a record pick that leaves a tap and an untap re-asks (CR 616.1f) once the
// land has entered.
func TestEntryOrderOwnChoiceBodyAsks(t *testing.T) {
	// playChoice puts an Untap Carrier on the battlefield, plays the choice
	// land from the hand, answers its as-enters colour ask, and returns the
	// posed entry-order ask: the land is still in the hand while the colour
	// is asked, and on the battlefield only after the first pick.
	playChoice := func(t *testing.T, e *entFix) (Decision, Obj) {
		t.Helper()
		sc := e.sc
		c := sc.perm(e.untapC, 0)
		if !sc.onBF(c) {
			t.Fatalf("carrier on battlefield %v", sc.onBF(c))
		}
		l := sc.hand(e.choiceLand, 0)
		e.play(l, 0)
		if sc.s.objs[l].zone != kop.ZoneHand {
			t.Fatalf("zone %d, want the choice land still in the hand while its colour is asked", sc.s.objs[l].zone)
		}
		d := sc.s.Step()
		if d.Kind != DkChoose || d.Sub != ChGenColor || d.N < 1 {
			t.Fatalf("decision %d/%d options %d, want the as-enters colour ask", d.Kind, d.Sub, d.N)
		}
		sc.s.applyGen(sc.k, d.Options[0])
		return e.ask(t), l
	}
	t.Run("ask_has_three_options", func(t *testing.T) {
		e := newEntFix(t)
		d, l := playChoice(t, e)
		if d.Player != 0 || d.N != 3 {
			t.Fatalf("player %d options %d, want player 0 with 3 options", d.Player, d.N)
		}
		got := srcs(d)
		if got[0] != l || got[1] != l || got[2] == l {
			t.Fatalf("options %v, want the land's tap and record candidates then the carrier [%d %d]", got, l, got[2])
		}
		var modes uint8
		for _, a := range d.Options[:d.N] {
			if a.Mode() > 2 {
				t.Fatalf("mode %d, want a scan index of the 3 candidates", a.Mode())
			}
			modes |= 1 << a.Mode()
		}
		if modes != 0b111 {
			t.Fatalf("modes %03b, want the scan indexes 0, 1 and 2 each once", modes)
		}
	})
	for _, tc := range []struct {
		name       string
		first      uint8 // scan index of the first pick
		second     uint8 // scan index of the re-ask pick; entNoSecond: no re-ask
		wantTapped bool
	}{
		{"carrier_first_ends_tapped", 2, entNoSecond, true},
		{"land_tap_first_ends_untapped", 0, entNoSecond, false},
		{"record_first_then_tap", 1, 0, false},
		{"record_first_then_carrier", 1, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEntFix(t)
			sc := e.sc
			d, l := playChoice(t, e)
			sc.s.applyGen(sc.k, pickScan(d, tc.first))
			if tc.second != entNoSecond {
				// the record pick leaves a tap and an untap: the land has
				// entered untapped (no tap applied yet) and the ask repeats
				if !sc.onBF(l) || e.tapped(l) {
					t.Fatalf("after the record pick: on battlefield %v tapped %v, want entered untapped with the ask left", sc.onBF(l), e.tapped(l))
				}
				d2 := e.ask(t)
				if d2.N != 2 {
					t.Fatalf("re-ask options %d, want 2", d2.N)
				}
				sc.s.applyGen(sc.k, pickScan(d2, tc.second))
			}
			if !sc.onBF(l) || e.tapped(l) != tc.wantTapped {
				t.Fatalf("picks %d,%d: on battlefield %v tapped %v, want tapped %v", tc.first, tc.second, sc.onBF(l), e.tapped(l), tc.wantTapped)
			}
			if sc.s.g.ro.n != 0 || sc.s.g.ro.pcol != 0 || sc.s.g.stalled != 0 || sc.s.ctl != ctlPriority {
				t.Fatalf("after the last answer: ro %+v stalled %d ctl %d", sc.s.g.ro.replHdr, sc.s.g.stalled, sc.s.ctl)
			}
			if d := sc.s.Step(); d.Kind != DkPriority {
				// the record's choice was answered before the move: the
				// entry asks nothing more
				t.Fatalf("next decision %d/%d, want priority", d.Kind, d.Sub)
			}
		})
	}
}

// entNoSecond: the first pick ends the ask (no re-ask expected).
const entNoSecond uint8 = 0xff
