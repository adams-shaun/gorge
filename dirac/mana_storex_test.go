package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// storeProbe is the storagex fixture pool: Bottomless Vault's only black
// source is its storage ability, and Shadowborn Apostle costs {B}.
type storeProbe struct {
	tbl     *kop.Table
	storage CardID
	spell   CardID
}

func storeProbeMatchup(t *testing.T) storeProbe {
	t.Helper()
	set := cardsettest.Set(t, "storagex")
	deck := []cardset.Entry{{Name: "Bottomless Vault", Count: 39}, {Name: "Shadowborn Apostle", Count: 1}}
	tbl, ids, _, err := set.Matchup(deck, []cardset.Entry{{Name: "Plains", Count: 40}})
	if err != nil {
		t.Fatal(err)
	}
	var p storeProbe
	p.tbl = tbl
	for _, c := range ids {
		switch tbl.Cards[c].Name {
		case "Bottomless Vault":
			p.storage = c
		case "Shadowborn Apostle":
			p.spell = c
		}
	}
	if p.storage == NoCard || p.spell == NoCard {
		t.Fatalf("the probe cards are missing from the table")
	}
	return p
}

// TestStoreManaGap: Bottomless Vault's single-colour storage-battery mana
// ability is a counter gap: scriptCtr reaches it, the potential walk counts
// its live counters, and activating it removes every counter and makes that
// many mana. The lowering itself is dirac/kgen's storageland_test.go.
func TestStoreManaGap(t *testing.T) {
	p := storeProbeMatchup(t)
	sc := newScenTableLib(t, p.tbl, p.storage, Config{NoElide: true})
	o := sc.perm(p.storage, 0)
	sl := sc.slot(o)
	sc.s.perms[sl].flags &^= pfTapped // Bottomless Vault enters tapped
	sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrStorage), 3)
	if !sc.s.scriptCtr(sc.k, 0).has(sl) {
		t.Fatalf("scriptCtr does not reach the storage source")
	}
	var pot avail
	sc.s.availPotential(sc.k, 0, &pot)
	if pot.total != 3 {
		t.Fatalf("availPotential.total = %d, want 3", pot.total)
	}
	sc.s.ctrGap(sc.k, 0, sl, -1)
	if n := sc.s.perms[sl].ctrN(sc.s, int(kop.CtrStorage)); n != 0 {
		t.Fatalf("storage counters left %d, want 0 (the cost removes every counter)", n)
	}
	if got := sc.s.pl[0].pool[kop.B]; got != 3 {
		t.Fatalf("pool B = %d, want 3", got)
	}
	if sc.s.perms[sl].flags&pfTapped == 0 {
		t.Fatalf("the storage source is untapped after its {T} activation")
	}
}

// TestStorePaysSpell: Bottomless Vault is the only source of black mana and
// Shadowborn Apostle costs {B}, so the cast is offered and paid only by the
// storage ability. Removing the gap from prep/scriptCtr/ctrGap leaves the
// cast unoffered: this fails without the wiring.
func TestStorePaysSpell(t *testing.T) {
	p := storeProbeMatchup(t)
	sc := newScenTableLib(t, p.tbl, p.storage, Config{NoElide: true})
	o := sc.perm(p.storage, 0)
	sc.s.perms[sc.slot(o)].flags &^= pfTapped // Bottomless Vault enters tapped
	sc.s.perms[sc.slot(o)].addCtrN(sc.s, int(kop.CtrStorage), 1)
	card := sc.hand(p.spell, 0)
	d := sc.s.Step()
	found := false
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(card)) {
			found = true
			sc.s.Apply(a)
			break
		}
	}
	if !found {
		t.Fatalf("the {B} spell is not offered with a stored counter: options %v", d.Options[:d.N])
	}
	for i := 0; i < 8 && sc.s.objs[card].zone == kop.ZoneHand; i++ {
		d = sc.s.Step()
		if d.Kind == DkPriority || d.N == 0 {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if z := sc.s.objs[card].zone; z != kop.ZoneStack && z != kop.ZoneGraveyard {
		t.Fatalf("the cast spell is in zone %d, want the stack or the graveyard", z)
	}
	if n := sc.s.perms[sc.slot(o)].ctrN(sc.s, int(kop.CtrStorage)); n != 0 {
		t.Fatalf("the payment left %d storage counters, want 0", n)
	}
}
