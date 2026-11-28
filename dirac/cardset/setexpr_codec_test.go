package cardset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSetExprCodecRoundTrip: a table carrying set expressions and an
// ability's SetRefs round-trips, and the sets reach the bytes.
func TestSetExprCodecRoundTrip(t *testing.T) {
	want := sampleData()
	want.Sets = []kop.SetExpr{
		{{Kind: kop.SetTargets, Arg: 3}, {Kind: kop.SetAndFilter, Arg: 2}},
		{{Kind: kop.SetController}, {Kind: kop.SetPlayersComplement}},
		{}, // an empty program is kept apart from nil
	}
	want.Defs[1].Abilities = []kop.Ability{{
		Prog:    []kop.Instr{{Op: kop.OpDestroy, Obj: kop.RefSet}},
		SetRefs: []kop.SetRef{{PC: 0, Set: 0}, {PC: 7, Set: 513}},
	}}
	b := EncodeTable(want, "somekey")
	if bytes.Equal(b, EncodeTable(sampleData(), "somekey")) {
		t.Fatal("the sets did not reach the encoded bytes")
	}
	got, _, err := DecodeTable(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("round trip differs:\nwant %+v\ngot  %+v", want.Sets, got.Sets)
	}
	if got.Sets[2] == nil || got.Defs[1].Abilities[0].SetRefs[1].Set != 513 {
		t.Error("an empty program or a wide set index did not survive")
	}
}

// sampleDataBaseSum is sha256 of sampleData's payload under the merge base's
// codec, before Sets and SetRefs existed (computed with that tree's
// EncodeTable/TableSum over this file's sampleData).
const sampleDataBaseSum = "8edbc7d107c3f9c1ce656d17c67baf1149ef694228c5088d86a1e575c17bdf41"

// TestTablesUnchangedBySetExpr: a table without sets encodes to the bytes it
// did before set expressions existed. The new fields are their structs' last,
// so every other field keeps its codec index; an empty value encodes to
// nothing (the fixture's digest is the merge base's); and every pool table this
// binary publishes re-encodes to its manifest digest, with every RefSet operand
// backed by one SetRefs entry naming a set the table holds (kgen lowers
// Defined$ compositions onto sets, so a pool may now carry them).
func TestTablesUnchangedBySetExpr(t *testing.T) {
	for _, c := range []struct {
		typ   reflect.Type
		field string
	}{
		{reflect.TypeFor[kop.Table](), "Sets"},
		{reflect.TypeFor[kop.Ability](), "SetRefs"},
		{reflect.TypeFor[PoolData](), "Sets"},
	} {
		if last := c.typ.Field(c.typ.NumField() - 1).Name; last != c.field {
			t.Errorf("%s's last field is %s, want %s: a field before it moves every later field's codec index", c.typ, last, c.field)
		}
	}
	sum, err := TableSum(EncodeTable(sampleData(), "somekey"))
	if err != nil {
		t.Fatal(err)
	}
	if sum != sampleDataBaseSum {
		t.Errorf("sampleData's payload digest is %s, the merge base's %s", sum, sampleDataBaseSum)
	}

	dir, err := TablesDir()
	if errors.Is(err, ErrNoCorpus) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 100 {
		t.Fatalf("precondition: %d pools in %s", len(rows), dir)
	}
	for _, r := range rows {
		pd, err := LoadFile(filepath.Join(dir, r.Pool+".ktab"))
		if err != nil {
			t.Fatal(err)
		}
		checkSetRefs(t, r.Pool, pd)
		payload := sha256.Sum256(encodeValue(nil, reflect.ValueOf(*pd)))
		if got := hex.EncodeToString(payload[:]); got != r.Sum {
			t.Errorf("%s: re-encodes to %s, manifest %s", r.Pool, got, r.Sum)
		}
	}
}

// checkSetRefs: each RefSet operand of pd has exactly one SetRefs entry at its
// pc, every entry names an in-range set at a RefSet pc, and a table with no
// RefSet operand carries no set.
func checkSetRefs(t *testing.T, pool string, pd *PoolData) {
	t.Helper()
	used := false
	for i := range pd.Defs {
		for j := range pd.Defs[i].Abilities {
			ab := &pd.Defs[i].Abilities[j]
			refs := map[int]int{}
			for _, sr := range ab.SetRefs {
				refs[int(sr.PC)]++
				if int(sr.Set) >= len(pd.Sets) {
					t.Errorf("%s: card %d ability %d pc %d names set %d of %d", pool, i, j, sr.PC, sr.Set, len(pd.Sets))
				}
				if int(sr.PC) >= len(ab.Prog) || ab.Prog[sr.PC].Obj != kop.RefSet {
					t.Errorf("%s: card %d ability %d has a SetRef at pc %d, which reads no RefSet", pool, i, j, sr.PC)
				}
			}
			for pc, in := range ab.Prog {
				if in.Obj != kop.RefSet {
					continue
				}
				used = true
				if refs[pc] != 1 {
					t.Errorf("%s: card %d ability %d pc %d reads RefSet with %d SetRefs entries", pool, i, j, pc, refs[pc])
				}
			}
		}
	}
	if !used && pd.Sets != nil {
		t.Errorf("%s: %d set expressions and no RefSet operand", pool, len(pd.Sets))
	}
}
