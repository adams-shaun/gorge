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

// tokCopyOp is replTokensOp where a copy-instead replacement is the creation's
// only replacement (the walk in repl_tokorder.go poses the order ask when a
// multiplier matches too). It poses the election and parks the op, or stalls.
func (s *State) tokCopyOp(k *prep, in *kop.Instr, p uint8, tok CardID, n, nCopy int, host Slot, na int) (tc CardID, cnt int, ok, parked bool) {
	ro := &s.g.ro
	if nCopy > 1 || na > 0 || !tokCopyRiders(in) || n > 255 ||
		ro.n > 0 || ro.on || ro.nca || s.g.nc.n > 0 || s.tokCopyHostStalls(k, host) {
		s.capFull(StallTokRewrite)
		return tok, 0, false, false
	}
	s.g.tok = tokOrd{on: true, p: p, left: uint8(n), tok: tok, card: tok, cnt: 1}
	s.g.tok.cand[0] = s.perms[host].obj
	s.res.ask, s.res.askP = raGenTokCopy, p
	return tok, 0, true, true
}

// tokCopyRiders reports whether the creation's riders are ones the copy takes
// as a plain mint does (gorge applyTokenMintRiders lands every rider on each
// token the replaced plan mints, the copy included): enters tapped, haste
// until end of turn, remembered or imprinted, counted, and the end-step
// sacrifice or exile. A comma-listed script, a Role mint and a token that
// enters attacking stay unmodelled.
func tokCopyRiders(in *kop.Instr) bool {
	switch in.Op {
	case kop.OpCreateToken:
		return in.B.N&^(kop.TrTapped|kop.TrHasteEOT) == 0
	case kop.OpCreateTokenEx:
		return in.B.N&(kop.TokAttach|kop.TokAttacking|kop.TokAttackAsk) == 0
	}
	return false
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
	d.add(MakeAction(AkChoose, uint8(s.tokCopyHost(s.k(), &s.g.tok)), 0, 1))
}

// tokCopyAnswer applies the election. The lone election (tokCopyOp) runs the
// op again; the contested walk (repl_tokorder.go) continues with the copy
// candidate consumed, the accepted copy minted for every token the walk's
// count names, and later multipliers still applied.
func (s *State) tokCopyAnswer(k *prep, a Action) {
	t := &s.g.tok
	if a.Mode() == 1 {
		t.setCopyTaken(true)
	}
	if t.nc == 0 {
		s.res.ask = raNone
		s.runProg(k)
		return
	}
	if s.tokAfterWalk(k, true) {
		return
	}
	s.runProg(k)
}

// mintTokenCopies makes n tokens under p that are copies of the permanent
// host with the creation in's riders (mintTokenCopy). True when the board is
// full (a stall).
func (s *State) mintTokenCopies(k *prep, in *kop.Instr, p uint8, host Obj, n int) bool {
	for i := 0; i < n; i++ {
		if s.mintTokenCopy(k, in, p, host) {
			return true
		}
	}
	return false
}

// mintTokenCopy makes one token under p that is a copy of the permanent host
// with the creation in's riders (tokCopyRiders; copyTokenEnterFrom).
// True when the board is full (a stall).
func (s *State) mintTokenCopy(k *prep, in *kop.Instr, p uint8, host Obj) bool {
	if s.nearCapacity(k) {
		s.stall(StallBoard)
		return true
	}
	sl := s.objs[host].slot
	t := s.newToken(s.cardAt(sl), p)
	s.objs[t].of |= ofCopy
	var cp kop.Instr
	if in.Op == kop.OpCreateToken && in.B.N&kop.TrTapped != 0 || in.Op == kop.OpCreateTokenEx && in.B.N&kop.TokTapped != 0 {
		cp.B.N = kop.CpTapped
	}
	if in.Op == kop.OpCreateTokenEx {
		s.g.tokMade++
		if in.B.N&kop.TokRememberObj != 0 {
			s.remember(t)
		}
		if in.B.N&kop.TokImprintObj != 0 {
			s.imprint(t) // zone_imprint.go
		}
	}
	e := &s.res.e
	was := e.ctrl
	e.ctrl = p
	s.copyTokenEnterFrom(k, &cp, t, NoSlot, sl, nil)
	e.ctrl = was
	if in.Op == kop.OpCreateTokenEx && in.B.N&(kop.TokSacAtEnd|kop.TokExileAtEnd) != 0 {
		s.tokenAtEOT(in, t)
	}
	if nsl := s.objs[t].slot; in.Op == kop.OpCreateToken && in.B.N&kop.TrHasteEOT != 0 && s.objs[t].zone == kop.ZoneBattlefield {
		s.perms[nsl].eotKw |= kop.KwHaste
		s.touchSl(nsl)
		s.promoKw(k, nsl, kop.KwHaste)
	}
	s.res.lastTok = t
	return false
}
