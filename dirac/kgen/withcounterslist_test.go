package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWithCountersTypeList: a ChangeZone to the battlefield whose
// WithCountersType$ lists two kinds lowers to one OpReturnEx that carries both
// (first kind in B, second kind + 1 in the count's high byte), in script
// order. Perennation is the corpus card. A list on any other move, a third
// kind, an unknown kind and a ChangeZoneAll sweep refuse by name.
func TestWithCountersTypeList(t *testing.T) {
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Perennation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	all, bs, _, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i, c := range all {
		if c.Name != "Perennation" {
			continue
		}
		if r := lowerRefusals(c); len(r) > 0 {
			t.Fatalf("Perennation refused: %v", r)
		}
		for _, a := range bs[i].def.Abilities {
			for _, in := range a.Prog {
				if in.Op != kop.OpReturnEx {
					continue
				}
				found = true
				if kop.CounterKind(in.B.N) != kop.CtrHexproof {
					t.Errorf("first kind %d, want Hexproof %d", in.B.N, kop.CtrHexproof)
				}
				w := int(uint16(in.A.N))
				if got, want := w&(1<<kop.RetCounterPairShift-1), 1; got != want {
					t.Errorf("count %d, want %d", got, want)
				}
				if got, want := w>>kop.RetCounterPairShift, int(kop.CtrIndestructible)+1; got != want {
					t.Errorf("second kind+1 %d, want Indestructible+1 %d", got, want)
				}
			}
		}
	}
	if !found {
		t.Fatal("Perennation lowered no OpReturnEx")
	}

	const ret = "SP$ ChangeZone | Cost$ B | Origin$ Graveyard | Destination$ Battlefield | ValidTgts$ Permanent.YouOwn"
	for _, c := range []struct{ name, root, want string }{
		{"three kinds", ret + " | WithCountersType$ Hexproof,Indestructible,Flying", "only a pair of kinds"},
		{"unknown second kind", ret + " | WithCountersType$ Hexproof,NOSUCHKIND", `"NOSUCHKIND"`},
		{"to exile", "SP$ ChangeZone | Cost$ B | Origin$ Graveyard | Destination$ Exile | ValidTgts$ Card.YouOwn | WithCountersType$ Hexproof,Flying", "battlefield entry only"},
		{"ChangeZoneAll", "SP$ ChangeZoneAll | Cost$ B | Origin$ Graveyard | Destination$ Battlefield | ChangeType$ Creature | WithCountersType$ Hexproof,Flying", "a list of counter kinds"},
	} {
		t.Run("refuses "+c.name, func(t *testing.T) {
			got, _ := czLowered(t, c.root)
			kdefWant(t, got, c.want)
		})
	}
	t.Run("one kind still lowers", func(t *testing.T) {
		got, ops := czLowered(t, ret+" | WithCountersType$ Hexproof")
		if len(got) != 0 || !strings.Contains(strings.Join(ops, " "), "counters=Hexproof") {
			t.Errorf("refusals %q ops %q", got, ops)
		}
	})
}
