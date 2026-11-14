package effects

// Granted protection from a host-relative quality (CR 702.16): Ward Sliver's
// `AddKeyword$ Protection:Card.ChosenColor:chosenColor`, Pentarch Ward's
// `...:chosenColor:Card.CardUID_HostCardUID` and Benevolent Blessing's
// `...:chosenColor:Aura.ControlledBy Player.PlayerUID_HostCardControllerUID,
// Equipment.ControlledBy Player.PlayerUID_HostCardControllerUID:SBA`.
//
// The keyword field layout is Protection:<quality>:<display>[:<exception>[:SBA]].
// The quality and the exception are host-relative, and rules' protectedFrom
// judges a DERIVED keyword string with no host in hand, so the grant is
// resolved where the static's keyword is built (rules/layers.go), the way
// AddType$ ChosenType is: the host's recorded colour becomes a colour word,
// HostCardUID the host's object id and HostCardControllerUID the host's
// controller seat. A host with no recorded colour grants nothing.

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

var protectionColourWords = [...]string{"white", "blue", "black", "red", "green"}

// ResolveGrantedProtection rewrites the Protection keywords of list that name
// the host's chosen colour; every other keyword passes through unchanged.
func ResolveGrantedProtection(list []string, host *state.Object) []string {
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
		if host != nil {
			if c := colourLetter(host.ChosenColor); c != 0 {
				word = protectionColourWords[strings.IndexByte("WUBRG", c)]
			}
		}
		if word == "" {
			continue
		}
		rest = strings.ReplaceAll(rest, "HostCardControllerUID", strconv.Itoa(int(host.Controller)))
		rest = strings.ReplaceAll(rest, "HostCardUID", strconv.Itoa(int(host.ID)))
		out = append(out, "Protection:"+word+rest)
	}
	if out == nil {
		return list
	}
	return out
}

// ProtectionExempt reports whether the exception field of a resolved
// Protection keyword spares source (CR 702.16: "this effect doesn't remove
// ..."). `Card.CardUID_<n>` names one object, and
// `<spec>.ControlledBy Player.PlayerUID_<n>` an object of that spec under
// seat n (hasSpec judges the spec). An SBA-flagged exception (Forge's fourth
// field) spares only on the state-based attachment check (sba). A clause this
// build cannot read spares nothing.
func ProtectionExempt(kw string, source state.ObjID, sba bool, controller func(state.ObjID) state.PlayerID, hasSpec func(spec string) bool) bool {
	const prefix = "Protection:"
	if len(kw) < len(prefix) || !strings.EqualFold(kw[:len(prefix)], prefix) {
		return false
	}
	if strings.Count(kw, ":") < 3 {
		return false // no exception field: the common keyword allocates nothing
	}
	f := strings.Split(kw[len(prefix):], ":")
	if len(f) < 3 || (len(f) > 3 && strings.EqualFold(strings.TrimSpace(f[3]), "SBA") && !sba) {
		return false
	}
	for c := range strings.SplitSeq(f[2], ",") {
		c = strings.TrimSpace(c)
		if n, ok := strings.CutPrefix(c, "Card.CardUID_"); ok {
			if id, err := strconv.Atoi(n); err == nil && state.ObjID(id) == source {
				return true
			}
			continue
		}
		spec, seat, ok := strings.Cut(c, ".ControlledBy Player.PlayerUID_")
		if !ok {
			continue
		}
		if n, err := strconv.Atoi(seat); err == nil && int(controller(source)) == n && hasSpec(spec) {
			return true
		}
	}
	return false
}
