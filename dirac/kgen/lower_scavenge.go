package kgen

import (
	"fmt"
	"strconv"
	"strings"
)

func init() { kwHandlers["Scavenge"] = (*lowerCtx).lowerScavengeKw }

// lowerScavengeKw lowers K:Scavenge:<cost> (CR 702.97a): gorge's kwScavenge
// (cards/kw_scavenge.go) expands it into one sorcery-speed graveyard
// AB$ PutCounter whose cost exiles the card itself and whose count is the
// SVar __kwScavengeX, lowered from that line (scavengeExiled reads the
// count). The head is also read by rules code (rules/quiet_facts.go:193,
// the alternative-cast facts shared with Embalm and Eternalize), which adds
// nothing to the ability's own lines. A face whose expansion a printed
// A: line suppressed refuses.
func (lc *lowerCtx) lowerScavengeKw(node, _ string) {
	if !lc.hasKeywordLine(kwLineOf(node)) {
		lc.refuseNode(node, "keyword %q without its expansion", "Scavenge")
	}
}

// scavengeExiled lowers the expansion's count, Exiled$CardPower (CR 702.97a:
// "a number of +1/+1 counters equal to the power of the card you exiled"):
// the one card the cost exiles is the scavenger itself, from a graveyard,
// where nothing a lowered card does changes a card's power, so the count is
// its printed power. A power that is characteristic-defining, or anything
// but a plain number, refuses: the kernel would have to read it in the
// graveyard (Boneyard Mycodrax, Dodgy Jalopy).
func (lc *lowerCtx) scavengeExiled(rest string) (lValue, error) {
	if rest != "CardPower" || lc.face.SVars["__kwScavengeX"] != "Exiled$CardPower" {
		return lValue{}, fmt.Errorf("value head Exiled$%s", rest)
	}
	pw, _, _ := strings.Cut(lc.face.PT, "/")
	n, err := strconv.Atoi(strings.TrimSpace(pw))
	if err != nil || lc.face.CharacteristicDefining() {
		return lValue{}, refuse("Exiled$CardPower of a scavenger with power %q", pw)
	}
	return cst(n), nil
}
