package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Paradigm (SOS; gorge rules/paradigm.go): "Then exile this
// spell. After you first resolve a spell with this name, you may cast a
// copy of it from exile without paying its mana cost at the beginning of
// each of your first main phases."
//
// gorge models it with two reads of the same keyword:
//   - spellRestZone sends a RESOLVED Paradigm spell to exile ahead of every
//     other resting zone but the flashback family's own exile (a countered
//     or fizzled one keeps spellFizzleZone's graveyard);
//   - paradigmMayPlay offers the exiled card itself (not a copy: gorge's
//     known approximation, copied as is) as a free cast to its owner while
//     it is the owner's first main phase, the owner has resolved a Paradigm
//     spell with that name (a Resolve event, never forgotten), and the card
//     was not put on the stack this turn.
//
// The resolved names ride genState.parad: bit CardDef.Paradigm-1 of the
// owner's byte, set as the spell resolves and never reset. "Put on the stack
// this turn" is the ring (cast_castledger.go): it
// names the objects pushed this turn, so a card exiled some other way this
// turn is still offered, as gorge's PutOnStack read does (zoneTurn could not
// tell the two apart). A table whose expressions read the ring
// (genPrep.castRingOn) records every push; a Paradigm table that does not
// records the Paradigm cards' pushes only (noteParadigmCast), and since
// nothing reads that ring per caster (stackedThisTurn scans both), a full
// caster's ring spills into the other player's: 2*castRingN Paradigm pushes
// per turn, and the next one stalls with StallCastRing rather than
// truncating.

// castParadigm is the AkCast mode of the exiled Paradigm card's free cast
// (castFree alone, like castPlotted: no permission record is spent).
const castParadigm = castFree

// StallParadigm: a Paradigm name index beyond genState.parad's 8 bits
// (kgen refuses a ninth name; a hand-built table stalls loudly).
const StallParadigm = 102

func init() { stallNames[StallParadigm] = "paradigm" }

// DebugParadigmAnyMain offers the free cast in the owner's second main
// phase too (mutation testing only: the lockstep must catch it).
var DebugParadigmAnyMain bool

// prepParadigm arms the Paradigm walker and the cast ring it reads.
func prepParadigm(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		if t.Cards[i].Paradigm != 0 {
			g.anyParadigm, g.anyHist = true, true // the push count is in the history bank
			return
		}
	}
}

// paradigmRest is gorge's spellRestZone Paradigm arm for resolved spell o
// whose resting zone is otherwise z: exile, and its owner's name bit is set
// (gorge paradigmResolved reads the Resolve event). A stack copy is not the
// card (gorge's IsCopy arm already exiles it, and its Resolve names no
// object that survives).
//
//go:noinline
func (s *State) paradigmRest(k *prep, o Obj, z kop.Zone) kop.Zone {
	ob := &s.objs[o]
	n := k.t.Cards[ob.card].Paradigm
	if n == 0 || ob.of&ofCopy != 0 {
		return z
	}
	if n > 8 {
		s.capFull(StallParadigm)
		return z
	}
	s.g.parad[ob.owner()] |= 1 << (n - 1)
	return kop.ZoneExile
}

// noteParadigmCast records the push of spell o in the cast ring when it is
// a Paradigm card (a table whose expressions read the ring records every
// push through noteCast instead), in the other player's ring once the
// caster's is full.
//
//go:noinline
func (s *State) noteParadigmCast(k *prep, o Obj, caster uint8, from kop.Zone) {
	if k.t.Cards[s.objs[o].card].Paradigm == 0 {
		return
	}
	if s.g.hist[caster][histCastN] >= castRingN {
		caster = s.nextAlive(caster) // storage only: another seat's ring
	}
	s.noteCast(o, caster, from)
}

// stackedThisTurn reports whether o was put on the stack this turn by
// either player (gorge putOnStackThisTurn: any PutOnStack naming the id
// since the last TurnChange). The ring stalls rather than truncating, so
// an absent entry is a real absence.
func (s *State) stackedThisTurn(o Obj) bool {
	for p := uint8(0); p < s.nSeats(); p++ {
		for _, r := range s.castPushes(p) {
			if r.obj == uint8(o)+1 {
				return true
			}
		}
	}
	return false
}

// offerParadigm offers p's exiled Paradigm cards' free cast (gorge
// mayPlayGrantScoped's Paradigm source, paradigmMayPlay): p's own first
// main phase, the card's own timing, a resolved spell of its name, and not
// put on the stack this turn.
func (s *State) offerParadigm(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	if s.active != p || s.step != kop.StepMain1 && !(DebugParadigmAnyMain && s.step == kop.StepMain2) {
		return
	}
	if s.g.parad[p] == 0 {
		return
	}
	pl := &s.pl[p]
	for i := 0; i < int(pl.exileN); i++ {
		o := pl.exile[i]
		ob := &s.objs[o]
		c := ob.card
		n := k.t.Cards[c].Paradigm
		if n == 0 || n > 8 || ob.owner() != p || ob.of&ofCopy != 0 || s.g.parad[p]&(1<<(n-1)) == 0 {
			continue
		}
		if !k.cards[c].instant && !sorc && !s.castFlash(k, p, c) {
			continue
		}
		if s.stackedThisTurn(o) {
			continue
		}
		if s.castHeldOut(o) || k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		if !s.spellTargetsFeasible(k, c, p) {
			continue
		}
		d.add(MakeAction(AkCast, uint8(o), 0, castParadigm))
	}
}

// FeatParadigmCard reports whether o's card has Paradigm: the oracle
// canonicalises its free cast from exile (castParadigm) as gorge's may-play
// cast, not plot's "plot_cast" route that castFree alone otherwise names.
func (s *State) FeatParadigmCard(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].Paradigm != 0
}
