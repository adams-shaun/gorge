package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// the value heads and filter properties the MageZero sample
// decks need (cold, out of line; nothing here is reached by a table whose
// cards do not lower these shapes).

// exprKvf evaluates ExPoisonOf, ExProduct and ExAttackersAll.
//
//go:noinline
func (s *State) exprKvf(k *prep, x *kop.Expr, ctx evalCtx) int {
	switch x.Kind {
	case kop.ExPoisonOf:
		// gorge evalPlayerRefProperty's Counters.Poison arm: the poison
		// counters of the referenced player (TargetedController: the
		// controller of the target)
		v := 0
		if p := s.refPlayer(k, x.Ref, ctx); p != 255 {
			v = int(s.g.poison[SeatIx(p)])
		}
		kvfHit(KvfPoisonLow + min(v, 3)/3)
		return v
	case kop.ExProduct:
		// gorge applyCountOpOperand: /Times.<SVar> multiplies by the
		// operand's value
		v := s.evalV(k, x.Sub[0], ctx) * s.evalV(k, x.Sub[1], ctx)
		if v != 0 {
			kvfHit(KvfProductNonzero)
		} else {
			kvfHit(KvfProductZero)
		}
		return v
	case kop.ExAttackersAll:
		// gorge Engine.AttackersThisTurn: every DeclareAttackers event's
		// attackers since the turn began, summed (only the active player
		// declares attackers, CR 508.1)
		v := int(s.g.attacked[0]) + int(s.g.attacked[1])
		if v != 0 {
			kvfHit(KvfAttackersNonzero)
		}
		return v
	}
	return 0
}

// outlawSubs finds the five outlaw creature types among the matchup's
// subtypes, subOutside for an absent one (only an all-creature-types
// permanent has it then), as partyRoles does the party roles.
func outlawSubs(t *kop.Table) (r [5]uint8) {
	for i, name := range [5]string{"Assassin", "Mercenary", "Pirate", "Rogue", "Warlock"} {
		r[i] = subOutside
		for j, s := range t.Subtypes {
			if s == name {
				r[i] = uint8(j)
			}
		}
	}
	return r
}

// predPermKvf evaluates PredNoCounters, PredLacksKeyword, PredOutlaw and
// PredMonoColor for the battlefield permanent at sl.
func (s *State) predPermKvf(k *prep, fl *kop.Filter, sl Slot) bool {
	switch fl.Pred {
	case kop.PredNoCounters:
		// gorge's '!' over wordHasCounters
		if s.predHasCounters(sl) {
			kvfHit(KvfNoCountersFalse)
			return false
		}
		kvfHit(KvfNoCountersTrue)
		return true
	case kop.PredLacksKeyword:
		return s.kwGen(k, sl)&(1<<uint(fl.PredN)) == 0
	case kop.PredOutlaw:
		// gorge outlawMatches: hasTypeCtx over the five, layer-aware
		out := false
		for _, i := range k.gen.outlawSub {
			if s.hasSubGen(k, sl, i) {
				out = true
				break
			}
		}
		kvfHit(KvfOutlawNo + b2i(out))
		return out != (fl.PredN == 1)
	case kop.PredMonoColor:
		// gorge wordMonoColor: exactly one current colour
		mono := bits.OnesCount8(uint8(s.colorsGen(k, sl)&kop.ColAny)) == 1
		kvfHit(KvfMonoNo + b2i(mono))
		return mono
	}
	return false
}

// predCardKvf is predPermKvf for a card off the battlefield (printed
// characteristics; fd: face down, so colourless and typeless).
func predCardKvf(k *prep, fl *kop.Filter, c CardID, fd bool) bool {
	cd := &k.t.Cards[c]
	switch fl.Pred {
	case kop.PredNoCounters:
		return true // off the battlefield nothing holds a counter here
	case kop.PredLacksKeyword:
		return k.cards[c].kw&(1<<uint(fl.PredN)) == 0
	case kop.PredOutlaw:
		out := false
		if !fd {
			for _, i := range k.gen.outlawSub {
				if i == subOutside && cd.AllCreatureTypes || int(i) < len(k.t.Subtypes) && cd.Subtypes.Has(int(i)) {
					out = true
					break
				}
			}
		}
		return out != (fl.PredN == 1)
	case kop.PredMonoColor:
		mono := !fd && bits.OnesCount8(uint8(cd.Colors&kop.ColAny)) == 1 // CR 708.2: a face-down card is colourless
		kvfHit(KvfMonoNo + b2i(mono))
		return mono
	}
	return false
}

// DebugValueHeadHits, when set, counts the branches taken, for the
// oracle's lockstep occurrence minima. Nil in production (every increment
// tests the pointer first); only a single-goroutine test sets it.
var DebugValueHeadHits *[ValueHeadNumHits]uint32

// The KvfHits indices.
const (
	KvfPoisonLow        = iota // ExPoisonOf read below three poison counters
	KvfPoisonHigh              // ExPoisonOf read at three or more (Corrupted)
	KvfProductZero             // ExProduct evaluated to zero
	KvfProductNonzero          // ExProduct evaluated nonzero
	KvfAttackersNonzero        // ExAttackersAll read with an attack this turn
	KvfNoCountersTrue          // PredNoCounters matched
	KvfNoCountersFalse         // PredNoCounters refused a permanent with counters
	KvfOutlawNo                // PredOutlaw read a non-outlaw
	KvfOutlawYes               // PredOutlaw read an outlaw
	KvfMonoNo                  // PredMonoColor read a non-monocoloured object
	KvfMonoYes                 // PredMonoColor read a monocoloured object
	ValueHeadNumHits
)

// kvfHit counts branch i when the test hook is set.
func kvfHit(i int) {
	if h := DebugValueHeadHits; h != nil {
		h[i]++
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
