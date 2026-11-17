// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// kwReinforce expands Reinforce N—[cost] (CR 702.77a): "[Cost], Discard this
// card: Put N +1/+1 counters on target creature", activated only from hand.
// It is the cycling shape (a hand-zone activation whose cost discards the
// card itself). param is "<N>:<cost>" ("2:1 G"); N is "X" on the X carriers,
// whose script declares SVar:X:Count$xPaid.
func kwReinforce(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("A", k) {
		return
	}
	cut := -1
	for j := 0; j < len(param); j++ {
		if param[j] == ':' {
			cut = j
			break
		}
	}
	if cut <= 0 || cut == len(param)-1 {
		return
	}
	n, cost := param[:cut], param[cut+1:]
	sa, _ := parseSA("", "AB$ PutCounter | Cost$ "+cost+" Discard<1/CARDNAME> | ActivationZone$ Hand | ValidTgts$ Creature | TgtPrompt$ Select target creature | CounterType$ P1P1 | CounterNum$ "+n+" | Keyword$ Reinforce | SpellDescription$ Reinforce "+n+"—"+cost)
	if sa != nil {
		sa.Params["KeywordLine"] = k
		f.Abilities = append(f.Abilities, sa)
	}
}

func init() { registerKeyword(kwReinforce, "Reinforce") }
