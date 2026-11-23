package kgen

import (
	"fmt"
	"math/bits"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The relation and keyword filter words (T13b): each is a kernel FilterWord
// literal (dirac/filter_*.go), emitted through the clause's spill program
// (filter_spill.go), so a filter that lowers today keeps its struct form and
// only a filter that refused gains a program.

// fwArm lowers one filter property p of the clause.
type fwArm func(c *lClause, p string) error

// fwArms maps a property spelling to its arm; filter.go's property switch
// asks it last, before "unknown property".
var fwArms = map[string]fwArm{}

func init() {
	fwName[kop.FwWithKeyword] = "FwWithKeyword"
	fwName[kop.FwChosenCtrl] = "FwChosenCtrl"
	fwName[kop.FwEnchantedControllerCtrl] = "FwEnchantedControllerCtrl"

	// with<K>, without<K> and !with<K> over a printed keyword the kernel keeps
	// a record of (dirac/filter_withkeyword.go printedKeyword); gorge registers
	// all three spellings (engine/effects/filter.go's keyword registry).
	for name, id := range fwKeywordWords {
		for _, p := range [...]struct {
			spell string
			neg   bool
		}{{"with" + name, false}, {"without" + name, true}, {"!with" + name, true}} {
			id, neg := id, p.neg
			fwArms[p.spell] = func(c *lClause, _ string) error {
				return c.fwLit(lSpillLit{W: kop.FwWithKeyword, Neg: neg, N: int(id)})
			}
		}
	}
	// withFlash reads the KwMask bit the kernel already keeps (the printed mask
	// for a card), the struct's PredHasKeyword form, which refuses Flash on the
	// battlefield. withHexproof is not lowered, with or without: gorge matches
	// by keyword head, so it also matches Hexproof:<spec>, which the plain
	// KwHexproof bit cannot see (TestWithHexproofStaysRefused).
	flashBit := bits.TrailingZeros32(uint32(kop.KwFlash))
	fwArms["withFlash"] = func(c *lClause, _ string) error { return c.setCmp(cmpHasKw, 0, flashBit, "", "") }
	for _, p := range [...]string{"withoutFlash", "!withFlash"} {
		fwArms[p] = func(c *lClause, _ string) error { return c.setCmp(cmpLacksKw, 0, flashBit, "", "") }
	}
	// ChosenCtrl: controlled by the player the source chose as it entered.
	fwArms["ChosenCtrl"] = func(c *lClause, _ string) error {
		return c.fwLit(lSpillLit{W: kop.FwChosenCtrl})
	}
	// EnchantedControllerCtrl: controlled by the controller of the permanent
	// the source is attached to. gorge refuses the '!' of it too (an absent
	// link inverts to nothing), so only the positive spelling lowers.
	fwArms["EnchantedControllerCtrl"] = func(c *lClause, _ string) error {
		return c.fwLit(lSpillLit{W: kop.FwEnchantedControllerCtrl})
	}
}

// fwKeywordWords are the printed-keyword words lowered to FwWithKeyword, by
// gorge's compact keyword name: only keywords whose printed record the kernel
// keeps (printedKeyword). A keyword with no record (Flanking, Cycling,
// Disturb ...) stays an unknown property.
var fwKeywordWords = map[string]kop.KwID{
	"Toxic": kop.KwToxic, "Horsemanship": kop.KwHorsemanship, "Morph": kop.KwMorph, "Megamorph": kop.KwMegamorph,
	"Suspend": kop.KwSuspend, "Cascade": kop.KwCascade, "Convoke": kop.KwConvoke, "Foretell": kop.KwForetell,
	"Madness": kop.KwMadness, "Multikicker": kop.KwMultikicker,
}

// fwLit adds one filter-word literal to the clause's program, moving a
// comparison already in the Pred slot into it first (as spillCmp does). A
// comparison with no word, or a full program, refuses.
func (c *lClause) fwLit(l lSpillLit) error {
	switch c.CmpStat {
	case cmpNone:
		c.CmpStat = cmpSpill
	case cmpSpill:
	default:
		first, ok := cmpLit(c.CmpStat, c.CmpOp, c.CmpN, c.CmpX, c.CmpCounter)
		if !ok {
			return fmt.Errorf("filter word %s beside a comparison with no word", fwName[l.W])
		}
		c.pushSpill(first)
		c.CmpStat, c.CmpOp, c.CmpN, c.CmpX, c.CmpCounter = cmpSpill, 0, 0, "", ""
	}
	if !c.pushSpill(l) {
		return fmt.Errorf("filter word %s: the clause's program is full", fwName[l.W])
	}
	return nil
}

// fwArmOrUnknown lowers property p by its arm, or is the unknown-property
// error: the tail of lClause.prop's switch.
func (c *lClause) fwArmOrUnknown(p string) error {
	if strings.HasPrefix(p, "with") || strings.HasPrefix(p, "!with") || p == "ChosenCtrl" || p == "EnchantedControllerCtrl" {
		if arm, ok := fwArms[p]; ok {
			return arm(c, p)
		}
	}
	return fmt.Errorf("unknown property %q", p)
}
