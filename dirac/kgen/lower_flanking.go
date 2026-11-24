package kgen

import (
	"io"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func init() { kwHandlers["Flanking"] = (*lowerCtx).lowerFlankingKw }

// lowerFlankingKw lowers a printed bare K:Flanking (CR 702.25a): gorge's
// kwFlanking (cards/kw_flanking.go) expands it into one
// Mode$ AttackerBlockedByCreature trigger (ValidCard$ Card.Self,
// ValidBlocker$ Creature.withoutFlanking; the Pump -1/-1 body names the
// blocker, TriggeredBlockerLKICopy), lowered from that line. The head is also
// read by rules code (engine/rules/trigger_blocks.go:191 flankingTrigger,
// flankingInstances, isFlankingMarker): the expansion trigger is a MARKER and
// the derived-keyword walk owns the instance count (CR 702.25b: each instance
// triggers separately; a creature granted flanking never lowers). With no granted flanking in a lowered pool the derived
// count is the printed one, so exactly one printed line is read: the marker
// fires once per (attacker, blocker without flanking) pair, as the kernel's
// by-creature become-blocked trigger does. CardDef.Flanking records the
// printed keyword for the W7WithoutFlanking word (dirac/filter_flanking.go)
// and the withFlanking word (FwWithKeyword, dirac/filter_withkeyword.go).
// A second K:Flanking line (two instances) refuses.
func (lc *lowerCtx) lowerFlankingKw(node, param string) {
	if param != "" {
		lc.refuseNode(node, "flanking with a parameter %q", param)
		return
	}
	if lc.out.Flanking {
		lc.refuseNode(node, "a second K:Flanking instance")
		return
	}
	line := kwLineOf(node)
	for _, t := range lc.face.Triggers {
		if t.Params["KeywordLine"] == line {
			lc.out.Flanking = true
			return
		}
	}
	lc.refuseNode(node, "keyword %q without its expansion", "Flanking")
}

// dumpLIRKwFlags writes the printed-keyword flags dumpLIR shares one line for:
// unleash, riot, enlist, flanking, then phasing.
func dumpLIRKwFlags(w io.Writer, c *lCard) {
	dumpLIRYes(w, c.Unleash, " unleash")
	dumpLIRYes(w, c.Riot, " riot")
	dumpLIRYes(w, c.Enlist, " enlist")
	dumpLIRYes(w, c.Flanking, " flanking")
	dumpLIRYes(w, c.Phasing, " phasing")
}

// opsAddKwFlags is opsUsage's add of the printed-keyword card fields Unleash,
// Riot, Enlist, Flanking and Phasing (opsdoc_flanking.go documents the last two,
// opsdoc_riot.go and opsdoc_enlist.go the first two of those).
func opsAddKwFlags(add func(label, card string), d *kop.CardDef, name string) {
	opsAddYes(add, d.Unleash, "card:Unleash", name)
	opsAddYes(add, d.Riot, "card:Riot", name)
	opsAddYes(add, d.Enlist, "card:Enlist", name)
	opsAddYes(add, d.Flanking, "card:Flanking", name)
	opsAddYes(add, d.Phasing, "card:Phasing", name)
}
