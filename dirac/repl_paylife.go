package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The as-enters "pay any amount of life" (CR 614.12; Phyrexian Processor,
// Minion of the Wastes, Nameless Race): an enters program of the single
// instruction OpExt ExtPayLifeX (kgen lower_paylife.go). gorge asks it at the
// entry boundary as a KChoose of "paylife" options 0..bound (engine/rules
// etbPayLifeOptions, the amount on Amount), the bound being the payer's life
// capped by the body's XMax$; the answer is paid as one life loss and the body
// stores it as the entering permanent's SVar number (events.StoreSVar).
//
// The kernel poses the same question as the as-enters number ask (akNumber),
// with its own decision kind ChGenPayLife so the answer's canonical form is the
// "paylife" choose kind and never the number's. gorge asks it with the spell
// still on the stack, so a cast permanent takes the pre-entry ask (preAskFor,
// enterChoiceKind: an R: Updated line is not skipped for this op) and the
// answer waits in s.g.pre.val until the enters program reaches the op; an
// entry with no pre-ask queues it (queueAskAt over the next instruction).
// Either way payLifeApply loses the life through loseLife and writes the amount
// into the number store (numStore): the paid X is what the stored name reads
// afterwards, until the permanent leaves the battlefield (numClear, CR 400.7).
// An entry no ask serves (the sweep's exile return, a decision already
// outstanding) pays nothing and stores nothing, gorge's unasked entry running
// the body with X = 0.

// payLifeNoCap is ExtPayLifeX's B for a body without XMax$.
const payLifeNoCap = 32767

// DebugPayLifeNoStore (TestPayLifeMutation): the answer pays its life but
// stores no number, so the stored name keeps its printed default.
var DebugPayLifeNoStore bool

// StallPayLife: the payable range does not fit the action's mode byte (the
// payer's life, or the XMax$ cap, past numberMidCeiling). A declared capacity
// limit; offering a shorter list would pose a question gorge does not.
const StallPayLife = 174

func init() { stallNames[StallPayLife] = "paylife" }

// payLifeExt is ExtPayLifeX met outside an as-enters program's ask path: the
// ask is posed by runReplProgAs, which queues it, so the op itself does nothing.
//
//kmanifest:ext ExtPayLifeX
func (s *State) payLifeExt(k *prep, in *kop.Instr) (ok, done bool) {
	return true, true
}

// isPayLifeOp reports whether in is ExtPayLifeX.
func isPayLifeOp(in *kop.Instr) bool { return in.Op == kop.OpExt && in.Ext() == kop.ExtPayLifeX }

// payLifeAsk is the pay-life ask being posed: its ExtPayLifeX, the entering
// object and card, and the ability index of the enters program.
type payLifeAsk struct {
	in *kop.Instr
	o  Obj
	c  CardID
	ab uint8
}

// payLifeAskOp finds the ExtPayLifeX the pending number ask poses (nil in the
// result when it is another number's): the pre-entry ask's card, whose
// program opens with the op, or the queued ask's op behind its program counter.
// numberBounds' lookup, over the same two sources.
func (s *State) payLifeAskOp(k *prep) (pa payLifeAsk) {
	if s.preAskPosed() || s.res.ask == raGenTokChoice || s.res.ask == raGenRetChoice {
		if s.g.pre.ask2 != 0 {
			return
		}
		c := s.preAskCard()
		if c == NoCard {
			return
		}
		cd := &k.t.Cards[c]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplEntersProgram && len(a.Prog) > 0 && isPayLifeOp(&a.Prog[0]) {
				return payLifeAsk{in: &a.Prog[0], o: s.g.pre.obj, c: c, ab: uint8(i)}
			}
		}
		return
	}
	if s.g.nask == 0 {
		return
	}
	a := &s.g.ask[0]
	if a.kind != akNumber || a.card == NoCard || a.sl == NoSlot {
		return
	}
	prog := k.t.Cards[a.card].Abilities[uint8(a.ab)].Prog
	if i := int(a.pc) - 1; i >= 0 && i < len(prog) && isPayLifeOp(&prog[i]) {
		return payLifeAsk{in: &prog[i], o: s.perms[a.sl].obj, c: a.card, ab: uint8(a.ab)}
	}
	return
}

// payLifeBound is the largest amount payer p may announce: their life, capped
// by the program's XMax$ value, read with the entering object as the source
// (gorge etbPayLifeBound's NumResolved over the entering object).
func (s *State) payLifeBound(k *prep, pa payLifeAsk, p uint8) int {
	bound := max(int(s.pl[p].life), 0)
	if pa.in.B.K == kop.VConst && pa.in.B.N == payLifeNoCap {
		return bound
	}
	saved := s.res
	s.res = resCtx{e: stackEntry{kind: seAbility, obj: pa.o, srcVer: s.objs[pa.o].ver, card: pa.c, ab: pa.ab, ctrl: p, trigObj: NoObj}}
	s.res.e.tgt = noTargets
	capv := s.value(k, pa.in.B)
	s.res = saved
	return min(bound, max(capv, 0))
}

// payLifeOptions fills the ask for payer d.Player: one option per amount
// 0..bound ascending, as gorge lists them, each over the entering object.
func (s *State) payLifeOptions(k *prep, d *Decision, pa payLifeAsk) {
	d.Sub = ChGenPayLife
	bound := s.payLifeBound(k, pa, d.Player)
	if bound > numberMidCeiling {
		s.capFull(StallPayLife)
		return
	}
	for n := 0; n <= bound; n++ {
		d.add(MakeAction(AkChoose, TCode(pa.o), 0, uint8(n)))
	}
}

// payLifeAnswer applies the answer of a queued number ask that ExtPayLifeX
// posed (a no-op for any other); a pre-entry answer is applied where the
// program reaches the op (runReplProgAs).
func (s *State) payLifeAnswer(k *prep, ask askRec, a Action) {
	if ask.card == NoCard || ask.pc == 0 {
		return
	}
	prog := k.t.Cards[ask.card].Abilities[uint8(ask.ab)].Prog
	if int(ask.pc) > len(prog) || !isPayLifeOp(&prog[ask.pc-1]) {
		return
	}
	s.payLifeApply(k, ask.sl, &prog[ask.pc-1], int(a.Mode()))
}

// payLifeApply pays n life for the permanent at sl and stores it under the
// op's name: the controller loses it, the entering permanent keeps it as its
// number (the Count$xPaid the body stores).
func (s *State) payLifeApply(k *prep, sl Slot, in *kop.Instr, n int) {
	p := &s.perms[sl]
	s.loseLife(k, p.ctrl, n)
	if DebugPayLifeNoStore {
		return
	}
	name, del, def := kop.StoreNumUnpack(in.C)
	s.numStore(p.obj, name, n, del, def)
}
