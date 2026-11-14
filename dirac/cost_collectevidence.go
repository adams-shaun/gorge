package dirac

import (
	"math/bits"
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CollectEvidence<N> as an activated ability's cost and as
// a printed trigger's own optional cost (CR 701.59a, 602.2b, 601.2h; Forensic
// Researcher, Polygraph Orb, Izoni, Sample Collector). The ward form
// (kw_wardcost.go) shares the graveyard selection g.evSel
// (one bit per graveyard index) and the pick shape.
//
// Activation (gorge cast_targets.go evidenceAsk, posed by postTargetAsks
// after the targets and the division, before the payment): the build stage
// bsEvidence asks the activator for any number of graveyard cards, AkDone
// offered once the picked cards' total mana value reaches N (gorge refuses a
// shorter answer and poses the ask again); all cards picked ends it. Both
// engines offer the activation only while the graveyard's total mana value
// reaches N (canActivate; gorge pay.EvidencePayable, CR 602.2b). The picked
// cards are exiled with the other cost objects (payAbilityCosts).
//
// Trigger's own cost (gorge cumulative.go triggeredCostComponentsPayable,
// evidence_cost.go evidenceStage): payable while the graveyard's total mana
// value reaches N; paying exiles the whole graveyard unasked when the
// greedy minimum (the fewest cards, highest values first) is every card,
// else the same pick (raCostEvidence). Cold and out of line.

// ChCostEvidence is gorge's "evidence" KChoose: one graveyard card per pick,
// AkDone once the picked total reaches N.
const ChCostEvidence ChoiceKind = 210

// payKindEvidence is the pay kind (execPay's kind, pickZ) of a trigger's own
// CollectEvidence cost. The instruction's C nibble is full (kop.PayEvidence
// is the ward's and equals PayManaX), so the cost rides PayManaX with
// B.N = kop.PayBSubEvidence and execPay widens it to this value.
const payKindEvidence = 16

// raCostEvidence: the trigger's own cost pick (ChCostEvidence).
const raCostEvidence uint8 = 241

// DebugCostEvidenceAnyDone (TestCostevidenceMutation): Done is offered
// after any single pick, whatever the total.
var DebugCostEvidenceAnyDone bool

// CostEvidenceStats counts the evidence payments a test run made.
type CostEvidenceStats struct {
	Activations atomic.Int64 // an activation paid with evidence
	Paid        atomic.Int64 // a trigger's own evidence cost paid
	Auto        atomic.Int64 // ... of which the graveyard was exiled unasked
}

// DebugCostEvidenceStats, when set (dirac/oracle/costevidence_test.go),
// counts the payments.
var DebugCostEvidenceStats *CostEvidenceStats

// prepCostEvidence sets anyOptCost for a spell's evidence part and anyXBoundAb:
// gorge offers such an activation whatever the X leaves, so a reversed one is
// held out like a card's second no-progress abort (canActivate).
func prepCostEvidence(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		if t.Cards[i].OptEvidence != 0 {
			g.anyOptCost = true // a spell's evidence part is an OptionalCost part (cost_optionalevidence.go)
		}
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbSpell && a.Evidence == kop.EvidenceTargetsMV {
				g.anyEvTargets = true
			}
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbActivated {
				for ti := 0; ti < int(a.NTargets); ti++ {
					g.anyXBoundAb = g.anyXBoundAb || xBoundTarget(t, &a.Targets[ti])
				}
			}
		}
	}
}

// buildEvidenceN is the evidence threshold of the activation being built,
// 0 for none.
func (s *State) buildEvidenceN(k *prep) uint8 {
	if s.bld().e.kind == seSpell {
		if n := s.optEvidenceN(k); n != 0 { // cost_optionalevidence.go
			return n
		}
		return s.targetsEvidenceN(k) // cost_collectevidencex.go
	}
	if s.bld().e.kind != seAbility {
		return 0
	}
	if a := s.buildAbility(k); a != nil {
		return a.Evidence
	}
	return 0
}

// graveTotalMV is the total mana value of player p's graveyard.
func (s *State) graveTotalMV(k *prep, p uint8) int {
	pl := &s.pl[SeatIx(p)]
	tot := 0
	for i := 0; i < int(pl.graveN); i++ {
		tot += s.graveMV(k, pl.grave[i])
	}
	return tot
}

// evSelTotal is the total mana value of player p's picked graveyard cards.
func (s *State) evSelTotal(k *prep, p uint8) int {
	pl := &s.pl[SeatIx(p)]
	tot := 0
	for i := 0; i < int(pl.graveN); i++ {
		if s.g.evSel&(1<<i) != 0 {
			tot += s.graveMV(k, pl.grave[i])
		}
	}
	return tot
}

// evidenceGreedyMin is the fewest graveyard cards of player p whose total
// mana value reaches need, the highest values first (gorge
// pay.EvidenceGreedyMin; only the count matters, the tie order does not).
func (s *State) evidenceGreedyMin(k *prep, p uint8, need int) int {
	pl := &s.pl[SeatIx(p)]
	var mv [MaxDeck]uint8
	n := int(pl.graveN)
	for i := 0; i < n; i++ {
		mv[i] = uint8(min(s.graveMV(k, pl.grave[i]), 255))
	}
	ex := s.evExcl(p)
	if ex >= 0 {
		mv[ex] = mv[n-1] // the reserved triggered card is no candidate (cost_collectevidencex.go)
		n--
	}
	tot, cnt := 0, 0
	for tot < need && cnt < n {
		best := cnt
		for j := cnt + 1; j < n; j++ {
			if mv[j] > mv[best] {
				best = j
			}
		}
		mv[cnt], mv[best] = mv[best], mv[cnt]
		tot += int(mv[cnt])
		cnt++
	}
	return cnt
}

// evidenceOptions poses the pick over player p's graveyard: every unpicked
// card, and Done once the picked total reaches need.
func (s *State) evidenceOptions(k *prep, d *Decision, p uint8, need int) {
	d.Kind, d.Sub, d.Player = DkChoose, ChCostEvidence, p
	pl := &s.pl[SeatIx(p)]
	ex := s.evExcl(p)
	for i := 0; i < int(pl.graveN); i++ {
		if s.g.evSel&(1<<i) == 0 && i != ex {
			d.add(MakeAction(AkChoose, TCode(pl.grave[i]), 0, 0))
		}
	}
	if s.g.evSel != 0 && (DebugCostEvidenceAnyDone || s.evSelTotal(k, p) >= need) {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
}

// evidencePick takes one pick of player p's graveyard; it reports whether
// every card is now picked (the answer is complete without a Done).
func (s *State) evidencePick(p uint8, a Action) bool {
	pl := &s.pl[SeatIx(p)]
	o := Obj(a.Source())
	for i := 0; i < int(pl.graveN); i++ {
		if pl.grave[i] == o {
			s.evPicks()[bits.OnesCount64(s.g.evSel)] = o
			s.g.evSel |= 1 << i
			break
		}
	}
	return s.g.evSel == s.evAllMask(p)
}

// evidenceSelectAll selects player p's whole graveyard in gorge's option
// order for evidence (pay.EvidenceOrder): mana value descending, then object.
func (s *State) evidenceSelectAll(k *prep, p uint8) {
	pl := &s.pl[SeatIx(p)]
	picks := s.evPicks()
	ex, c := s.evExcl(p), 0
	for i := 0; i < int(pl.graveN); i++ {
		if i == ex {
			continue
		}
		o := pl.grave[i]
		mv := s.graveMV(k, o)
		j := c
		c++
		for ; j > 0; j-- {
			prev := s.graveMV(k, picks[j-1])
			if prev > mv || prev == mv && picks[j-1] < o {
				break
			}
			picks[j] = picks[j-1]
		}
		picks[j] = o
	}
	s.g.evSel = s.evAllMask(p)
}

// exileEvidence exiles the selected graveyard cards in pick order and clears
// the selection. It returns the cards exiled; the caller fires
// TrigCollectEvidence when there was one (fireEvidence).
func (s *State) exileEvidence() int {
	n := bits.OnesCount64(s.g.evSel)
	s.g.evSel = 0
	for _, o := range s.evPicks()[:n] {
		s.exileFromGrave(o)
	}
	return n
}

// evidenceStageOpts is buildStep's bsEvidence for an ability or spell owing
// need: it poses the pick in d. The offer already held a graveyard worth need
// (canActivate, optCostOffer), and nothing between the offer and this stage
// exiles from it.
//
//go:noinline
func (s *State) evidenceStageOpts(k *prep, d *Decision, need int) {
	s.evidenceOptions(k, d, s.bld().e.ctrl, need)
}

// answerCostEvidence applies a bsEvidence answer: a pick, or Done.
//
//go:noinline
func (s *State) answerCostEvidence(k *prep, a Action) {
	if a.Kind() == AkDone || s.evidencePick(s.bld().e.ctrl, a) {
		s.nextStage(k)
	}
}

// payCostEvidence exiles the activation's picked cards with its other cost
// objects (gorge payCast: EvidenceCost per card, after the exile parts).
//
//go:noinline
func (s *State) payCostEvidence(k *prep) {
	if DebugCostEvidenceStats != nil {
		DebugCostEvidenceStats.Activations.Add(1)
	}
	if s.exileEvidence() > 0 && k.gen.anyExplEvTrig {
		s.fireEvidence(k, s.bld().e.ctrl) // trig_exploreevidence.go
	}
}

// unpushActivation takes the reversed activation off the stack and gives back
// the activation count its push took (CR 733.1; gorge ActivationReversed).
//
//go:noinline
func (s *State) unpushActivation(k *prep) {
	e := &s.bld().e
	s.sp--
	s.bld().bfl &^= bflPushed
	if s.npend > s.bld().pushNP {
		// CR 733.1: the reversal drops the ward and BecomesTarget triggers
		// the CR 601.2c target choice queued after the push (gorge abortCast's
		// proposalTriggers) and gives back the once-per-turn records they
		// consumed (gorge's proposalLimits); nothing else queues between the
		// push and a reversal, and an activation's push moves no zone
		s.dropPendingLimits(k, s.bld().pushNP)
	}
	if ob := &s.objs[e.obj]; ob.zone == kop.ZoneBattlefield && ob.activations() > 0 {
		s.touch(e.obj)
		ob.acts--
	}
}

// startCostEvidence answers "pay" to a trigger's own CollectEvidence cost
// (gorge evidenceStage): a graveyard short of N declines; one the threshold
// needs whole is exiled unasked; else the pick begins.
//
//go:noinline
func (s *State) startCostEvidence(k *prep) {
	p := s.g.pickP
	need := int(s.g.pickLeft)
	if need <= 0 || s.graveTotalMV(k, p)-s.evExclMV(k, p) < need {
		s.g.pickZ = payKindEvidence
		s.payDone(k, false)
		return
	}
	if s.evidenceGreedyMin(k, p, need) == s.evCandN(p) {
		if DebugCostEvidenceStats != nil {
			DebugCostEvidenceStats.Paid.Add(1)
			DebugCostEvidenceStats.Auto.Add(1)
		}
		s.evidenceSelectAll(k, p)
		s.exileTrigNewFirst()
		if s.exileEvidence() > 0 && k.gen.anyExplEvTrig {
			s.fireEvidence(k, p)
		}
		s.payDone(k, true)
		return
	}
	s.g.evSel = 0
	s.res.ask, s.res.askP = raCostEvidence, p
}

func (s *State) askOptionsCostEvidence(k *prep, d *Decision) bool {
	if s.res.ask != raCostEvidence {
		return s.askOptionsInit(k, d) // act_initiative.go
	}
	s.evidenceOptions(k, d, s.g.pickP, int(s.g.pickLeft))
	return true
}

func (s *State) applyAskCostEvidence(k *prep, a Action) bool {
	if s.res.ask != raCostEvidence {
		return s.applyAskInit(k, a) // act_initiative.go
	}
	if a.Kind() != AkDone && !s.evidencePick(s.g.pickP, a) {
		return true // more picks or Done
	}
	if DebugCostEvidenceStats != nil {
		DebugCostEvidenceStats.Paid.Add(1)
	}
	s.exileTrigNewFirst()
	if s.exileEvidence() > 0 && k.gen.anyExplEvTrig {
		s.fireEvidence(k, s.g.pickP)
	}
	s.payDone(k, true)
	return true
}
