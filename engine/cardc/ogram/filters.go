package ogram

import (
	"errors"
	"sort"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// FilterStats is what SliceFilters found on the uniquely read faces of a
// slice.
type FilterStats struct {
	Names, Faces, Operands, Lowered int
	// Refused counts the operands that did not lower, by RefusedError.Key
	// ("error" for any other failure).
	Refused map[string]int
}

// RefusedRow is one Refused key and its count.
type RefusedRow struct {
	Key string
	N   int
}

// TopRefused lists the refused keys by count, most first, ties by key.
func (s FilterStats) TopRefused() []RefusedRow {
	out := make([]RefusedRow, 0, len(s.Refused))
	for k, n := range s.Refused {
		out = append(out, RefusedRow{k, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// SliceFilters walks the uniquely read faces in dir whose face or card name
// is in names, finds their filter operands and lowers each. visit, when not
// nil, sees every operand with its lowering (f nil and err set when it does
// not lower).
func SliceFilters(dir string, names []string, visit func(face string, op Operand, f *ir.Filter, err error) error) (FilterStats, error) {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	st := FilterStats{Names: len(want), Refused: map[string]int{}}
	schema, err := LoadSchema(dir)
	if err != nil {
		return st, err
	}
	err = ReadFaces(dir, func(face *Face) error {
		if !want[face.Name] && !want[face.Card] {
			return nil
		}
		st.Faces++
		ops, err := schema.FilterOperands(face.Tree)
		if err != nil {
			return err
		}
		for _, op := range ops {
			st.Operands++
			f, lerr := op.Lower()
			var re *RefusedError
			switch {
			case lerr == nil:
				st.Lowered++
			case errors.As(lerr, &re):
				st.Refused[re.Key]++
			default:
				st.Refused["error"]++
			}
			if visit != nil {
				if err := visit(face.Name, op, f, lerr); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return st, err
}
