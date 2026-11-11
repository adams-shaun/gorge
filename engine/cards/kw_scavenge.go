// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

func kwScavenge(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("A", k) {
		return
	}
	// CR 702.97a (Scavenge): "'Scavenge [cost]' means '[Cost], Exile this
	// card from your graveyard: Put a number of +1/+1 counters equal to the
	// power of the card you exiled on target creature. Activate only as a
	// sorcery.'" The ability is the ordinary graveyard activation the
	// Encore/Embalm expansions already produce: the cost exiles the card
	// itself (ExileFromGrave<1/CARDNAME>), which publishes it on the paid
	// list, so the counter count reads that card's last known power through
	// the Exiled$CardPower paid-list ref (CR 608.2h).
	//
	// param is "<cost>" ("2 G"); no corpus line carries a further
	// colon-separated field, so the first field is the cost.
	cost, _, _ := strings.Cut(param, ":")
	sa, _ := parseSA("", "AB$ PutCounter | Cost$ "+cost+" ExileFromGrave<1/CARDNAME> | ActivationZone$ Graveyard | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ __kwScavengeX | SorcerySpeed$ True | Keyword$ Scavenge | SpellDescription$ Scavenge "+cost)
	if sa != nil {
		f.setSVar("__kwScavengeX", "Exiled$CardPower")
		sa.Params["KeywordLine"] = k
		f.Abilities = append(f.Abilities, sa)
	}
}

func init() { registerKeyword(kwScavenge, "Scavenge") }
