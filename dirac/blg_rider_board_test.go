package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// blgRiderBoard is blgRiderBoardSubs with every basic land type granted.
func blgRiderBoard(t *testing.T, cfg Config, names ...string) (*scen, map[string]CardID, map[string]Obj) {
	t.Helper()
	return blgRiderBoardSubs(t, cfg, []string{"Plains", "Island", "Swamp", "Mountain", "Forest"}, names...)
}

// blgRiderBoardSubs builds a scenario over the manarider fixture pool with
// one permanent of each named card and a synthetic grant card on the
// battlefield: a StatContinuous override making every permanent a land of
// subs' basic types (the Ashaya class: gorge's land-type words carry the
// subtypes' intrinsic abilities, CR 305.6). The pool holds the bounded rider
// cards but no grant card, so the grant is a test-only card.
func blgRiderBoardSubs(t *testing.T, cfg Config, subs []string, names ...string) (*scen, map[string]CardID, map[string]Obj) {
	t.Helper()
	set := cardsettest.Set(t, "manarider")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	// The pool's names include the basic lands but no card of it names a
	// basic subtype, so the matchup's subtype list is empty: append the
	// grant's ones (test tables do).
	var sm kop.SubMask
	for _, n := range subs {
		i := -1
		for j, st := range tbl.Subtypes {
			if st == n {
				i = j
				break
			}
		}
		if i < 0 {
			i = len(tbl.Subtypes)
			tbl.Subtypes = append(tbl.Subtypes, n)
		}
		sm[i/64] |= 1 << (uint(i) % 64)
	}
	tbl.Overrides = append(tbl.Overrides, kop.Override{AddTypes: kop.TLand, AddSub: sm})
	tbl.Filters = append(tbl.Filters, kop.Filter{}) // the empty filter matches every permanent
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Land Grant",
		Types: kop.TEnchantment,
		Abilities: []kop.Ability{{
			Kind: kop.AbStatic,
			Static: kop.Static{
				Kind:     kop.StatOverride,
				Filter:   kop.FilterID(len(tbl.Filters) - 1),
				Override: uint16(len(tbl.Overrides)),
			},
		}},
	})
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTable(t, tbl, cfg)
	objs := map[string]Obj{}
	for _, n := range names {
		c, ok := id[n]
		if !ok {
			t.Fatalf("%s not in the manarider table", n)
		}
		objs[n] = sc.perm(c, 0)
	}
	sc.perm(id["Test Land Grant"], 0)
	if !sc.k.gen.blgOn {
		t.Fatal("precondition: blgOn unset: the grant's AddSub carries no basic land type")
	}
	return sc, id, objs
}

// blgRiderReady clears the counters and the tap state a pool table's def
// enters with, then puts ctr counters of kind k on the source: its
// activation cost is payable.
func blgRiderReady(sc *scen, sl Slot, k, ctr int) {
	pm := &sc.s.perms[sl]
	pm.subCtrN(sc.s, k, pm.ctrN(sc.s, k))
	if ctr > 0 {
		pm.addCtrN(sc.s, k, ctr)
	}
	pm.flags &^= pfTapped | pfSick
}
