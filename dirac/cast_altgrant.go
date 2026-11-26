package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Granted mana alternative costs: a battlefield StatAlternativeCost static
// flagged kop.StAltGrant (Fist of Suns, Jodah and Leyline of Mutation's
// "you may pay {W}{U}{B}{R}{G} rather than pay the mana cost for spells that
// you cast"; Rooftop Storm's {0} for Zombie creature spells; Runeforge
// Champion's {1} for Rune spells). gorge alternativeCosts lists one entry per
// grant that applies, battlefield statics first, then the card's own face
// statics (Land Grant's, Blasphemous Edict's), and the cast option carries the
// entry's 1-based position as AltCostIndex. The kernel's action carries that
// index in its target byte, castAltIdxTgt + the index: grant i of the n that
// apply is index i+1. The card's own static (prepAltFree) keeps castAltTgt
// (cast_altfree.go); a card with both stalls (offerAltGrantHand).

// castAltIdxTgt in an AkCast's target byte (mode 0, from the hand) is a cast
// for the alternative cost at AltCostIndex tgt-castAltIdxTgt (canon
// translate.go reads it back as the index).
const castAltIdxTgt = 0x10

// FeatCastAltIdxTgt and FeatMaxAltGrants are castAltIdxTgt and maxAltGrants
// for canon/translate.go.
const (
	FeatCastAltIdxTgt = castAltIdxTgt
	FeatMaxAltGrants  = maxAltGrants
)

// maxAltGrants bounds the grants a cast lists: AltCostIndex rides a nibble of
// the canonical cast mode, and 0x10+index must stay a target byte no other
// cast uses. More grants than that cannot be indexed: the game stalls loudly
// (offerAltGrantHand) rather than list a truncated set.
const maxAltGrants = 7

// altGrantRef is one applicable grant: the granting card and the index of its
// StatAlternativeCost ability.
type altGrantRef struct {
	card CardID
	ab   uint8
}

// altGrants lists the battlefield grants that let p cast hand card o (card c)
// for a mana alternative cost, in the order gorge's alternativeCosts lists
// them: p's battlefield in entry order (battlefieldOrder), a permanent's
// grants in script order. It returns how many grants apply, which may exceed
// maxAltGrants (out holds the first maxAltGrants). A grant is read relative to its controller
// (ValidPlayer$ You), so only p's own permanents grant, and a permanent that
// lost its abilities grants nothing.
func (s *State) altGrants(k *prep, p uint8, o Obj, c CardID, out *[maxAltGrants]altGrantRef) int {
	var hit slotMask
	for m := s.statM.and(s.ctrlM[p]); m.any(); m = m.rest() {
		src := m.first()
		if s.perms[src].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || s.ovrLoses(k, src) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			if s.altGrantApplies(k, p, src, &cd.Abilities[i], c) {
				hit.set(src)
				break
			}
		}
	}
	if !hit.any() {
		return 0
	}
	var ord [MaxPerm]Slot
	n := 0
	for i, nn := 0, s.battlefieldOrder(p, hit, ord[:], 0); i < nn; i++ {
		src := ord[i]
		cd := &k.t.Cards[s.cardAt(src)]
		for j := range cd.Abilities {
			if j < 255 && s.altGrantApplies(k, p, src, &cd.Abilities[j], c) {
				if n < maxAltGrants {
					out[n] = altGrantRef{card: s.cardAt(src), ab: uint8(j)}
				}
				n++
			}
		}
	}
	return n
}

// altGrantApplies reports whether ability a of p's permanent src is a grant of
// a mana alternative cost that applies to the cast of card c: the ValidCard$
// filter (read with p as "you"; none matches any spell) and the static's
// condition (read with src as its source and p the controller).
func (s *State) altGrantApplies(k *prep, p uint8, src Slot, a *kop.Ability, c CardID) bool {
	if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatAlternativeCost || a.Static.StFlags&kop.StAltGrant == 0 {
		return false
	}
	if a.Static.Filter != 0 && !s.matchCard(k, a.Static.Filter, c, p, p) {
		return false
	}
	return a.Static.Cond2 == 0 || s.condEval(k, a.Static.Cond2, evalCtx{src: s.perms[src].obj, srcSl: src, ctrl: p})
}

func (s *State) altGrantAb(k *prep, g altGrantRef) *kop.Ability {
	return &k.t.Cards[g.card].Abilities[g.ab]
}

// offerAltGrantHand is offerAltGrants behind the gate that costs a deck with
// no grant (or a board with no static) one inlined test.
func (s *State) offerAltGrantHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if k.gen != nil && k.gen.anyAltGrant && s.statM.any() {
		s.offerAltGrants(k, d, p, o, c, av)
	}
}

// offerAltGrants offers hand card o's casts for the mana alternative costs
// the battlefield grants (altGrants): one cast per grant, at its AltCostIndex.
// The gates are offerAltFreeHand's: target and additional-cost feasibility,
// and the planner-surface rule (the payment planner skips AltCostIndex
// options, gorge payment_plan.go), so a priced alternative is offered only
// while the floating pool pays what the cost statics make of it. A free grant
// ({0}) still owes the increases (CR 601.2f, 118.9d: Thalia's {1}). A card
// with an either-or additional cost, or with the non-granting may-play
// alternative, shares its index space with the grants in a way the kernel
// does not name, and so does a card with its own alternative static (Land
// Grant's, Blasphemous Edict's: gorge lists it after the grants, at index n+1,
// which cast_altfree.go's offers do not shift): the game stalls loudly, as it
// does for more than maxAltGrants grants.
//
//go:noinline
func (s *State) offerAltGrants(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	var gr [maxAltGrants]altGrantRef
	n := s.altGrants(k, p, o, c, &gr)
	if n == 0 {
		return
	}
	if n > maxAltGrants || k.t.Cards[c].HasAltAdd || k.gen.altFree != nil && k.gen.altFree[c] != 0 {
		s.capFull(StallZeroAltShared)
		return
	}
	if k.gen.anyZeroAlt {
		if zn, _ := s.zeroAlts(k, p, o); zn > 0 {
			s.capFull(StallZeroAltShared)
			return
		}
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || s.castHeldOut(o) {
		return
	}
	for i := 0; i < n; i++ {
		ac := s.adjustCost(k, p, c, o, s.altGrantAb(k, gr[i]).Cost.Mana)
		if ac != (kop.ManaCost{}) && (!av.affords(&ac, pipMaskOf(&ac), 0) || !s.windowPoolPays(p, &ac, false)) {
			continue
		}
		d.add(MakeAction(AkCast, TCode(o), TCode(castAltIdxTgt+1+i), 0))
	}
}

// altTgtAbility is the alternative-cost ability the mode-0 hand cast a of
// player p names: the card's own static (castAltTgt) or the grant at its
// index; nil when the target byte names neither.
func (s *State) altTgtAbility(k *prep, p uint8, a Action) *kop.Ability {
	if k.gen == nil || a.Mode() != 0 {
		return nil
	}
	o := Obj(a.Source())
	c := s.objs[o].card
	own := func() *kop.Ability {
		if k.gen.altFree == nil || k.gen.altFree[c] == 0 {
			return nil
		}
		return &k.t.Cards[c].Abilities[k.gen.altFree[c]-1]
	}
	if a.Target() == castAltTgt {
		return own()
	}
	if !k.gen.anyAltGrant || a.Target() <= castAltIdxTgt || a.Target() > castAltIdxTgt+maxAltGrants {
		return nil
	}
	idx := int(a.Target()) - castAltIdxTgt // the AltCostIndex
	var gr [maxAltGrants]altGrantRef
	if n := s.altGrants(k, p, o, c, &gr); idx <= min(n, maxAltGrants) {
		return s.altGrantAb(k, gr[idx-1])
	}
	return nil
}

// noteAltTgtCast marks the genState the cast beginCast starts with the
// alternative-cost ability a's target byte names (legal.go's cast apply): the
// cast is an alternative cast (altCast), and a grant's price is the granting
// static's, read back by altAbOf.
func (s *State) noteAltTgtCast(k *prep, p uint8, a Action) {
	ab := s.altTgtAbility(k, p, a)
	if ab == nil {
		return
	}
	s.g.altCast = true
	s.g.altGrantAb = 0
	if a.Target() < castAltIdxTgt {
		return
	}
	o := Obj(a.Source())
	var gr [maxAltGrants]altGrantRef
	n := s.altGrants(k, p, o, s.objs[o].card, &gr)
	if idx := int(a.Target()) - castAltIdxTgt; idx <= min(n, maxAltGrants) {
		s.g.altGrantC, s.g.altGrantAb = gr[idx-1].card, gr[idx-1].ab+1
	}
}

// altAbOf is the alternative-cost ability the cast being built pays: the
// grant noteAltTgtCast recorded, else the card's own static.
func (s *State) altAbOf(k *prep) *kop.Ability {
	if g := s.g.altGrantAb; g != 0 {
		return &k.t.Cards[s.g.altGrantC].Abilities[g-1]
	}
	e := &s.bld().e
	return &k.t.Cards[e.card].Abilities[k.gen.altFree[e.card]-1]
}
