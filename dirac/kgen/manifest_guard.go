package kgen

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/adams-shaun/gorge"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/manifest"
)

// The manifest guard holds invariant 3 of dirac/manifest: no inactive code
// reaches a table. A built card that carries a code whose entry is reserved,
// blocked or deferred is refused, class manifest:<status>:<Family>/<Name>
// (manifest.Entry.RefusalClass), so a lowering that names a code before the
// kernel implements it never plays.
//
// The guard reads every typed field of the card (a kop family type: TrigEvent,
// StaticKind, ReplKind, CounterKind, Zone, Op, the flag words, ...), each OpExt
// instruction's sub-op, and what those fields reach through the pool tables:
// the expressions (VExpr), filters (FilterID, VCountFilter), conditions
// (CondID), emblems (OpCreateEmblem) and overrides (StatOverride). A code
// carried in an untyped operand (an int16 C, a ValueRef constant) is the
// owning op's own namespace and is not read.

// codeGuard is the inactive kop codes of the manifest this binary embeds.
type codeGuard struct {
	inactive map[reflect.Type]map[int64]*manifest.Entry // by the family's kop type
	bits     map[reflect.Type]bool                      // flag families
	ext      map[int64]*manifest.Entry                  // inactive ExtOps
	// rel says whether a type reached from a card can hold anything the guard
	// reads; filled once from the roots, read-only after.
	rel map[reflect.Type]bool
}

var (
	tValueRef = reflect.TypeOf(kop.ValueRef{})
	tFilterID = reflect.TypeOf(kop.FilterID(0))
	tCondID   = reflect.TypeOf(kop.CondID(0))
	tInstr    = reflect.TypeOf(kop.Instr{})
	tStatic   = reflect.TypeOf(kop.Static{})
	kopPath   = tInstr.PkgPath()
)

// embeddedGuard is the guard of this binary's manifest: like the pool lists,
// a build never reads the working tree.
var embeddedGuard = sync.OnceValues(func() (*codeGuard, error) {
	es, err := manifest.ReadEntries(gorge.TableSources())
	if err != nil {
		return nil, fmt.Errorf("manifest guard: %v", err)
	}
	return newCodeGuard(es), nil
})

func newCodeGuard(es []*manifest.Entry) *codeGuard {
	g := &codeGuard{inactive: map[reflect.Type]map[int64]*manifest.Entry{}, bits: map[reflect.Type]bool{}, ext: map[int64]*manifest.Entry{}, rel: map[reflect.Type]bool{}}
	byFam := map[string]map[int64]*manifest.Entry{}
	for _, e := range es {
		f := e.Fam()
		if f == nil || f.Pkg != "kop" || e.Status.Kind == manifest.Active {
			continue
		}
		if e.Family == "ExtOp" {
			g.ext[e.Value] = e
			continue
		}
		if byFam[e.Family] == nil {
			byFam[e.Family] = map[int64]*manifest.Entry{}
		}
		byFam[e.Family][e.Value] = e
	}
	// Every named kop type a root reaches whose family has inactive entries.
	seen := map[reflect.Type]bool{}
	var collect func(t reflect.Type)
	collect = func(t reflect.Type) {
		if seen[t] {
			return
		}
		seen[t] = true
		if f := manifest.FamilyNamed(t.Name()); t.PkgPath() == kopPath && f != nil && byFam[f.Name] != nil {
			g.inactive[t], g.bits[t] = byFam[f.Name], f.Bits
		}
		switch t.Kind() {
		case reflect.Struct:
			for i := 0; i < t.NumField(); i++ {
				collect(t.Field(i).Type)
			}
		case reflect.Array, reflect.Slice:
			collect(t.Elem())
		}
	}
	for _, r := range []any{kop.CardDef{}, kop.Expr{}, kop.Filter{}, kop.Cond{}, kop.Emblem{}, kop.Override{}} {
		collect(reflect.TypeOf(r))
	}
	for t := range seen {
		g.relevant(t, map[reflect.Type]bool{})
	}
	return g
}

// relevant fills g.rel[t].
func (g *codeGuard) relevant(t reflect.Type, busy map[reflect.Type]bool) bool {
	if r, ok := g.rel[t]; ok {
		return r
	}
	if busy[t] {
		return false
	}
	busy[t] = true
	r := g.inactive[t] != nil || t == tValueRef || t == tFilterID || t == tCondID || t == tInstr || t == tStatic
	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			r = g.relevant(t.Field(i).Type, busy) || r
		}
	case reflect.Array, reflect.Slice:
		r = g.relevant(t.Elem(), busy) || r
	}
	g.rel[t] = r
	return r
}

// guardScan reads one pool's built cards, each pool table record once.
type guardScan struct {
	g    *codeGuard
	pt   *poolTables
	memo map[[2]int]*manifest.Entry // {table, index} -> its first inactive code
	busy map[[2]int]bool
}

const (
	gtExpr = iota
	gtFilter
	gtCond
	gtEmblem
	gtOverride
)

// refuseInactiveCodes refuses each built card (bs[i] is all[i]'s build) that
// carries an inactive manifest code.
func refuseInactiveCodes(pt *poolTables, all []*lCard, bs []built) error {
	g, err := embeddedGuard()
	if err != nil {
		return err
	}
	g.refuse(pt, all, bs)
	return nil
}

func (g *codeGuard) refuse(pt *poolTables, all []*lCard, bs []built) {
	if len(g.inactive) == 0 && len(g.ext) == 0 {
		return
	}
	s := &guardScan{g: g, pt: pt, memo: map[[2]int]*manifest.Entry{}, busy: map[[2]int]bool{}}
	for i, c := range all {
		if len(c.Refusals) > 0 {
			continue
		}
		if e := s.walk(reflect.ValueOf(&bs[i].def).Elem()); e != nil {
			c.Refusals = append(c.Refusals, refusal{Card: c.Name, Node: "manifest", Reason: e.RefusalClass() + " (a table code of " + e.Path + ")"})
		}
	}
}

// table walks pool table record idx once, memoised; a cycle reads nothing.
func (s *guardScan) table(kind, idx int) *manifest.Entry {
	k := [2]int{kind, idx}
	if e, ok := s.memo[k]; ok {
		return e
	}
	if s.busy[k] {
		return nil
	}
	s.busy[k] = true
	var v reflect.Value
	switch {
	case kind == gtExpr && idx > 0 && idx < len(s.pt.exprs):
		v = reflect.ValueOf(&s.pt.exprs[idx]).Elem()
	case kind == gtFilter && idx > 0 && idx < len(s.pt.filters):
		v = reflect.ValueOf(&s.pt.filters[idx]).Elem()
	case kind == gtCond && idx > 0 && idx < len(s.pt.conds):
		v = reflect.ValueOf(&s.pt.conds[idx]).Elem()
	case kind == gtEmblem && idx >= 0 && idx < len(s.pt.emblems):
		v = reflect.ValueOf(&s.pt.emblems[idx]).Elem()
	case kind == gtOverride && idx > 0 && idx <= len(s.pt.overrides):
		v = reflect.ValueOf(&s.pt.overrides[idx-1]).Elem()
	}
	var e *manifest.Entry
	if v.IsValid() {
		e = s.walk(v)
	}
	s.memo[k] = e
	return e
}

// walk is the first inactive code v carries or reaches, or nil.
func (s *guardScan) walk(v reflect.Value) *manifest.Entry {
	t := v.Type()
	if !s.g.rel[t] {
		return nil
	}
	if in := s.g.inactive[t]; in != nil {
		var x int64
		if v.CanInt() {
			x = v.Int()
		} else {
			x = int64(v.Uint())
		}
		if s.g.bits[t] {
			for b := 0; b < 64 && x>>b != 0; b++ {
				if e := in[x&(1<<b)]; x&(1<<b) != 0 && e != nil {
					return e
				}
			}
		} else if e := in[x]; e != nil {
			return e
		}
	}
	switch t {
	case tValueRef:
		vr := kop.ValueRef{K: kop.VKind(v.Field(0).Uint()), N: int16(v.Field(1).Int())}
		switch vr.K {
		case kop.VExpr:
			return s.table(gtExpr, int(vr.N))
		case kop.VCountFilter:
			return s.table(gtFilter, int(vr.N))
		}
		return nil
	case tFilterID:
		return s.table(gtFilter, int(v.Uint()))
	case tCondID:
		return s.table(gtCond, int(v.Uint()))
	case tInstr:
		in := v.Addr().Interface().(*kop.Instr)
		if e := s.g.ext[int64(in.Ext())]; e != nil {
			return e
		}
		if in.Op == kop.OpCreateEmblem {
			if e := s.table(gtEmblem, int(in.C)); e != nil {
				return e
			}
		}
	case tStatic:
		if st := v.Addr().Interface().(*kop.Static); st.Kind == kop.StatOverride {
			if e := s.table(gtOverride, int(st.Override)); e != nil {
				return e
			}
		}
	}
	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if e := s.walk(v.Field(i)); e != nil {
				return e
			}
		}
	case reflect.Array, reflect.Slice:
		if !s.g.rel[t.Elem()] {
			return nil
		}
		for i := 0; i < v.Len(); i++ {
			if e := s.walk(v.Index(i)); e != nil {
				return e
			}
		}
	}
	return nil
}
