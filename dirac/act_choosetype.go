package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// a mid-resolution ChooseType (gorge effects/choose.go effChooseType)
// stores the chosen creature subtype in res.chosen. A filter that reads
// Creature.ChosenType while the source is a resolving spell (src ==
// NoSlot) reads that resolution's choice; resChosenType marks it without
// growing resCtx (bad's bits 0..MaxTargets-1 are the target-illegality
// flags).
const resChosenType uint8 = 0x80

// chosenTypeSub is the subtype a filter's ChosenType test compares: the
// source permanent's choice (an as-enters choice, or the answer a
// triggered / activated ability's ChooseType recorded on it), or, for a
// spell source, the resolution's mid-resolution ChooseType answer.
func (s *State) chosenTypeSub(src Slot) (uint8, bool) {
	if src != NoSlot {
		ch := s.perms[src].chosen
		return ch, ch != 255
	}
	if s.res.active && s.res.bad&resChosenType != 0 {
		return s.res.chosen, s.res.chosen != 255
	}
	return 0, false
}

// resolutionTypeSource is the source object a mid-resolution ChooseType
// records its answer on, and its battlefield slot when it is a permanent.
// gorge stores Object.ChosenType on the ability's source (effects/choose.go
// effChooseType). A second cast of the same card reuses the recorded type;
// each activation or trigger resolution asks again (CR 608.2). A source that
// already left the battlefield has no slot here.
func (s *State) resolutionTypeSource() (Obj, Slot) {
	if !s.res.active || s.res.e.obj == NoObj {
		return NoObj, NoSlot
	}
	o := s.res.e.obj
	if s.objs[o].slot == NoSlot || s.objs[o].zone != kop.ZoneBattlefield {
		return o, NoSlot
	}
	return o, s.objs[o].slot
}

// an as-enters "choose a creature type" narrowed by
// ValidTypes$ (Dawn-Blessed Pennant, Eclipsed Realms: "choose Elemental,
// Elf, ..."): gorge creatureTypeOptions filters its list with
// effects.FilterTypeWords. kgen marks the enters program's OpChooseType
// with B = 1 and the card carries the allowed vocabulary indexes
// (kop.CardDef.TypeValid). The oracle maps a type option by its label, so
// the kernel's vocabulary order matches gorge's owned-types-first order.

// DebugChooseTypeAllVocab is the mutation switch: the ValidTypes$ filter
// is skipped and the whole vocabulary is offered (TestChoosetypeMutation).
var DebugChooseTypeAllVocab bool

// typeAskCard is the card whose as-enters type ask is being posed: the
// pre-entry ask's object, else the queued ask's permanent (colorExclude's
// lookup).
func (s *State) typeAskCard() CardID {
	switch {
	case s.preAskPosed() || s.res.ask == raGenTokChoice || s.res.ask == raGenRetChoice:
		return s.preAskCard()
	case s.g.nask > 0:
		return s.cardAt(s.g.ask[0].sl)
	}
	return NoCard
}

// typeOptions fills an as-enters creature-type ask narrowed by ValidTypes$;
// false when the entering card's enters program has no such choice (the
// caller offers the whole vocabulary).
//
//go:noinline
func (s *State) typeOptions(k *prep, d *Decision) bool {
	c := s.typeAskCard()
	if c == NoCard || DebugChooseTypeAllVocab {
		return false
	}
	cd := &k.t.Cards[c]
	if len(cd.TypeValid) == 0 {
		return false
	}
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplEntersProgram {
			continue
		}
		for j := range a.Prog {
			if in := &a.Prog[j]; in.Op == kop.OpChooseType && in.B.N != 0 {
				for _, v := range cd.TypeValid {
					d.add(MakeAction(AkChoose, 0, uint8(v>>8), uint8(v)))
				}
				return true
			}
		}
	}
	return false
}

// prepChooseType sets the prep gates of this file.
func prepChooseType(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		c := &t.Cards[i]
		if len(c.TypeValid) != 0 {
			g.anyTypeValid = true
		}
		for j := range c.Abilities {
			a := &c.Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.AddChosen {
				g.anyAddChosen = true
			}
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigSpellCast && filterChosenType(t, a.Trig.Filter) {
				g.chosenCardTgt = true // Chronicle of Victory's "a spell of the chosen type"
			}
			for ti := range a.Targets {
				if ts := &a.Targets[ti]; ts.Kind == kop.TgtCard && filterChosenType(t, ts.Filter) {
					g.chosenCardTgt = true
				}
			}
		}
	}
}

// AddType$ ChosenType (Adaptive Automaton, Roaming Throne, Lifecraft
// Engine): gorge staticContinuous resolves ChosenType against the static's
// host's recorded choice on every scan (static_layerwalk.go resolveChosenTypes); a
// host with no choice grants nothing. The kernel's choice is perm.chosen,
// a matchup subtype index or subOutside (a type no card of the matchup
// has or names). A grant of an outside type is not a SubMask bit: the
// only reader that can ask for it is a ChosenType filter whose own source
// chose an outside type, and that is exact when the grant is the filter's
// own source's (the Automaton's lord reads the Automaton's choice). Two
// DIFFERENT sources that both chose outside types can't be told apart
// (the kernel keeps no vocabulary index): that one comparison is loud
// (StallBoard), never a guess.

// chosenGrant is the subtype an AddChosen static a of the permanent at src
// grants: perm.chosen, false when there is no choice.
func (s *State) chosenGrant(a *kop.Ability, src Slot) (uint8, bool) {
	if !a.Static.AddChosen || src == NoSlot {
		return 0, false
	}
	ch := s.perms[src].chosen
	return ch, ch != 255
}

// chosenGrants reports whether a (at src) grants matchup subtype i.
func (s *State) chosenGrants(k *prep, a *kop.Ability, src Slot, i uint8) bool {
	ch, ok := s.chosenGrant(a, src)
	return ok && ch == i && !outsideSub(k, ch)
}

// chosenGrantMask is add plus a's granted chosen subtype (a matchup one).
func (s *State) chosenGrantMask(k *prep, a *kop.Ability, src Slot, add kop.SubMask) kop.SubMask {
	if ch, ok := s.chosenGrant(a, src); ok && !outsideSub(k, ch) && int(ch) < kop.MaxSubtypes {
		add = add.With(int(ch))
	}
	return add
}

// lfxChosen applies a walked AddChosen static's grant to c: a matchup
// subtype bit, or an outside-type grant (c.outs) naming its type
// (outsideSub code).
func lfxChosen(k *prep, e *lfx, c *dchars) {
	switch ch := e.ch; {
	case ch == 255 || e.src == NoSlot:
	case outsideSub(k, ch):
		g := outGrant{code: ch, src: uint8(e.src) + 1}
		for i := uint8(0); i < c.nOuts && i < outsMax; i++ {
			if c.outs[i] == g {
				return // the same source's same grant again
			}
		}
		if c.nOuts == outsOver {
			return
		}
		if c.nOuts >= outsMax {
			c.nOuts = outsOver
			return
		}
		c.outs[c.nOuts] = g
		c.nOuts++
	case int(ch) < kop.MaxSubtypes:
		c.subs = c.subs.With(int(ch))
	}
}

// curOutsideHas reports whether the walked characteristics cur have the
// outside type src chose (src's perm.chosen is outsideSub): src's own
// grant on cur, a known source of the same type, or a different type.
func (s *State) curOutsideHas(k *prep, cur *dchars, src Slot) bool {
	if !k.gen.anyAddChosen || cur.nOuts == 0 || src == NoSlot {
		return false
	}
	if cur.nOuts == outsOver {
		s.capFull(StallOutsideType) // more outside grants than the walk holds
		return false
	}
	own := uint8(src) + 1
	for i := uint8(0); i < cur.nOuts; i++ {
		if cur.outs[i].src == own {
			return true // src's own grant is on cur
		}
	}
	ch := s.perms[src].chosen
	for i := uint8(0); i < cur.nOuts; i++ {
		if cur.outs[i].code != ch {
			continue
		}
		if !outsideAliased(k, ch) {
			return true
		}
		s.capFull(StallOutsideType) // another source's aliased outside type: equal or not is unknown
		return false
	}
	return false // every grant's code differs: different types (CR 205.3m)
}

// outsideChosenHas is curOutsideHas off the walk: the AddChosen statics
// whose source chose an outside type and whose filter takes sl.
//
//go:noinline
func (s *State) outsideChosenHas(k *prep, sl, src Slot) bool {
	if k.gen == nil || !k.gen.anyAddChosen || src == NoSlot {
		return false
	}
	if k.gen.fxNeed != 0 && s.walkSlot(k, sl) {
		c := s.chars(k, sl, upTo4)
		return s.curOutsideHas(k, &c, src)
	}
	found, other := false, false
	s.eachStaticC(k, sclsSub, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous {
			return true
		}
		if ch, ok := s.chosenGrant(a, st.sl); !ok || !outsideSub(k, ch) {
			return true
		}
		if _, outside := s.selfGrantKept(k, sl, st.sl); !outside {
			return true // a newer setter on sl overwrote its own grant (static_selfgrant.go)
		}
		if s.statCond(k, st) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerSub) {
			if st.sl == src {
				found = true
				return false
			}
			if sc := s.perms[src].chosen; s.perms[st.sl].chosen == sc {
				if !outsideAliased(k, sc) {
					found = true
					return false
				}
				other = true
			}
		}
		return true
	})
	if !found && other {
		s.capFull(StallOutsideType) // another source's outside type: equal or not is unknown
	}
	return found
}

// A card target or a spell-cast trigger whose filter reads the chosen type
// (Dawn-Blessed Pennant's "return target card of the chosen type from your
// graveyard", Chronicle of Victory's "whenever you cast a spell of the
// chosen type"): gorge's ChosenType predicate reads the source's recorded
// choice (effects/filter_type.go typePredicateChosenType, sourceChosen).
// matchCardGen sees only a resolution's mid-resolution answer, so targetOK
// and castTrigGen pass the source here.

// filterChosenType reports whether some alternative of filter f reads the
// source's chosen type or colour.
func filterChosenType(t *kop.Table, f kop.FilterID) bool {
	for n := 0; f != 0 && int(f) < len(t.Filters) && n < 64; n++ {
		if t.Filters[f].Flags2&(kop.F2ChosenType|kop.F2ChosenColor) != 0 {
			return true
		}
		f = t.Filters[f].Or
	}
	return false
}

// matchCardTgt is matchCardGen for a card target of the ability whose
// source permanent is at src (NoSlot once it left; self is its object), or
// for a spell-cast trigger of the listener at src.
func (s *State) matchCardTgt(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, o Obj, src Slot, self Obj) bool {
	if !k.gen.chosenCardTgt {
		return s.matchCardGen(k, f, c, owner, ctrl, o)
	}
	return s.matchCardSrc(k, f, c, owner, ctrl, o, src, self)
}

// srcChosen is the chosen type of the ability source: the permanent's, or
// once it left (a Sac<1/CARDNAME> cost) the resolving entry's last known
// information of it (gorge sourceChosen reads SourceChosenType); 255 none.
func (s *State) srcChosen(src Slot, self Obj) uint8 {
	if src != NoSlot {
		return s.perms[src].chosen
	}
	if s.res.active && s.res.e.obj == self && self != NoObj && s.res.e.li != 0 {
		if r := s.lk(s.res.e.li); r.obj == self {
			return r.chosen
		}
	}
	return 255
}

// matchCardSrc reads each ChosenType alternative against the source's
// choice (a printed changeling has every type), the rest through
// matchCardOne. A resolution's own ChooseType answer keeps matchCardGen.
//
//go:noinline
func (s *State) matchCardSrc(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, o Obj, src Slot, self Obj) bool {
	if f == 0 {
		return true
	}
	ch := s.srcChosen(src, self)
	if ch == 255 && src == NoSlot || s.res.active && s.res.bad&(resChosenType|resChosenCard) != 0 {
		return s.matchCardGen(k, f, c, owner, ctrl, o)
	}
	for f != 0 {
		fl := &k.t.Filters[f]
		if fl.Flags2&kop.F2ChosenType != 0 {
			cd := &k.t.Cards[c]
			has := ch != 255 && int(ch) < len(k.t.Subtypes) && cd.Subtypes.Has(int(ch))
			if outsideSub(k, ch) {
				has = cd.AllCreatureTypes
			}
			cp := *fl
			cp.Flags2 &^= kop.F2ChosenType
			if has && s.matchCardOne(k, &cp, c, owner, ctrl, o) {
				return true
			}
		} else if fl.Flags2&kop.F2ChosenColor != 0 {
			// Diamond Mare's "a spell of the chosen colour": the source's choice
			// is a colour index, read against the card's colours
			cp := *fl
			cp.Flags2 &^= kop.F2ChosenColor
			if ch < 5 {
				bit := kop.ColW << ch
				if cp.Colors != 0 {
					cp.Colors &= bit
				} else {
					cp.Colors = bit
				}
				if cp.Colors != 0 && s.matchCardOne(k, &cp, c, owner, ctrl, o) {
					return true
				}
			}
		} else if s.matchCardOne(k, fl, c, owner, ctrl, o) {
			return true
		}
		f = fl.Or
	}
	return false
}

// the Pauper keyword-mechanics / mana-misc / singletons
// carriers of the MageZero sample decks (cold code, out of line).

// A card-type ChooseType (Winding Way; gorge effChooseType over
// TypeChoiceLabels("Card", ValidTypes$)): OpChooseType with C = the offered
// card types. gorge poses one "type" option per type in its vocabulary's
// alphabetical order; the oracle canonicalises a label outside the
// creature-type vocabulary as 0x8000 | FNV-1a(label) & 0x7fff in
// (target, mode), so each option carries that value. The answer is the
// resolution's chosen card type: res.chosen = its index in cardTypeChoice,
// res.bad's resChosenType | resChosenCard, which a Card.ChosenType filter
// (F2ChosenType) reads as "has that card type".
const resChosenCard uint8 = 0x40

// cardTypeChoice is gorge's chooseCardTypes vocabulary (effects
// type_choices.go) with each type's kop bit.
var cardTypeChoice = [...]struct {
	name string
	bit  kop.TypeMask
}{
	{"Artifact", kop.TArtifact}, {"Battle", kop.TBattle}, {"Creature", kop.TCreature},
	{"Enchantment", kop.TEnchantment}, {"Instant", kop.TInstant}, {"Kindred", kop.TKindred},
	{"Land", kop.TLand}, {"Planeswalker", kop.TPlaneswalker}, {"Sorcery", kop.TSorcery},
}

// cardTypeLabelH is each cardTypeChoice label's canonical option value.
var cardTypeLabelH = func() (h [len(cardTypeChoice)]uint16) {
	for i, c := range cardTypeChoice {
		v := uint32(2166136261)
		for j := 0; j < len(c.name); j++ {
			v ^= uint32(c.name[j])
			v *= 16777619
		}
		h[i] = 0x8000 | uint16(v&0x7fff)
	}
	return
}()

// chooseCardTypeOptions adds the card-type ask's options (mask m).
//
//go:noinline
func chooseCardTypeOptions(d *Decision, m kop.TypeMask) {
	for i, c := range cardTypeChoice {
		if m&c.bit != 0 {
			d.add(MakeAction(AkChoose, 0, uint8(cardTypeLabelH[i]>>8), uint8(cardTypeLabelH[i])))
		}
	}
}

// cardTypeAnswer is the cardTypeChoice index an answer names.
func cardTypeAnswer(a Action) uint8 {
	v := uint16(a.Target())<<8 | uint16(a.Mode())
	for i := range cardTypeLabelH {
		if cardTypeLabelH[i] == v {
			return uint8(i)
		}
	}
	return 255
}

// chosenCardTypeHas reports whether card types t include the resolution's
// chosen card type (resChosenCard).
func (s *State) chosenCardTypeHas(t kop.TypeMask) bool {
	ch := s.res.chosen
	return int(ch) < len(cardTypeChoice) && t&cardTypeChoice[ch].bit != 0
}
