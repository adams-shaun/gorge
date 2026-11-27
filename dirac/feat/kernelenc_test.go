package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/encoder"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestKernelEncoderAgreesWithViewRegistry: every kernel encoder has a view
// encoder of the same name whose Schema is field for field the same and whose
// Hash is the digest a net header carries; and a warm kernel encoder encodes
// a state and its options without allocating.
func TestKernelEncoderAgreesWithViewRegistry(t *testing.T) {
	kernel := ListKernel()
	if len(kernel) < 2 {
		t.Fatalf("kernel registry holds %d encoders, want entity and entity-pod", len(kernel))
	}
	prev := ""
	for _, sp := range kernel {
		if sp.Schema.Name <= prev {
			t.Errorf("ListKernel not sorted: %q after %q", sp.Schema.Name, prev)
		}
		prev = sp.Schema.Name
		ve, ok := encoder.Lookup(sp.Schema.Name)
		if !ok {
			t.Errorf("kernel encoder %q has no view encoder", sp.Schema.Name)
			continue
		}
		vs, ks := ve.Schema(), sp.Schema
		if vs.Hash() != ks.Hash() {
			t.Errorf("%s: view Hash %#016x, kernel Hash %#016x", ks.Name, vs.Hash(), ks.Hash())
		}
		// Hash is Pinned for both, so compare every field the digest would
		// cover too: a drift must fail here, not at a net load.
		if vs.Version != ks.Version || vs.DenseW != ks.DenseW || vs.SparseRows != ks.SparseRows ||
			vs.RawW != ks.RawW || vs.OptDenseW != ks.OptDenseW || vs.ExtW != ks.ExtW || vs.Hidden != ks.Hidden {
			t.Errorf("%s: view schema %+v, kernel schema %+v", ks.Name, vs, ks)
		}
		if ks.Hidden {
			t.Errorf("%s: a kernel encoder must not be Hidden", ks.Name)
		}
		// The unpinned digest the two sides compute must agree as well.
		vs.Pinned, ks.Pinned = 0, 0
		if vs.Hash() != ks.Hash() {
			t.Errorf("%s: unpinned view Hash %#016x, kernel Hash %#016x", ks.Name, vs.Hash(), ks.Hash())
		}
	}
}

func TestKernelEncoderAllocFree(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	tbl, da, db, err := set.Matchup(
		[]cardset.Entry{{Name: "Mountain", Count: 20}, {Name: "Healer's Hawk", Count: 20}},
		[]cardset.Entry{{Name: "Plains", Count: 20}, {Name: "Healer's Hawk", Count: 20}},
	)
	if err != nil {
		t.Fatal(err)
	}
	pool := NewPool(tbl, make([]CardSpec, len(tbl.Cards)))
	for _, sp := range ListKernel() {
		enc := sp.New(pool)
		if enc.Schema() != sp.Schema {
			t.Errorf("%s: instance Schema differs from the registered one", sp.Schema.Name)
		}
		if r, ok := enc.(interface{ Reserve() }); ok {
			r.Reserve()
		}
		s := dirac.New(tbl, da, db, 1)
		var d dirac.Decision
		for n := 0; n < 40; n++ {
			d = s.Step()
			if d.Kind == dirac.DkNone || len(d.Options) == 0 {
				break
			}
			s.Apply(d.Options[0])
		}
		d = s.Step()
		if d.Kind == dirac.DkNone || len(d.Options) == 0 {
			t.Fatalf("%s: no decision to encode", sp.Schema.Name)
		}
		dst := make([]Option, 0, len(d.Options)+8)
		enc.EncodeState(s, &d) // warm
		dst = enc.EncodeOptions(&d, dst[:0])
		st := enc.EncodeState(s, &d)
		for i := sp.Schema.DenseW; i < len(st.Dense); i++ {
			if st.Dense[i] != 0 {
				t.Errorf("%s: dense[%d]=%v written past the schema width %d", sp.Schema.Name, i, st.Dense[i], sp.Schema.DenseW)
				break
			}
		}
		if got := testing.AllocsPerRun(20, func() {
			enc.EncodeState(s, &d)
			dst = enc.EncodeOptions(&d, dst[:0])
		}); got != 0 {
			t.Errorf("%s: %v allocs per encode, want 0", sp.Schema.Name, got)
		}
	}
}
