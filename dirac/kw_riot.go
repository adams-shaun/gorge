package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// akRiot is 35, past akProtector (34, face_battle.go): the ask kinds
// continue across their files.
const akRiot uint8 = 35 // a Riot entry's counter-or-haste election (CR 702.136a; kw_riot.go)

// kw:Riot (CR 702.136; gorge engine/rules/cast_etbchoice.go's faceETBChoice
// walk, enteringRiot): "You may have this permanent enter the battlefield
// with either a +1/+1 counter on it or with haste." The printed K:Riot is
// CardDef.Riot (kop.KwRiot): the kernel poses the as-enters ask on every
// face-up battlefield entry, and "counter" puts one +1/+1 counter the way
// the entry counters are put (placeEntryCounters, beside Unleash's) while
// "haste" grants the keyword. Gorge asks the choice at the spell's
// resolution with the spell still on the stack (cast.go chooseRiot; the
// same as-enters shape as Unleash's, CR 614.12) and parks a non-cast entry
// on the ask (riotMove), so the kernel mirrors both: a cast creature's
// election rides the pre-entry ask (riotPreKind, like the colour and type
// elections) and any other entry queues the ask at the entry's start,
// ahead of the entrant's ETB triggers and of Unleash's ask (gorge's
// faceETBChoice walks riot before unleash). The option list is gorge's, in
// its order: the counter ("Enter with a +1/+1 counter") is option 0, the
// haste ("Gain haste") is option 1, so the two engines share option 0.
//
// The haste grant: gorge appends "Haste" to the entered object's
// IntrinsicKeywords (apply_zone.go, beside the "counter" half's entry
// counter grant), so it lasts for as long as the object is there. The
// kernel sets the permanent's override keywords (perm.ovr.kw, cleared only
// by a turn animation's ovTypesEOT cleanup or a copy exception's reset —
// neither reaches a Riot carrier) and records the grant's timestamp
// (promoKw) so the layer-6 walk's lose-all-abilities ordering sees it where
// the table needs one.
//
// A GRANTED Riot is kop.Static.Riot on a Continuous static (kgen's
// lower_riotgrant.go: Spider-Punk's other Spiders, Uncivil Unrest's and
// Rhythm of the Wild's nontoken creatures), read beside CardDef.Riot, as
// gorge's enteringRiot reads the derived keyword list. A non-cast entry
// matches the grant on the entrant's new permanent slot (riotPermGranted);
// a cast creature's pre-entry election matches it on the card while it is
// still a spell (riotCardGranted, matchCardGen), so the ask keeps gorge's
// place at the spell's resolution. A face that also prints Unleash refuses
// at lowering (gorge asks both as-enters elections; the kernel's pre-entry
// ask carries one kind, and no corpus card has both). A face-down manifest
// or cloak entry is a vanilla 2/2 and poses nothing (CR 708.5, gorge's
// IsFaceDownEntry guard).
//
// Cold and out of line: a table with no printed or granted Riot takes none
// of these paths (prep.riot/prep.riotGrant, prep bools like unleash so the
// hot hooks cost one byte load).
const _ = kop.KwRiot

// RiotStats counts the granted Riot elections the kernel applied in a
// lockstep game (DebugRiotStats; TestDemoRiotLockstep reads it so a green
// game is one where a granted Riot was asked; nothing in the engine reads
// it).
type RiotStats struct {
	Granted int64 // elections a static's grant posed (the card does not print Riot)
}

// DebugRiotStats, when set, receives the counts.
var DebugRiotStats *RiotStats

// DebugNoGrantedRiot (TestDemoRiotMutation): the kernel reads no granted
// Riot, so a granted election gorge asks is never posed.
var DebugNoGrantedRiot bool

// tableHasRiot reports whether some card of t prints Riot.
func tableHasRiot(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].Riot {
			return true
		}
	}
	return false
}

// tableGrantsRiot reports whether some static of t grants Riot.
func tableGrantsRiot(t *kop.Table) bool {
	scan := func(abs []kop.Ability) bool {
		for j := range abs {
			if a := &abs[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Riot {
				return true
			}
		}
		return false
	}
	for i := range t.Cards {
		if scan(t.Cards[i].Abilities) {
			return true
		}
	}
	for i := range t.Emblems {
		if scan(t.Emblems[i].Abilities) {
			return true
		}
	}
	return false
}

// riotEntryAsk queues the CR 702.136a as-enters ask of the entrant at sl when
// its card prints Riot or a live static grants it: gorge parks the battlefield
// move on this ask (riotMove, beside Unleash's), so it is queued at the
// entry's start, ahead of the entry counters' own asks and of Unleash's
// (gorge's faceETBChoice walks riot before unleash). A cast creature's
// election was asked with the spell still on the stack (riotPreKind); the
// answer lands as the entry's counter or haste grant. The ask is posed at the
// next priority, before the entrant's ETB triggers resolve.
func (s *State) riotEntryAsk(k *prep, sl Slot, o Obj) {
	if !k.riot {
		return
	}
	pre := s.g.pre.state == 2 && s.g.pre.kind == akRiot && s.g.pre.obj == o
	printed := k.t.Cards[s.objs[o].card].Riot
	granted := !printed && s.riotPermGranted(k, sl)
	if !printed && !granted && !pre {
		return
	}
	if pre {
		// a cast creature's election was asked with the spell still on
		// the stack (riotPreKind); the answer lands as the entry's grant
		s.riotApply(k, sl, uint8(s.g.pre.val))
	} else {
		s.queueAskAt(sl, akRiot, NoCard, 0, 0)
	}
	if granted && DebugRiotStats != nil {
		DebugRiotStats.Granted++
	}
}

// riotPreKind is akRiot when card c prints Riot or a live static grants it:
// gorge asks a cast creature's election at the spell's resolution, with the
// spell still on the stack (cast.go chooseRiot, the same as-enters shape as
// Unleash's, CR 614.12), so the kernel poses it through the pre-entry ask
// (preAskFor) instead of queueing it after the entry; 0 otherwise.
func (s *State) riotPreKind(k *prep, c CardID, o Obj, ctrl uint8) uint8 {
	if !k.riot {
		return 0
	}
	if k.t.Cards[c].Riot || s.riotCardGranted(k, c, o, ctrl) {
		return akRiot
	}
	return 0
}

// riotPermGranted reports whether the permanent at sl is granted Riot by a
// live Continuous static: the layer-6 grant is read like toxic's
// (kw_toxicgrant.go), a grant older than a loss of the recipient's abilities
// granting nothing.
func (s *State) riotPermGranted(k *prep, sl Slot) bool {
	if !k.riotGrant || DebugNoGrantedRiot {
		return false
	}
	lts, lost := s.lossAt(k, sl)
	granted := false
	s.eachStatic(k, func(st statSrc) bool {
		ab := st.a
		if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatContinuous || !ab.Static.Riot ||
			lost && s.grantTs(st) < lts {
			return true
		}
		if s.statCond(k, st) && s.matchPermGen(k, ab.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			granted = true
			return false
		}
		return true
	})
	return granted
}

// riotCardGranted reports whether card c about to enter under ctrl is granted
// Riot by a live Continuous static. A cast creature's election is asked while
// the card is still a spell (riotPreKind), so the grant is matched on the
// card (matchCardGen), the way the pre-entry as-enters choices read a
// would-enter object: ctrl is the spell's controller (the card's "controller
// on the stack") and the static's own controller is the filter's "you", so a
// grant to "creatures you control" does not reach an opponent's spell.
// "Other" is every battlefield source (the entering card is not one), so the
// card's printed characteristics answer the filter as gorge's derived read
// does at the spell's resolution.
func (s *State) riotCardGranted(k *prep, c CardID, o Obj, ctrl uint8) bool {
	if !k.riotGrant || DebugNoGrantedRiot {
		return false
	}
	granted := false
	s.eachStatic(k, func(st statSrc) bool {
		ab := st.a
		if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatContinuous || !ab.Static.Riot {
			return true
		}
		if s.statCond(k, st) && s.matchCardGen(k, ab.Static.Filter, c, ctrl, st.ctrl, o) {
			granted = true
			return false
		}
		return true
	})
	return granted
}

// riotOptions fills the queued as-enters ask: the counter-or-haste election
// over the entering permanent, gorge's two options in its order (the
// counter is option 0 and the kernel's mode 0, the haste option 1 and mode
// 1, so both engines share option 0).
func (s *State) riotOptions(d *Decision, o Obj) {
	d.Sub = ChGenRiot
	d.add(MakeAction(AkChoose, TCode(o), 0, 0)) // enter with a +1/+1 counter
	d.add(MakeAction(AkChoose, TCode(o), 0, 1)) // gain haste
}

// riotApply applies an election: "counter" puts one +1/+1 counter on the
// entered permanent the way the entry counters are put (CR 702.136a, gorge's
// EntryCounterGrantsForFace); "haste" grants the keyword for as long as the
// permanent is there (gorge's IntrinsicKeywords append, apply_zone.go).
func (s *State) riotApply(k *prep, sl Slot, mode uint8) {
	p := &s.perms[sl]
	if mode == 0 {
		s.placeEntryCounters(k, sl, int(kop.CtrP1P1), 1, p.ctrl)
		return
	}
	p.ovr.kw |= kop.KwHaste
	s.promoKw(k, sl, kop.KwHaste)
	s.touchSl(sl)
}

// riotAnswer applies an answer of the queued ask.
func (s *State) riotAnswer(k *prep, ask askRec, a Action) {
	s.riotApply(k, ask.sl, a.Mode())
}
