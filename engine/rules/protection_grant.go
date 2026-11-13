package rules

// Granted protection from a host-relative quality (CR 702.16): Ward Sliver's
// `AddKeyword$ Protection:Card.ChosenColor:chosenColor`, Pentarch Ward's
// `...:chosenColor:Card.CardUID_HostCardUID` and Benevolent Blessing's
// `...:chosenColor:Aura.ControlledBy Player.PlayerUID_HostCardControllerUID,
// Equipment.ControlledBy Player.PlayerUID_HostCardControllerUID:SBA`.
//
// The keyword field layout is Protection:<quality>:<display>[:<exception>[:SBA]].
// The quality and the exception are host-relative, and protectedFrom judges a
// DERIVED keyword string with no host in hand, so the grant is resolved where
// the static's keyword is built (layers.go), the way resolveChosenTypes
// resolves AddType$ ChosenType: the host's recorded colour becomes a colour
// word, HostCardUID the host's object id and HostCardControllerUID the host's
// controller seat. A host with no recorded colour grants nothing.

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

var protectionColourWords = [...]string{"white", "blue", "black", "red", "green"}

// resolveGrantedProtection rewrites the Protection keywords of list that name
// the host's chosen colour; every other keyword passes through unchanged.
func resolveGrantedProtection(list []string, host state.ObjID, o *state.Object) []string {
	const quality = "Protection:Card.ChosenColor"
	var out []string
	for i, kw := range list {
		rest, ok := strings.CutPrefix(kw, quality)
		if !ok || (rest != "" && rest[0] != ':') {
			if out != nil {
				out = append(out, kw)
			}
			continue
		}
		if out == nil {
			out = append(make([]string, 0, len(list)), list[:i]...)
		}
		word := ""
		if o != nil {
			if cols, ok := pay.ResolveChosenColors("ChosenColor", o); ok && len(cols) == 1 {
				if n := strings.IndexByte("WUBRG", cols[0][0]); n >= 0 {
					word = protectionColourWords[n]
				}
			}
		}
		if word == "" {
			continue
		}
		rest = strings.ReplaceAll(rest, "HostCardControllerUID", strconv.Itoa(int(o.Controller)))
		rest = strings.ReplaceAll(rest, "HostCardUID", strconv.Itoa(int(host)))
		out = append(out, "Protection:"+word+rest)
	}
	if out == nil {
		return list
	}
	return out
}

// protectionException splits the exception field off a Protection keyword:
// the comma-separated clauses (CR 702.16: "this effect doesn't remove ...")
// and whether they apply only to the state-based attachment check (Forge's
// SBA flag). No exception returns nil.
func protectionException(kw string) (clauses []string, sbaOnly bool) {
	const prefix = "Protection:"
	if len(kw) < len(prefix) || !strings.EqualFold(kw[:len(prefix)], prefix) {
		return nil, false
	}
	f := strings.Split(kw[len(prefix):], ":")
	if len(f) < 3 || strings.TrimSpace(f[2]) == "" {
		return nil, false
	}
	for c := range strings.SplitSeq(f[2], ",") {
		if c = strings.TrimSpace(c); c != "" {
			clauses = append(clauses, c)
		}
	}
	return clauses, len(f) > 3 && strings.EqualFold(strings.TrimSpace(f[3]), "SBA")
}

// protectionOracle is the slice of the engine protectedFromKW reads.
type protectionOracle interface {
	Keywords(state.ObjID) []string
	sourceHasQuality(source state.ObjID, q string) bool
	controllerOf(state.ObjID) state.PlayerID
}

// protectionExempt reports whether a resolved exception clause of a keyword
// spares source: `Card.CardUID_<n>` names one object, and
// `<spec>.ControlledBy Player.PlayerUID_<n>` an object of that spec under
// seat n. A clause this build cannot read spares nothing.
func protectionExempt(h protectionOracle, kw string, source state.ObjID, sba bool) bool {
	clauses, sbaOnly := protectionException(kw)
	if len(clauses) == 0 || (sbaOnly && !sba) {
		return false
	}
	for _, c := range clauses {
		if n, ok := strings.CutPrefix(c, "Card.CardUID_"); ok {
			if id, ok := parseInt10(n); ok && id == int64(source) {
				return true
			}
			continue
		}
		spec, seat, ok := strings.Cut(c, ".ControlledBy Player.PlayerUID_")
		if !ok {
			continue
		}
		if n, ok := parseInt10(seat); ok && int64(h.controllerOf(source)) == n && h.sourceHasQuality(source, spec) {
			return true
		}
	}
	return false
}
