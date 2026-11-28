package ogram

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
)

// RefusedError is a filter that does not lower. Key names the variant:
// "ObjectFilter.InZone" (a release variant the adapter does not map;
// "ObjectFilter.Not(InZone)" for a Not mtgish has no negated row for),
// "Property.ColorCount" (one of its arguments), "ObjectRef.Target" (a
// SameObject reference with no mtgish reference node), or "mtgish
// Permanents.IsNamed" (a mapped variant mtgish's row table refuses).
type RefusedError struct{ Key string }

func (e *RefusedError) Error() string { return "ogram: no lowering for " + e.Key }

func refuse(key string) error { return &RefusedError{Key: key} }

// LowerFilter lowers n, an ObjectFilter node of a reading, as a Permanents
// filter (the group of an operand with no declared domain; Operand.Lower
// uses the operand's own group).
func LowerFilter(n json.RawMessage) (*ir.Filter, error) { return LowerFilterIn("Permanents", n) }

// LowerFilterIn lowers n, an ObjectFilter node, as a filter of the mtgish
// group enum g: the adapter respells it in mtgish's encoding and
// mtgish.LowerFilterNode lowers it through mtgish's row table.
func LowerFilterIn(g string, n json.RawMessage) (*ir.Filter, error) {
	if g == "" {
		return nil, refuse("ObjectKind.Ability")
	}
	var x any
	if err := json.Unmarshal(n, &x); err != nil {
		return nil, err
	}
	m, err := (&adapter{g: g}).filter(x)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	f, err := mtgish.LowerFilterNode(g, raw)
	var re *mtgish.RefusalError
	if errors.As(err, &re) {
		return nil, &RefusedError{Key: "mtgish " + re.Key}
	}
	return f, err
}

type adapter struct{ g string }

// node is an mtgish node of the adapter's group enum.
func (a *adapter) node(variant string, args ...any) map[string]any {
	m := map[string]any{"_" + a.g: variant}
	if len(args) == 1 {
		m["args"] = args[0]
	}
	return m
}

// kindArgs reads an adjacent-enum node of the release.
func kindArgs(x any) (string, any, bool) {
	m, ok := x.(map[string]any)
	if !ok {
		return "", nil, false
	}
	k, ok := m["kind"].(string)
	return k, m["args"], ok
}

// anyVariant is each group's "any object" variant.
var anyVariant = map[string]string{
	"Permanents":        "AnyPermanent",
	"Spells":            "AnySpell",
	"CardsInGraveyards": "AnyCardInAnyGraveyard",
}

// plain maps the release's unit variants to mtgish's, plain and under Not.
var plain = map[string][2]string{
	"Colorless": {"IsColorless", ""},
	"Token":     {"IsToken", "IsNonToken"},
	"Tapped":    {"IsTapped", "IsUntapped"},
	"Attacking": {"IsAttacking", ""},
	"Blocking":  {"IsBlocking", ""},
}

// worded maps the release's one-word variants (a card type, supertype or
// subtype) to mtgish's, plain and under Not.
var worded = map[string][2]string{
	"CardType":  {"IsCardtype", "IsNonCardtype"},
	"Supertype": {"IsSupertype", "IsNonSupertype"},
	"Subtype":   {"IsCreatureType", "IsNonCreatureType"},
}

func (a *adapter) filter(x any) (any, error) {
	kind, args, ok := kindArgs(x)
	if !ok {
		return nil, fmt.Errorf("ogram: not an ObjectFilter node: %T", x)
	}
	if p, ok := plain[kind]; ok {
		return a.node(p[0]), nil
	}
	if p, ok := worded[kind]; ok {
		return a.node(p[0], args), nil
	}
	switch kind {
	case "Any":
		v, ok := anyVariant[a.g]
		if !ok {
			v = "AnyCard"
		}
		return a.node(v), nil
	case "AllOf", "AnyOf":
		l, _ := args.([]any)
		var out []any
		for _, e := range l {
			m, err := a.filter(e)
			if err != nil {
				return nil, err
			}
			out = append(out, m)
		}
		if kind == "AllOf" {
			return a.node("And", out), nil
		}
		return a.node("Or", out), nil
	case "Color":
		return a.node("IsColor", map[string]any{"_Color": args}), nil
	case "HasKeyword":
		kw, err := keywordName(args)
		if err != nil {
			return nil, err
		}
		return a.node("HasAbility", map[string]any{"_CheckHasable": kw}), nil
	case "Name":
		return a.node("IsNamed", map[string]any{"_NameFilter": "NamedCard", "args": args}), nil
	case "Not":
		return a.not(args)
	case "SameObject":
		return a.same(args, false)
	case "Property":
		return a.property(args)
	case "ControlledBy", "OwnedBy":
		ps, err := players(args)
		if err != nil {
			return nil, err
		}
		if kind == "ControlledBy" {
			return a.node("ControlledByAPlayer", ps), nil
		}
		return a.node("OwnedByAPlayer", ps), nil
	}
	return nil, refuse("ObjectFilter." + kind)
}

// not maps Not over a variant mtgish has a negated row for.
func (a *adapter) not(inner any) (any, error) {
	kind, args, _ := kindArgs(inner)
	if p, ok := plain[kind]; ok && p[1] != "" {
		return a.node(p[1]), nil
	}
	if p, ok := worded[kind]; ok {
		return a.node(p[1], args), nil
	}
	switch kind {
	case "Color":
		return a.node("IsNonColor", map[string]any{"_Color": args}), nil
	case "HasKeyword":
		kw, err := keywordName(args)
		if err != nil {
			return nil, err
		}
		return a.node("DoesntHaveAbility", map[string]any{"_CheckHasable": kw}), nil
	case "SameObject":
		return a.same(args, true)
	}
	return nil, refuse("ObjectFilter.Not(" + kind + ")")
}

// singleRow is each group's "is that object" row (mtgish's PIs); Not of it
// is the group's Other row.
var singleRow = map[string]string{"Permanents": "SinglePermanent", "Spells": "SingleSpell"}

// refNodes maps the release's unit ObjectRef variants, by group, to the
// mtgish reference node of that group's single-object enum, lowered by
// mtgish's fe_ref.go rows.
var refNodes = map[string]map[string][2]string{
	"Source": {
		"Permanents":        {"Permanent", "ThisPermanent"},
		"Spells":            {"Spell", "ThisSpell"},
		"Cards":             {"Card", "ThisCard"},
		"CardsInGraveyards": {"CardInGraveyards", "ThisGraveyardCard"},
	},
	"EventSubject": {
		"Permanents":        {"Permanent", "Trigger_ThatPermanent"},
		"Spells":            {"Spell", "Trigger_ThatSpell"},
		"CardsInGraveyards": {"CardInGraveyards", "Trigger_ThatGraveyardCard"},
	},
	// AttachedTo(Source): the permanent the source is attached to.
	"AttachedTo(Source)": {"Permanents": {"Permanent", "HostPermanent"}},
}

// same maps SameObject(ref) to the group's Single* row, or under Not to
// its Other row. A Target ref needs a declared target slot the adapter
// cannot declare: it is refused, as is any ref with no mtgish reference node.
func (a *adapter) same(ref any, negate bool) (any, error) {
	kind, args, ok := kindArgs(ref)
	if !ok {
		return nil, fmt.Errorf("ogram: not an ObjectRef node: %T", ref)
	}
	key := kind
	if of, _, _ := kindArgs(args); kind == "AttachedTo" && of == "Source" {
		key = "AttachedTo(Source)"
	}
	r, ok := refNodes[key][a.g]
	if !ok {
		return nil, refuse("ObjectRef." + kind)
	}
	arg := map[string]any{"_" + r[0]: r[1]}
	if negate {
		return a.node("Other", arg), nil
	}
	row, ok := singleRow[a.g]
	if !ok {
		return nil, refuse("ObjectFilter.SameObject(" + a.g + ")")
	}
	return a.node(row, arg), nil
}

func keywordName(x any) (string, error) {
	kind, args, ok := kindArgs(x)
	if !ok {
		return "", fmt.Errorf("ogram: not a Keyword node: %T", x)
	}
	if kind == "WithReminder" {
		m, _ := args.(map[string]any)
		return keywordName(m["keyword"])
	}
	return kind, nil
}

var propertyVariant = map[string]string{"ManaValue": "ManaValueIs", "Power": "PowerIs", "Toughness": "ToughnessIs"}

var comparison = map[string]string{
	"Equal": "EqualTo", "NotEqual": "NotEqualTo", "Less": "LessThan", "LessOrEqual": "LessThanOrEqualTo",
	"Greater": "GreaterThan", "GreaterOrEqual": "GreaterThanOrEqualTo",
}

// property is Property{property, test}: a mana value, power or toughness
// compared with a constant.
func (a *adapter) property(args any) (any, error) {
	m, _ := args.(map[string]any)
	p, _ := m["property"].(string)
	v, ok := propertyVariant[p]
	if !ok {
		return nil, refuse("Property." + p)
	}
	tk, targs, _ := kindArgs(m["test"])
	if tk != "Compare" {
		return nil, refuse("NumberTest." + tk)
	}
	tm, _ := targs.(map[string]any)
	c, _ := tm["comparison"].(string)
	cmp, ok := comparison[c]
	if !ok {
		return nil, refuse("Comparison." + c)
	}
	ek, eargs, _ := kindArgs(tm["value"])
	if ek != "Constant" {
		return nil, refuse("Expr." + ek)
	}
	n := map[string]any{"_GameNumber": "Integer", "args": eargs}
	return a.node(v, map[string]any{"_Comparison": cmp, "args": n}), nil
}

// players maps a PlayerSet to an mtgish Players node: the ability's
// controller, its opponents, or every player.
func players(x any) (any, error) {
	kind, args, ok := kindArgs(x)
	if !ok {
		return nil, fmt.Errorf("ogram: not a PlayerSet node: %T", x)
	}
	switch kind {
	case "All":
		return map[string]any{"_Players": "AnyPlayer"}, nil
	case "One", "OpponentsOf":
		ref, _, _ := kindArgs(args)
		if ref != "AbilityController" {
			return nil, refuse("PlayerRef." + ref)
		}
		if kind == "One" {
			return map[string]any{"_Players": "SinglePlayer", "args": map[string]any{"_Player": "You"}}, nil
		}
		return map[string]any{"_Players": "Opponent"}, nil
	}
	return nil, refuse("PlayerSet." + kind)
}
