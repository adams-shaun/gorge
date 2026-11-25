package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// sbaMasks is what one pass's board scan found.
type sbaMasks struct {
	die, gy, pwz, aur, anni slotMask
	umbra                   umbraOrder // the pass's umbra-spared hosts (kw_umbraarmor.go)
}

// sbaScan is sba's per-permanent scan of one pass: the creatures that die,
// everything that goes to the graveyard, the planeswalkers and Auras among
// them and the -1/-1 annihilations. busy reports that it read state the
// clean-scan key (static_sbaskip.go) does not hold.
//
// r is the caller's: the scan resets it in place (the umbra order's pairs are
// read only up to its n), sparing each pass a zeroing and two copies of the
// umbra order's 384-byte pair array.
func (s *State) sbaScan(k *prep, r *sbaMasks) (busy bool) {
	var die, gy, pwz, aur, anni slotMask
	// statics in force but none that can lower a toughness (kprio2)
	ptQuiet := s.statM.any() && s.sbaPTQuiet(k)
	umbra := &r.umbra // the pass's umbra-spared hosts (kw_umbraarmor.go)
	umbra.n, umbra.victim = 0, slotMask{}
	m := s.occ
	for m.any() {
		sl := m.first()
		m = m.rest()
		p := &s.perms[sl]
		if p.p1 > 0 && p.ctrLow(int(kop.CtrM1M1)) > 0 {
			anni = anni.with(sl) // CR 704.5q, after the departures: a casualty leaves with its counters (CR 704.8)
		}
		t := s.typesAt(k, sl)
		if t&kop.TCreature != 0 {
			// toughness can only be <= damage or <= 0 when something
			// lowered it or damage is marked
			// A timestamped override record can set the base P/T (a
			// value-driven Animate: HandSize can be 0), so the printed
			// toughness is not enough for the no-damage shortcut.
			if p.damage == 0 && p.flags&pfDeathtouched == 0 && p.ctrLow(int(kop.CtrM1M1)) == 0 && p.eotT >= 0 && !s.sbaPTMoved(k, sl) && (!s.statM.any() || ptQuiet && p.ovr.walk&walkOn == 0) &&
				p.ovr.flags&(ovSetPT|ovSetPTEOT|ovCopy) == 0 && k.cards[p.card].tough > 0 {
				goto noncreature
			}
			pw, tough := s.pt(k, sl)
			if tough <= 0 {
				die = die.with(sl)
				gy = gy.with(sl)
				continue
			}
			lethal := tough
			if k.gen != nil && k.gen.uses.has(useLethalPow) && p.damage > 0 {
				// LethalDamageByPower$ (static_lethalbypower.go): the
				// threshold reads power, which the clean-scan key was not
				// built around: never a cached clean scan
				busy = true
				lethal = s.lethalThreshold(k, sl, pw, tough)
			}
			// The deathtouch mark alone is lethal (CR 704.5h): a wither or
			// infect hit marks no damage (gorge destroyLethalDamage). No
			// marked damage is never lethal, whatever the threshold.
			if (p.damage > 0 && int(p.damage) >= lethal || p.flags&pfDeathtouched != 0) &&
				s.kw(k, sl)&kop.KwIndestructible == 0 {
				busy = true // destroyLine and the regeneration read the replacements: never a cached clean scan
				if r := s.destroyLine(k, sl, true); r != drNone {
					if r == drHeal {
						s.cfgx |= cfgxDestroyHealPass // CR 614.1: the counters leave, so a 0-toughness creature still dies next pass
					}
					continue
				}
				if s.canRegen(sl) {
					s.regenerate(sl)
					continue
				}
				if s.sbaSpare(k, sl, &die, umbra) {
					continue // the shield counter or umbra armor took the destruction (kw_umbraarmor.go)
				}
				die = die.with(sl)
			}
		}
	noncreature:
		// one test for the common permanent: none of the three checks
		// below applies without an Aura, a planeswalker or an attachment
		if t&(kop.TAura|kop.TPlaneswalker) == 0 && p.attached == NoSlot {
			continue
		}
		if t&kop.TAura != 0 {
			busy = true // legality reads the host's derived keywords, the Enchant filter and the enchanted seat's loss, none of them in the scan key
		}
		if t&kop.TAura != 0 && s.atchSeat(k, sl) != 0 {
			// a player-attached Aura (the Enchant:Player curses): gorge
			// attachmentSBAs' HasAttachedPlayer arm (engine/rules/attach.go:128)
			// -- a lost seat cannot stay enchanted (CR 800.4a) and an
			// Aura whose own Enchant spec no longer admits the seat
			// (a control change moved a YouCtrl line) goes to the
			// graveyard like any illegal attachment. The player-keyword
			// protection grant gorge's playerProtectedFrom reads has no
			// kernel arm: no supported table grants one beside a curse.
			if !s.playerAuraLegal(k, sl) {
				gy = gy.with(sl)
				aur = aur.with(sl)
			}
		} else if t&kop.TAura != 0 && (p.attached == NoSlot || !s.enchantLegal(k, sl)) && !s.sbaBestowDetach(k, sl) {
			// unattached, or attached to something it can no longer
			// legally enchant (CR 704.5m: Imprisoned in the Moon makes
			// Witness Protection's creature a land); a bestowed Aura
			// detaches and stays instead (CR 702.103f)
			gy = gy.with(sl)
			aur = aur.with(sl)
		}
		if t&kop.TAura == 0 && p.attached != NoSlot && s.occ.has(p.attached) && s.typesAt(k, p.attached)&kop.TCreature == 0 {
			busy = true // the legend ask decides it (legendOwed), which the scan key does not hold
		}
		if t&kop.TAura == 0 && p.attached != NoSlot && s.occ.has(p.attached) && s.typesAt(k, p.attached)&kop.TCreature == 0 && !s.legendOwed(k) {
			// an Equipment whose creature is no longer one (Imprisoned
			// in the Moon) becomes unattached and stays (CR 704.5n;
			// gorge attachmentSBAs; pair 9856). Under a legend ask it
			// stays attached: gorge parks the pass's batch with the ask
			// (CR 704.3), and the pass after the answer detaches it
			// (Crystallized Serah transformed beside another)
			h := p.attached
			s.touchSl(h)
			if k.gen != nil && k.gen.anyUnattach {
				// gorge's Equipment-bearer arm publishes the detach with
				// the bearer (CR 704.5n, attachmentSBAs)
				s.fireUnattached(k, sl, h)
			}
			p.attached = NoSlot
			s.touchSl(sl)
			if k.gen != nil && k.gen.anyAegis {
				s.aegisUnlink(k, sl, h, false) // zone_exilethenattach.go
			}
		}
		if t&kop.TPlaneswalker != 0 && p.ctrLow(int(kop.CtrLoyalty)) == 0 {
			gy = gy.with(sl)
			pwz = pwz.with(sl)
		}
	}
	r.die, r.gy, r.pwz, r.aur, r.anni = die, gy, pwz, aur, anni
	return busy
}
