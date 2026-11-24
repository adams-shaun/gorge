package gorgebe

import (
	"reflect"
	"slices"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() {
	registerStep(ir.VerbIf, printIf)
	registerStep(ir.VerbForEachPlayer, printForEachPlayer)
}

var refType = reflect.TypeOf(ir.Ref{})

// countIterPlayers counts the *ir.Ref of kind RefIterPlayer reachable from v.
func countIterPlayers(v reflect.Value) int {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return 0
		}
		if v.Type().Elem() == refType && v.Interface().(*ir.Ref).Kind == ir.RefIterPlayer {
			return 1 + countIterPlayers(v.Elem().FieldByName("Of")) +
				countIterPlayers(v.Elem().FieldByName("Filter")) + countIterPlayers(v.Elem().FieldByName("N")) +
				countIterPlayers(v.Elem().FieldByName("Union"))
		}
		return countIterPlayers(v.Elem())
	case reflect.Struct:
		n := 0
		for i := 0; i < v.NumField(); i++ {
			n += countIterPlayers(v.Field(i))
		}
		return n
	case reflect.Slice:
		n := 0
		for i := 0; i < v.Len(); i++ {
			n += countIterPlayers(v.Index(i))
		}
		return n
	}
	return 0
}

func printForEachPlayer(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	bad := unprintable("verb:for-each-player", st.Src)
	if st.Optional || st.Obj == nil || st.Obj.Kind != ir.RefAll || st.Obj.Filter == nil ||
		st.Obj.Filter.Subject != ir.SubjPlayer || st.Then == nil || len(st.Then.Steps) != 1 {
		return nil, 0, bad
	}
	inner := st.Then.Steps[0]
	if inner == nil || inner.Who == nil || inner.Who.Kind != ir.RefIterPlayer ||
		countIterPlayers(reflect.ValueOf(inner)) != 1 {
		return nil, 0, bad
	}
	f := stepFns[inner.Verb]
	if f == nil {
		return nil, 0, unprintable("verb:"+inner.Verb.String(), inner.Src)
	}
	cp := *inner
	cp.Who = st.Obj
	n, used, err := f(p, []*ir.Step{&cp})
	if err != nil {
		return nil, 0, err
	}
	if used != 1 {
		return nil, 0, bad
	}
	return n, 1, nil
}

func declaredCopy(p *pr) []bool {
	if f := p.frame(); f != nil {
		return slices.Clone(f.declared)
	}
	return nil
}

func setDeclared(p *pr, d []bool) {
	if f := p.frame(); f != nil {
		copy(f.declared, d)
	}
}

// sameParams reports whether a and b are the same API with the same parameter
// keys; diff lists the keys whose values differ.
func sameParams(a, b *node) (diff []string, ok bool) {
	if a.API != b.API || len(a.Params) != len(b.Params) || a.Sub != nil || b.Sub != nil {
		return nil, false
	}
	for i := range a.Params {
		if a.Params[i].K != b.Params[i].K {
			return nil, false
		}
		if a.Params[i].V != b.Params[i].V {
			diff = append(diff, a.Params[i].K)
		}
	}
	return diff, true
}

func printIf(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if st.Optional {
		return nil, 0, unprintable("verb:if", st.Src)
	}
	c, err := p.cond(st.Cond)
	if err != nil {
		return nil, 0, err
	}
	saved := declaredCopy(p)
	then, err := p.program(st.Then)
	if err != nil {
		return nil, 0, err
	}
	if st.Else == nil {
		if err := gate(then, p.plain(c)); err != nil {
			return nil, 0, err
		}
		return then, 1, nil
	}
	afterThen := declaredCopy(p)
	nsv := len(p.svars)
	setDeclared(p, saved)
	els, err := p.program(st.Else)
	if err != nil {
		return nil, 0, err
	}
	// Amount merge: one verb, one differing amount, a count to choose it by.
	if (c.SVar != "" || c.Body != "") && st.Then != nil && len(st.Then.Steps) == 1 && len(st.Else.Steps) == 1 &&
		st.Then.Steps[0].Verb == st.Else.Steps[0].Verb {
		if p.mergeBranches(c, then, els) {
			setDeclared(p, afterThen)
			return then, 1, nil
		}
	}
	// Two conditioned chains. Print the else branch again from the declared
	// state before the if, so a slot only it names is declared there.
	afterThenState := afterThen
	p.svars = p.svars[:nsv]
	setDeclared(p, saved)
	els, err = p.program(st.Else)
	if err != nil {
		return nil, 0, err
	}
	afterElse := declaredCopy(p)
	for i := range saved {
		if !saved[i] && afterThenState[i] && afterElse[i] {
			return nil, 0, unprintable("if:shared-target", st.Src)
		}
	}
	for i := range afterElse {
		afterElse[i] = afterElse[i] || afterThenState[i]
	}
	setDeclared(p, afterElse)
	c = p.plain(c)
	if err := gate(then, c); err != nil {
		return nil, 0, err
	}
	nc, err := c.not()
	if err != nil {
		return nil, 0, err
	}
	if err := gate(els, nc); err != nil {
		return nil, 0, err
	}
	then.last().Sub = els
	return then, 1, nil
}

// gate adds c's ability conditions to every node of n's chain.
func gate(n *node, c condForm) error {
	ps, err := c.onAbility()
	if err != nil {
		return err
	}
	for ; n != nil; n = n.Sub {
		for _, x := range ps {
			n.set(x.K, x.V)
		}
	}
	return nil
}
