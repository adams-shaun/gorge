// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import (
	"strconv"
	"strings"
)

// kwAwaken expands K:Awaken:<N>:<cost> (CR 702.113a). The cost half is an
// alternative cost rules reads as the altAwaken cast mode; this half adds the
// rider "If you do, put N +1/+1 counters on target land you control. That
// land becomes a 0/0 Elemental creature with haste. It's still a land." as a
// pair of Condition$ Awakened links appended to the END of the spell
// ability's SubAbility$ chain, so the spell's own instructions resolve first
// and a plain cast (no Awakened provenance) skips both links and announces
// no land target (rules' castSubTargetsOwed).
func kwAwaken(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	n, _, ok := strings.Cut(param, ":")
	n = strings.TrimSpace(n)
	if _, err := strconv.Atoi(n); !ok || err != nil {
		return
	}
	put := "__kwAwaken" + strconv.Itoa(i)
	if _, done := f.SVars[put]; done {
		return
	}
	var sp *SA
	for _, a := range f.Abilities {
		if a.Kind == "SP" {
			sp = a
			break
		}
	}
	if sp == nil {
		return
	}
	// Find where the printed chain ends before minting anything, so an
	// unresolvable chain leaves the face untouched. A chain link may already
	// be resolved (Sub) or still name an SVar (SubAbility$): descend the
	// resolved links first, then the SVar-named ones.
	last := sp
	for last.Sub != nil {
		last = last.Sub
	}
	tail := ""
	for next, d := last.Params["SubAbility"], 0; next != ""; d++ {
		body, found := f.SVars[next]
		if !found || d > maxSVarDepth {
			return
		}
		tail = next
		next = ""
		if sub, _ := parseSA("", body); sub != nil {
			next = sub.Params["SubAbility"]
		}
	}
	animate := put + "Animate"
	f.setSVar(put, "DB$ PutCounter | ValidTgts$ Land.YouCtrl | TgtPrompt$ Select target land you control"+
		" | CounterType$ P1P1 | CounterNum$ "+n+" | Condition$ Awakened | SubAbility$ "+animate)
	f.setSVar(animate, "DB$ Animate | Defined$ ParentTarget | Power$ 0 | Toughness$ 0"+
		" | Types$ Creature,Elemental | Keywords$ Haste | Duration$ Permanent | Condition$ Awakened")
	link := " | SubAbility$ " + put
	if tail == "" {
		last.Params["SubAbility"] = put
		last.Line += link
		return
	}
	f.SVars[tail] += link
}

func init() { registerKeyword(kwAwaken, "Awaken") }
