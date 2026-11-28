package ir

import (
	"strings"
	"testing"
)

// TestValidateRef holds ValidateRef to its rules: kind in range, Of exactly
// where the kind uses it, a Union of two or more, Memory exactly on
// RefMemory, Role only on the event kinds (the replaced-target and new-card
// roles only on an event object), AllZones only on RefAll, and a valid Filter.
func TestValidateRef(t *testing.T) {
	self := func() *Ref { return &Ref{Kind: RefSelf} }
	you := func() *Ref { return &Ref{Kind: RefYou} }
	goodFilter := &Filter{Op: FPred, Pred: PAny, Subject: SubjObject}
	badFilter := &Filter{Op: FAll}
	good := []*Ref{
		self(), you(),
		{Kind: RefTarget, Slot: 1},
		{Kind: RefControllerOf, Of: &Ref{Kind: RefTarget}},
		{Kind: RefAttachedTo, Of: self()},
		{Kind: RefEventObject, Role: RoleAttacker, LKI: true},
		{Kind: RefEventPlayer, Role: RoleActivator},
		{Kind: RefEventPlayer},
		{Kind: RefMemory, Memory: MemDelayedRemembered, LKI: true},
		{Kind: RefMemory, Memory: MemChosenPlayer},
		{Kind: RefUnion, Union: []*Ref{self(), you()}},
		{Kind: RefAll, Filter: goodFilter},
		{Kind: RefAll, Filter: goodFilter, AllZones: true},
		{Kind: RefEventObject, Role: RoleReplacedTarget},
		{Kind: RefEventObject, Role: RoleNewCard, LKI: true},
		{Kind: RefTopOfLibrary, N: &Value{Kind: VConst, N: 1}},
		{Kind: RefTopOfLibrary, Of: you(), N: &Value{Kind: VConst, N: 1}},
		{Kind: RefChoose, Filter: goodFilter},
	}
	for _, r := range good {
		if err := ValidateRef(r); err != nil {
			t.Errorf("ValidateRef(%s) = %v, want nil", r, err)
		}
	}
	bad := []struct {
		r    *Ref
		want string
	}{
		{nil, "nil ref"},
		{&Ref{}, "no kind"},
		{&Ref{Kind: refKindCount}, "kind"},
		{&Ref{Kind: RefControllerOf}, "needs of"},
		{&Ref{Kind: RefSelf, Of: you()}, "takes no of"},
		{&Ref{Kind: RefUnion, Union: []*Ref{self()}}, "two or more"},
		{&Ref{Kind: RefSelf, Union: []*Ref{self(), you()}}, "takes no union"},
		{&Ref{Kind: RefMemory}, "needs a memory"},
		{&Ref{Kind: RefSelf, Memory: MemRemembered}, "takes no memory"},
		{&Ref{Kind: RefMemory, Memory: memoryCount}, "memory"},
		{&Ref{Kind: RefSelf, Role: RoleCard}, "takes no role"},
		{&Ref{Kind: RefEventObject, Role: eventRoleCount}, "role"},
		{&Ref{Kind: RefAll, Filter: badFilter}, "filter"},
		{&Ref{Kind: RefSelf, AllZones: true}, "takes no all-zones"},
		{&Ref{Kind: RefEventPlayer, Role: RoleReplacedTarget}, "takes no object role"},
		{&Ref{Kind: RefEventPlayer, Role: RoleNewCard}, "takes no object role"},
		{&Ref{Kind: RefControllerOf, Of: &Ref{}}, "no kind"},
		{&Ref{Kind: RefUnion, Union: []*Ref{self(), nil}}, "nil ref"},
	}
	for _, c := range bad {
		err := ValidateRef(c.r)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("ValidateRef(%v) = %v, want an error containing %q", c.r, err, c.want)
		}
	}
}

// TestRefRoleAndLKIPrintAndCompare: Role, LKI and AllZones are part of a
// Ref's meaning.
func TestRefRoleAndLKIPrintAndCompare(t *testing.T) {
	a := &Ref{Kind: RefEventObject, Role: RoleCard}
	b := &Ref{Kind: RefEventObject, Role: RoleCard, LKI: true}
	c := &Ref{Kind: RefEventObject, Role: RoleSource}
	if Equal(a, b) || Equal(a, c) || a.String() == b.String() || a.String() == c.String() {
		t.Errorf("Role/LKI ignored:\n%s\n%s\n%s", a, b, c)
	}
	if got, want := b.String(), "(ref event-object :role card :lki)"; got != want {
		t.Errorf("print %s, want %s", got, want)
	}
	d := &Ref{Kind: RefAll}
	e := &Ref{Kind: RefAll, AllZones: true}
	if Equal(d, e) {
		t.Errorf("AllZones ignored by Equal: %s", e)
	}
	if got, want := e.String(), "(ref all :all-zones)"; got != want {
		t.Errorf("print %s, want %s", got, want)
	}
}
