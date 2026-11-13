package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CollectEvidence<X> as a trigger's own cost (Incinerator of the Guilty) and
// CollectEvidence<N> beside the dies trigger's own triggered-card exile
// (Lamplight Phoenix). Both ride cost_collectevidence.go's evidence stage.
//
// Announced X (gorge cumulative.go triggeredCostXAsk, costCarriesEvidenceX):
// the payer announces X over 0..min(64, the graveyard's total mana value)
// BEFORE the pay / don't-pay election (the "trigger_cost_x" ask, trig_maypayx.go's
// raTrigX); the answer folds into a literal threshold, X = 0 leaves the
// election decline-only, and the paid evidence stage is the literal-N one
// (startCostEvidence). The body reads Count$xPaid as the announced X
// (trigXBind). The cost is OpIfPaid with PayManaX and B.N PayBSubEvidenceX,
// A the cap.
//
// Triggered-card exile beside evidence (gorge cumulative.go
// triggeredCostComponentsPayable: the component reserves its candidates, then
// the evidence stage runs over the rest of the graveyard): payable while the
// triggered card is still in a graveyard and the other cards reach N; paying
// exiles the triggered card, then the evidence stage runs on what is left. The
// cost is PayExileGrave with B.N PayBSubTrigNewEvidence, A the threshold.
// Cold and out of line.

// payKindTrigNewEvidence is the pay kind (execPay's kind, pickZ) of the
// triggered-card exile paired with a CollectEvidence<A> part.
const payKindTrigNewEvidence = 20

// execTrigEvidenceX poses the X announcement: 0..the most the graveyard's
// total mana value (capped) allows. False = suspended.
//
//go:noinline
func (s *State) execTrigEvidenceX(k *prep, p uint8, skip uint16, cap int) bool {
	s.g.pickF = skip
	s.g.pickZ = payKindEvidence
	s.g.pickLeft = uint8(min(cap, trigXCap, s.graveTotalMV(k, p)))
	s.g.pickP = p
	s.res.ask, s.res.askP = raTrigX, p
	return false
}

// applyAskEvidenceX folds the announced X in and poses the pay / don't-pay
// election: a zero threshold is decline-only (gorge's component gate refuses
// need <= 0).
//
//go:noinline
func (s *State) applyAskEvidenceX(k *prep, a Action) bool {
	s.g.pickLeft = a.Mode()
	if a.Mode() == 0 || s.graveTotalMV(k, s.g.pickP) < int(a.Mode()) {
		s.g.pickF |= pfNoPay
	}
	s.res.ask, s.res.askP = raGenPay, s.g.pickP
	return true
}

// trigNewEvidencePayable: the triggered card is still in a graveyard and the
// rest of the payer's graveyard reaches need (gorge reserves the triggered
// card's pick before the evidence stage).
func (s *State) trigNewEvidencePayable(k *prep, p uint8, need int) bool {
	o := s.trigNewInGrave()
	if o == NoObj {
		return false
	}
	pl := &s.pl[SeatIx(p)]
	rest := s.graveTotalMV(k, p)
	for i := 0; i < int(pl.graveN); i++ {
		if pl.grave[i] == o {
			rest -= s.graveMV(k, o)
			break
		}
	}
	return rest >= need
}

// startTrigNewEvidence answers "pay": the evidence stage runs over the rest
// of the graveyard with the triggered card reserved (it leaves the graveyard
// with the picks at settlement, gorge settleTriggeredMandatory: components
// first).
//
//go:noinline
func (s *State) startTrigNewEvidence(k *prep) {
	s.startCostEvidence(k)
}

// evExcl is the index of the reserved triggered card in the payer p's
// graveyard while the evidence stage of a Lamplight Phoenix cost runs, else -1.
func (s *State) evExcl(p uint8) int {
	if s.ctl == ctlBuild || s.g.pickZ != payKindTrigNewEvidence || s.res.e.kind != seTrigger {
		// an activation's or spell's evidence pick never excludes a card: a
		// declined Lamplight Phoenix cost leaves pickZ and res.e behind
		return -1
	}
	o := s.trigNewInGrave()
	if o == NoObj {
		return -1
	}
	pl := &s.pl[SeatIx(p)]
	for i := 0; i < int(pl.graveN); i++ {
		if pl.grave[i] == o {
			return i
		}
	}
	return -1
}

// evExclMV is the reserved card's mana value (0 when none).
func (s *State) evExclMV(k *prep, p uint8) int {
	if i := s.evExcl(p); i >= 0 {
		return s.graveMV(k, s.pl[SeatIx(p)].grave[i])
	}
	return 0
}

// evCandN is how many cards of player p's graveyard the evidence may pick.
func (s *State) evCandN(p uint8) int {
	n := int(s.pl[SeatIx(p)].graveN)
	if s.evExcl(p) >= 0 {
		n--
	}
	return n
}

// evAllMask is the selection of every evidence candidate of player p.
func (s *State) evAllMask(p uint8) uint64 {
	m := uint64(1)<<s.pl[SeatIx(p)].graveN - 1
	if i := s.evExcl(p); i >= 0 {
		m &^= 1 << i
	}
	return m
}

// exileTrigNewFirst exiles the reserved triggered card ahead of the evidence
// picks and hands the stage back to the plain evidence kind.
func (s *State) exileTrigNewFirst() {
	if s.g.pickZ == payKindTrigNewEvidence {
		s.exileTrigNew()
		s.g.pickZ = payKindEvidence
	}
}

// evidenceXBind makes the announced X the resolving trigger's Count$xPaid
// (the paid evidence stage kept it in pickLeft).
//
//go:noinline
func (s *State) evidenceXBind(k *prep) {
	if in := &s.prog(k)[s.res.pc]; in.C&0xF == kop.PayManaX && in.B.N == kop.PayBSubEvidenceX {
		s.res.e.x = s.g.pickLeft
	}
}

// targetsEvidenceN is the evidence a spell being built owes when its X is
// the targets' total mana value (kop.EvidenceTargetsMV; gorge
// cast_targets.go evidenceAmount over the AllTargeted$ union), 0 for none,
// capped at a byte.
func (s *State) targetsEvidenceN(k *prep) uint8 {
	if k.gen == nil || !k.gen.anyEvTargets {
		return 0
	}
	if a := s.buildAbility(k); a == nil || a.Kind != kop.AbSpell || a.Evidence != kop.EvidenceTargetsMV {
		return 0
	}
	e := &s.bld().e
	n := 0
	for i := 0; i < int(e.nt); i++ {
		if t := e.tgt[i]; TCode(t) != NoTarget && isObjTarget(TCode(t)) {
			n += s.objTargetMV(k, Obj(t))
		}
	}
	return uint8(min(n, 255))
}

// payTargetsEvidence exiles the spell's picked graveyard cards with its other
// cost objects.
//
//go:noinline
func (s *State) payTargetsEvidence(k *prep) {
	if s.targetsEvidenceN(k) == 0 {
		return
	}
	if s.exileEvidence() > 0 && k.gen.anyExplEvTrig {
		s.fireEvidence(k, s.bld().e.ctrl) // trig_exploreevidence.go
	}
}

// spellEvidenceTargets: card cd's spell ability owes the targets' total mana
// value in evidence (kop.EvidenceTargetsMV).
func spellEvidenceTargets(cd *kop.CardDef) bool {
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbSpell && a.Evidence == kop.EvidenceTargetsMV {
			return true
		}
	}
	return false
}
