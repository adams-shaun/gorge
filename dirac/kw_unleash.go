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
// (EntryCounterGrantsForFace, beside Riot's). A resolution's move parks the
// same way (unleashReturnKind through cloneReturnPark: the card waits in its
// zone, and the rest of the resolution follows the answer, CR 614.12a). Any
// other entry poses the ask from enterGen (before the entrant's ETB triggers)
// over the generated ask queue (akUnleash, posed at the next priority like the
// other as-enters asks) and, on "yes", puts the counter the way the entry
// counters are put (placeEntryCounters: the would-enter-board settlement,
// CR 614.12). A cast creature spell is asked earlier: gorge poses the choice
// at the spell's resolution with the spell still on the stack, so
// unleashPreKind makes it a pre-entry ask (s.g.pre, like the colour and type
// elections) and the entry applies the answer. The two options are gorge's, in its order: "yes" first (its
// option 0, the counter), then "no". The ask reads the printed keyword only,
// both in gorge (faceETBChoice's and applyUnleashReplacement's face read,
// beside enteringRiot's derived one) and in the kernel: a GRANTED Unleash
// asks no entry election, it only gates blocking (below).
//
// The can't-block half: canBlock refuses a blocker whose card carries
// Unleash and at least one +1/+1 counter, from any source, beside the
// Suspected gate (gorge combat/block.go reads the same two facts beside
// Suspected).
//
// A GRANTED Unleash is kop.Static.Unleash on a Continuous static (kgen's
// splitStaticUnleashGrant, Tesak, Judith's Hellhound's other Dogs): it
// grants the block gate only, read beside CardDef.Unleash by
// State.unleashPermGranted as gorge's derived keyword list reads it
// (combat/board.go HasKW(blocker, KWUnleash), layer-6 included); the entry
// election is printed-only, as gorge reads it off the printed face. Unleash has no KwMask bit, so the grant is its own static
// field (the arena takes KwMask bits only, which are full). A grant older
// than a loss of the recipient's abilities grants nothing (the layer-6
// timestamp order, like riotPermGranted's).
//
// Cold and out of line: a table with no printed Unleash takes none of the
// entry-ask paths (prep.unleash, a prep bool like ascend so the hot hooks
// cost one byte load); a table with no grant takes none of the block-gate
// walk (prep.unleashGrant, like riotGrant).

// KwUnleash is the manifest's KwID for the keyword (dirac/manifest/KwID/
// KwUnleash, state: code 0): the code this mechanic implements; the printed
// carrier is CardDef.Unleash and the grant carrier is kop.Static.Unleash
// (the arena takes KwMask bits only, which are full).
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

// tableGrantsUnleash reports whether some static of t grants Unleash.
func tableGrantsUnleash(t *kop.Table) bool {
	scan := func(abs []kop.Ability) bool {
		for j := range abs {
			if a := &abs[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Unleash {
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

// unleashEntryAsk queues the CR 702.86a as-enters counter ask of the entrant
// at sl when its card prints Unleash (a granted Unleash asks nothing, as
// gorge reads this choice off the printed face): gorge parks the battlefield
// move on this ask (applyUnleashReplacement, posed on every face-up entry),
// so it is queued at the entry's start, ahead of the entry counters' own
// asks. A face-down manifest or cloak entry is a vanilla 2/2 and poses
// nothing (CR 708.5, gorge's IsFaceDownEntry guard). A queued ask is posed at
// the next priority, before the entrant's ETB triggers resolve.
func (s *State) unleashEntryAsk(k *prep, sl Slot, o Obj) {
	if k.unleash && k.t.Cards[s.objs[o].card].Unleash {
		if p := &s.g.pre; p.state == 2 && p.kind == akUnleash && p.obj == o {
			// the election was asked before the entry (a cast spell on the
			// stack, a parked move's card in its zone); the answer lands as the
			// entry counter and is spent, so the move's next card can park
			p.state = 0
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

// unleashReturnKind is akUnleash when a resolution's move of card c onto the
// battlefield parks on the counter election: gorge asks it with the card
// still in its zone, ahead of the rest of the resolution (CR 614.12a; Wakanda
// Forever!'s hand search follows the answer for the Tesak its battlefield
// search found). Only the moves rerunReturn resumes park; a face-down return
// asks nothing (CR 708.5).
func (s *State) unleashReturnKind(k *prep, c CardID) uint8 {
	kind, prog := s.unleashPreKind(k, c), s.prog(k)
	if kind == 0 || int(s.res.pc) >= len(prog) {
		return 0
	}
	switch in := &prog[s.res.pc]; in.Op {
	case kop.OpReturn:
		if in.C&kop.RetFaceDown != 0 {
			return 0
		}
		return kind
	case kop.OpReturnEx, kop.OpSearchEx, kop.OpChooseMove, kop.OpDigEx, kop.OpDigUntil,
		kop.OpChangeZoneAll, kop.OpChangeZoneAllEx, kop.OpNinjutsu:
		return kind
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

// unleashPermGranted reports whether the permanent at sl is granted Unleash
// by a live Continuous static: the layer-6 grant is read like riot's
// (kw_riot.go), a grant older than a loss of the recipient's abilities
// granting nothing.
func (s *State) unleashPermGranted(k *prep, sl Slot) bool {
	if !k.unleashGrant || DebugNoGrantedUnleash {
		return false
	}
	lts, lost := s.lossAt(k, sl)
	granted := false
	s.eachStatic(k, func(st statSrc) bool {
		ab := st.a
		if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatContinuous || !ab.Static.Unleash ||
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

// unleashAskObj is the object the posed counter ask is about: a cast
// creature's, a parked move's or a parked copy token's election rides s.g.pre
// (asked before it enters); any other entry's is the queued ask[0].
func (s *State) unleashAskObj() Obj {
	if p := &s.g.pre; p.kind == akUnleash && (p.state == 1 || s.res.ask == raGenTokChoice || s.res.ask == raGenRetChoice) {
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

// DebugNoGrantedUnleash (TestHostedCmdrGrantedUnleashMutation): the kernel
// reads no granted Unleash, so a granted election gorge asks is never posed
// and a granted carrier with a counter blocks.
var DebugNoGrantedUnleash bool

// unleashNoBlock reports whether the blocker at b is refused by its Unleash
// keyword: it carries the keyword, printed or granted, and at least one
// +1/+1 counter. The refusal is the keyword's own CantBlock static (Forge
// lowers it as one, Mode$ CantBlock beside the keyword), so a
// lose-all-abilities effect that removes the keyword lifts it: the counter
// stays, the creature blocks (CR 613.1f; gorge combat/block.go reads the
// derived keyword list the same way). A granted Unleash read is the layer-6
// walk (unleashPermGranted), which also orders the grant against a loss of
// abilities by timestamp.
func (s *State) unleashNoBlock(k *prep, b Slot) bool {
	if s.perms[b].ctrN(s, int(kop.CtrP1P1)) == 0 {
		return false
	}
	if k.t.Cards[s.cardAt(b)].Unleash {
		if _, lost := s.lossAt(k, b); !lost {
			return true
		}
		// Stasis Field: the printed Unleash is gone with all abilities, but a
		// grant newer than the loss holds (CR 613.7), read below
	}
	return s.unleashPermGranted(k, b)
}
