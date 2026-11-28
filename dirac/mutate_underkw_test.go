package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A merged permanent has the keywords of every card in its pile (CR 702.140e),
// the ones with no KwMask bit and the ward cost included. Each fixture card
// mutates UNDER a card, so the top card prints none of what is asserted.

// underKwCards are underKwTable's cards.
type underKwCards struct {
	bird, bolt                        CardID // the vanilla top card and an instant that targets it
	warden                            CardID // a top card printing ward {1}
	plague, rot, rider, menace, sneak CardID // the under cards
	infectF, skulkF                   kop.FilterID
}

// underKwTable is mutateTable with the under-card fixtures. With grant it also
// holds a card whose static grants ward, which routes every ward through
// wardGranted.
func underKwTable(grant bool) (*kop.Table, underKwCards) {
	tbl, mutator, bird, _, bolt := mutateTable()
	c := underKwCards{bird: bird, bolt: bolt}
	add := func(cd kop.CardDef) CardID {
		id := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, cd)
		return id
	}
	under := func(cd kop.CardDef) CardID {
		cd.Types, cd.Power, cd.Toughness = kop.TCreature, 1, 1
		cd.HasMutate, cd.MutateCost = true, kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.G: 1}}
		cd.MutateTgt = tbl.Cards[mutator].MutateTgt
		return add(cd)
	}
	c.warden = add(kop.CardDef{Name: "Test Warden", Types: kop.TCreature, Power: 1, Toughness: 3,
		Keywords: kop.KwWard, HasWard: true, WardN: 1})
	c.plague = under(kop.CardDef{Name: "Test Plague", Infect: true, Keywords: kop.KwWard, HasWard: true, WardN: 2})
	c.rot = under(kop.CardDef{Name: "Test Rot", Wither: true, Toxic: 2})
	c.rider = under(kop.CardDef{Name: "Test Rider", Evasion: kop.EvHorsemanship})
	c.menace = under(kop.CardDef{Name: "Test Intimidator", Evasion: kop.EvIntimidate})
	c.sneak = under(kop.CardDef{Name: "Test Sneak", Evasion: kop.EvSkulk})
	if grant {
		f := kop.FilterID(len(tbl.Filters))
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TArtifact, Ctrl: kop.CtrlYou})
		add(kop.CardDef{Name: "Test Ward Granter", Types: kop.TEnchantment, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: f, Ward: 3}},
		}})
	}
	word := func(n int16) kop.FilterID {
		f := kop.FilterID(len(tbl.Filters))
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Pred: kop.PredWord7, PredN: n})
		return f
	}
	c.infectF, c.skulkF = word(int16(kop.W7WithInfect)), word(int16(kop.W7WithSkulk))
	return tbl, c
}

// underPile mutates under beneath a new permanent of top controlled by player
// 0 and returns the pile.
func underPile(t *testing.T, sc *scen, under, top CardID) Obj {
	t.Helper()
	sc.lands(0, v.Forest, 1)
	b := sc.perm(top, 0)
	mutateCast(t, sc, sc.hand(under, 0), b, false)
	resolveStack(t, sc)
	if c := sc.s.cardAt(sc.slot(b)); c != top {
		t.Fatalf("top card %d, want %d", c, top)
	}
	if rec := sc.s.mergeRecOf(b); rec == nil || rec.n != 1 || sc.s.objs[underObj(rec, 0)].card != under {
		t.Fatalf("card %d is not the pile's under card", under)
	}
	return b
}

// boltAsksWard has player 1 bolt tgt on player 0's turn and returns the
// amounts of the ward triggers on the stack and the lands the first accepted
// ward payment tapped (-1: no payment was asked).
func boltAsksWard(t *testing.T, sc *scen, bolt CardID, tgt Obj) (amts []int, paid int) {
	t.Helper()
	var lands []Obj
	for i := 0; i < 6; i++ {
		lands = append(lands, sc.perm(v.Mountain, 1))
	}
	o := sc.hand(bolt, 1)
	sc.do(t, AkPass, scAny, scAny)
	sc.do(t, AkCast, TCode(o), scAny)
	sc.do(t, AkTarget, scAny, TCode(tgt))
	for i := 0; i < 24; i++ {
		d := sc.s.Step()
		if amts == nil {
			for j := 0; j < int(sc.s.sp); j++ {
				if e := &sc.s.stack[j]; e.ab == abWard {
					amts = append(amts, int(e.trigAmt))
				}
			}
			slices.Sort(amts)
		}
		if d.Kind == DkChoose && d.Sub == ChWardPay {
			before := tappedN(sc, lands)
			for _, a := range d.Options[:d.N] {
				if a.Mode() == 1 {
					sc.s.Apply(a)
					for k := 0; k < 8 && sc.s.Step().Kind != DkPriority; k++ {
						sc.s.Apply(sc.s.Step().Options[0])
					}
					return amts, tappedN(sc, lands) - before
				}
			}
			t.Fatalf("the ward ask offers no payment: %v", d.Options[:d.N])
		}
		if d.Kind == DkPriority {
			if sc.s.sp == 0 && sc.s.npend == 0 {
				break
			}
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		sc.s.Apply(d.Options[0])
	}
	return amts, -1
}

func TestMutateUnderCardInfectAndWard(t *testing.T) {
	tbl, c := underKwTable(false)

	t.Run("infect", func(t *testing.T) {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		b := underPile(t, sc, c.plague, c.bird)
		if p, _ := sc.s.pt(sc.k, sc.slot(b)); p != 1 || tbl.Cards[c.bird].Infect {
			t.Fatalf("precondition: pile power %d, top card infect %v; want the 1-power bird without infect", p, tbl.Cards[c.bird].Infect)
		}
		sc.combat(t, []Obj{b}, nil)
		if got, life := sc.s.Poison(1), sc.s.pl[1].life; got != 1 || life != 20 {
			t.Fatalf("defender poison %d life %d, want 1 and 20: the under card's infect", got, life)
		}
	})

	for _, tc := range []struct {
		name  string
		grant bool
		top   CardID
		want  []int
		paid  int
	}{
		{"under ward", false, c.bird, []int{2}, 2},
		{"under ward beside a ward grant table", true, c.bird, []int{2}, 2},
		{"top and under ward", false, c.warden, []int{1, 2}, 0},
		{"top and under ward beside a ward grant table", true, c.warden, []int{1, 2}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tbl, c := underKwTable(tc.grant)
			sc := newScenTable(t, tbl, Config{NoElide: true})
			if got := sc.k.gen.anyWardGrant; got != tc.grant {
				t.Fatalf("precondition: anyWardGrant %v, want %v", got, tc.grant)
			}
			b := underPile(t, sc, c.plague, tc.top)
			amts, paid := boltAsksWard(t, sc, c.bolt, b)
			if !slices.Equal(amts, tc.want) {
				t.Fatalf("ward triggers %v, want %v", amts, tc.want)
			}
			if tc.paid != 0 && paid != tc.paid {
				t.Fatalf("the ward payment tapped %d lands, want %d", paid, tc.paid)
			}
		})
	}
}

func TestMutateUnderCardNonMaskKeywords(t *testing.T) {
	tbl, c := underKwTable(false)
	// pile builds a pile over a bird beside a plain bird, which prints none
	// of the keywords: the control each assertion differs from.
	pile := func(t *testing.T, under CardID) (sc *scen, b, plain Obj) {
		t.Helper()
		sc = newScenTable(t, tbl, Config{NoElide: true})
		b = underPile(t, sc, under, c.bird)
		return sc, b, sc.perm(c.bird, 0)
	}

	t.Run("toxic", func(t *testing.T) {
		sc, b, _ := pile(t, c.rot)
		sc.combat(t, []Obj{b}, nil)
		if got, life := sc.s.Poison(1), sc.s.pl[1].life; got != 2 || life != 19 {
			t.Fatalf("defender poison %d life %d, want 2 and 19: the under card's toxic 2", got, life)
		}
	})

	t.Run("wither", func(t *testing.T) {
		sc, b, _ := pile(t, c.rot)
		blk := sc.perm(c.warden, 1)
		sc.combat(t, []Obj{b}, [][2]Obj{{blk, b}})
		if p, tg := sc.s.pt(sc.k, sc.slot(blk)); p != 0 || tg != 2 {
			t.Fatalf("blocker %d/%d, want 0/2: the under card's wither", p, tg)
		}
	})

	for _, tc := range []struct {
		name  string
		under CardID
		bit   uint8
		power int
	}{
		{"horsemanship", c.rider, kop.EvHorsemanship, 1},
		{"intimidate", c.menace, kop.EvIntimidate, 1},
		{"skulk", c.sneak, kop.EvSkulk, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, b, plain := pile(t, tc.under)
			blk := sc.perm(c.bird, 1)
			// skulk needs a blocker with greater power than the 1-power bird
			sc.s.perms[sc.slot(blk)].p1 = uint16(tc.power - 1)
			if p, _ := sc.s.pt(sc.k, sc.slot(blk)); p != tc.power {
				t.Fatalf("precondition: blocker power %d, want %d", p, tc.power)
			}
			if got := sc.s.evasionAt(sc.k, sc.slot(plain)); got != 0 {
				t.Fatalf("precondition: a plain bird has evasion %b", got)
			}
			if got := sc.s.evasionAt(sc.k, sc.slot(b)); got != tc.bit {
				t.Fatalf("pile evasion %b, want the under card's %b", got, tc.bit)
			}
			if !sc.s.evasionEvades(sc.k, sc.slot(blk), sc.slot(b)) {
				t.Fatalf("the blocker can block the pile")
			}
		})
	}

	t.Run("filter words", func(t *testing.T) {
		for _, tc := range []struct {
			under CardID
			id    kop.KwID
		}{
			{c.rot, kop.KwToxic}, {c.rider, kop.KwHorsemanship}, {c.plague, kop.KwPrintedWard},
		} {
			sc, b, plain := pile(t, tc.under)
			has := func(o Obj) bool {
				fc := fctx{k: sc.k, zone: kop.ZoneBattlefield, sl: sc.slot(o)}
				return sc.s.fwWithKeyword(&fc, &kop.FLit{Arg: int16(tc.id)})
			}
			if has(plain) || !has(b) {
				t.Fatalf("with keyword %d: plain bird %v pile %v, want false and true", tc.id, has(plain), has(b))
			}
		}
		for _, tc := range []struct {
			under CardID
			f     kop.FilterID
		}{
			{c.plague, c.infectF}, {c.sneak, c.skulkF},
		} {
			sc, b, plain := pile(t, tc.under)
			has := func(o Obj) bool { return sc.s.matchPermGen(sc.k, tc.f, sc.slot(o), 0, NoSlot, layerKw) }
			if has(plain) || !has(b) {
				t.Fatalf("filter %d: plain bird %v pile %v, want false and true", tc.f, has(plain), has(b))
			}
		}
	})

	t.Run("lost abilities", func(t *testing.T) {
		sc, b, _ := pile(t, c.plague)
		sl := sc.slot(b)
		if !sc.s.infectAt(sc.k, sl) {
			t.Fatalf("precondition: the pile has no infect to lose")
		}
		sc.s.perms[sl].ovr.flags |= ovLoseAll
		if sc.s.infectAt(sc.k, sl) {
			t.Fatalf("a pile that lost its abilities kept its under card's infect")
		}
	})
}
