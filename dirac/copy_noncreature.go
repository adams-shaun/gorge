package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The as-enters copy of a permanent that is not a creature or an artifact
// (CR 707.2, 614.12): the permanent enters with the copied card's
// characteristics, so what a card of that type does as it enters happens to
// the copy.
//
//   - A planeswalker enters with loyalty counters equal to the copied card's
//     printed loyalty (CR 306.5b; gorge events.EntryCounterGrantsForFace reads
//     the template's copiable face): copyEnterTypes.
//   - An Aura enters attached (CR 303.4f): the copy was not cast as an Aura
//     spell, so its controller chooses what it enchants as it enters, an ask
//     posed with the copy election (raised on the stack, after the clone
//     answer, like the template's own choice: gorge's second as-enters
//     election, answered "enchant"; rules/aura_entry.go auraEntryGate).
//     One legal host attaches without an ask, none leaves the copy
//     unattached for the state-based action (CR 303.4g, 704.5m: a stack
//     object has no zone to stay in). The hosts are walked in gorge's order,
//     seat by seat in battlefield order (auraEnchantCandidates), by the
//     enchant ability of the copied card and its colours (protection, CR
//     702.16c): cloneAuraHosts.
//   - A Saga enters with a lore counter (CR 702.151a) and a Battle with its
//     printed defense counters (CR 310.4), the same entry grants zones.go
//     places on a printed card. The Battle's protector is NOT granted: gorge's
//     applySiegeProtector runs before the copy face is installed and records
//     none for a copied Battle, so the kernel matches gorge and the gorge fix
//     is a separate ticket (CR 310.10 wants one).
//   - Its name is the copy's NewName$ exception when it has one (CR 707.9b):
//     copyExcName, read by FeatNameCard and the layer walk.
//
// Not modelled: an Aura that enchants a player or a graveyard card (the
// kernel stalls, StallCopyAllAsk), and the type-conditional counter rider
// (Spark Double's additional +1/+1 or loyalty counter; kgen refuses that
// chain shape rather than lowering it without the counters).

// DebugCloneNoLoyalty enters a planeswalker copy with no loyalty counters (the
// mutation test TestCloneNonCreatureLoyaltyMutation): the state-based action
// puts it into the graveyard, where gorge keeps it.
var DebugCloneNoLoyalty bool

// cloneAuraHosts lists the permanents an Aura of card tc under ctrl's control
// can enchant, in gorge's candidate order (seat order, then each battlefield's
// entry order). ok is false for an Aura whose enchant ability is not a
// permanent's (Enchant player, a graveyard card) or when more than MaxOptions
// hosts are legal: the callers stall.
func (s *State) cloneAuraHosts(k *prep, tc CardID, ctrl uint8, out *[MaxOptions]Slot) (n int, ok bool) {
	if k.t.Cards[tc].Enchant.Kind != kop.TgtObject {
		return 0, false
	}
	var ord [MaxPerm]Slot
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		no := s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), ord[:], 0)
		for _, sl := range ord[:no] {
			if s.objs[s.perms[sl].obj].of&ofPending != 0 || !s.enchantHostOK(k, tc, ctrl, sl) {
				continue
			}
			if n == MaxOptions {
				return n, false // gorge's list is unbounded: a truncated one would diverge, so the callers stall
			}
			out[n] = sl
			n++
		}
	}
	return n, true
}

// cloneAuraAsk poses the enchant ask of an Aura template tc after the clone
// answer (cloneTemplateAsk): two or more legal hosts. tplChoice: the template
// also opens with a choice of its own, whose order against the enchant ask is
// not modelled. asked reports that the host ask was posed; ok is false when the
// call stalled (an Enchant that is not a permanent's, or two or more hosts
// beside the template's own choice).
//
//go:noinline
func (s *State) cloneAuraAsk(k *prep, tc CardID, tplChoice bool) (asked, ok bool) {
	var buf [MaxOptions]Slot
	n, ok := s.cloneAuraHosts(k, tc, s.stack[s.sp-1].ctrl, &buf)
	if !ok || n > 1 && tplChoice {
		s.stall(StallCopyAllAsk)
		return false, false
	}
	if n < 2 {
		return false, true // zero or one host: no ask, the template's own choice (if any) is asked instead
	}
	p := &s.g.pre
	p.ask2, p.kind, p.state = akEnchant, akEnchant, 1
	return true, true
}

// cloneAuraOptions fills the enchant ask of the Aura copy on the stack: the
// hosts of the copied card (the clone answer in s.g.pre.val names the
// template; preAskCard reads it when ask2 is set), as ChGenEnchant's objects.
func (s *State) cloneAuraOptions(k *prep, d *Decision) {
	d.Sub = ChGenEnchant
	var buf [MaxOptions]Slot
	n, _ := s.cloneAuraHosts(k, s.preAskCard(), s.stack[s.sp-1].ctrl, &buf)
	for i := 0; i < n; i++ {
		d.add(MakeAction(AkChoose, TCode(s.perms[buf[i]].obj), 0, 0))
	}
}

// cloneAuraAnswer is the answer's host slot, held in s.g.pre.val2.
func (s *State) cloneAuraAnswer(a Action) uint8 { return uint8(s.objs[a.Source()].slot) }

// copyEnterTypes is becomeCopyEnter's part for the copied card c entering at
// sl: the planeswalker's loyalty, the Saga's lore counter, the Battle's
// defense counters, and the Aura's host. host is the answered enchant ask
// (NoSlot when none was posed).
//
//go:noinline
func (s *State) copyEnterTypes(k *prep, sl Slot, c CardID, host Slot) {
	pc := &k.cards[c]
	ty := pc.types
	p := &s.perms[sl]
	if ty&kop.TPlaneswalker != 0 && !DebugCloneNoLoyalty {
		if n := int(k.t.Cards[c].Loyalty); n > 0 {
			s.placeCounters(k, sl, int(kop.CtrLoyalty), n, p.ctrl, ctrPutBy)
		}
	}
	if pc.sagaN != 0 {
		s.placeCounters(k, sl, int(kop.CtrLore), 1, p.ctrl, ctrPutBy) // CR 702.151a
	}
	if ty&kop.TBattle != 0 {
		s.enterDefense(k, sl, c) // CR 310.4; the protector is gorge's bug (see the file comment)
	}
	if ty&kop.TAura == 0 {
		return
	}
	if host == NoSlot {
		var buf [MaxOptions]Slot
		n, ok := s.cloneAuraHosts(k, c, p.ctrl, &buf)
		if !ok {
			s.stall(StallCopyAllAsk)
			return
		}
		if n != 1 {
			return // none: the copy stays unattached (CR 303.4g); several were asked before the move
		}
		host = buf[0]
	}
	if s.occ.has(host) {
		s.attachQuiet(k, sl, host)
		s.touchSl(host)
	}
}

// cloneReturnAuraPark poses the enchant ask of an Aura template an
// as-enters clone elected on its way back to the battlefield (a blink, a
// reanimation): the copy was not cast as an Aura spell, so the CR 303.4f host
// is chosen before the move, as returnAura asks it for a printed Aura. The
// answer is held in s.g.pre.val2/ask2 and consumed by becomeCopyEnter. False
// when the elected template is not an Aura or has fewer than two legal hosts
// (one attaches without an ask, none leaves it unattached for the SBA).
//
//go:noinline
func (s *State) cloneReturnAuraPark(k *prep) bool {
	p := &s.g.pre
	if p.kind != akClone || p.ask2 == akEnchant {
		return false
	}
	t := Obj(p.val)
	if t == NoObj || s.objs[t].zone != kop.ZoneBattlefield {
		return false
	}
	tc := s.cardAt(s.objs[t].slot)
	if k.cards[tc].types&kop.TAura == 0 {
		return false
	}
	ctrl := s.res.askP
	var buf [MaxOptions]Slot
	n, ok := s.cloneAuraHosts(k, tc, ctrl, &buf)
	if !ok || n > 1 && s.enterChoiceKind(k, tc) != 0 {
		s.stall(StallCopyAllAsk) // Enchant player/card, or the host/template-choice order (cloneAuraAsk)
		return true
	}
	if n < 2 {
		return false
	}
	p.ask2, p.kind, p.state = akEnchant, akEnchant, preReturnCloneAura
	s.res.ask, s.res.askP = raGenRetChoice, ctrl
	return true
}

// cloneTokenAuraPark is cloneReturnAuraPark for a token that elected an Aura
// template as it entered (raGenTokChoice): the host ask reuses the copy ask's
// enchant arm (akEnchant with ask2), whose answer enters the parked token
// attached.
//
//go:noinline
func (s *State) cloneTokenAuraPark(k *prep) bool {
	t := Obj(s.g.pre.val)
	if t == NoObj || s.objs[t].zone != kop.ZoneBattlefield {
		return false
	}
	tc := s.cardAt(s.objs[t].slot)
	if k.cards[tc].types&kop.TAura == 0 {
		return false
	}
	ctrl := s.res.askP
	var buf [MaxOptions]Slot
	n, ok := s.cloneAuraHosts(k, tc, ctrl, &buf)
	if !ok || n > 1 && s.enterChoiceKind(k, tc) != 0 {
		s.stall(StallCopyAllAsk)
		return true
	}
	if n < 2 {
		return false
	}
	s.g.pre.ask2, s.g.pre.kind, s.g.pre.state = akEnchant, akEnchant, preTokenAura
	return true
}

// copyExcName is the name card (index+1, 0: none) the copy exceptions of the
// permanent at sl name (CR 707.9b, NewName$): the last one applied.
//
//go:noinline
func (s *State) copyExcName(k *prep, sl Slot) (nc uint16) {
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc {
			if o := &k.t.Overrides[r.r.idx-1]; o.NameCard != 0 {
				nc = o.NameCard
			}
		}
	}
	return nc
}
