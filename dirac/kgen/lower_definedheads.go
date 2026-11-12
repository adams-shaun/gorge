package kgen

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
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
// pump.go's object loop skips players and registerPlayerPumpGrant gives the
// controller the hexproof/shroud/protection keywords): `Defined$ You & Valid
// <filter>` with KW$ only (Dawn's Truce, Veil of Summer). It emits the
// controller's hexproof (a TurnRepl record), then a PumpAll of the same
// keywords over the filter (the set read once, at resolution); Veil's
// hexproof from blue and from black is one TurnRepl TurnReplHexUB for the
// player and Permanent.YouCtrl together. done is false for any other subject;
// any other key or compound refuses by name.
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
	ps.get("Defined")
	f, err := lc.filter(strings.TrimPrefix(d, "You & Valid "))
	if err != nil {
		return true, err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return true, refuse("Pump Defined$ %q: a player alternative", d)
	}
	if hexproofFromBlueBlack(kw) {
		// Veil of Summer: hexproof from blue and from black for the controller
		// and every permanent they control as it resolves (the one filter the
		// kernel's mechHexUB plane carries)
		all, err := lc.filter("Permanent.YouCtrl")
		if err != nil {
			return true, err
		}
		if f.String() != all.String() {
			return true, refuse("Pump Defined$ %q: hexproof from colours over %s", d, f)
		}
		lc.emit(lInstr{Op: "TurnRepl", C: kop.TurnReplHexUB, Note: "you and your permanents have hexproof from blue and from black this turn"})
		return true, nil
	}
	ks, err := pumpKeywords(kw)
	if err != nil {
		return true, err
	}
	hex := false
	for _, k := range ks {
		switch {
		case k == "Hexproof":
			hex = true
		case k == "Shroud" || strings.HasPrefix(k, "ProtectionFrom"):
			// gorge registerPlayerPumpGrant gives the player these too; the
			// kernel carries only the controller's hexproof
			return true, refuse("Pump Defined$ %q: a player %s grant", d, k)
		}
	}
	if hex {
		lc.emit(lInstr{Op: "TurnRepl", C: kop.TurnReplPlayerHex, Note: "you have hexproof this turn"})
	}
	lc.emit(lInstr{Op: "PumpAll", F: f, A: cst(0), B: cst(0), CS: strings.Join(ks, "&")})
	return true, nil
}

// hexproofFromBlueBlack reports whether a Pump's KW$ is exactly "Hexproof:Black
// & Hexproof:Blue" in either order (Veil of Summer).
func hexproofFromBlueBlack(kw string) bool {
	parts := strings.Split(kw, "&")
	if len(parts) != 2 {
		return false
	}
	a, b := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	return a == "Hexproof:Black" && b == "Hexproof:Blue" || a == "Hexproof:Blue" && b == "Hexproof:Black"
}
