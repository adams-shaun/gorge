package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// gorge's CR 616.1 order ask on a token creation (engine/rules
// replacement_token.go driveTokenReplacements). Two or more token
// replacements match one token and their bodies do not all commute (a script
// rewriter, Holo-Engineer, composes differently with a multiplier or with
// another rewriter than a multiplier does with a multiplier), so gorge asks
// the token's creator which applies first, applies the pick at its scan
// point and re-asks while two or more non-commuting candidates remain. Every
// token of an effect is its own TokenCreate event (effects/token.go), so each
// asks on its own, and its answer mints before the next token's ask.
//
// The CreateToken ops walk such a creation token by token (execCreateToken):
// tokOrd holds the walk, ro the posed ask (the candidates are the sources of
// the replacements in gorge's scan order, replOrderOptions offers them), and
// the answer applies the pick and, when the walk is over, re-runs the op's
// instruction with the decided token (tokAnswer; the CR 508.4 defender ask
// precedent, raGenEnterDefender). A creator who has left the game makes no
// choices (CR 800.4a): the walk applies the candidates in scan order and no
// ask is posed. The other mint sites (amass, incubate, empower, endure,
// recruit, the copies) read the rewritten card but cannot suspend: a creation
// contested at one of them is still the declared StallTokRewrite.

// tokOrd is a CreateToken op's walk over a contested creation.
type tokOrd struct {
	on     bool   // the op is deciding its tokens one by one
	ready  bool   // the current token is decided: card, cnt
	minted bool   // an earlier token of the op has been made
	p      uint8  // the creator, who answers
	left   uint8  // tokens of the op still to decide, the current one included
	nc     uint8  // the candidates
	used   uint8  // candidates applied to the current token (bit per cand)
	tok    CardID // the card the op names
	card   CardID // the current token's card so far
	cnt    int16  // how many of it so far
	ab     [4]uint8
	cand   [4]Obj // the replacements' sources, in gorge's scan order
}

// tokCands collects the token replacements that match a token of card tok
// created under p, in gorge's scan order (p's battlefield in entry order), into
// t, and returns how many are rewriters. over is set when there are more than
// the ask holds.
func (s *State) tokCands(k *prep, p uint8, tok CardID, t *tokOrd) (rew int, over bool) {
	g := k.gen
	var bf [MaxPerm]Slot
	for _, b := range bf[:s.battlefieldOrder(p, s.statM.and(s.ctrlM[p]), bf[:], 0)] {
		if g.replBits[s.cardAt(b)]&rbDblTok == 0 || s.genLost(k, b) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(b)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement {
				continue
			}
			switch a.Repl.Kind {
			case kop.ReplRewriteTokens:
				if DebugTokRewriteOff {
					continue
				}
				rew++
			case kop.ReplDoubleTokens:
				if a.Repl.Filter != 0 && !s.matchCardGen(k, a.Repl.Filter, tok, p, p, NoObj) {
					continue
				}
			default:
				continue
			}
			if int(t.nc) == len(t.cand) {
				return rew, true
			}
			t.cand[t.nc], t.ab[t.nc] = s.perms[b].obj, uint8(i)
			t.nc++
		}
	}
	return rew, false
}

// tokAbility is candidate j's replacement ability.
func (s *State) tokAbility(k *prep, j int) *kop.Ability {
	t := &s.g.tok
	return &k.t.Cards[s.cardAt(s.objs[t.cand[j]].slot)].Abilities[t.ab[j]]
}

// tokApply applies candidate j to the current token: a rewriter makes it its
// card, a multiplier multiplies the number of tokens when its filter still
// matches the card (gorge re-checks each replacement's gate per planned mint,
// tokenReplacementMatchesMint).
func (s *State) tokApply(k *prep, j int) {
	t := &s.g.tok
	a := s.tokAbility(k, j)
	switch a.Repl.Kind {
	case kop.ReplRewriteTokens:
		t.card = CardID(a.Repl.C)
	case kop.ReplDoubleTokens:
		if a.Repl.Filter != 0 && !s.matchCardGen(k, a.Repl.Filter, t.card, t.p, t.p, NoObj) {
			return
		}
		x := 2
		if a.Repl.A > 2 {
			x = int(a.Repl.A)
		}
		if t.cnt < 1<<8 {
			t.cnt *= int16(x)
		}
	}
}

// tokWalk applies the unapplied candidates in scan order until two or more
// of them that do not all commute remain and ask is set (true: the ask is
// due) or none remain. The remaining suffix is re-collected after each pick
// as gorge's walk does: a lone rewriter among two or more candidates does
// not commute (tokenReplacementsCommute), multipliers alone do.
func (s *State) tokWalk(k *prep, ask bool) bool {
	t := &s.g.tok
	for {
		left, rew, first := 0, 0, -1
		for j := 0; j < int(t.nc); j++ {
			if t.used&(1<<j) != 0 {
				continue
			}
			if first < 0 {
				first = j
			}
			left++
			if s.tokAbility(k, j).Repl.Kind == kop.ReplRewriteTokens {
				rew++
			}
		}
		if left == 0 {
			return false
		}
		if ask && left > 1 && rew > 0 {
			return true
		}
		t.used |= 1 << first
		s.tokApply(k, first)
	}
}

// tokPose starts the next token of the walk: true when its order ask is
// posed (the op suspends), false when it is decided without one.
func (s *State) tokPose(k *prep) bool {
	t := &s.g.tok
	t.used, t.card, t.cnt = 0, t.tok, 1
	if s.tokWalk(k, !s.pl[t.p].lost) {
		s.g.ro.replHdr = replHdr{n: t.nc, used: t.used, p: t.p, cand: t.cand}
		s.res.ask, s.res.askP = raGenReplOrder, t.p
		return true
	}
	t.ready = true
	return false
}

// tokAnswer is the creator's pick: it applies at its scan point, the walk
// goes on, and when it ends the op's instruction runs again with the token.
func (s *State) tokAnswer(k *prep, a Action) {
	t, ro := &s.g.tok, &s.g.ro
	j := a.Mode()
	if j >= t.nc || t.used&(1<<j) != 0 {
		return // not an offered option
	}
	t.used |= 1 << j
	s.tokApply(k, int(j))
	if s.tokWalk(k, true) {
		ro.used = t.used // asked again
		return
	}
	ro.replHdr = replHdr{}
	t.ready = true
	s.res.ask = raNone
	s.runProg(k)
}

// replTokensOp is replTokensCard for the CreateToken ops, which can suspend:
// the card the n tokens are and how many, or parked when the creation is
// contested (the walk is then in s.g.tok, decided token by token) or not ok
// when the game stalled.
func (s *State) replTokensOp(k *prep, in *kop.Instr, p uint8, tok CardID, n int) (tc CardID, cnt int, ok, parked bool) {
	g := k.gen
	if !g.anyTokRew || g.anyRepl&rbDblTok == 0 || n <= 0 || p > 1 {
		tc, cnt, ok = s.replTokensCard(k, p, tok, n)
		return tc, cnt, ok, false
	}
	var t tokOrd
	rew, over := s.tokCands(k, p, tok, &t)
	if over {
		s.capFull(StallReplCands) // loud: never truncate the candidates
		return tok, 0, false, false
	}
	if _, na, over := s.tokAddCards(k, p, tok); over {
		s.capFull(StallReplCands)
		return tok, 0, false, false
	} else if na > 0 {
		// an adder beside another matching replacement is gorge's CR 616.1 order
		// ask (an adder does not commute with a multiplier or a rewriter); a
		// Role mint (TokAttach) is no plain mint to add beside
		// a listed script (TrList) shares a creation event with the ops beside it
		if t.nc > 0 || in.Op == kop.OpCreateTokenEx && in.B.N&kop.TokAttach != 0 ||
			in.Op == kop.OpCreateToken && in.B.N&kop.TrList != 0 {
			s.capFull(StallTokRewrite)
			return tok, 0, false, false
		}
		return tok, n, true, false
	}
	if rew == 0 || t.nc < 2 {
		tc, cnt, ok = s.replTokensCard(k, p, tok, n)
		return tc, cnt, ok, false
	}
	ro := &s.g.ro
	if n > 255 || ro.n > 0 || ro.on || ro.nca || s.g.nc.n > 0 || in.Op == kop.OpCreateTokenEx && in.B.N&kop.TokAttach != 0 {
		// a decision is outstanding, or a Role mint (repl_tokrewrite.go)
		s.capFull(StallTokRewrite)
		return tok, 0, false, false
	}
	t.on, t.p, t.left, t.tok = true, p, uint8(n), tok
	s.g.tok = t
	if s.tokPose(k) {
		return tok, 0, true, true
	}
	return tok, 0, true, false
}

// execCreateToken is the CreateToken ops (execGen). The tokens are the
// op's card and count after the token replacements; a contested creation is
// decided one token at a time (replTokensOp), the op suspending on each ask
// and running again to make the token the answer decided.
func (s *State) execCreateToken(k *prep, in *kop.Instr) (ok, done bool) {
	in0 := in // a rewriter swaps the card on a copy
	t := &s.g.tok
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		p = s.res.e.ctrl
	}
	var tc CardID
	var n int
	if !t.on {
		if in.Op == kop.OpCreateTokenEx && s.tokenDefenderAsk(k, in, p) {
			return false, true // the CR 508.4 defender ask (filter_combatwords.go)
		}
		var good, parked bool
		tc, n, good, parked = s.replTokensOp(k, in, p, CardID(in.C), s.value(k, in.A))
		if parked {
			return false, true // the order ask
		}
		if !good {
			return true, true // StallTokRewrite
		}
	}
	if t.on {
		tc, n, t.ready = t.card, int(t.cnt), false
	}
	if tc != CardID(in.C) {
		if in.Op == kop.OpCreateTokenEx && in.B.N&kop.TokAttach != 0 {
			// a Role mint rewritten: gorge mints the rewriter's card as a plain
			// script mint (not an Aura, so nothing is withheld); the attach path
			// would look for a host for the non-Aura card (repl_tokrewrite.go)
			s.capFull(StallTokRewrite)
			return true, true
		}
		rw := *in // a rewriter: the mint makes its card (repl_tokrewrite.go)
		rw.C = int16(tc)
		in = &rw
	}
	if in.Op == kop.OpCreateTokenEx {
		if !t.minted {
			s.g.tokMade = 0
		}
		if in.B.N&kop.TokAttach != 0 {
			if ok, done := s.createAttachedToken(k, in, p, n); done {
				return ok, true
			}
		}
	}
	var cs [4]CardID
	na := 0
	if !t.on && n > 0 {
		// an adder's token enters with the op's own (it is alone: replTokensOp)
		cs, na, _ = s.tokAddCards(k, p, CardID(in0.C))
	}
	if na > 0 {
		if s.createTokensHeld(k, in, p, n, cs[:na]) {
			return true, true
		}
	} else if s.mintTokens(k, in, p, n) {
		return true, true
	}
	if t.on {
		t.minted = true
		if t.left--; t.left > 0 {
			if s.tokPose(k) {
				return false, true
			}
			return s.execCreateToken(k, in0)
		}
		*t = tokOrd{}
	}
	s.g.tokDefSet = false
	return true, true
}

// mintTokens makes n tokens of in.C under p with the op's riders; true when
// the board is full (a stall).
func (s *State) mintTokens(k *prep, in *kop.Instr, p uint8, n int) bool {
	if n > 1 {
		return s.createTokensHeld(k, in, p, n, nil)
	}
	for i := 0; i < n; i++ {
		if s.nearCapacity(k) {
			s.stall(StallBoard)
			return true
		}
		o := s.newToken(CardID(in.C), p)
		tapped := in.B.N&kop.TrTapped != 0
		if in.Op == kop.OpCreateTokenEx {
			tapped = in.B.N&kop.TokTapped != 0
			s.g.tokMade++
			if in.B.N&kop.TokRememberObj != 0 {
				s.remember(o)
			}
			if in.B.N&kop.TokImprintObj != 0 {
				s.imprint(o) // zone_imprint.go
			}
		}
		sl := s.enter(k, o, p, tapped, kop.ZoneLibrary)
		if in.Op == kop.OpCreateTokenEx && in.B.N&(kop.TokSacAtEnd|kop.TokExileAtEnd) != 0 {
			s.tokenAtEOT(in, o)
		}
		if in.Op == kop.OpCreateTokenEx && in.B.N&kop.TokAttacking != 0 {
			s.tokenAttacks(k, in, sl, p) // filter_combatwords.go
		}
		if k.gen.anyTokCr {
			s.fireTokenCreated(k, sl)
		}
		if in.Op == kop.OpCreateToken && in.B.N&kop.TrHasteEOT != 0 {
			s.perms[sl].eotKw |= kop.KwHaste
			s.touchSl(sl)
			s.promoKw(k, sl, kop.KwHaste)
		}
		s.res.lastTok = o
	}
	return false
}

// tokAddCards collects the ReplAddToken replacements p controls that match a
// token of card tok, the card each adds, in battlefield order. over is set
// when there are more than the buffer holds.
func (s *State) tokAddCards(k *prep, p uint8, tok CardID) (cs [4]CardID, n int, over bool) {
	g := k.gen
	for m := s.tokSrcM(p); m.any(); m = m.rest() {
		b := m.first()
		if g.replBits[s.cardAt(b)]&rbDblTok == 0 || s.genLost(k, b) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(b)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplAddToken {
				continue
			}
			if a.Repl.Filter != 0 && !s.matchCardGen(k, a.Repl.Filter, tok, p, p, NoObj) {
				continue
			}
			if n == len(cs) {
				return cs, n, true
			}
			cs[n] = CardID(a.Repl.C)
			n++
		}
	}
	return cs, n, false
}

// mintTokAdds makes the extra token of each ReplAddToken that matched a
// creation of n tokens of card tok under p (one per creation event, gorge's
// fixed Amount$ AddToken block: replacement_token.go
// applyTokenReplacementToPlan appends it after the event's own tokens). It is
// one more mint of the effect, so it takes the effect's riders in (entering
// tapped, remembered, imprinted: gorge's Token effect applies them to every
// planned mint). True when the board is full (a stall).
func (s *State) mintTokAdds(k *prep, in *kop.Instr, p uint8, tok CardID, n int) bool {
	if n <= 0 || !k.gen.anyTokRew || k.gen.anyRepl&rbDblTok == 0 || p > 1 {
		return false
	}
	cs, na, _ := s.tokAddCards(k, p, tok)
	for _, c := range cs[:na] {
		rw := *in
		rw.C = int16(c)
		if s.mintTokens(k, &rw, p, 1) {
			return true
		}
	}
	return false
}
