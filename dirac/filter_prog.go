package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Filter programs (kop.PredProg): a filter whose predicate is a conjunction of
// literals, each a kop.FilterWord with its own handler (dirac/filter_<name>.go,
// "//kmanifest:fword FwX"; gen writes fwRun and fwHandled, manifest_gen.go). A matcher
// applies the filter's struct fields first and, for PredProg, evaluates the
// program with an fctx built from what the matcher has.

// fctx is the object a filter program reads and the context it reads it in.
// A matcher fills what it has: a field it has not is its zero value, and a
// word that needs one returns false (fail closed, gorge's unbound context).
// zone is the zone the candidate is read in: ZoneBattlefield with sl set is a
// permanent, and ZoneNone is a candidate whose zone is not known; a word reads
// a slot only when perm() holds, since slot 0 is a real slot.
type fctx struct {
	k      *prep
	zone   kop.Zone
	o      Obj // the candidate's object, NoObj when it has none
	sl     Slot
	c      CardID
	owner  uint8 // the candidate's owner (a card matcher's)
	ctrl   uint8 // the reading source's controller
	src    Slot  // the reading source's slot, NoSlot when it has none
	srcObj Obj
	layer  uint8
	fd     bool // the candidate is face down (CR 708.2)
	// walk marks a layer-walk view, where a word may not re-enter the walk
	// and reads cur. cur and ctx are copies, never pointers into the
	// caller's locals: escape analysis does not tell fctx's fields apart,
	// so a pointer here would move every caller's local to the heap
	// (the k that evalV leaks taints the whole struct).
	walk   bool
	cur    dchars
	hasCtx bool // ctx is the matcher's evaluation context; unset reads ctxCur
	ctx    evalCtx
}

// perm reports whether the candidate is a battlefield permanent at slot sl.
func (c *fctx) perm() bool { return c.zone == kop.ZoneBattlefield && c.sl != NoSlot }

// card reports whether the candidate is a card read off the battlefield: its
// printed characteristics, or the stack's.
func (c *fctx) card() bool { return c.sl == NoSlot && !c.walk && c.c != NoCard }

// filterProg evaluates program id's literals in order against c and stops at
// the first that fails (a literal's Neg inverts its word).
func (s *State) filterProg(id kop.FilterProgID, c *fctx) bool {
	lits := c.k.t.FilterProgs[id].Lits
	for i := range lits {
		l := &lits[i]
		if s.fwRun(c, l) == l.Neg {
			return false
		}
	}
	return true
}

// useFilterProg is the table-prep flag for a table with a PredProg filter.
// Holding it builds the table's genPrep (every program reads the generated
// forms, and the non-generated matchers have no arm for PredProg) and runs
// checkFilterProgs, so a bad program is refused at prep, never mid-game.
var useFilterProg = newGenUse(tableHasFilterProg)

// tableHasFilterProg reports whether a filter of t is a program, and refuses
// a program an evaluation would trip over: an index outside
// Table.FilterProgs, or a literal whose word has no handler.
func tableHasFilterProg(t *kop.Table) bool {
	any := false
	for i := range t.Filters {
		if t.Filters[i].Pred == kop.PredProg {
			any = true
			checkFilterProg(t, t.Filters[i].PredN)
		}
	}
	return any
}

func checkFilterProg(t *kop.Table, id kop.FilterProgID) {
	if id <= 0 || int(id) >= len(t.FilterProgs) {
		panic("kernel: filter program index outside Table.FilterProgs")
	}
	for _, l := range t.FilterProgs[id].Lits {
		if !fwHandled(l.W) {
			panic("kernel: filter word has no handler")
		}
		if l.F != 0 && int(l.F) >= len(t.Filters) {
			panic("kernel: filter program's nested filter outside Table.Filters")
		}
	}
}

// permFctx is the fctx of the battlefield permanent at sl read for a source
// controlled by ctrl at slot src (matchPermOne, matchCurOne).
func (s *State) permFctx(k *prep, sl Slot, ctrl uint8, src Slot, layer uint8) fctx {
	p := &s.perms[sl]
	return fctx{k: k, zone: kop.ZoneBattlefield, o: p.obj, sl: sl, c: s.cardAt(sl), owner: s.objs[p.obj].owner(), ctrl: ctrl, src: src, srcObj: NoObj, layer: layer}
}

// cmpV is the literal's comparison of have against its operand: V read in the
// matcher's evaluation context (a constant needs none).
func (s *State) cmpV(c *fctx, l *kop.FLit, have int) bool {
	if l.V.K == kop.VConst {
		return cmp(l.Op, have, int(l.V.N))
	}
	if c.hasCtx {
		return cmp(l.Op, have, s.evalV(c.k, l.V, c.ctx))
	}
	return cmp(l.Op, have, s.evalV(c.k, l.V, s.ctxCur()))
}
