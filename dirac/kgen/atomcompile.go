package kgen

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// AtomCompiler lowers a bare filter spec or Count$ head through the same
// lowering and build steps a card's abilities take, into one private pair of
// kernel tables (Tables). The oracle's atom harness (dirac/oracle atomdiff)
// evaluates those literals beside gorge's matcher and count evaluator, so a
// word's kernel form is never written by hand. A spec the lowering refuses
// returns its refusal.
type AtomCompiler struct {
	lc *lowerCtx
	cb *cardBuilder
}

// NewAtomCompiler is an empty compiler: the tables hold only the zero filter
// and the unused first expression (FilterID 0 and Exprs[0]).
func NewAtomCompiler() *AtomCompiler { return NewAtomCompilerOver(&kop.Table{}) }

// NewAtomCompilerOver is a compiler whose tables start as a copy of t's
// filters and expressions and name t's subtypes, so what it lowers is indexed
// as a card of t's matchup would carry it (Historic's Saga is t's Saga), and
// every index the game's own filters, statics and CDAs hold still names what
// it did when the overlay replaces t's tables.
func NewAtomCompilerOver(t *kop.Table) *AtomCompiler {
	p := &pool{subs: map[string]bool{}} // no pool card: only the typed words and the corpus creature types are known
	pt := newPoolTables(&lowered{}, nil)
	pt.filters = append(pt.filters[:0], t.Filters...)
	pt.exprs = append(pt.exprs[:0], t.Exprs...)
	if len(pt.filters) == 0 {
		pt.filters = append(pt.filters, kop.Filter{})
	}
	if len(pt.exprs) == 0 {
		pt.exprs = append(pt.exprs, kop.Expr{})
	}
	if len(t.FilterProgs) > 0 {
		pt.progs = append([]kop.FilterProg(nil), t.FilterProgs...)
		pt.progKey = map[string]kop.FilterProgID{}
	}
	pt.subtypes = append([]string(nil), t.Subtypes...)
	for i, w := range pt.subtypes {
		pt.subIdx[w] = i
	}
	return &AtomCompiler{
		lc: &lowerCtx{p: p, known: p.knownSubtype, out: &lCard{}, face: &cards.Face{SVars: map[string]string{}}, mode: -1},
		cb: &cardBuilder{pt: pt, abMap: map[int]int{}},
	}
}

// Filter lowers the Valid spec as a filter read over zone (kop.ZoneBattlefield
// for a permanent; a card zone otherwise) and returns its chain's head.
func (a *AtomCompiler) Filter(spec string, zone kop.Zone) (kop.FilterID, error) {
	clear(a.cb.pt.missing)
	f, err := a.lc.filter(spec)
	if err != nil {
		return 0, err
	}
	id, err := a.cb.filter(f, zone)
	if err != nil {
		return 0, err
	}
	// The pool's Saga index is what Historic carries out of lowering; a
	// matchup renumbers it (cardset remapSubtypePreds), and a matchup with no
	// Saga keeps 0: artifact or legendary only. That sentinel is the matchup's
	// own form, a read the kernel answers, not a subtype the table lacks: the
	// Saga miss Historic's lowering recorded is dropped when the spec itself
	// names no Saga (Outlaw's missing types still skip, Missing).
	remapped := false
	for f := id; f != 0; f = a.cb.pt.filters[f].Or {
		if fl := &a.cb.pt.filters[f]; fl.Pred == kop.PredHistoric && fl.PredN > 0 {
			fl.PredN = 0
			if i, ok := a.cb.pt.subIdx["Saga"]; ok {
				fl.PredN = int16(i) + 1
			}
			remapped = true
		}
	}
	if remapped && !strings.Contains(spec, "Saga") {
		delete(a.cb.pt.missing, "Saga")
	}
	return id, nil
}

// Count lowers the "Count$..." head as an amount. A constant comes back as a
// VConst reference, a per-object count as VCountFilter (N a filter of Tables),
// any other computed one as a VExpr over Tables' exprs.
func (a *AtomCompiler) Count(head string) (kop.ValueRef, error) {
	clear(a.cb.pt.missing)
	v, err := a.lc.svarValue(head, 0)
	if err != nil {
		return kop.ValueRef{}, err
	}
	return a.cb.value(v)
}

// Tables is the filters and expressions lowered so far (the slices alias the
// compiler's own; a probe set copies nothing, so lower everything first).
func (a *AtomCompiler) Tables() ([]kop.Filter, []kop.Expr) {
	return a.cb.pt.filters, a.cb.pt.exprs
}

// Progs is the filter-program table the lowered filters index (kop.PredProg's
// PredN): the game's own programs, then those the atoms lowered. nil when no
// program exists, which keeps the game's table.
func (a *AtomCompiler) Progs() []kop.FilterProg {
	return a.cb.pt.progs
}

// Missing is the subtype names the last Filter or Count asked for that the
// table has none of, sorted. A matchup's table keeps only the subtypes some
// card or filter of its pool needs, so a lowering that names one (Outlaw's
// five types over a matchup with no Warlock) reads an index no object holds:
// the kernel cannot answer it over this table, and the caller should skip it.
func (a *AtomCompiler) Missing() []string {
	var out []string
	for w := range a.cb.pt.missing {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}
