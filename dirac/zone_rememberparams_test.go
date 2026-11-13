package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newRemParamsScen is an FDN-table scenario (a generated table: k.gen is
// set) resolving a spell of player 0 with no targets.
func newRemParamsScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 20}, {Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	e := stackEntry{card: id["Plains"], ctrl: 0, obj: NoObj, kind: seSpell, trigObj: NoObj}
	for i := range e.tgt {
		e.tgt[i] = uint8(NoTarget)
	}
	sc.s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	return sc, id
}

// remCount evaluates ExRememberedCount.
func remCount(sc *scen) int {
	sc.k.t.Exprs = append(sc.k.t.Exprs, kop.Expr{Kind: kop.ExRememberedCount})
	return sc.s.expr(sc.k, int16(len(sc.k.t.Exprs)-1), evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot, e: &sc.s.res.e})
}

// TestChangeZoneAllRemembers: ChangeZoneAll RememberChanged$ (MvRemember on
// OpChangeZoneAllEx) remembers every swept permanent, and a 13th remembered
// object stalls (StallRemember, maxRem 12).
func TestChangeZoneAllRemembers(t *testing.T) {
	sc, id := newRemParamsScen(t)
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: kop.TCreature})
	f := kop.FilterID(len(sc.k.t.Filters) - 1)
	in := kop.Instr{Op: kop.OpChangeZoneAllEx, F: f, C: int16(kop.ZoneExile) | kop.MvRemember<<3}
	for i := 0; i < 3; i++ {
		sc.perm(id["Savannah Lions"], uint8(i&1))
	}
	sc.s.execChangeZoneAll(sc.k, &in, false)
	if got := remCount(sc); got != 3 {
		t.Fatalf("ExRememberedCount %d after exiling 3 creatures, want 3", got)
	}
	for i := 0; i < int(sc.s.g.nrem); i++ {
		if sc.s.objs[sc.s.g.rem[i]].zone != kop.ZoneExile {
			t.Fatalf("remembered object %d is not in exile", sc.s.g.rem[i])
		}
	}

	// a whole graveyard's worth (Sins of the Father's chapter III, std-trc
	// pair 35, stalled at 13 before the set held 64)
	sc, id = newRemParamsScen(t)
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: kop.TCreature})
	sc.s.g.on = true
	for i := 0; i < 40; i++ {
		sc.perm(id["Savannah Lions"], uint8(i&1))
	}
	sc.s.execChangeZoneAll(sc.k, &in, false)
	if got := remCount(sc); got != 40 || sc.s.g.stalled != 0 {
		t.Fatalf("40 swept creatures: %d remembered, stall %d", got, sc.s.g.stalled)
	}

	sc, id = newRemParamsScen(t)
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: kop.TCreature})
	sc.s.g.on = true
	for i := 0; i < maxRem+1; i++ {
		sc.perm(id["Savannah Lions"], uint8(i&1))
	}
	sc.s.execChangeZoneAll(sc.k, &in, false)
	if sc.s.g.stalled != StallRemember {
		t.Fatalf("%d swept creatures: stall %d, want StallRemember", maxRem+1, sc.s.g.stalled)
	}
}

// TestForgetOtherKeepsResolvedObject: ForgetOtherRemembered$ resolves the
// Defined$ Remembered objects before forgetting the set (gorge
// effChangeZone), so the return moves both; RememberChanged$ re-adds them.
func TestForgetOtherKeepsResolvedObject(t *testing.T) {
	for _, keep := range []bool{true, false} {
		sc, id := newRemParamsScen(t)
		s := sc.s
		a, b := sc.perm(id["Savannah Lions"], 0), sc.perm(id["Savannah Lions"], 0)
		for _, o := range []Obj{a, b} {
			s.leave(sc.k, s.objs[o].slot, kop.ZoneExile, false)
		}
		s.g.rem[0], s.g.rem[1], s.g.nrem = a, b, 2
		fl := int16(kop.MvForgetOther)
		if keep {
			fl |= kop.MvRemember
		}
		in := kop.Instr{Op: kop.OpReturnEx, Obj: kop.RefRemembered, C: int16(kop.ZoneBattlefield) | fl<<3}
		s.execGen(sc.k, &in)
		for _, o := range []Obj{a, b} {
			if s.objs[o].zone != kop.ZoneBattlefield {
				t.Fatalf("remembered %d not returned (zone %d)", o, s.objs[o].zone)
			}
		}
		want := 0
		if keep {
			want = 2
		}
		if int(s.g.nrem) != want || keep && (s.g.rem[0] != a || s.g.rem[1] != b) {
			t.Fatalf("MvRemember=%v: remembered %v, want %d (the returned objects)", keep, s.g.rem[:s.g.nrem], want)
		}
	}
}

// TestRememberLKIController: a targeted bounce with RememberLKI$ (OpBounce
// MvRemember|MvLKI) of a permanent player 1 controls and player 0 owns; a
// Remembered + Card.YouCtrl read then answers with the captured controller
// (gorge rememberedControllerLKI, CR 608.2h), not the owner the hand gives.
func TestRememberLKIController(t *testing.T) {
	sc, id := newRemParamsScen(t)
	s := sc.s
	o := sc.obj(id["Savannah Lions"], 0)
	sl := s.enter(sc.k, o, 1, false, kop.ZoneNone)
	s.perms[sl].flags &^= pfSick
	s.npend = 0
	s.res.e.tgt[0], s.res.e.tver[0], s.res.e.nt = uint8(o), s.objs[o].ver, 1
	in := kop.Instr{Op: kop.OpBounce, Obj: kop.RefTarget0, C: kop.MvRemember | kop.MvLKI}
	s.execGen(sc.k, &in)
	if s.objs[o].zone != kop.ZoneHand || s.g.nrem != 1 || s.g.rem[0] != o {
		t.Fatalf("bounce: zone %d, remembered %v", s.objs[o].zone, s.g.rem[:s.g.nrem])
	}
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(sc.k.t.Filters) - 1)
	for p, want := range [2]int{0, 1} {
		ctx := evalCtx{ctrl: uint8(p), src: NoObj, srcSl: NoSlot, e: &s.res.e}
		if got := s.countRefMatches(sc.k, kop.RefRemembered, f, ctx); got != want {
			t.Fatalf("Remembered Card.YouCtrl for player %d: %d, want %d", p, got, want)
		}
	}
	if got := s.remController(); got != 1 {
		t.Fatalf("RememberedController %d, want the captured controller 1", got)
	}
}

// TestDiscardUnlessType: Discard NumCards$ 2 UnlessType$ Creature
// (OpDiscardUnless; gorge effDiscard's discard_unless election, CR 701.9b) over
// four hands: none eligible (no election, the 2-card pick), one eligible
// answered unless (discarded with no pick), two eligible answered unless (a
// 1-card pick over exactly those, in hand order), and ordinary with an
// eligible card present (the 2-card pick over the whole hand).
func TestDiscardUnlessType(t *testing.T) {
	type tc struct {
		name   string
		hand   []string // "L" Savannah Lions (eligible), "P" Plains
		unless int      // the election's answer: 1 unless, 0 ordinary, -1 no election expected
		pick   []int    // hand positions the next ChDiscard ask must offer, nil: none
	}
	for _, c := range []tc{
		{"none eligible", []string{"P", "P", "P"}, -1, []int{0, 1, 2}},
		{"one eligible, unless", []string{"P", "L", "P"}, 1, nil},
		{"two eligible, unless", []string{"L", "P", "L"}, 1, []int{0, 2}},
		{"ordinary", []string{"L", "P", "P"}, 0, []int{0, 1, 2}},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc, id := newRemParamsScen(t)
			s, k := sc.s, sc.k
			k.t.Filters = append(k.t.Filters, kop.Filter{Types: kop.TCreature})
			pl := id["Plains"]
			in := kop.Instr{Op: kop.OpDiscardUnless, Obj: kop.RefController, A: kop.Const(2),
				F: kop.FilterID(len(k.t.Filters) - 1)}
			k.t.Cards[pl].Abilities = append(k.t.Cards[pl].Abilities, kop.Ability{Prog: []kop.Instr{in}})
			src := sc.perm(pl, 0)
			var hand []Obj
			for _, h := range c.hand {
				cid := pl
				if h == "L" {
					cid = id["Savannah Lions"]
				}
				hand = append(hand, sc.hand(cid, 0))
			}
			e := stackEntry{kind: seAbility, obj: src, srcVer: s.objs[src].ver, card: pl, ab: uint8(len(k.t.Cards[pl].Abilities) - 1), trigObj: NoObj}
			for i := range e.tgt {
				e.tgt[i] = uint8(NoTarget)
			}
			s.stack[0], s.sp = e, 1
			s.resolveTop(k)
			d := s.Step()
			if c.unless >= 0 {
				if d.Kind != DkChoose || d.Sub != ChGenDiscUnless || d.N != 2 {
					t.Fatalf("decision kind %d sub %d n %d, want the unless election", d.Kind, d.Sub, d.N)
				}
				s.Apply(MakeAction(AkChoose, 0, 0, uint8(c.unless)))
				d = s.Step()
			}
			if c.pick == nil {
				if d.Kind == DkChoose && d.Sub == ChDiscard {
					t.Fatalf("a pick was asked with one eligible card: %v", d.Options[:d.N])
				}
				if s.objs[hand[1]].zone != kop.ZoneGraveyard || s.pl[0].handN != 2 {
					t.Fatalf("the single eligible card was not discarded alone (zone %d, hand %d)", s.objs[hand[1]].zone, s.pl[0].handN)
				}
				return
			}
			if d.Kind != DkChoose || d.Sub != ChDiscard || int(d.N) != len(c.pick) {
				t.Fatalf("decision kind %d sub %d n %d, want a ChDiscard pick over %d cards", d.Kind, d.Sub, d.N, len(c.pick))
			}
			for i, h := range c.pick {
				if Obj(d.Options[i].Source()) != hand[h] {
					t.Fatalf("pick option %d is %d, want hand card %d (%d)", i, d.Options[i].Source(), h, hand[h])
				}
			}
			want := 2
			if c.unless == 1 {
				want = 1
			}
			for n := 0; n < want; n++ {
				if d.Kind != DkChoose || d.Sub != ChDiscard {
					t.Fatalf("pick %d: decision kind %d sub %d", n, d.Kind, d.Sub)
				}
				s.Apply(d.Options[0])
				d = s.Step()
			}
			if got := int(s.pl[0].handN); got != len(c.hand)-want {
				t.Fatalf("hand %d after the discard, want %d", got, len(c.hand)-want)
			}
		})
	}
}
