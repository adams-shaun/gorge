package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Casualty N (CR 702.153a, Streets of New Capenna), copied from gorge (rules
// optional_sacrifice.go's optSacCasualty row, cast_begin.go
// optionalSacrificeAsk, cast_payment.go castAnswerCasualty, cast_commit.go's
// casualty trigger):
//
//   - the casualty cast is its own AkCastCasualty option (gorge Mode
//     "casualty"), offered beside the plain cast when the caster controls a
//     creature with power >= CardDef.Casualty. The printed fixed-N form
//     only: kgen refuses Casualty:X.
//   - bsCasualty (build.cas == casElected) poses the sacrifice election
//     after the either-or cost and the gift, before convoke / X / targets
//     (gorge poses optionalSacrificeAsk there): one DkChoose/ChGenCasualty
//     per qualifying creature, no decline. The pick joins the cost picks
//     as a sacrifice, so it settles at payment, after the targets, with
//     the other sacrifices (gorge: "stays on the battlefield until payCast").
//   - cas is build state, not a stack flag (stackEntry has no spare bit).
//     When the cast completes with casPaid, one trigger is queued above the
//     spell (gorge: a respondable ability, not a copy at payment). It is
//     the delayed-trigger entry (abDelayed) whose action is OpCopySpell;
//     resolveDelayed copies the spell through copySpell (copy_gen.go),
//     whose controller may choose new targets (CR 707.10).
const (
	casElected uint8 = 1 // build.cas: the cast is the casualty cast
	casPaid    uint8 = 2 // a creature was chosen to be sacrificed
)

// beginCasualtyCast begins the casualty cast of o: the plain cast plus the
// casualty election (gorge's "casualty" cast option).
func (s *State) beginCasualtyCast(k *prep, o Obj, mode uint8) {
	s.g.bargainCast = k.t.Cards[s.objs[o].card].Bargain
	s.beginCast(k, o, mode, 0) // mode: a permission's cast (offerBargainPlay), else 0; a hand cast is bargained too
	s.g.bargainCast = false
	s.bld().cas = casElected
	if k.t.Cards[s.bld().e.card].Teamwork != 0 {
		s.bld().e.flags |= sfKicked // the declared teamwork intent (kw_teamwork.go)
	}
}

// casualtyCands lists p's creatures with power >= n, in battlefield order.
func (s *State) casualtyCands(k *prep, p uint8, n int, d *Decision) {
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if s.isCreature(k, sl) && s.power(k, sl) >= n && !s.picked(s.perms[sl].obj) {
			d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
		}
	}
}

// casualtyOffers: gorge's hand walk offers the casualty mode only when the
// floating pool pays the plain cast; its planner prices no casualty mode, so
// the potential walk lists none (potential_plan.go: unsupported shape). Under
// a gorge surface the kernel must mirror that; with no surface (the kernel's
// own legal set) the mode is offered whenever a qualifying creature exists.
func (s *State) casualtyOffers(p uint8, o Obj) bool {
	if s.cfg&(cfgGorgeLowering|cfgGorgeAutopay) == 0 {
		return true
	}
	return s.FeatPoolPays(p, o, 0)
}

// casualtyOffer: p may cast o with casualty (a creature of power >= N).
func (s *State) casualtyOffer(k *prep, p uint8, c CardID, o Obj) bool {
	n := s.casualtyN(k, p, c, o, true) // the offer: the card is in the hand
	if n == 0 || k.gen == nil {
		return false
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		if sl := m.first(); s.isCreature(k, sl) && s.power(k, sl) >= n {
			return true
		}
	}
	return false
}

// casualtyOptions poses the sacrifice election; false = no creature
// qualifies (gorge: "casualty no longer payable; casting without casualty").
func (s *State) casualtyOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if k.t.Cards[e.card].Bargain {
		return s.bargainOptions(k, d) // kw_bargain.go
	}
	if k.t.Cards[e.card].Teamwork != 0 {
		return s.teamworkOptions(k, d) // kw_teamwork.go
	}
	d.N = 0
	s.casualtyCands(k, e.ctrl, s.casualtyN(k, e.ctrl, e.card, e.obj, s.castFromHandBy(e, e.ctrl)), d)
	if d.N == 0 {
		s.bld().cas = 0
		return false
	}
	d.Kind, d.Sub, d.Player = DkChoose, ChGenCasualty, e.ctrl
	return true
}

// answerCasualty records the chosen creature as a sacrifice pick.
func (s *State) answerCasualty(k *prep, a Action) {
	if k.t.Cards[s.bld().e.card].Teamwork != 0 {
		s.answerTeamwork(a) // kw_teamwork.go: the election stays open
		return
	}
	s.bld().picks[s.bld().pickN] = Obj(a.Source())
	s.bld().pickKs[s.bld().pickN] = pkSac
	s.bld().pickN++
	s.bld().cas = casPaid
	s.nextStage(k)
}

// casualtyTrigger applies the paid sacrifice after the cast: casualty queues
// its copy trigger; a bargained cast instead marks its stack entry (the
// kicked provenance), as gorge's FlagBargained does (kw_bargain.go).
func (s *State) casualtyTrigger(k *prep, e *stackEntry) {
	if s.bld().cas == casTWDecline {
		s.teamworkClear() // kw_teamwork.go
		return
	}
	if s.bld().cas != casPaid {
		return
	}
	if k.t.Cards[e.card].Bargain {
		if !DebugBargainNoFlag {
			s.stack[s.sp-1].flags |= sfKicked
		}
		return
	}
	s.enqueue(e.obj, s.objs[e.obj].ver, e.card, abDelayed, e.ctrl, e.obj, e.ctrl, int16(kop.OpCopySpell))
}
