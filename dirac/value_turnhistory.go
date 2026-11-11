package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the turn history bank. Every function here is cold and
// out of line; the hot sites hold one gate each (genPrep.anyHist at
// the cast, the trigger's own flag at the matchers).

// History bank slots (genState.hist[player][slot]).
const (
	histSpells  = 0 // spells the player cast this turn (gorge pay.SpellsCastThisTurn: PutOnStack events since TurnChange)
	histMayPlay = 1 // may-play casts of cards the player owns this turn (gorge scanMayPlaysThisTurn CastInfo)
	histCastN   = 2 // spells the player has in the cast ring (cast_castledger.go)
)

// histCast folds a cast into the history bank as gorge's payCast does, after
// the payment and before the cast triggers (gorge emits the ManaExpend
// CastInfo before the deferred SpellCast trigger): the caster's spell count,
// then the mana the cast spent (CR 700.14; every pool mana paid for the
// spell, X included; a convoke pick is not pool mana, so it is removed
// from the CastTotalManaSpent record but stays in the expend tally) and the expend triggers
// that total crosses (gorge folds the tally unconditionally and fires
// trig:ManaExpend from it, trigmatch.ManaExpendMatches).
//
//go:noinline
func (s *State) histCast(k *prep, e *stackEntry, cost *kop.ManaCost) {
	h := &s.g.hist[e.ctrl]
	if h[histSpells] < 255 {
		h[histSpells]++
	}
	if s.bld().bfl&bflTre != 0 {
		s.noteCastTreasure(e)
	}
	if e.flags&sfMayPlay != 0 {
		// gorge's count is per owner, not caster: an opponent's card cast
		// through a permission consumes the owner's MayPlayLimit
		o := s.objs[e.obj].owner()
		if s.g.hist[o][histMayPlay] < 255 {
			s.g.hist[o][histMayPlay]++
		}
	}
	spent := cost.MV()
	if e.flags&sfFree == 0 {
		spent += cost.XAmount(int(e.x))
	}
	spentMana := spent
	if k.gen.anyConvoke && harmonizeEntry(&k.t.Cards[e.card], e) {
		// harmonize's tap pays generic, not mana (CR 702.180a), and is no
		// part of the expend tally either (as improvise's)
		spentMana = s.convokeApplyPicks(k, *cost).MV()
		spent = spentMana
	} else if k.gen.anyConvoke && s.tapPays(k, e.ctrl, e.card, e.obj, s.castFromHandBy(e, e.ctrl)) {
		spentMana -= s.convokePickCount()
		if spentMana < 0 {
			spentMana = 0
		}
		if s.isImprovise(k, e.ctrl, e.card, e.obj, s.castFromHandBy(e, e.ctrl)) {
			// gorge's improvise picks do not count as mana for the expend
			// tally either (CountsMana is convoke-only); convoke picks do
			spent -= s.improvisePickCount()
			if spent < 0 {
				spent = 0
			}
		}
	}
	if k.gen.anySpent {
		s.recordSpent(e.obj, spentMana)
	}
	if spent <= 0 {
		return
	}
	s.g.spent[e.ctrl] = uint16(min(int(s.g.spent[e.ctrl])+spent, 0xFFFF))
	if s.subs[kop.TrigManaExpend].any() {
		s.firePlayer(k, kop.TrigManaExpend, e.ctrl, int16(spent))
	}
}

// expendCrossed: the cast that spent amt took who's total spent this turn
// from below the trigger's N to N or more (gorge ManaExpendMatches: prev <
// n && total >= n; a cast starting at or above N fires nothing).
//
//go:noinline
func (s *State) expendCrossed(t *kop.Trigger, who uint8, amt int16) bool {
	total := int(s.g.spent[who])
	return total-int(amt) < int(t.N) && total >= int(t.N)
}

// resolveMark is the used-record ability byte of a resolution count (the
// once-per-turn records keep ability indices below it).
const resolveMark = 0x80

// countResolve records one resolution of the resolving triggered ability
// (gorge foldResolve: the per-ability ResolvedThisTurn tally, incremented
// before the body runs, so the body's read includes it). A record per
// resolution in the once-per-turn pool, keyed by the source object and the
// ability body; the ver is kept for the other uses of the pool but is not
// part of the tally's key (resolvedThisTurn). The pool's cap stalls loud
// (StallUsed).
//
//go:noinline
func (s *State) countResolve(k *prep, e *stackEntry) {
	if int(s.g.nused) < maxUsed {
		s.g.used[s.g.nused] = usedRec{obj: e.obj, ab: resolveMark | resolveTally(k, e), ver: e.srcVer}
		s.g.nused++
	} else {
		s.capFull(StallUsed)
	}
}

// resolveTally is the ability index a resolution is tallied under: the
// first counted ability of the card whose body is the same program. Two
// trigger lines sharing one Execute$ body are one ability to the oracle text
// (Victor's "whenever an enchantment you control enters and whenever you
// fully unlock a Room" is one Eerie ability) and gorge keys the tally by the
// body (events.ResolvedAbilityKey), so a resolution through either line
// counts into the one tally.
//
//go:noinline
func resolveTally(k *prep, e *stackEntry) uint8 {
	abs := k.t.Cards[e.card].Abilities
	a := &abs[e.ab]
	for j := uint8(0); j < e.ab; j++ {
		b := &abs[j]
		if b.AFlags&kop.AfResolveCount == 0 || b.Kind != a.Kind || len(b.Prog) != len(a.Prog) {
			continue
		}
		same := true
		for i := range a.Prog {
			if a.Prog[i] != b.Prog[i] {
				same = false
				break
			}
		}
		if same {
			return j
		}
	}
	return e.ab
}

// resolvedThisTurn is ExResolvedThisTurn: the resolving ability's
// resolutions this turn (Count$ResolvedThisTurn). Keyed by the source object
// and the ability body only, never the object's ver: gorge
// events.ResolvedAbilityKey is (source ObjID, ability body) and the source
// object's id survives its zone changes, so a permanent that left and
// returned keeps the turn's count, as Forge's ActivationTable does (its key
// is the host card's printed SpellAbility, which a zone change does not
// replace). South Pole Voyager's tally still reads 2 after it is exiled and
// returned (Airbender Ascension), so its second resolution still draws.
//
//go:noinline
func (s *State) resolvedThisTurn(k *prep) int {
	if !s.res.active {
		return 0
	}
	e := &s.res.e
	ab := resolveMark | resolveTally(k, e)
	n := 0
	for i := 0; i < int(s.g.nused); i++ {
		if r := s.g.used[i]; r.obj == e.obj && r.ab == ab {
			n++
		}
	}
	return n
}
