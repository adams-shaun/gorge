package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Riot (CR 702.136a; gorge cast_etbchoice.go faceETBChoice, replacement_etb.go
// applyRiotReplacement, events apply_zone.go): "You may have this permanent
// enter with an additional +1/+1 counter on it. If you don't, it gains haste."
// Forge scripts it as K:Riot, an as-enters choice between the two.
//
// gorge poses the choice on every face-up entry of a card whose printed face
// carries the keyword, first among the face's as-enters choices (riot before
// unleash, the read-ahead chapter and the Repls), with two options of kind
// "riot" over the entering permanent: option 0 "Enter with a +1/+1 counter",
// option 1 "Gain haste". The "counter" answer folds into the move as an entry
// counter grant (EntryCounterGrantsForFace); the "haste" answer appends
// "Haste" to the object's IntrinsicKeywords as it enters, so the keyword stays
// for as long as the permanent does and is gone when it leaves (a bounced and
// replayed card is a new object, CR 400.7). A face-down manifest or cloak
// entry poses nothing (CR 708.5). Only the PRINTED keyword is read: a granted
// Riot (Rhythm of the Wild, Uncivil Unrest, Spider-Punk's Spiders) asks
// nothing in gorge, so kgen refuses those carriers.
//
// The kernel poses the same ask as unleash does (kw_unleash.go): a cast
// creature's election is asked at the spell's resolution with the spell still
// on the stack (riotPreKind, the pre-entry ask), any other entry's is queued
// from enterGen (riotEntryAsk) and posed at the next priority. The options are
// gorge's, in its order: mode 0 = the counter, mode 1 = haste. "Counter" puts
// one +1/+1 counter the way the entry counters are put (placeEntryCounters,
// the would-enter-board settlement, CR 614.12). "Haste" sets the permanent's
// mechRiotHaste mark, which ctrKw folds into the BASE keyword set beside the
// counter keywords (a lose-all-abilities effect clears it like the printed
// keyword, CR 613.1f); the arena clears the mark when the slot takes a new
// permanent.
//
// Cold and out of line: a table with no printed Riot takes none of these
// paths (useRiot).

// KwRiot is the manifest's KwID for the keyword (dirac/manifest/KwID/KwRiot):
// the code this mechanic implements; the printed carrier is CardDef.Riot.
const _ = kop.KwRiot

// akRiot is 34, after akUnleash (33): a Riot entry's counter-or-haste election
// (CR 702.136a; kw_riot.go).
const akRiot uint8 = 34

// useRiot marks a table with a printed-riot card.
var useRiot = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].Riot {
			return true
		}
	}
	return false
})

// RiotStats counts what a lockstep run exercised (nil costs one test on this
// cold path).
type RiotStats struct {
	Counter, Haste int // the two answers applied
}

// DebugRiotStats, when non-nil, tallies the riot answers.
var DebugRiotStats *RiotStats

// DebugRiotNoHaste drops the haste half of the election (TestRiotMutation: the
// lockstep must catch it).
var DebugRiotNoHaste bool

// riotOn reports whether the table prints Riot on some card.
func (s *State) riotOn(k *prep) bool { return k.gen != nil && k.gen.uses.has(useRiot) }

// riotEntryAsk queues the CR 702.136a as-enters ask of the entrant at sl when
// its card prints Riot, or applies the answer a cast creature's pre-entry ask
// already took. Like unleashEntryAsk it poses nothing for a face-down entry
// (the caller's face == 0 guard).
func (s *State) riotEntryAsk(k *prep, sl Slot, o Obj) {
	if !s.riotOn(k) || !k.t.Cards[s.objs[o].card].Riot {
		return
	}
	if p := &s.g.pre; p.state == 2 && p.kind == akRiot && p.obj == o {
		s.riotApply(k, sl, uint8(p.val))
		return
	}
	s.queueAskAt(sl, akRiot, NoCard, 0, 0)
}

// riotPreKind is akRiot when card c prints Riot: gorge asks a cast creature's
// election at the spell's resolution, with the spell still on the stack (the
// same as-enters shape as the unleash election), so the kernel poses it
// through the pre-entry ask (preAskFor); 0 otherwise.
func (s *State) riotPreKind(k *prep, c CardID) uint8 {
	if s.riotOn(k) && k.t.Cards[c].Riot {
		return akRiot
	}
	return 0
}

// riotOptions fills the as-enters ask: gorge's two options over the entering
// permanent in its order (mode 0 = the +1/+1 counter, 1 = haste).
func (s *State) riotOptions(d *Decision, o Obj) {
	d.Sub = ChGenRiot
	d.add(MakeAction(AkChoose, TCode(o), 0, 0)) // enter with a +1/+1 counter
	d.add(MakeAction(AkChoose, TCode(o), 0, 1)) // gain haste
}

// riotAnswer applies an answer of the queued ask.
func (s *State) riotAnswer(k *prep, ask askRec, a Action) {
	s.riotApply(k, ask.sl, a.Mode())
}

// riotApply applies the election mode (0 counter, 1 haste) to the permanent at sl.
func (s *State) riotApply(k *prep, sl Slot, mode uint8) {
	st := DebugRiotStats
	if mode == 0 {
		if st != nil {
			st.Counter++
		}
		s.placeEntryCounters(k, sl, int(kop.CtrP1P1), 1, s.perms[sl].ctrl)
		return
	}
	if st != nil {
		st.Haste++
	}
	if DebugRiotNoHaste {
		return
	}
	s.touchSl(sl)
	s.mSlSet(k, mechRiotHaste, sl)
}

// riotHasteAt reports whether the permanent p holds its riot haste.
func (s *State) riotHasteAt(p *perm) bool {
	k := s.k()
	return s.riotOn(k) && s.mSl(k, mechRiotHaste, s.objs[p.obj].slot)
}

// arenaRiotSpectacle marks the arena mechanics the table's Riot and Spectacle
// cards use: the haste mark, and the kicked mark a spectacle permanent carries.
func arenaRiotSpectacle(t *kop.Table, u *[numMech]bool) {
	for i := range t.Cards {
		if t.Cards[i].Riot {
			u[mechRiotHaste] = true
		}
		if t.Cards[i].HasSpectacle {
			u[mechKicked] = true
		}
	}
}
