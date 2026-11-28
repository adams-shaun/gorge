package forge

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func rk(k ir.RefKind) *ir.Ref { return &ir.Ref{Kind: k} }

func evObj(role ir.EventRole, lki bool) *ir.Ref {
	return &ir.Ref{Kind: ir.RefEventObject, Role: role, LKI: lki}
}

func evPl(role ir.EventRole) *ir.Ref { return &ir.Ref{Kind: ir.RefEventPlayer, Role: role} }

func memRef(m ir.Memory, lki bool) *ir.Ref { return &ir.Ref{Kind: ir.RefMemory, Memory: m, LKI: lki} }

func ctlOf(r *ir.Ref) *ir.Ref { return &ir.Ref{Kind: ir.RefControllerOf, Of: r} }

func ownOf(r *ir.Ref) *ir.Ref { return &ir.Ref{Kind: ir.RefOwnerOf, Of: r} }

func union(rs ...*ir.Ref) *ir.Ref { return &ir.Ref{Kind: ir.RefUnion, Union: rs} }

func allOf(t *testing.T, filter string) *ir.Ref {
	t.Helper()
	f, r := LowerFilter(filter)
	if r != nil {
		t.Fatalf("LowerFilter(%q): %v", filter, r)
	}
	return &ir.Ref{Kind: ir.RefAll, Filter: f}
}

// TestLowerRefTable lowers one written string per atom, projection and form.
func TestLowerRefTable(t *testing.T) {
	obj, pl := ir.SubjObject, ir.SubjPlayer
	attached := &ir.Ref{Kind: ir.RefAttachedTo, Of: rk(ir.RefSelf)}
	cases := []struct {
		text string
		want ir.Subject
		ref  *ir.Ref
	}{
		{"Self", obj, rk(ir.RefSelf)},
		{"You", pl, rk(ir.RefYou)},
		{"Targeted", obj, rk(ir.RefTarget)},
		{"Targeted", pl, rk(ir.RefTarget)},
		{"ParentTarget", obj, rk(ir.RefTarget)},
		{"TargetedPlayer", pl, rk(ir.RefTarget)},
		{"Enchanted", obj, attached},
		{"Equipped", obj, attached},
		{"TriggeredCard", obj, evObj(ir.RoleCard, false)},
		{"TriggeredCardLKICopy", obj, evObj(ir.RoleCard, true)},
		{"TriggeredNewCardLKICopy", obj, evObj(ir.RoleCard, true)},
		{"TriggeredSource", obj, evObj(ir.RoleSource, false)},
		{"TriggeredSourceLKICopy", obj, evObj(ir.RoleSource, true)},
		{"TriggeredAttacker", obj, evObj(ir.RoleAttacker, false)},
		{"TriggeredAttackerLKICopy", obj, evObj(ir.RoleAttacker, true)},
		{"TriggeredBlocker", obj, evObj(ir.RoleBlocker, false)},
		{"TriggeredBlockerLKICopy", obj, evObj(ir.RoleBlocker, true)},
		{"TriggeredTarget", obj, evObj(ir.RoleTarget, false)},
		{"TriggeredTarget", pl, evObj(ir.RoleTarget, false)},
		{"TriggeredTargetLKICopy", obj, evObj(ir.RoleTarget, true)},
		{"TriggeredPlayer", pl, evPl(ir.RoleNone)},
		{"TriggeredActivator", pl, evPl(ir.RoleActivator)},
		{"TriggeredDefendingPlayer", pl, rk(ir.RefDefendingPlayer)},
		{"ReplacedCard", obj, evObj(ir.RoleReplaced, false)},
		{"ReplacedTarget", obj, evObj(ir.RoleReplaced, false)},
		{"ReplacedPlayer", pl, evPl(ir.RoleReplaced)},
		{"Remembered", obj, memRef(ir.MemRemembered, false)},
		{"Remembered", pl, memRef(ir.MemRemembered, false)},
		{"Imprinted", obj, memRef(ir.MemImprinted, false)},
		{"ChosenCard", obj, memRef(ir.MemChosenCard, false)},
		{"ExiledWith", obj, memRef(ir.MemExiledWithSource, false)},
		{"ExiledWithSource", obj, memRef(ir.MemExiledWithSource, false)},
		{"RememberedLKI", obj, memRef(ir.MemRemembered, true)},
		{"DelayTriggerRemembered", obj, memRef(ir.MemDelayedRemembered, false)},
		{"DelayTriggerRememberedLKI", obj, memRef(ir.MemDelayedRemembered, true)},
		{"ChosenPlayer", pl, memRef(ir.MemChosenPlayer, false)},
		{"TopOfLibrary", obj, &ir.Ref{Kind: ir.RefTopOfLibrary, N: &ir.Value{Kind: ir.VConst, N: 1}}},
		{"ActivePlayer", pl, rk(ir.RefActivePlayer)},
		// projections
		{"TargetedController", pl, ctlOf(rk(ir.RefTarget))},
		{"TargetedOwner", pl, ownOf(rk(ir.RefTarget))},
		{"TriggeredCardController", pl, ctlOf(evObj(ir.RoleCard, false))},
		{"TriggeredCardOwner", pl, ownOf(evObj(ir.RoleCard, false))},
		{"RememberedController", pl, ctlOf(memRef(ir.MemRemembered, false))},
		{"EnchantedController", pl, ctlOf(attached)},
		{"TargetedAndYou", pl, union(rk(ir.RefTarget), rk(ir.RefYou))},
		// unions
		{"Self & Targeted", obj, union(rk(ir.RefSelf), rk(ir.RefTarget))},
		{"You & TriggeredPlayer & ChosenPlayer", pl,
			union(rk(ir.RefYou), evPl(ir.RoleNone), memRef(ir.MemChosenPlayer, false))},
		// filter forms
		{"Valid Creature.YouCtrl", obj, allOf(t, "Creature.YouCtrl")},
		{"ValidAll Card.token", obj, allOf(t, "Card.token")},
		{"Valid Player.Opponent", pl, allOf(t, "Player.Opponent")},
		{"Player", pl, allOf(t, "Player")},
		{"Opponent", pl, allOf(t, "Opponent")},
		{"Player.Opponent", pl, allOf(t, "Player.Opponent")},
		// "each other player": every player but you (no player is the source).
		{"Player.Other", pl, &ir.Ref{Kind: ir.RefAll, Filter: all(player, p(player, ir.PAny, "Player"),
			n(player, r(p(player, ir.PPlayerIs, "Other"), ref(ir.RefYou))))}},
		{"Player.!Other", pl, &ir.Ref{Kind: ir.RefAll, Filter: all(player, p(player, ir.PAny, "Player"),
			n(player, n(player, r(p(player, ir.PPlayerIs, "Other"), ref(ir.RefYou)))))}},
		{"Player.StrictlyOther", pl, &ir.Ref{Kind: ir.RefAll, Filter: all(player, p(player, ir.PAny, "Player"),
			n(player, r(p(player, ir.PPlayerIs, "StrictlyOther"), ref(ir.RefYou))))}},
		{"You & Valid Permanent.YouCtrl", obj, nil},
	}
	for _, c := range cases {
		got, r := LowerRef(c.text, c.want)
		if c.ref == nil {
			if r == nil || r.Kind != RefuseWrongSubject {
				t.Errorf("LowerRef(%q, %s) = %v, %v; want a wrong-subject refusal", c.text, c.want, got, r)
			}
			continue
		}
		if r != nil {
			t.Errorf("LowerRef(%q, %s) refused: %v", c.text, c.want, r)
			continue
		}
		if err := ir.ValidateRef(got); err != nil {
			t.Errorf("LowerRef(%q, %s) = %s does not validate: %v", c.text, c.want, got, err)
		}
		if !ir.Equal(got, c.ref) {
			t.Errorf("LowerRef(%q, %s)\n got %s\nwant %s", c.text, c.want, got, c.ref)
		}
		if got.Src == "" {
			t.Errorf("LowerRef(%q, %s): root has no Src", c.text, c.want)
		}
	}
	// ParentTarget keeps its spelling so a later pass can re-slot it.
	if got, _ := LowerRef("ParentTarget", obj); got == nil || got.Src != "ParentTarget" {
		t.Errorf("ParentTarget Src = %v", got)
	}
}

// TestLowerRefRefusals: every refusal kind the reader adds, plus the empty
// text and an inner filter refusal on a player form.
func TestLowerRefRefusals(t *testing.T) {
	obj, pl := ir.SubjObject, ir.SubjPlayer
	_, inner := LowerFilter("Creature.frobnicated")
	if inner == nil {
		t.Fatal("precondition: LowerFilter must refuse Creature.frobnicated")
	}
	if _, named := LowerFilter("Creature.controlsPermanent.namedFrob Widget"); named == nil ||
		named.Atom != "controlsPermanent.namedFrob Widget" {
		t.Fatalf("precondition: the inner refusal must carry the name, got %v", named)
	}
	cases := []struct {
		text string
		want ir.Subject
		kind RefusalKind
		atom string
	}{
		{"", obj, RefuseEmpty, ""},
		{"Self & ", obj, RefuseEmpty, ""},
		{"Self", pl, RefuseWrongSubject, "Self"},
		{"You", obj, RefuseWrongSubject, "You"},
		{"TargetedController", obj, RefuseWrongSubject, "TargetedController"},
		{"Valid Creature.YouCtrl", pl, RefuseWrongSubject, "Valid"},
		{"Player.Opponent", obj, RefuseWrongSubject, "Player"},
		{"Player.Self", pl, RefuseUnknownPredicate, "Self"},
		{"Player.StrictlySelf", pl, RefuseUnknownPredicate, "StrictlySelf"},
		{"Player.!Self", pl, RefuseUnknownPredicate, "!Self"},
		{"Valid Any.YouCtrl", obj, RefuseWrongSubject, "Valid"},
		{"Valid Any.YouCtrl", pl, RefuseWrongSubject, "Valid"},
		{"Valid Any", obj, RefuseWrongSubject, "Valid"},
		{"Valid Any", pl, RefuseWrongSubject, "Valid"},
		{"Valid Player,Creature", pl, RefuseWrongSubject, "Valid"},
		{"Player.Opponent,Creature", obj, RefuseWrongSubject, "Player"},
		{"RememberedLKI", pl, RefuseWrongSubject, "RememberedLKI"},
		{"Self", ir.SubjNone, RefuseWrongSubject, "none"},
		{"Frobnicator", obj, RefuseUnknownReferent, "Frobnicator"},
		{"Frobnicator2Controller", pl, RefuseUnknownReferent, "FrobnicatorNController"},
		{"Targeted.Creature", obj, RefuseUnknownReferent, "Targeted."},
		{"Frob Nicator", obj, RefuseUnknownReferent, "Frob"},
		{"Remembered.nonLand+sharesTypeWith Remembered", obj, RefuseUnknownReferent, "Remembered."},
		{"Valid Creature.controlsPermanent.namedFrob Widget", obj, RefuseValidForm, "nested-filter:controlsPermanent.named"},
		{"Player.controlsPermanent.namedFrob Widget", pl, RefuseNestedFilter, "controlsPermanent.named"},
		{"YouAndSelfAndFrob", pl, RefuseUnknownReferent, "YouAndSelfAndFrob"},
		{"TriggeredSpellAbility", obj, RefuseNeedsAbilityRef, "TriggeredSpellAbility"},
		{"TriggeredSourceSAController", pl, RefuseNeedsAbilityRef, "TriggeredSourceSAController"},
		{"Parent", obj, RefuseNeedsAbilityRef, "Parent"},
		{"Valid Creature.frobnicated", obj, RefuseValidForm, inner.Kind.String() + ":" + inner.Atom},
		{"RememberedFrob", obj, RefuseNeedsMemoryKind, "RememberedFrob"},
		{"ChosenWidget", obj, RefuseNeedsMemoryKind, "ChosenWidget"},
	}
	for k := firstRefRefusal; k <= lastRefRefusal; k++ {
		found := false
		for _, c := range cases {
			found = found || c.kind == k
		}
		if !found {
			t.Errorf("refusal kind %s has no case", k)
		}
	}
	for _, c := range cases {
		got, r := LowerRef(c.text, c.want)
		if r == nil {
			t.Errorf("LowerRef(%q, %s) = %s, want refusal %s: %s", c.text, c.want, got, c.kind, c.atom)
			continue
		}
		if got != nil {
			t.Errorf("LowerRef(%q, %s) returned both a ref and a refusal", c.text, c.want)
		}
		if r.Kind != c.kind || r.Atom != c.atom {
			t.Errorf("LowerRef(%q, %s) = %s: %q, want %s: %q", c.text, c.want, r.Kind, r.Atom, c.kind, c.atom)
		}
	}
}

// TestLowerRefKeepsFilterShape pins that the player "Other" reading lives in
// the reference reader only: LowerFilter still returns "is not the source"
// for Player.Other (dirac/kgen's typed filter reader matches that shape), and
// a LowerRef call does not change what a later LowerFilter returns.
func TestLowerRefKeepsFilterShape(t *testing.T) {
	want := all(player, p(player, ir.PAny, "Player"), n(player, r(p(player, ir.PIs, "Other"), ref(ir.RefSelf))))
	for i := 0; i < 2; i++ {
		got, rf := LowerFilter("Player.Other")
		if rf != nil || !ir.Equal(got, want) {
			t.Fatalf("pass %d: LowerFilter(Player.Other) = %+v, %v; want %+v", i, got, rf, want)
		}
		if _, rf := LowerRef("Player.Other", player); rf != nil {
			t.Fatalf("LowerRef(Player.Other) refused: %v", rf)
		}
	}
}
