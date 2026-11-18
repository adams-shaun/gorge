package oracle

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// perturbLeaf changes v's first scalar leaf (an array's element 0, a
// struct's first field), so a field that reads it sees a different value.
func perturbLeaf(t *testing.T, v reflect.Value) {
	t.Helper()
	switch v.Kind() {
	case reflect.Struct:
		perturbLeaf(t, v.Field(0))
	case reflect.Array:
		perturbLeaf(t, v.Index(0))
	case reflect.Bool:
		v.SetBool(!v.Bool())
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		v.SetInt(v.Int() + 1)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		v.SetUint(v.Uint() + 1)
	default:
		t.Fatalf("perturbLeaf: unhandled kind %v", v.Kind())
	}
}

// resetProjection is a reset Projection with n bound cards, each distinct.
func resetProjection(n int) *Projection {
	p := new(Projection)
	p.reset()
	p.NInst = dirac.Obj(n)
	for i := range n {
		p.Cards[i].NameHash = uint32(i + 1)
		p.Cards[i].Zone = ZoneLibrary
	}
	return p
}

// TestProjectionResetIsWholeReset: reset leaves p as `*p = Projection{}` and
// a whole-array unbound fill did, from a zero Projection, a projected one,
// and a projected one with every field changed.
func TestProjectionResetIsWholeReset(t *testing.T) {
	var want Projection
	for i := range want.Cards {
		want.Cards[i] = unboundCard
	}
	var zero Projection
	zero.reset()
	if zero != want {
		t.Fatal("reset of a zero Projection is not the whole reset")
	}
	for _, n := range []int{0, 1, 40, MaxInst - 1, MaxInst} {
		p := resetProjection(n)
		p.reset()
		if *p != want {
			t.Fatalf("reset of a projection with %d cards is not the whole reset", n)
		}
	}
}

// TestProjectionResetClearsEveryField: every Projection field reset is given
// a non-zero value it clears (a field added to Projection and not to reset
// fails here).
func TestProjectionResetClearsEveryField(t *testing.T) {
	var want Projection
	want.reset()
	typ := reflect.TypeFor[Projection]()
	for i := range typ.NumField() {
		f := typ.Field(i)
		if f.Name == "Cards" {
			continue
		}
		p := resetProjection(3)
		perturbLeaf(t, reflect.ValueOf(p).Elem().Field(i))
		p.reset()
		if *p != want {
			t.Errorf("reset leaves Projection.%s set", f.Name)
		}
	}
}

// TestProjectionEqualReadsEveryField: equal agrees with == on projections
// with unbound tails, field by field (a field added to Projection and not to
// equal fails here), and on every bound card.
func TestProjectionEqualReadsEveryField(t *testing.T) {
	a := resetProjection(5)
	if b := resetProjection(5); !a.equal(b) || *a != *b {
		t.Fatal("equal projections compare unequal")
	}
	typ := reflect.TypeFor[Projection]()
	for i := range typ.NumField() {
		f := typ.Field(i)
		b := resetProjection(5)
		perturbLeaf(t, reflect.ValueOf(b).Elem().Field(i))
		if *a == *b {
			t.Fatalf("perturbing Projection.%s changes nothing", f.Name)
		}
		if a.equal(b) {
			t.Errorf("equal misses Projection.%s", f.Name)
		}
	}
	for c := range 5 {
		b := resetProjection(5)
		b.Cards[c].Counters[CtrP1P1]++
		if a.equal(b) {
			t.Errorf("equal misses Cards[%d]", c)
		}
	}
}

// BenchmarkProjectionCompare is the lockstep's per-decision projection work
// on a 60-card game: resetting both projections and comparing them. "prefix"
// is reset and equal, whose cost follows the bound cards; "whole" is the
// whole-struct reset, MaxInst fill and == they replace.
func BenchmarkProjectionCompare(b *testing.B) {
	b.Run("prefix", func(b *testing.B) {
		pr, pk := resetProjection(60), resetProjection(60)
		for b.Loop() {
			pr.reset()
			pk.reset()
			pr.NInst, pk.NInst = 60, 60
			if !pr.equal(pk) {
				b.Fatal("unequal")
			}
		}
	})
	b.Run("whole", func(b *testing.B) {
		pr, pk := resetProjection(60), resetProjection(60)
		for b.Loop() {
			for _, p := range [2]*Projection{pr, pk} {
				*p = Projection{}
				for i := range p.Cards {
					p.Cards[i] = unboundCard
				}
				p.NInst = 60
			}
			if *pr != *pk {
				b.Fatal("unequal")
			}
		}
	})
}
