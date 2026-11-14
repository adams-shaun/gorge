package dirac

import (
	"reflect"
	"slices"
	"testing"
)

func TestForkPool(t *testing.T) {
	a := NewConfig(testTable(), nil, nil, 1, Config{})
	outer := a.fork()
	inner := outer.fork()
	if outer == a || inner == outer || inner.pool != a.pool {
		t.Fatal("nested forks must be distinct States on one pool")
	}
	inner.turn = a.turn + 1
	outer.unfork()
	if again := outer.fork(); again != inner || again.turn != a.turn {
		t.Fatal("a released depth must be reused, as a fresh copy")
	}
	outer.unfork()
	a.unfork()

	var b State
	b.CopyFrom(a)
	if b.pool != nil {
		t.Fatal("CopyFrom shared the source's fork pool")
	}
	if allocs := testing.AllocsPerRun(10, func() {
		a.fork().fork()
		a.unfork()
		a.unfork()
	}); allocs != 0 {
		t.Fatalf("a warm fork allocates %.0f times", allocs)
	}
}

// TestCopyFromMatchesEverySlot plays games on one State and copies them to
// another that has seen later boards, so a copy can only be exact if nothing
// is written above permHi and CopyFrom clears what dst held above src's.
func TestCopyFromMatchesEverySlot(t *testing.T) {
	var src, dst State
	for g, game := range fdnBenchTables(t, 8) {
		src.Init(game.tbl, game.da, game.db, uint64(g)+1, Config{})
		pol := NewRandomPolicy(uint64(g) + 1)
		var d Decision
		for {
			src.StepInto(&d)
			if d.Kind == DkNone {
				break
			}
			src.Apply(pol.Pick(&d))
			for _, p := range src.perms[src.permHi:] {
				if p != (perm{}) {
					t.Fatalf("game %d turn %d: a slot above permHi %d is not zero", g, src.turn, src.permHi)
				}
			}
			dst.CopyFrom(&src)
			if !dst.SameGame(&src) {
				t.Fatalf("game %d turn %d: the copy differs from its source", g, src.turn)
			}
		}
	}
}

// TestCopyFromRebasesColdArrays: a copy has its own arrays, laid out as its
// source's whichever layout it held before, and reuses a large enough buffer.
func TestCopyFromRebasesColdArrays(t *testing.T) {
	var small, large coldLayout
	coldMS.set(&small, 1)
	coldMS.set(&large, 2)
	coldExw.set(&large, maxExw)
	small.size()
	large.size()
	var src, dst State
	src.layOut(&large)
	src.cold.ms[1].set(3)
	dst.CopyFrom(&src)
	if !dst.cold.ms[1].has(3) || len(dst.cold.exw) != maxExw || &dst.cold.buf[0] == &src.cold.buf[0] {
		t.Fatal("the copy does not hold its own arrays")
	}
	dst.cold.ms[1].clr(3)
	if !src.cold.ms[1].has(3) {
		t.Fatal("a write to the copy reached its source")
	}
	var other State
	other.layOut(&small)
	if allocs := testing.AllocsPerRun(10, func() {
		dst.CopyFrom(&other)
		dst.CopyFrom(&src)
	}); allocs != 0 {
		t.Fatalf("copying between layouts allocates %.0f times", allocs)
	}
	dst.CopyFrom(&other)
	if len(dst.cold.ms) != 1 || len(dst.cold.exw) != 0 || !slices.Equal(dst.cold.buf, other.cold.buf) {
		t.Fatal("a copy kept its old layout")
	}
}

// TestColdArraysHoldNoPointers: the garbage collector does not scan the buffer,
// so a pointer in an element would not keep its target alive.
func TestColdArraysHoldNoPointers(t *testing.T) {
	var scalar func(reflect.Type) bool
	scalar = func(e reflect.Type) bool {
		switch e.Kind() {
		case reflect.Array:
			return scalar(e.Elem())
		case reflect.Struct:
			for i := range e.NumField() {
				if !scalar(e.Field(i).Type) {
					return false
				}
			}
			return true
		}
		return e.Kind() <= reflect.Complex128 && e.Kind() != reflect.Invalid && e.Kind() != reflect.Uintptr
	}
	c := reflect.TypeOf(coldArrays{})
	for i := range c.NumField() {
		if f := c.Field(i); f.Type.Kind() == reflect.Slice && !scalar(f.Type.Elem()) {
			t.Errorf("%s: %s holds a pointer", f.Name, f.Type.Elem())
		}
	}
	coldRegs.seal()
	for _, d := range coldRegs.descs {
		if e := reflect.TypeOf(d.zero); !scalar(e) {
			t.Errorf("cold region %s: %s holds a pointer", d.key, e)
		}
	}
}

// TestSharedForkPool: forks taken on two States sharing a warm pool are distinct
// copies of their own State, and allocate nothing.
func TestSharedForkPool(t *testing.T) {
	p := NewForkPool()
	var a, b State
	a.UseForkPool(p)
	b.UseForkPool(p)
	a.turn, b.turn = 1, 2
	if allocs := testing.AllocsPerRun(10, func() {
		fa := a.fork()
		fb := b.fork()
		if fa == fb || fa.turn != 1 || fb.turn != 2 {
			t.Fatal("forks on a shared pool must be distinct copies")
		}
		b.unfork()
		a.unfork()
	}); allocs != 0 {
		t.Fatalf("a warm shared pool allocates %.0f times", allocs)
	}
}

var cloneDst State

// BenchmarkCloneFDN copies a mid-game FDN State.
func BenchmarkCloneFDN(b *testing.B) {
	g := fdnBenchTables(b, 1)[0]
	var s State
	s.Init(g.tbl, g.da, g.db, 1, Config{})
	pol := NewRandomPolicy(1)
	var d Decision
	for s.turn < 12 {
		s.StepInto(&d)
		if d.Kind == DkNone {
			b.Fatal("the game ended before mid-game")
		}
		s.Apply(pol.Pick(&d))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloneDst.CopyFrom(&s)
	}
}

// Nothing writes a battlefield slot at or above permHi: CopyFrom would lose it.
func TestPermHiBoundsEveryWrite(t *testing.T) {
	var zero perm
	for _, g := range fdnBenchTables(t, 40) {
		var s State
		s.Init(g.tbl, g.da, g.db, 1, Config{})
		pol := NewRandomPolicy(1)
		var d Decision
		for n := 0; n < 4000 && !s.over; n++ {
			s.StepInto(&d)
			if d.Kind == DkNone {
				break
			}
			s.Apply(pol.Pick(&d))
			for sl := int(s.permHi); sl < len(s.perms); sl++ {
				if s.perms[sl] != zero {
					t.Fatalf("after %d steps slot %d is written, permHi %d", n, sl, s.permHi)
				}
			}
		}
		if s.permHi == 0 {
			t.Fatal("a game with no permanents proves nothing")
		}
	}
}
