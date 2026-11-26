package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Granted intimidate, infect, skulk and horsemanship (CR 702.13a, 702.90, 702.118a, 702.31b). None has a
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
		case "Skulk":
			marks |= kop.KwMarkSkulk
		case "Horsemanship":
			marks |= kop.KwMarkHorsemanship
		default:
			if cards.IsHiddenSwitchPTKeyword(k) {
				// CR 613.4d until end of turn (gorge PumpGrant.SwitchPT, a
				// layer-7 SubSwitch effect, read by the same helper): the
				// mark the kernel's P/T fold applies last
				marks |= kop.KwMarkSwitchPT
				continue
			}
			keep = append(keep, k)
		}
	}
	return strings.Join(keep, " & "), marks
}

// takePermanentHorsemanship takes the bare "Horsemanship" term out of a
// Duration$ Permanent KW$ list: the rest of the list and whether it was there.
// A permanent grant has no cleanup-step mark to ride (ExtKwMark's bit is
// cleared there), so it is kop.Override.Horsemanship on the grant's override
// record, which ends with the permanent (Riding the Dilu Horse). Every other
// duration still sends the term through pumpKeywords, which refuses it.
func takePermanentHorsemanship(kw string) (rest string, has bool) {
	var keep []string
	for _, k := range strings.Split(kw, "&") {
		if k = strings.TrimSpace(k); k == "Horsemanship" {
			has = true
			continue
		}
		keep = append(keep, k)
	}
	if !has {
		return kw, false
	}
	return strings.Join(keep, " & "), true
}

// splitStaticKwMarks is splitKwMarks for a Continuous static's AddKeyword$
// list: the rest, and the static's Evasion / Infect grant.
func splitStaticKwMarks(kw string) (rest string, evasion uint8, infect bool) {
	rest, marks := splitKwMarks(kw)
	if marks&kop.KwMarkSwitchPT != 0 {
		// a static's switch is no until-end-of-turn mark: keep the term so
		// the keyword read still refuses it
		rest = strings.TrimPrefix(rest+" & HIDDEN "+cards.HiddenSwitchPTKeyword, " & ")
	}
	if marks&kop.KwMarkIntimidate != 0 {
		evasion = kop.EvIntimidate
	}
	if marks&kop.KwMarkHorsemanship != 0 {
		// Sun Quan, Lord of Wu: Static.Evasion EvHorsemanship, read by
		// evasionGranted beside the printed bit (dirac/kw_kwmark.go)
		evasion |= kop.EvHorsemanship
	}
	if marks&kop.KwMarkSkulk != 0 {
		evasion |= kop.EvSkulk
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
	if marks&kop.KwMarkSkulk != 0 {
		w = append(w, "Skulk")
	}
	if marks&kop.KwMarkHorsemanship != 0 {
		w = append(w, "Horsemanship")
	}
	if marks&kop.KwMarkSwitchPT != 0 {
		w = append(w, "SwitchPT")
	}
	return strings.Join(w, " & ")
}
