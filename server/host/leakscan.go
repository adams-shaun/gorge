package host

import (
	"reflect"
	"sort"
	"strings"
	"unsafe"

	"github.com/adams-shaun/gorge/engine/cards"
)

// leakScan walks everything reachable from a value -- exported and
// unexported fields, through pointers, interfaces, slices and maps -- and
// records every place it finds a secret card: a string that contains a
// secret card name, or a pointer to a secret card, one of its faces, or one
// of its faces' abilities, triggers, statics or replacements (the pointers a
// card-text cache keys on). Pointers into package cards are leaves: the card
// corpus links every card to its registry's catalog, so descending would
// walk the whole corpus, which is not game state.
//
// skip names struct fields the walk does not enter, by "Type.Field"; each
// one is a known residual of the leak audit, and the scan still
// counts how often it met each so a test can say what it did not look at.
type leakScan struct {
	names   []string
	ptrs    map[uintptr]string // secret card-structure address -> card name
	skip    map[string]bool
	skipped map[string]int
	seen    map[leakVisit]bool
	path    []string
	found   map[string][]string // secret name -> paths (first few)
}

type leakVisit struct {
	p uintptr
	t reflect.Type
	n int
}

func newLeakScan(secret []*cards.Card, skip ...string) *leakScan {
	s := &leakScan{ptrs: map[uintptr]string{}, skip: map[string]bool{}, skipped: map[string]int{}}
	seenName := map[string]bool{}
	for _, c := range secret {
		if c == nil || len(c.Faces) == 0 {
			continue
		}
		name := c.Faces[0].Name
		if !seenName[name] {
			seenName[name] = true
			s.names = append(s.names, name)
		}
		s.ptrs[uintptr(unsafe.Pointer(c))] = name
		for _, f := range c.Faces {
			if f == nil {
				continue
			}
			s.ptrs[uintptr(unsafe.Pointer(f))] = name
			for _, sa := range f.Abilities {
				s.ptrs[uintptr(unsafe.Pointer(sa))] = name
			}
			for i := range f.Triggers {
				s.ptrs[uintptr(unsafe.Pointer(&f.Triggers[i]))] = name
			}
			for i := range f.Statics {
				s.ptrs[uintptr(unsafe.Pointer(&f.Statics[i]))] = name
			}
			for i := range f.Repls {
				s.ptrs[uintptr(unsafe.Pointer(&f.Repls[i]))] = name
			}
		}
	}
	sort.Strings(s.names)
	for _, k := range skip {
		s.skip[k] = true
	}
	return s
}

// scan walks v under the root label and returns the secret names found,
// each with up to three paths.
func (s *leakScan) scan(label string, v any) map[string][]string {
	s.seen = map[leakVisit]bool{}
	s.found = map[string][]string{}
	s.path = append(s.path[:0], label)
	s.walk(reflect.ValueOf(v))
	return s.found
}

var cardsPkg = reflect.TypeOf(cards.Card{}).PkgPath()

func (s *leakScan) hit(name string) {
	if ps := s.found[name]; len(ps) < 3 {
		s.found[name] = append(ps, strings.Join(s.path, ""))
	}
}

func (s *leakScan) str(x string) {
	if x == "" {
		return
	}
	for _, n := range s.names {
		if strings.Contains(x, n) {
			s.hit(n)
		}
	}
}

func basicKind(k reflect.Kind) bool {
	switch k {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return true
	}
	return false
}

func (s *leakScan) walk(v reflect.Value) {
	if !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.String:
		s.str(v.String())
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		p := v.Pointer()
		if v.Type().Elem().PkgPath() == cardsPkg {
			if name, ok := s.ptrs[p]; ok {
				s.hit(name)
			}
			return
		}
		key := leakVisit{p, v.Type(), -1}
		if s.seen[key] {
			return
		}
		s.seen[key] = true
		s.path = append(s.path, "*")
		s.walk(v.Elem())
		s.path = s.path[:len(s.path)-1]
	case reflect.Interface:
		if !v.IsNil() {
			s.walk(v.Elem())
		}
	case reflect.Struct:
		t := v.Type()
		if t.PkgPath() == cardsPkg || t.PkgPath() == "sync" || t.PkgPath() == "sync/atomic" || t.PkgPath() == "weak" {
			return
		}
		for i := 0; i < v.NumField(); i++ {
			f := t.Field(i)
			if s.skip[t.Name()+"."+f.Name] {
				s.skipped[t.Name()+"."+f.Name]++
				continue
			}
			s.path = append(s.path, "."+f.Name)
			s.walk(v.Field(i))
			s.path = s.path[:len(s.path)-1]
		}
	case reflect.Slice:
		if v.IsNil() || v.Len() == 0 || basicKind(v.Type().Elem().Kind()) {
			return
		}
		key := leakVisit{v.Pointer(), v.Type(), v.Len()}
		if s.seen[key] {
			return
		}
		s.seen[key] = true
		s.elems(v)
	case reflect.Array:
		if v.Len() == 0 || basicKind(v.Type().Elem().Kind()) {
			return
		}
		s.elems(v)
	case reflect.Map:
		if v.IsNil() {
			return
		}
		key := leakVisit{v.Pointer(), v.Type(), -2}
		if s.seen[key] {
			return
		}
		s.seen[key] = true
		it := v.MapRange()
		s.path = append(s.path, "[k]")
		for it.Next() {
			s.walk(it.Key())
			s.walk(it.Value())
		}
		s.path = s.path[:len(s.path)-1]
	}
}

func (s *leakScan) elems(v reflect.Value) {
	s.path = append(s.path, "[]")
	for i := 0; i < v.Len(); i++ {
		s.walk(v.Index(i))
	}
	s.path = s.path[:len(s.path)-1]
}
