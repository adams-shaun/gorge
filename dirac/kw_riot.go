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
// neither reaches a printed-Riot carrier) and records the grant's
// timestamp (promoKw) so the layer-6 walk's lose-all-abilities ordering
// sees it where the table needs one.
//
// A GRANTED Riot (a static's AddKeyword$, Rhythm of the Wild, Spider-Punk)
// stays refused: gorge's enteringRiot would ask for the derived read too,
// and the kernel reads only the printed field. A face that also prints
// Unleash refuses at lowering (gorge asks both as-enters elections; the
// kernel's pre-entry ask carries one kind, and no corpus card has both).
// A face-down manifest or cloak entry is a vanilla 2/2 and poses nothing
// (CR 708.5, gorge's IsFaceDownEntry guard).
//
// Cold and out of line: a table with no printed Riot takes none of these
// paths (prep.riot, a prep bool like unleash so the hot hooks cost one
// byte load).
const _ = kop.KwRiot

// tableHasRiot reports whether some card of t prints Riot.
func tableHasRiot(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].Riot {
			return true
		}
	}
	return false
}

// riotEntryAsk queues the CR 702.136a as-enters ask of the entrant at sl
// when its card prints Riot: gorge parks the battlefield move on this ask
// (riotMove, beside Unleash's), so it is queued at the entry's start, ahead
// of the entry counters' own asks and of Unleash's (gorge's faceETBChoice
// walks riot before unleash). A cast creature's election was asked with the
// spell still on the stack (riotPreKind); the answer lands as the entry's
// counter or haste grant. The ask is posed at the next priority, before the
// entrant's ETB triggers resolve.
func (s *State) riotEntryAsk(k *prep, sl Slot, o Obj) {
	if k.riot && k.t.Cards[s.objs[o].card].Riot {
		if p := &s.g.pre; p.state == 2 && p.kind == akRiot && p.obj == o {
			// a cast creature's election was asked with the spell still on
			// the stack (riotPreKind); the answer lands as the entry's grant
			s.riotApply(k, sl, uint8(p.val))
			return
		}
		s.queueAskAt(sl, akRiot, NoCard, 0, 0)
	}
}

// riotPreKind is akRiot when card c prints Riot: gorge asks a cast
// creature's election at the spell's resolution, with the spell still on
// the stack (cast.go chooseRiot, the same as-enters shape as Unleash's,
// CR 614.12), so the kernel poses it through the pre-entry ask (preAskFor)
// instead of queueing it after the entry; 0 otherwise.
func (s *State) riotPreKind(k *prep, c CardID) uint8 {
	if k.riot && k.t.Cards[c].Riot {
		return akRiot
	}
	return 0
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
