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
// CR 614.12). A cast creature spell is asked earlier: gorge poses the choice
// at the spell's resolution with the spell still on the stack, so
// unleashPreKind makes it a pre-entry ask (s.g.pre, like the colour and type
// elections) and the entry applies the answer. The two options are gorge's, in its order: "yes" first (its
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
		if p := &s.g.pre; p.state == 2 && p.kind == akUnleash && p.obj == o {
			// a cast creature's election was asked with the spell still on the
			// stack (unleashPreKind); the answer lands as the entry counter
			if p.val == 1 {
				s.placeEntryCounters(k, sl, int(kop.CtrP1P1), 1, s.perms[sl].ctrl)
			}
			return
		}
		s.queueAskAt(sl, akUnleash, NoCard, 0, 0)
	}
}

// unleashPreKind is akUnleash when card c prints Unleash: gorge asks a cast
// creature's counter election at the spell's resolution, with the spell still
// on the stack (the same as-enters shape as the colour and type elections,
// CR 614.12), so the kernel poses it through the pre-entry ask (preAskFor)
// instead of queueing it after the entry; 0 otherwise.
func (s *State) unleashPreKind(k *prep, c CardID) uint8 {
	if k.unleash && k.t.Cards[c].Unleash {
		return akUnleash
	}
	return 0
}

// unleashTokKind is kind, or akUnleash when kind is 0 and the copy token of
// card c prints Unleash: gorge mints the copy outside every zone and parks its
// battlefield move on the counter ask (applyUnleashReplacement), so the kernel
// parks the minted copy the way an as-enters colour choice does
// (raGenTokChoice, s.g.pre) and the entry applies the answer
// (unleashEntryAsk).
func (s *State) unleashTokKind(k *prep, kind uint8, c CardID) uint8 {
	if kind != 0 {
		return kind
	}
	return s.unleashPreKind(k, c)
}

// unleashAskObj is the object the posed counter ask is about: a cast
// creature's or a parked copy token's election rides s.g.pre (asked before
// it enters); any other entry's is the queued ask[0].
func (s *State) unleashAskObj() Obj {
	if p := &s.g.pre; p.kind == akUnleash && (p.state == 1 || s.res.ask == raGenTokChoice) {
		return p.obj
	}
	return s.perms[s.g.ask[0].sl].obj
}

// unleashOptions fills the queued as-enters ask: the counter election over
// the entering permanent, gorge's two options in its order ("counter" is
// option 0, the deterministic bot's answer).
func (s *State) unleashOptions(d *Decision, o Obj) {
	d.Sub = ChGenUnleash
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
// keyword: it carries the printed keyword and at least one +1/+1 counter. The
// refusal is the keyword's own CantBlock static (Forge lowers it as one, Mode$
// CantBlock beside the keyword), so a lose-all-abilities effect that removes
// the printed keyword lifts it: the counter stays, the creature blocks (CR
// 613.1f; gorge combat/block.go reads the derived keyword list the same way).
// Unleash has no KwMask bit and a granted Unleash is refused at lowering, so
// no layer-6 grant can bring the refusal back.
func (s *State) unleashNoBlock(k *prep, b Slot) bool {
	if !k.t.Cards[s.cardAt(b)].Unleash || s.perms[b].ctrN(s, int(kop.CtrP1P1)) == 0 {
		return false
	}
	if _, lost := s.lossAt(k, b); lost {
		return false // Stasis Field: the printed Unleash is gone with all abilities
	}
	return true
}
