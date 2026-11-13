package search

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Per-creature attack declarations (Options.PerCreature), as MageZero's
// search declares attacks one creature at a time. A searched attackers
// decision becomes a chain of points, one per eligible creature, whose
// candidates are {not attacking} and each defender the creature can
// attack. A "not attacking" candidate plays no action; an attack plays one
// AkAttack; AkDone closes the chain.

// ParsePerCreature parses the spec value of :percreature.
func ParsePerCreature(v string) (uint8, error) {
	switch v {
	case "attack":
		return PerCreatureAttack, nil
	case "block":
		return PerCreatureBlock, nil
	case "both", "":
		return PerCreatureAttack | PerCreatureBlock, nil
	}
	return 0, fmt.Errorf("percreature=%q: want attack, block or both", v)
}

// validatePerCreature refuses PerCreature's bad values (Options.Validate).
func (o *Options) validatePerCreature() error {
	if o.PerCreature&^(PerCreatureAttack|PerCreatureBlock) != 0 {
		return fmt.Errorf("search: PerCreature %d has a bit outside attack (1) and block (2)", o.PerCreature)
	}
	return nil
}

func offersDone(d *dirac.Decision) bool {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkDone {
			return true
		}
	}
	return false
}

// chainKind is the action kind a chain over d walks.
func chainKind(d *dirac.Decision) dirac.ActionKind {
	if d.Kind == dirac.DkBlockers {
		return dirac.AkBlock
	}
	return dirac.AkAttack
}

// chainOn reports whether the decision kind d.Kind is searched per creature.
func (o *Options) chainKindOn(k dirac.DecisionKind) bool {
	return k == dirac.DkAttackers && o.PerCreature&PerCreatureAttack != 0 ||
		k == dirac.DkBlockers && o.PerCreature&PerCreatureBlock != 0
}

func hasAttackBy(d *dirac.Decision, x uint8) bool {
	ck := chainKind(d)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == ck && a.Source() == dirac.TCode(x) {
			return true
		}
	}
	return false
}

// chainAfter writes the distinct attack sources of d that follow x in
// option order to out and returns how many (a creature's offers are
// contiguous).
func chainAfter(d *dirac.Decision, x uint8, out *[MaxPerm]uint8) int {
	n, seenX, last := 0, false, -1
	ck := chainKind(d)
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != ck || int(a.Source()) == last {
			continue
		}
		last = int(a.Source())
		if a.Source() == dirac.TCode(x) {
			seenX = true
		} else if seenX && n < len(out) {
			out[n] = uint8(a.Source())
			n++
		}
	}
	return n
}

// isChainPoint reports whether pt is a per-creature point: its bot
// candidate is empty or an attack (a classic declaration ends in AkDone).
func isChainPoint(pt *point) bool {
	return pt.n > 0 && (pt.off[1] == pt.off[0] || dirac.Action(pt.acts[pt.off[1]-1]).Kind() != dirac.AkDone)
}

// chainObj is the creature a chain point decides: the source of a longest
// candidate's last act.
func chainObj(pt *point) uint8 {
	best, x := 0, uint8(0)
	for i := 0; i < pt.n; i++ {
		if l := int(pt.off[i+1] - pt.off[i]); l > best {
			best, x = l, uint8(dirac.Action(pt.acts[pt.off[i+1]-1]).Source())
		}
	}
	return x
}

// chainCands fills pt with the candidates of one creature's point. It
// reports false, leaving pt alone, at a chain start whose decision
// withholds AkDone (the classic point is built there).
func (sr *Searcher) chainCands(s *dirac.State, d *dirac.Decision, bot, limit int, pt *point) bool {
	if d.Kind == dirac.DkBlockers {
		return sr.chainBlockCands(s, d, bot, limit, pt)
	}
	var x uint8
	switch {
	case sr.rootBuild && sr.chainRoot != 0:
		x = sr.chainRoot - 1
	case sr.chainCur != 0:
		x = sr.chainCur - 1
	default:
		if !offersDone(d) {
			return false
		}
		found := false
		for i := 0; i < int(d.N) && !found; i++ {
			if a := d.Options[i]; a.Kind() == dirac.AkAttack {
				x, found = uint8(a.Source()), true
			}
		}
		if !found {
			return true
		}
	}
	sr.chainX = x
	var botSet [MaxPerm]uint32
	nb := sr.botDeclaration(s, d, bot, &botSet)
	bp, has := uint32(0), false
	for _, a := range botSet[:nb] {
		if dirac.Action(a).Source() == dirac.TCode(x) {
			bp, has = a, true
			break
		}
	}
	if has {
		pt.addSeq([]uint32{bp})
		if !s.MustAttack(dirac.Obj(x)) && pt.n < limit {
			pt.addSeq(nil)
		}
	} else {
		pt.addSeq(nil)
	}
	for i := 0; i < int(d.N) && pt.n < limit; i++ {
		a := d.Options[i]
		if a.Kind() != dirac.AkAttack || a.Source() != dirac.TCode(x) || (has && uint32(a) == bp) {
			continue
		}
		pt.addSeq([]uint32{uint32(a)})
	}
	return true
}

// chainClose ends a chain's declaration in w: AkDone if offered, else the
// bot's remaining picks and then AkDone.
func (sr *Searcher) chainClose(w *world) {
	done := dirac.MakeAction(dirac.AkDone, 0, 0, 0)
	k0, p0 := w.d.Kind, w.d.Player
	if Offered(&w.s, done, k0, p0, &sr.td) {
		w.s.Apply(done)
		return
	}
	sr.St.ChainFallbacks++
	w.s.StepInto(&w.d)
	bot := sr.pickBot(&w.s, &w.d)
	var picks [MaxPerm]uint32
	n := sr.botDeclaration(&w.s, &w.d, bot, &picks)
	for _, a := range picks[:n] {
		if Offered(&w.s, dirac.Action(a), k0, p0, &sr.td) {
			w.s.Apply(dirac.Action(a))
		}
	}
	if Offered(&w.s, done, k0, p0, &sr.td) {
		w.s.Apply(done)
	}
}

// chainPlay is play for a per-creature point (done reports it handled the
// candidate): apply it, then build the next creature's point, or close the
// declaration and advance.
//
//go:noinline
func (sr *Searcher) chainPlay(w *world, pt *point, idx int, next *point) (ok, undone, done bool) {
	if !sr.Opts.chainKindOn(w.d.Kind) || !isChainPoint(pt) {
		return false, false, false
	}
	x := chainObj(pt)
	var after [MaxPerm]uint8
	na := chainAfter(&w.d, x, &after)
	u0 := w.s.Undos()
	k0, p0, turn := w.d.Kind, w.d.Player, w.s.Turn()
	for j := pt.off[idx]; j < pt.off[idx+1]; j++ {
		a := dirac.Action(pt.acts[j])
		if j > pt.off[idx] && !Offered(&w.s, a, k0, p0, &sr.td) {
			continue
		}
		w.s.Apply(a)
	}
	if w.s.Undos() != u0 {
		return false, true, true
	}
	w.plies++
	w.s.StepInto(&w.d)
	if w.d.Kind != k0 || w.d.Player != p0 || w.s.Turn() != turn {
		return sr.advance(w, next), false, true
	}
	react := sr.Opts.InstWin == InstWinReact
	if w.d.Kind == dirac.DkBlockers {
		for _, c := range after[:na] {
			if !hasAttackBy(&w.d, c) {
				continue
			}
			if react {
				sr.curReact = &w.react
				w.react.observe(&w.s, sr.seat)
			}
			next.n, sr.nForced = 0, 0
			sr.chainCur = c + 1
			bot := sr.pickBot(&w.s, &w.d)
			built := false
			if w.d.N < 2 {
				// buildPoint refuses a lone option: only the forced tail is left
				sr.chainCands(&w.s, &w.d, bot, MaxCands, next)
			} else {
				built = sr.buildPoint(&w.s, &w.d, bot, next)
			}
			sr.chainCur = 0
			if built {
				sr.St.ChainPoints++
				return true, false, true
			}
			if next.n == 0 {
				sr.chainApplyTail(&w.s, &w.d, k0, p0)
				if w.d.Kind != k0 || w.d.Player != p0 || w.s.Turn() != turn {
					return sr.advance(w, next), false, true
				}
			}
			break // the scan covered every blocker after c
		}
		sr.chainClose(w)
		return sr.advance(w, next), false, true
	}
	for _, c := range after[:na] {
		if !hasAttackBy(&w.d, c) {
			continue
		}
		if react {
			sr.curReact = &w.react
			w.react.observe(&w.s, sr.seat)
		}
		next.n = 0
		sr.chainCur = c + 1
		bot := sr.pickBot(&w.s, &w.d)
		built := sr.buildPoint(&w.s, &w.d, bot, next)
		sr.chainCur = 0
		if built {
			sr.St.ChainPoints++
			return true, false, true
		}
		if next.n != 1 {
			break
		}
		// a lone candidate (a creature that must attack): played unsearched
		for j := next.off[0]; j < next.off[1]; j++ {
			w.s.Apply(dirac.Action(next.acts[j]))
		}
		w.s.StepInto(&w.d)
		if w.d.Kind != k0 || w.d.Player != p0 || w.s.Turn() != turn {
			return sr.advance(w, next), false, true
		}
	}
	sr.chainClose(w)
	return sr.advance(w, next), false, true
}

// chainSearch is Search's root for a per-creature attackers decision: one
// search per creature with two or more candidates, the whole declaration
// returned. It reports nil, changing nothing, when the decision is not a
// chain (the caller runs the classic search).
func (sr *Searcher) chainSearch(seatSeed uint64) *Result {
	d := &sr.rootD
	if !sr.Opts.chainKindOn(d.Kind) || sr.Opts.Sims <= 0 || !offersDone(d) {
		return nil
	}
	if k := sr.kindAt(&sr.root, d); k != kindAttackers && k != kindBlockers {
		return nil
	}
	sr.inChain = true
	var decl [MaxPerm + 1]dirac.Action
	nd := 0
	var tot Stats
	k0, p0, turn := d.Kind, d.Player, sr.root.Turn()
	closed := false
	c := uint8(0) // 1+obj; 0 the first creature
	for {
		bot := sr.pickBot(&sr.root, d)
		sr.chainPt.n, sr.chainPt.opp, sr.chainPt.seat, sr.chainPt.off[0] = 0, false, 0, 0
		sr.nForced, sr.chainFrom = 0, 0
		sr.chainCur = c
		sr.chainCands(&sr.root, d, bot, MaxCands, &sr.chainPt)
		sr.chainCur = 0
		pt := &sr.chainPt
		if pt.n == 0 {
			for _, a := range sr.chainForced[:sr.nForced] {
				if Offered(&sr.root, dirac.Action(a), k0, p0, &sr.td) && nd < MaxPerm {
					sr.root.Apply(dirac.Action(a))
					decl[nd] = dirac.Action(a)
					nd++
				}
			}
			sr.nForced = 0
			sr.root.StepInto(d)
			closed = d.Kind != k0 || d.Player != p0 || sr.root.Turn() != turn
			break
		}
		x := sr.chainX
		var after [MaxPerm]uint8
		na := chainAfter(d, x, &after)
		var step [MaxPerm + 1]dirac.Action
		ns := 0
		if pt.n >= 2 {
			sr.chainRoot, sr.chainFrom = x+1, c
			r := sr.Search(&sr.root, seatSeed)
			tot.Simulations += r.Stats.Simulations
			tot.Completed += r.Stats.Completed
			tot.Expanded += r.Stats.Expanded
			tot.Plays += r.Stats.Plays
			tot.EnvSteps += r.Stats.EnvSteps
			tot.WorldRoots += r.Stats.WorldRoots
			tot.OppPoints += r.Stats.OppPoints
			tot.ChainPoints += r.Stats.ChainPoints
			tot.ChainFallbacks += r.Stats.ChainFallbacks
			if r.Stats.Searched == 1 {
				tot.ChainSteps++
				tot.Searched = 1
				for _, a := range r.Acts[:min(r.NActs, len(step))] {
					step[ns] = a
					ns++
				}
			} else {
				for j := pt.off[0]; j < pt.off[1] && ns < len(step); j++ {
					step[ns] = dirac.Action(pt.acts[j])
					ns++
				}
			}
		} else {
			for j := pt.off[0]; j < pt.off[1] && ns < len(step); j++ {
				step[ns] = dirac.Action(pt.acts[j])
				ns++
			}
		}
		for j, a := range step[:ns] {
			if j > 0 && d.Kind == dirac.DkBlockers && !Offered(&sr.root, a, k0, p0, &sr.td) {
				continue
			}
			sr.root.Apply(a)
			decl[nd] = a
			nd++
		}
		sr.root.StepInto(d)
		if d.Kind != k0 || d.Player != p0 || sr.root.Turn() != turn {
			closed = true // dirac closed the declaration itself
			break
		}
		c = 0
		for _, y := range after[:na] {
			if hasAttackBy(d, y) {
				c = y + 1
				break
			}
		}
		if c == 0 {
			break
		}
	}
	// the declaration ends in AkDone as a classic candidate's does, even
	// when dirac closed it itself (its consumer drops an unoffered Done)
	done := dirac.MakeAction(dirac.AkDone, 0, 0, 0)
	if !closed {
		if !Offered(&sr.root, done, k0, p0, &sr.td) {
			tot.ChainFallbacks++
			bot := sr.pickBot(&sr.root, d)
			var picks [MaxPerm]uint32
			n := sr.botDeclaration(&sr.root, d, bot, &picks)
			for _, a := range picks[:n] {
				if Offered(&sr.root, dirac.Action(a), k0, p0, &sr.td) && nd < MaxPerm {
					sr.root.Apply(dirac.Action(a))
					decl[nd] = dirac.Action(a)
					nd++
				}
			}
		}
	}
	decl[nd] = done
	nd++
	sr.inChain, sr.chainRoot, sr.chainFrom = false, 0, 0
	res := &sr.merged
	if tot.Searched == 0 {
		*res = Result{}
		tot.Skipped = 1
	}
	res.Acts = [MaxPerm + 1]dirac.Action{}
	copy(res.Acts[:], decl[:nd])
	res.NActs = nd
	res.Stats = tot
	return res
}

// chainApplyTail plays the forced blocks a chain scan found after its last
// point (no point left: every remaining blocker is decided).
func (sr *Searcher) chainApplyTail(s *dirac.State, d *dirac.Decision, k0 dirac.DecisionKind, p0 uint8) {
	for _, a := range sr.chainForced[:sr.nForced] {
		if Offered(s, dirac.Action(a), k0, p0, &sr.td) {
			s.Apply(dirac.Action(a))
		}
	}
	sr.nForced = 0
	s.StepInto(d)
}

// blockModel is the search-side model of a declaration's menace rule
// (ks-percreature-b): per attacker the blockers declared so far and the
// blockers it needs, so that a candidate is admitted only when every
// short attacker can still be completed by distinct later blockers.
type blockModel struct {
	n    int
	atk  [MaxPerm]uint8
	cnt  [MaxPerm]int
	need [MaxPerm]int
}

func (m *blockModel) find(a uint8) int {
	for i := 0; i < m.n; i++ {
		if m.atk[i] == a {
			return i
		}
	}
	if m.n == len(m.atk) {
		return -1
	}
	m.atk[m.n] = a
	m.n++
	return m.n - 1
}

func newBlockModel(s *dirac.State, d *dirac.Decision, m *blockModel) {
	m.n = 0
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == dirac.AkBlock {
			m.find(uint8(a.Target()))
		}
	}
	for o := 0; o < s.NumObjs(); o++ {
		if a := s.HBotBlocking(dirac.Obj(o)); a != dirac.NoObj {
			if i := m.find(uint8(a)); i >= 0 {
				m.cnt[i]++
			}
		}
	}
	tab := s.Table()
	for i := 0; i < m.n; i++ {
		a := dirac.Obj(m.atk[i])
		need := int(tab.Cards[s.ObjCard(a)].MinBlockers)
		if need == int(kop.MinBlockersAll) {
			need = s.MinBlockersOf(a)
		}
		if s.HasKeyword(a, kop.KwMenace) && need < 2 {
			need = 2
		}
		m.need[i] = need
	}
}

// blockMatch is blockFeasible's augmenting-path matching of deficit slots
// to distinct blockers (dirac combat.go augment's algorithm).
type blockMatch struct {
	s       *dirac.State
	later   []uint8
	slotAtk [MaxPerm]uint8
	owner   [MaxPerm]int16
	seen    [MaxPerm]bool
}

func (bm *blockMatch) augment(slot int) bool {
	for j, b := range bm.later {
		if bm.seen[j] || !bm.s.HBotBlockPair(dirac.Obj(b), dirac.Obj(bm.slotAtk[slot])) {
			continue
		}
		bm.seen[j] = true
		if bm.owner[j] < 0 || bm.augment(int(bm.owner[j])) {
			bm.owner[j] = int16(slot)
			return true
		}
	}
	return false
}

// blockFeasible reports whether, with attacker index extra (-1 none) given
// one more blocker, every attacker's deficit (0 < cnt < need) can be matched
// to distinct blockers among later.
func blockFeasible(s *dirac.State, m *blockModel, extra int, later []uint8) bool {
	var bm blockMatch
	ns := 0
	for i := 0; i < m.n; i++ {
		c := m.cnt[i]
		if i == extra {
			c++
		}
		for r := c; c > 0 && r < m.need[i]; r++ {
			if ns == len(bm.slotAtk) {
				return false
			}
			bm.slotAtk[ns] = m.atk[i]
			ns++
		}
	}
	if ns == 0 {
		return true
	}
	if ns > len(later) {
		return false
	}
	bm.s, bm.later = s, later
	for j := range later {
		bm.owner[j] = -1
	}
	for sl := 0; sl < ns; sl++ {
		for j := range later {
			bm.seen[j] = false
		}
		if !bm.augment(sl) {
			return false
		}
	}
	return true
}

const noBlock = ^uint32(0) // chainBlockCands' "no block" candidate

// chainBlockCands is chainCands for a blockers decision: scan the blockers
// in option order from the chain's start; a blocker whose only admissible
// answer is "no block" is passed, one whose only answer is a block is a
// forced pick (sr.chainForced), and the first blocker with two or more
// admissible answers gets the point, every candidate prefixed with the
// forced picks. With no such blocker pt.n stays 0 and the forced tail is
// left in sr.chainForced. It reports false at a chain start whose decision
// withholds AkDone (lure: the classic point is built there).
func (sr *Searcher) chainBlockCands(s *dirac.State, d *dirac.Decision, bot, limit int, pt *point) bool {
	var ord [MaxPerm]uint8
	no, last := 0, -1
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == dirac.AkBlock && int(a.Source()) != last && no < len(ord) {
			last = int(a.Source())
			ord[no] = uint8(a.Source())
			no++
		}
	}
	from := uint8(0)
	switch {
	case sr.rootBuild && sr.chainRoot != 0:
		from = sr.chainFrom
	case sr.chainCur != 0:
		from = sr.chainCur
	default:
		if !offersDone(d) {
			return false
		}
	}
	i0 := 0
	if from != 0 {
		for i0 < no && ord[i0] != from-1 {
			i0++
		}
	}
	sr.nForced = 0
	if i0 >= no {
		return true
	}
	var m blockModel
	newBlockModel(s, d, &m)
	for i := i0; i < no; i++ {
		x, later := ord[i], ord[i+1:no]
		var adm [MaxPerm]uint32
		na := 0
		noneOK := blockFeasible(s, &m, -1, later)
		for j := 0; j < int(d.N); j++ {
			a := d.Options[j]
			if a.Kind() != dirac.AkBlock || a.Source() != dirac.TCode(x) {
				continue
			}
			if k := m.find(uint8(a.Target())); k >= 0 && blockFeasible(s, &m, k, later) && na < len(adm) {
				adm[na] = uint32(a)
				na++
			}
		}
		switch tot := na + chainBoolInt(noneOK); {
		case tot >= 2:
			sr.chainX = x
			if limit < 2 {
				limit = 2
			}
			sr.fillBlockPoint(s, d, bot, limit, pt, x, noneOK, adm[:na])
			return true
		case na == 1:
			sr.chainForced[sr.nForced] = adm[0]
			sr.nForced++
			if k := m.find(uint8(dirac.Action(adm[0]).Target())); k >= 0 {
				m.cnt[k]++
			}
		}
	}
	return true
}

func chainBoolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// fillBlockPoint writes blocker x's candidates, each the forced prefix then
// its own act (none for "no block"): the bot's answer when admissible,
// else the first admissible one, as candidate 0.
func (sr *Searcher) fillBlockPoint(s *dirac.State, d *dirac.Decision, bot, limit int, pt *point, x uint8, noneOK bool, adm []uint32) {
	var botSet [MaxPerm]uint32
	nb := sr.botDeclaration(s, d, bot, &botSet)
	bp, has := noBlock, false
	for _, a := range botSet[:nb] {
		if dirac.Action(a).Source() == dirac.TCode(x) {
			bp, has = a, true
			break
		}
	}
	first := noBlock
	switch {
	case has && has0(adm, bp):
		first = bp
	case !has && noneOK:
	case noneOK:
	default:
		first = adm[0]
	}
	nf := sr.nForced
	var buf [MaxPerm + 1]uint32
	copy(buf[:], sr.chainForced[:nf])
	add := func(c uint32) {
		if pt.n >= limit {
			return
		}
		if c == noBlock {
			pt.addSeq(buf[:nf])
			return
		}
		buf[nf] = c
		pt.addSeq(buf[:nf+1])
	}
	add(first)
	if noneOK && first != noBlock {
		add(noBlock)
	}
	for _, a := range adm {
		if a != first {
			add(a)
		}
	}
	sr.nForced = 0 // the prefix is in the candidates
}

func has0(l []uint32, a uint32) bool {
	for _, x := range l {
		if x == a {
			return true
		}
	}
	return false
}
