package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugAtkTaxCountSelfTaps (test only, TestFourSeatAtkTaxSelfTapsLockstep): the
// walk lists a self-Taps watcher with no mana ability (Scaretiller) as a fixed
// {C} source the window taps.
var DebugAtkTaxCountSelfTaps bool

// atkTaxDebugSelfTaps is DebugAtkTaxCountSelfTaps' reading of the creature at sl.
func (s *State) atkTaxDebugSelfTaps(k *prep, sl Slot) bool {
	cd := &k.t.Cards[s.cardAt(sl)]
	if s.perms[sl].flags&pfTapped != 0 || s.activationSick(k, sl) {
		return false
	}
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && atkTaxSelfTaps(cd, a) {
			return true
		}
	}
	return false
}

// atkTaxSelfTaps reports whether Taps trigger a of card c is a watcher the
// attack-tax walk models: "whenever CARDNAME becomes tapped" (TfSelf) on a card
// with no mana ability of its own, so no window tap of a source fires it (CR
// 603.2, 605.1a; gorge's window taps only a permanent with a mana ability). The
// declared attackers' own taps fire it as in a game with no tax (CR 508.1f,
// 508.1j). A Taps watcher of another permanent, a batch or a tap for mana is
// fired by a window tap and stays a gap.
func atkTaxSelfTaps(c *kop.CardDef, a *kop.Ability) bool {
	if a.Trig.Flags&kop.TfSelf == 0 || a.Trig.Flags2&kop.Tf2Batch != 0 || a.Trig.Flags5&kop.Tf5TapsForMana != 0 || a.Trig.Whose != kop.WhoseAny {
		return false
	}
	for j := range c.Abilities {
		if c.Abilities[j].Kind == kop.AbMana {
			return false
		}
	}
	return true
}

// atkTaxTapsLive reports whether a Taps trigger of t can fire from a window
// tap: manaTaps (a Taps trigger and a mana tap in the table) narrowed to the
// watchers the walk does not model (atkTaxSelfTaps). A self watcher whose card
// an effect grants a mana ability (Careful Cultivation's enchanted creature) is
// tapped by the window as a granted source (atkTaxAlts), firing its trigger as
// any tap for mana does (manaTapped; CR 605.4a).
func atkTaxTapsLive(t *kop.Table, g *genPrep) bool {
	if !g.manaTaps {
		return false
	}
	for i := range t.Cards {
		c := &t.Cards[i]
		for j := range c.Abilities {
			if a := &c.Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && !atkTaxSelfTaps(c, a) {
				return true
			}
		}
	}
	return false
}
