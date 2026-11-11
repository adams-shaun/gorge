package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplCopyFirstToken: "the first time you would create one or more tokens each
// turn, you may instead create that many tokens that are copies of equipped
// creature / enchanted permanent" (Mirrormind Crown, Moonlit Meditation; gorge
// replacement_token.go poseChosenTokenReplacement).
//
// gorge matches the replacement once per TokenCreate event (every token of an
// effect is its own event) and reads its CheckSVar$ gate, no token its controller
// owns has entered the battlefield this turn, at each one. Where it holds the
// controller is asked once, decline first and the permanent the source is
// attached to second; accepting makes that token a copy. Either way the token
// enters, which closes the gate: the rest of a creation of several tokens is
// plain. The gate is the replacement's Repl.Cond over the per-add entry list
// (value_enteredledger.go), so the election is asked at most once per
// replacement per turn, and no State is kept for it.
//
// A CreateToken op without riders asks (tokCopyOp). Every other mint site, a
// second matching token replacement (gorge's CR 616.1 order ask), and a copy the
// kernel cannot enter at once (an Aura, a face-down permanent, one that asks
// as it enters) stall: StallTokRewrite.

// raGenTokCopy is the copy-instead election (ChGenTokCopy): decline, or the
// permanent the source is attached to (source = its object).
const raGenTokCopy = raGenManifestDread + 79

// tokCopyApplies reports the host of the ReplCopyFirstToken ability a of the
// permanent at b (controlled by p) when it applies now: the source is attached
// to a permanent and its gate holds.
func (s *State) tokCopyApplies(k *prep, b Slot, a *kop.Ability, p uint8) (host Slot, ok bool) {
	h := s.perms[b].attached
	if h == NoSlot || !s.occ.has(h) {
		return NoSlot, false
	}
	if a.Repl.Cond != 0 && !s.replCondHolds(k, b, p, a.Repl.Cond) {
		return NoSlot, false
	}
	return h, true
}

// tokCopyActive counts the ReplCopyFirstToken replacements p controls that
// apply to a creation now, and the first one's host.
func (s *State) tokCopyActive(k *prep, p uint8) (n int, host Slot) {
	g := k.gen
	host = NoSlot
	for m := s.tokSrcM(p); m.any(); m = m.rest() {
		b := m.first()
		if g.replBits[s.cardAt(b)]&rbDblTok == 0 || s.genLost(k, b) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(b)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplCopyFirstToken {
				continue
			}
			if h, ok := s.tokCopyApplies(k, b, a, p); ok {
				if n == 0 {
					host = h
				}
				n++
			}
		}
	}
	return n, host
}

// tokCopyOp is replTokensOp where a copy-instead replacement applies: nOther
// other token replacements match the creation (rewriters and multipliers, then
// adders na). It poses the election and parks the op, or stalls.
func (s *State) tokCopyOp(k *prep, in *kop.Instr, p uint8, tok CardID, n, nCopy int, host Slot, nOther, na int) (tc CardID, cnt int, ok, parked bool) {
	ro := &s.g.ro
	if nCopy > 1 || nOther > 0 || na > 0 || in.Op != kop.OpCreateToken || in.B.N != 0 || n > 255 ||
		ro.n > 0 || ro.on || ro.nca || s.g.nc.n > 0 || s.tokCopyHostStalls(k, host) {
		s.capFull(StallTokRewrite)
		return tok, 0, false, false
	}
	s.g.tok = tokOrd{on: true, p: p, left: uint8(n), tok: tok, card: tok, cnt: 1}
	s.g.tok.cand[3] = s.perms[host].obj
	s.res.ask, s.res.askP = raGenTokCopy, p
	return tok, 0, true, true
}

// tokCopyHostStalls reports a host whose copy the kernel cannot enter at once:
// an Aura (it asks what it enchants), a face-down permanent, or a card that
// asks as it enters (a colour or type choice, a clone election).
func (s *State) tokCopyHostStalls(k *prep, host Slot) bool {
	c := s.cardAt(host)
	return k.cards[c].types&kop.TAura != 0 || s.faceDown(k, host) || s.enterChoiceKind(k, c) != 0 ||
		k.gen.anyBecomeCopy && s.cloneAskKind(k, c) != 0
}

// tokCopyOptions builds the election's options: decline, then the host.
func (s *State) tokCopyOptions(d *Decision) {
	d.Sub = ChGenTokCopy
	d.add(MakeAction(AkChoose, 0, 0, 0))
	d.add(MakeAction(AkChoose, uint8(s.g.tok.cand[3]), 0, 1))
}

// tokCopyAnswer applies the election and runs the op again: its walk (the
// token-by-token decision of repl_tokorder.go, with no candidate) makes the
// first token a copy of the host when accepted, plain when declined, and the
// rest plain.
func (s *State) tokCopyAnswer(k *prep, a Action) {
	t := &s.g.tok
	if a.Mode() == 1 {
		t.ab[3] = 1
	}
	s.res.ask = raNone
	s.runProg(k)
}

// mintTokenCopy makes one token under p that is a copy of the permanent host
// (a copy minted as an untapped, non-attacking token: copyTokenEnterFrom).
// True when the board is full (a stall).
func (s *State) mintTokenCopy(k *prep, p uint8, host Obj) bool {
	if s.nearCapacity(k) {
		s.stall(StallBoard)
		return true
	}
	sl := s.objs[host].slot
	t := s.newToken(s.cardAt(sl), p)
	s.objs[t].of |= ofCopy
	e := &s.res.e
	was := e.ctrl
	e.ctrl = p
	s.copyTokenEnterFrom(k, &kop.Instr{}, t, NoSlot, sl, nil)
	e.ctrl = was
	return false
}
