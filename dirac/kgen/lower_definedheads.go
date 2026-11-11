package kgen

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// triggeredQualBases are the Triggered-card family bases whose dotted
// spelling gorge filters (triggeredCardQualified, context.go).
var triggeredQualBases = [...]string{"TriggeredCard.", "TriggeredCardLKICopy.", "TriggeredNewCardLKICopy."}

// triggeredQualGuard lowers Defined$ <TriggeredCard base>.<qualifier>: gorge
// keeps the base's objects the qualifier admits (g.Obj(t.Obj), the object as
// it is now) and a non-match is known-empty, never Self. It returns the
// node's SkipIf guard (the base ref matching Card.<qualifier>) and rewrites
// the node's Defined$ to the bare base. Animate and a node with its own
// ValidTgts$ are left to their own handling.
func (lc *lowerCtx) triggeredQualGuard(sa *cards.SA, ps *params) (lCond, error) {
	d, ok := ps.m["Defined"]
	if !ok || sa.API == "Animate" {
		return lCond{}, nil
	}
	for _, b := range triggeredQualBases {
		if !strings.HasPrefix(d, b) {
			continue
		}
		if strings.Contains(d, " & ") {
			return lCond{}, nil
		}
		if _, ok := ps.m["ValidTgts"]; ok {
			return lCond{}, refuse("Defined$ %q beside its own ValidTgts$", d)
		}
		base := strings.TrimSuffix(b, ".")
		ref, err := lc.defined(base)
		if err != nil {
			return lCond{}, err
		}
		f, err := lc.filter("Card." + strings.TrimPrefix(d, b))
		if err != nil {
			return lCond{}, err
		}
		ps.m["Defined"] = base
		return lCond{Kind: cRefMatches, Ref: ref, F: f, Op: opGE, A: cst(1)}, nil
	}
	return lCond{}, nil
}

// pumpYouAndValid lowers the one compound Pump subject gorge's definedSpec
// joins with " & " (effects/context.go: the parts' target sets, You first;
// pump.go skips players in the object loop and gives a player the keyword
// grant): `Defined$ You & Valid <filter>` with KW$ only (Dawn's Truce). It
// emits the controller's keyword grant, then a PumpAll of the same keywords
// over the filter (the set read once, at resolution). done is false for any
// other subject; any other key or compound refuses by name.
func (lc *lowerCtx) pumpYouAndValid(ps *params) (done bool, err error) {
	d := strings.TrimSpace(ps.m["Defined"])
	if !strings.HasPrefix(d, "You & ") {
		return false, nil
	}
	if !strings.HasPrefix(d, "You & Valid ") || strings.Contains(strings.TrimPrefix(d, "You & Valid "), " & ") {
		return true, refuse("Pump Defined$ %q: a compound other than You & Valid <filter>", d)
	}
	var extra []string
	for k := range ps.m {
		switch k {
		case "Defined", "KW", "SpellDescription", "StackDescription", "SubAbility":
		default:
			if !ps.used[k] {
				extra = append(extra, k)
			}
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		return true, refuse("Pump Defined$ %q with %s$", d, extra[0])
	}
	kw, ok := ps.get("KW")
	if !ok {
		return true, refuse("Pump Defined$ %q without KW$", d)
	}
	if _, mb := splitKWSentence(kw, mustBeBlockedKW); mb {
		return true, refuse("Pump Defined$ %q with the must-be-blocked sentence", d)
	}
	ks, err := pumpKeywords(kw)
	if err != nil {
		return true, err
	}
	ps.get("Defined")
	f, err := lc.filter(strings.TrimPrefix(d, "You & Valid "))
	if err != nil {
		return true, err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return true, refuse("Pump Defined$ %q: a player alternative", d)
	}
	cs := strings.Join(ks, "&")
	lc.emit(lInstr{Op: "GrantKeyword", Obj: lRef{Kind: rYou}, CS: cs})
	lc.emit(lInstr{Op: "PumpAll", F: f, A: cst(0), B: cst(0), CS: cs})
	return true, nil
}
