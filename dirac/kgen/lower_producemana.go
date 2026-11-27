package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// produceManaRepl lowers the one ProduceMana replacement shape the kernel's
// tap-extra static models exactly: a basic land the replacement's controller
// taps for mana makes a multiple of its normal production.
//
// Virtue of Strength is the card:
//
//	R:Event$ ProduceMana | ActiveZones$ Battlefield | ValidActivator$ You |
//	  ValidCard$ Land.Basic | ReplaceWith$ ProduceThrice
//	SVar:ProduceThrice: a ReplaceMana effect with ReplaceAmount 3
//
// gorge applies the replacement per ManaAdd (rules replacement_mana.go's
// replaceMana rewrite: ReplaceAmount multiplies the whole production) only
// when the producing permanent matches ValidCard$, the activator matches
// ValidActivator$ You, and the mana came from a tap. A basic land's printed
// mana ability produces exactly one mana, so the whole shape is the kernel's
// reflected tap-extra (kop.StatTapExtra with StTapReflectDefer and B = N-1): the
// controller's own tap of a basic land adds N-1 more mana of the type that
// tap produced, beside its own one (dirac/mana_tapextra.go). The static is
// marked StTapReflectDefer, not StTapReflect: gorge's payment plan does not
// fold a replacement (only a TapsForMana trigger), so the plan taps the same
// number of lands as gorge and the extra floats at each tap. Reflected mode
// already knows how to read "of the produced type", so no new kop ReplKind is
// needed.
//
// Every other ProduceMana spelling stays refused: a replacement over
// permanents generally (Mana Reflection, Nyxbloom Ancient), a colour rewrite
// (Contamination, Damping Sphere), a command-zone static (Mirri, Deep Water),
// a non-tap production (Fertile Ground's extra, a triggered source) or a
// multiple-word ValidCard$ has no tap-extra model here.
func (lc *lowerCtx) produceManaRepl(r cards.Repl, rp *params) error {
	if v, ok := rp.get("ActiveZones"); ok && strings.TrimSpace(v) != "Battlefield" {
		return refuse("ProduceMana replacement ActiveZones$ %q", v)
	}
	if v, ok := rp.get("ValidActivator"); !ok || strings.TrimSpace(v) != "You" {
		return refuse("ProduceMana replacement ValidActivator$ %q", v)
	}
	vc, ok := rp.get("ValidCard")
	if !ok || strings.TrimSpace(vc) != "Land.Basic" {
		return refuse("ProduceMana replacement ValidCard$ %q", vc)
	}
	if r.With == nil || r.With.API != "ReplaceMana" || r.With.Sub != nil {
		return refuse("ProduceMana replacement body %q", replBodyAPI(r.With))
	}
	body := newParams(r.With.Params)
	amt, ok := body.get("ReplaceAmount")
	if !ok {
		return refuse("ProduceMana replacement ReplaceMana body without ReplaceAmount$")
	}
	n, err := strconv.Atoi(strings.TrimSpace(amt))
	if err != nil || n < 2 || n > 20 {
		return refuse("ProduceMana replacement ReplaceAmount$ %q", amt)
	}
	body.consume("ReplaceAmount")
	if left := body.unread(); len(left) > 0 {
		return refuse("ProduceMana replacement ReplaceMana body params %v", left)
	}
	rp.consume("ActiveZones", "ValidActivator", "ValidCard")
	if left := rp.unread(); len(left) > 0 {
		return refuse("ProduceMana replacement params %v", left)
	}
	f, err := lc.filter(strings.TrimSpace(vc))
	if err != nil {
		return err
	}
	// The tapped basic land already makes its one mana; the replacement's
	// multiplier N is N-1 more of the same type (gorge's ReplaceAmount
	// multiplies the whole ManaAdd). Reflected mode reads the produced type.
	lc.out.Statics = append(lc.out.Statics, lStatic{
		Kind: "TapExtra", Flags: []string{"TapReflectDefer"}, F: f, TapAmt: uint8(n - 1),
	})
	return nil
}

// replBodyAPI names a replacement body for a refusal without dereferencing a
// nil SA.
func replBodyAPI(sa *cards.SA) string {
	if sa == nil {
		return "<none>"
	}
	return sa.API
}
