package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Granted intimidate and infect (CR 702.13a, 702.90). Neither keyword has a
// kop.KwMask bit (all 32 are taken), and printed ones are CardDef fields
// (Evasion EvIntimidate, Infect), so a grant is carried apart from the
// keyword bits, the way a granted landwalk is (lower_landwalkgrant.go):
//
//   - a Continuous static's AddKeyword$ term: kop.Static.Evasion /
//     kop.Static.Infect, a layer-6 grant the kernel reads beside the printed
//     field (Bellowing Tanglewurm, Vow of Malice, Grafted Exoskeleton);
//   - a Pump / PumpAll KW$ term: OpExt ExtKwMark, an until-end-of-turn mark
//     on each subject (Tainted Strike, Vector Asp, Hideous Visage).
//
// gorge reads both as the derived keyword (combat.CanBlock's intimidate,
// effects/damage.go HasKeyword(source, "Infect")), blind to where it came
// from. An Animate's Keywords$ (an override record's AddKw) still refuses
// through pumpKeywords.

// splitKwMarks takes the bare "Intimidate" and "Infect" terms out of a KW$
// list: the rest of the list and the kop.KwMark* bits of the terms taken.
func splitKwMarks(kw string) (rest string, marks int) {
	var keep []string
	for _, k := range strings.Split(kw, "&") {
		switch k = strings.TrimSpace(k); k {
		case "Intimidate":
			marks |= kop.KwMarkIntimidate
		case "Infect":
			marks |= kop.KwMarkInfect
		default:
			keep = append(keep, k)
		}
	}
	return strings.Join(keep, " & "), marks
}

// splitStaticKwMarks is splitKwMarks for a Continuous static's AddKeyword$
// list: the rest, and the static's Evasion / Infect grant.
func splitStaticKwMarks(kw string) (rest string, evasion uint8, infect bool) {
	rest, marks := splitKwMarks(kw)
	if marks&kop.KwMarkIntimidate != 0 {
		evasion = kop.EvIntimidate
	}
	return rest, evasion, marks&kop.KwMarkInfect != 0
}

// kwMarkNames spells the marks for a refusal or a note.
func kwMarkNames(marks int) string {
	var w []string
	if marks&kop.KwMarkIntimidate != 0 {
		w = append(w, "Intimidate")
	}
	if marks&kop.KwMarkInfect != 0 {
		w = append(w, "Infect")
	}
	return strings.Join(w, " & ")
}
