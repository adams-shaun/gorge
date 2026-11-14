package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// laterStoreProbe is the storagex fixture's later-ability shape: Mage-Ring
// Network's plain {T} first mana ability beside its {T}, remove X STORAGE
// counters one. Sol Ring and Mind Stone can be cast only from that source's
// mana.
type laterStoreProbe struct {
	tbl     *kop.Table
	storage CardID
	one     CardID // Sol Ring, {1}
	two     CardID // Mind Stone, {2}
}

func laterStoreProbeMatchup(t *testing.T) laterStoreProbe {
	t.Helper()
	set := cardsettest.Set(t, "storagex")
	deck := []cardset.Entry{
		{Name: "Mage-Ring Network", Count: 38},
		{Name: "Mind Stone", Count: 1},
		{Name: "Sol Ring", Count: 1},
	}
	tbl, ids, _, err := set.Matchup(deck, []cardset.Entry{{Name: "Mage-Ring Network", Count: 40}})
	if err != nil {
		t.Fatal(err)
	}
	var p laterStoreProbe
	p.tbl = tbl
	for _, c := range ids {
		switch tbl.Cards[c].Name {
		case "Mage-Ring Network":
			p.storage = c
		case "Sol Ring":
			p.one = c
		case "Mind Stone":
			p.two = c
		}
	}
	if p.storage == NoCard || p.one == NoCard || p.two == NoCard {
		t.Fatalf("the later-store probe cards are missing from the table")
	}
	return p
}

// TestStoreLaterPaysGeneric: Mage-Ring Network holds 2 STORAGE counters and is
// the only mana source. A {2} spell must be paid by the later counter-X
// ability, removing both counters; a {1} spell must prefer the plain {T} first
// ability and leave both counters. The later-ability prep mark is what makes
// scriptCtr reach the counter-X ability, so reverting it fails the {2} case
// (the cast is unoffered) and would fail the precondition here.
func TestStoreLaterPaysGeneric(t *testing.T) {
	p := laterStoreProbeMatchup(t)
	for _, tc := range []struct {
		name    string
		spell   CardID
		wantCtr int
	}{
		{"two counters pay the {2} spell", p.two, 0},
		{"plain tap pays the {1} spell", p.one, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := newScenTableLib(t, p.tbl, p.storage, Config{NoElide: true})
			o := sc.perm(p.storage, 0)
			sl := sc.slot(o)
			sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrStorage), 2)
			if got := sc.s.perms[sl].ctrN(sc.s, int(kop.CtrStorage)); got != 2 {
				t.Fatalf("precondition: the source holds %d counters, want 2", got)
			}
			if !sc.s.scriptCtr(sc.k, 0).has(sl) {
				t.Fatalf("precondition: the later {T} SubCounter<X/STORAGE> ability is not reachable")
			}
			card := sc.hand(tc.spell, 0)
			d := sc.s.Step()
			offered := false
			for i := 0; i < int(d.N); i++ {
				if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(card)) {
					offered = true
					sc.s.Apply(a)
					break
				}
			}
			if !offered {
				t.Fatalf("the spell is not offered from the stored mana: options %v", d.Options[:d.N])
			}
			for i := 0; i < 8 && sc.s.objs[card].zone == kop.ZoneHand; i++ {
				d = sc.s.Step()
				if d.Kind == DkPriority || d.N == 0 {
					break
				}
				sc.s.Apply(d.Options[0])
			}
			if z := sc.s.objs[card].zone; z == kop.ZoneHand {
				t.Fatalf("the cast spell is still in hand")
			}
			if got := sc.s.perms[sl].ctrN(sc.s, int(kop.CtrStorage)); got != tc.wantCtr {
				t.Fatalf("the payment left %d storage counters, want %d", got, tc.wantCtr)
			}
		})
	}
}

// TestStoreManaAbRejectsExtraProduction: the store predicate must not honour a
// production whose rider, deferred trigger or non-spell payment the kernel's
// storeGap cannot run. kgen's storeGapMana already refuses those shapes; this
// keeps the kernel's predicate (which kgen calls through ManaAbilityHonoured)
// from accepting one and silently dropping it.
func TestStoreManaAbRejectsExtraProduction(t *testing.T) {
	base := func() kop.Ability {
		return kop.Ability{
			Kind:       kop.AbMana,
			ManaColors: kop.ColB,
			AFlags:     kop.AfManaDynamic,
			ManaV:      kop.ValueRef{K: kop.VCountersSelf, N: int16(kop.CtrStorage)},
			Cost:       kop.Cost{Flags: kop.CostTap | kop.CostLoyaltyX, Ctr: kop.CtrStorage},
		}
	}
	for _, tc := range []struct {
		name string
		mut  func(*kop.Ability)
		want bool
	}{
		{"base store", func(*kop.Ability) {}, true},
		{"life rider", func(a *kop.Ability) { a.ManaRider, a.ManaRiderN = kop.ManaRiderOppGainLife, 1 }, false},
		{"deplete rider", func(a *kop.Ability) { a.ManaRider, a.ManaRiderCtr = kop.ManaRiderDeplete, kop.CtrStorage }, false},
		{"deferred production", func(a *kop.Ability) { a.ManaDeferred = true }, false},
		{"non-spell payment", func(a *kop.Ability) { a.AFlags |= kop.AfManaNonSpell }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := base()
			tc.mut(&a)
			if got := kop.StoreManaAb(&a); got != tc.want {
				t.Fatalf("StoreManaAb = %v, want %v", got, tc.want)
			}
		})
	}
}
