package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// hostPowerTable appends a "source you control" filter and a 2/4 creature
// whose noncombat ReplDamagePlus to an opponent or an opponent's permanent
// adds its own power (ReplDmgPowHost) to vanilla.
func hostPowerTable() (*kop.Table, CardID) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Exprs = []kop.Expr{{}} // prepareGen only runs a table with generated side tables
	fid := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlYou})
	host := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Host Power", Types: kop.TCreature, Power: 2, Toughness: 4,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{
			Kind:   kop.ReplDamagePlus,
			C:      int16(kop.ReplDmgOppPlayer | kop.ReplDmgOppPerm | kop.ReplDmgNonCombat | kop.ReplDmgPowHost),
			Filter: fid}}}})
	return &tbl, host
}

// TestDamagePlusHostPower: noncombat damage a source the host's controller
// controls deals to an opponent or an opponent's permanent gains the host's
// current power (a pump counts); combat damage and damage to the
// controller's own permanents are unchanged; a negative-power host stalls.
func TestDamagePlusHostPower(t *testing.T) {
	tbl, host := hostPowerTable()
	sc := newScenTable(t, tbl, Config{NoElide: true})
	src := sc.perm(v.GrizzlyBears, 0)
	h := sc.perm(host, 0)
	own := sc.perm(v.GrizzlyBears, 0)
	opp := sc.perm(v.GrizzlyBears, 1)
	hs := sc.slot(h)
	if pw, _ := sc.s.ptGen(sc.k, hs); pw != 2 {
		t.Fatalf("host power %d, want 2", pw)
	}
	deal := func(dst uint8, sl Slot, combat bool) int {
		return sc.s.replPlus(sc.k, 0, sc.slot(src), v.GrizzlyBears, dst, sl, combat, 3)
	}
	for _, c := range []struct {
		name   string
		dst    uint8
		sl     Slot
		combat bool
		want   int
	}{
		{"noncombat to opponent", 1, NoSlot, false, 5},
		{"noncombat to opponent's permanent", 1, sc.slot(opp), false, 5},
		{"combat to opponent", 1, NoSlot, true, 3},
		{"noncombat to own permanent", 0, sc.slot(own), false, 3},
		{"noncombat to self", 0, NoSlot, false, 3},
	} {
		if got := deal(c.dst, c.sl, c.combat); got != c.want {
			t.Errorf("%s: %d damage, want %d", c.name, got, c.want)
		}
	}
	if !sc.s.eotAdd(sc.k, hs, 3, 0) {
		t.Fatal("pump failed")
	}
	if pw, _ := sc.s.ptGen(sc.k, hs); pw != 5 {
		t.Fatalf("pumped host power %d, want 5", pw)
	}
	if got := deal(1, NoSlot, false); got != 8 {
		t.Errorf("pumped host: %d damage, want 8 (3 + 5)", got)
	}
	if sc.s.Stalled() {
		t.Fatalf("stalled %s", StallName(sc.s.StallReason()))
	}
	if !sc.s.eotAdd(sc.k, hs, -8, 0) {
		t.Fatal("shrink failed")
	}
	if pw, _ := sc.s.ptGen(sc.k, hs); pw >= 0 {
		t.Fatalf("shrunk host power %d, want negative", pw)
	}
	deal(1, NoSlot, false)
	if got := sc.s.StallReason(); got != StallReplPlus {
		t.Fatalf("negative-power host: stall %q, want replplus", StallName(got))
	}
}
