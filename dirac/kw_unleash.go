package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kw:Unleash (CR 702.86; gorge engine/rules/unleash.go), printed as
// CardDef.Unleash (kop.KwUnleash): "You may have this permanent enter the
// battlefield with a +1/+1 counter on it. While it has a +1/+1 counter on
// it, it can't block."
//
// The choice half: gorge parks the battlefield move on the as-enters ask
// (applyUnleashReplacement, posed on every face-up entry) and folds the
// "counter" answer into the move as an entry counter grant
// (EntryCounterGrantsForFace, beside Riot's). The kernel poses the same ask
// from enterGen's entry (before the entrant's ETB triggers) over the
// generated ask queue (akUnleash, posed at the next priority like the other
// as-enters asks) and, on "yes", puts the counter the way the entry
// counters are put (placeEntryCounters: the would-enter-board settlement,
// CR 614.12). The two options are gorge's, in its order: "yes" first (its
// option 0, the counter), then "no".
//
// The can't-block half: canBlock refuses a blocker whose card carries
// Unleash and at least one +1/+1 counter, from any source, beside the
// Suspected gate (gorge combat/block.go reads the same two facts beside
// Suspected). The kernel reads the printed keyword only; a GRANTED Unleash
// (a static's AddKeyword$) is refused at lowering.
//
// Cold and out of line: a table with no printed Unleash takes none of these
// paths (prep.unleash, a prep bool like ascend so the hot hooks cost one
// byte load).

// KwUnleash is the manifest's KwID for the keyword (dirac/manifest/KwID/
// KwUnleash, state: code 0): the code this mechanic implements; the printed
// carrier is CardDef.Unleash, and a later granted-Unleash carrier would
// grant through it (the arena takes KwMask bits only, which are full).
const _ = kop.KwUnleash

// tableHasUnleash reports whether some card of t prints Unleash.
func tableHasUnleash(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].Unleash {
			return true
		}
	}
	return false
}

// unleashEntryAsk queues the CR 702.86a as-enters counter ask of the entrant
// at sl when its card prints Unleash: gorge parks the battlefield move on
// this ask (applyUnleashReplacement, posed on every face-up entry), so it is
// queued at the entry's start, ahead of the entry counters' own asks. A
// face-down manifest or cloak entry is a vanilla 2/2 and poses nothing
// (CR 708.5, gorge's IsFaceDownEntry guard). The ask is posed at the next
// priority, before the entrant's ETB triggers resolve.
func (s *State) unleashEntryAsk(k *prep, sl Slot, o Obj) {
	if k.unleash && k.t.Cards[s.objs[o].card].Unleash {
		s.queueAskAt(sl, akUnleash, NoCard, 0, 0)
	}
}

// unleashOptions fills the queued as-enters ask: the counter election over
// the entering permanent, gorge's two options in its order ("counter" is
// option 0, the deterministic bot's answer).
func (s *State) unleashOptions(d *Decision, ask askRec) {
	d.Sub = ChGenUnleash
	o := s.perms[ask.sl].obj
	d.add(MakeAction(AkChoose, TCode(o), 0, 1)) // enter with a +1/+1 counter
	d.add(MakeAction(AkChoose, TCode(o), 0, 0)) // enter without one
}

// unleashAnswer applies an answer of the queued ask: "yes" puts one +1/+1
// counter on the entered permanent, the way the entry counters are put; "no"
// enters plain.
func (s *State) unleashAnswer(k *prep, ask askRec, a Action) {
	if a.Mode() != 1 {
		return
	}
	p := &s.perms[ask.sl]
	s.placeEntryCounters(k, ask.sl, int(kop.CtrP1P1), 1, p.ctrl)
}

// unleashNoBlock reports whether the blocker at b is refused by its Unleash
// keyword: it carries the printed keyword and at least one +1/+1 counter.
func (s *State) unleashNoBlock(k *prep, b Slot) bool {
	return k.t.Cards[s.cardAt(b)].Unleash && s.perms[b].ctrN(s, int(kop.CtrP1P1)) > 0
}
