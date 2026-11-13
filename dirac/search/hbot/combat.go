package hbot

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// combat.go is botpolicy's default combat policy: the KAttackers arm of
// decide (chooseAttackersMode with AR7 lethal pressure and the swarm
// subset, no combined-lethal AR8, then LegalAttackChoices and Clamp) and
// the KBlockers arm (chooseBlockers, then legalBlockChoices and Clamp),
// over gorge's whole option list in gorge's order:
//
//   - attackers: one option per (creature, defender), defender-major --
//     each opponent still in the game in ascending seat order, then the
//     attacking creatures in battlefield entry order, each creature's
//     player pair before its pairs with that opponent's planeswalkers
//     (walkers in entry order);
//   - blockers: one option per (blocker, attacker), each blocker in its
//     controller's battlefield entry order, then each attacker in the
//     active player's.
//
// The kernel poses a declaration as single picks. Each pick rebuilds the
// declaration's original option set (the pairs still offered plus the ones
// declared this step), computes gorge's whole answer over it and serves
// the answer's first pair not yet declared that is offered, Done once none
// is left. Commander clocks (closesClock) never apply.

// maxCOpts bounds a declaration's option list.
const maxCOpts = 1024

// copt is one gorge combat option.
type copt struct {
	src dirac.Obj // the attacker (attackers) / the blocker (blockers)
	tgt uint8     // the attack target byte / the attacker blocked
	def uint8     // attackers: the defending seat (the walker's controller for a walker pair)
	req bool      // attackers: the creature must attack (gorge's Required)
}

// combatScratch is the combat policy's fixed scratch (in board).
type combatScratch struct {
	n    int
	opts [maxCOpts]copt

	// per creature of the declaration (attackers: the attacking creature;
	// blockers: the attacker), in first-occurrence order
	na     int
	atObj  [dirac.MaxPerm]dirac.Obj
	atPos  [dirac.MaxPerm]int
	atOpt0 [dirac.MaxPerm]int // first option of the creature
	atNOpt [dirac.MaxPerm]int
	atOpts [dirac.MaxPerm][dirac.MaxPerm + 1]int16
	atOrd  [dirac.MaxPerm]int  // processing order (blockers: sorted)
	atReq  [dirac.MaxPerm]bool // attackers: the creature must attack (gorge's Required)

	// defending creatures (attackers) in board order
	nDef  int
	def   [dirac.MaxPerm * 2]*creature
	canIx [dirac.MaxPerm * 2]int

	// swarm / forced options
	nSw    int
	swAt   [maxCOpts]int // atk index
	swOi   [maxCOpts]int
	swIn   [maxCOpts]bool // the pair counts toward its defender's subset
	forced [maxCOpts]bool
	ar8For [dirac.MaxPerm]int // per atk index: forced option + 1, 0 none

	nCh    int
	chosen [maxCOpts]int
	used   [dirac.MaxObj]bool

	walkers [dirac.MaxPerm]dirac.Obj
	nWalk   int

	// blockCombat scratch
	team  [dirac.MaxPerm * 2]*creature
	taken [dirac.MaxPerm * 2]int32
	dt    [dirac.MaxPerm * 2]bool
	dead  [dirac.MaxPerm * 2]bool
}

var zeroCre creature

// creGet is Creatures.Get: the zero creature for a non-creature.
func (b *board) creGet(o dirac.Obj) *creature {
	if c := b.creOf(o); c != nil {
		return c
	}
	return &zeroCre
}

// canBlockLike is botpolicy's canBlockLike(a, b).
func canBlockLike(a, bl *creature) bool {
	if bl.tapped {
		return false
	}
	if a.has(kop.KwFlying) && !bl.has(kop.KwFlying) && !bl.has(kop.KwReach) {
		return false
	}
	return true
}

// blockCombat is botpolicy's blockCombat(a, team): whether a dies, and
// cs.dead[i] whether team[i] dies.
func (cs *combatScratch) blockCombat(a *creature, team []*creature) bool {
	n := len(team)
	for i := 0; i < n; i++ {
		cs.taken[i], cs.dt[i], cs.dead[i] = 0, false, false
	}
	aRem := a.remTough()
	var aTaken int32
	aDT := false
	fsA := a.has(kop.KwFirstStrike)
	aAssign := func() {
		remaining := a.power
		for i := 0; i < n; i++ {
			if remaining <= 0 {
				break
			}
			need := team[i].remTough()
			give := remaining
			if i < n-1 && give > need {
				give = need
			}
			if give > 0 {
				cs.taken[i] += give
				if a.has(kop.KwDeathtouch) {
					cs.dt[i] = true
				}
			}
			remaining -= give
		}
	}
	aHit := func(i int) {
		if bl := team[i]; bl.power > 0 {
			aTaken += bl.power
			if bl.has(kop.KwDeathtouch) {
				aDT = true
			}
		}
	}
	aLethal := func() bool { return aTaken >= aRem || aDT }
	anyFS := fsA
	for i := 0; i < n; i++ {
		anyFS = anyFS || team[i].has(kop.KwFirstStrike)
	}
	if anyFS {
		if fsA {
			aAssign()
		}
		for i := 0; i < n; i++ {
			if team[i].has(kop.KwFirstStrike) {
				aHit(i)
			}
		}
		if aLethal() {
			return true // dead stays all false (gorge returns the zero slice)
		}
		if !fsA {
			aAssign()
		}
		for i := 0; i < n; i++ {
			if team[i].has(kop.KwFirstStrike) {
				continue
			}
			if cs.taken[i] >= team[i].remTough() || cs.dt[i] {
				continue
			}
			aHit(i)
		}
	} else {
		aAssign()
		for i := 0; i < n; i++ {
			aHit(i)
		}
	}
	for i := 0; i < n; i++ {
		cs.dead[i] = cs.taken[i] >= team[i].remTough() || cs.dt[i]
	}
	return aLethal()
}

// killBlockCost is botpolicy's killBlockCost over defender def's creatures
// in cs.def[:cs.nDef].
func (cs *combatScratch) killBlockCost(a *creature, def uint8) (int32, bool) {
	nc := 0
	for i := 0; i < cs.nDef; i++ {
		if cs.def[i].ctrl == def && canBlockLike(a, cs.def[i]) {
			cs.canIx[nc] = i
			nc++
		}
	}
	if nc == 0 {
		return 0, false
	}
	best := int32(-1)
	consider := func(c int32) {
		if best == -1 || c < best {
			best = c
		}
	}
	menace := a.has(kop.KwMenace)
	if !menace {
		for j := 0; j < nc; j++ {
			bl := cs.def[cs.canIx[j]]
			cs.team[0] = bl
			if cs.blockCombat(a, cs.team[:1]) && cs.dead[0] {
				consider(bl.pt())
			}
		}
	}
	// stable sort can by (pt, id)
	for i := 1; i < nc; i++ {
		x := cs.canIx[i]
		j := i
		for j > 0 {
			y := cs.canIx[j-1]
			py, px := cs.def[y].pt(), cs.def[x].pt()
			if py < px || py == px && cs.def[y].obj <= cs.def[x].obj {
				break
			}
			cs.canIx[j] = y
			j--
		}
		cs.canIx[j] = x
	}
	firstStrike := a.has(kop.KwFirstStrike)
	for j := 0; j < nc; j++ {
		firstStrike = firstStrike || cs.def[cs.canIx[j]].has(kop.KwFirstStrike)
	}
	var power int32
	for k := 1; k <= nc; k++ {
		power += cs.def[cs.canIx[k-1]].power
		if menace && k < 2 {
			continue
		}
		if power < a.remTough() {
			continue
		}
		for i := 0; i < k; i++ {
			cs.team[i] = cs.def[cs.canIx[i]]
		}
		if !cs.blockCombat(a, cs.team[:k]) {
			continue
		}
		var cost int32
		for i := 0; i < k; i++ {
			if cs.dead[i] {
				cost += cs.def[cs.canIx[i]].pt()
			}
		}
		consider(cost)
		if !firstStrike {
			break
		}
	}
	return best, best != -1
}

// ---- attackers ------------------------------------------------------------

// attackers answers a DkAttackers pick.
func (bt *Bot) attackers(s *dirac.State, d *dirac.Decision) int {
	b := &bt.b
	b.fillCombat(s, bt.f, d.Player)
	cs := &b.cs
	cs.buildAttackOpts(b, s, d)
	cs.chooseAttackers(b, d.Player)
	// serve the answer's first undeclared, offered pair
	for j := 0; j < cs.nCh; j++ {
		o := cs.opts[cs.chosen[j]]
		if _, ok := s.HBotAttackTarget(o.src); ok {
			continue // declared already
		}
		if i := optIndex(d, dirac.MakeAction(dirac.AkAttack, dirac.TCode(o.src), dirac.TCode(o.tgt), 0)); i >= 0 {
			return i
		}
	}
	if i := doneIndex(d); i >= 0 {
		return i
	}
	return 0
}

// buildAttackOpts lists gorge's attack options: every creature that is
// offered or already declared, in battlefield entry order, each with the
// defending player then each planeswalker the defender controls.
func (cs *combatScratch) buildAttackOpts(b *board, s *dirac.State, d *dirac.Decision) {
	me := d.Player
	cs.n = 0
	if s.NumSeats() > 2 {
		for q := uint8(0); q < s.NumSeats(); q++ {
			if q != me && !s.FeatLost(q) {
				cs.attackOptsAt(b, s, d, q, true)
			}
		}
	} else {
		cs.attackOptsAt(b, s, d, 1-me, false)
	}
	cs.groupAttackers()
}

// attackOptsAt appends defender q's block of attack options: each attacking
// creature (offered, or declared already) in battlefield entry order, its
// player pair then a pair with each planeswalker q controls. pairs keeps
// an undeclared creature's pairs to those offered (gorge drops a pair its
// creature cannot attack, a goaded creature's goader among them, CR
// 701.15b); a declared creature's pairs are all listed, as at two seats.
func (cs *combatScratch) attackOptsAt(b *board, s *dirac.State, d *dirac.Decision, q uint8, pairs bool) {
	cs.nWalk = 0
	for _, o := range s.FeatBattlefield(q, b.objs[:0]) {
		if s.HBotIsPlaneswalker(o) && cs.nWalk < len(cs.walkers) {
			cs.walkers[cs.nWalk] = o
			cs.nWalk++
		}
	}
	for _, o := range s.FeatBattlefield(d.Player, b.objs[:0]) {
		_, declared := s.HBotAttackTarget(o)
		if !declared {
			offered := false
			for i := 0; i < int(d.N); i++ {
				a := d.Options[i]
				if a.Kind() == dirac.AkAttack && dirac.Obj(a.Source()) == o {
					offered = true
					break
				}
			}
			if !offered {
				continue
			}
		}
		if cs.n+1+cs.nWalk > maxCOpts {
			break
		}
		req := s.MustAttack(o)
		all := declared || !pairs
		if pt := dirac.PlayerTarget0 + dirac.TCode(q); all || optIndex(d, dirac.MakeAction(dirac.AkAttack, dirac.TCode(o), pt, 0)) >= 0 {
			cs.opts[cs.n] = copt{src: o, tgt: uint8(pt), def: q, req: req}
			cs.n++
		}
		for w := 0; w < cs.nWalk; w++ {
			if wt := uint8(cs.walkers[w]); all || optIndex(d, dirac.MakeAction(dirac.AkAttack, uint8(o), wt, 0)) >= 0 {
				cs.opts[cs.n] = copt{src: o, tgt: wt, def: q, req: req}
				cs.n++
			}
		}
	}
}

// groupAttackers groups cs.opts by attacking creature, in first-occurrence
// order (chooseAttackersMode's attackers list).
func (cs *combatScratch) groupAttackers() {
	cs.na = 0
	for i := 0; i < cs.n; i++ {
		at := -1
		for j := cs.na - 1; j >= 0; j-- {
			if cs.atObj[j] == cs.opts[i].src {
				at = j
				break
			}
		}
		if at < 0 {
			if cs.na >= dirac.MaxPerm {
				continue
			}
			at = cs.na
			cs.atObj[at] = cs.opts[i].src
			cs.atReq[at] = cs.opts[i].req
			cs.atPos[at] = i
			cs.atOpt0[at] = i
			cs.atNOpt[at] = 0
			cs.na++
		}
		if cs.atNOpt[at] < len(cs.atOpts[at]) {
			cs.atOpts[at][cs.atNOpt[at]] = int16(i)
			cs.atNOpt[at]++
		}
	}
}

// blockersOf counts defender def's creatures that can block a.
func (cs *combatScratch) blockersOf(a *creature, def uint8) int {
	n := 0
	for i := 0; i < cs.nDef; i++ {
		if cs.def[i].ctrl == def && canBlockLike(a, cs.def[i]) {
			n++
		}
	}
	return n
}

// blockRisk is chooseAttackersMode's blockRisk against defender def's
// creatures.
func (cs *combatScratch) blockRisk(a *creature, def uint8) int {
	t := 1
	blockers := cs.blockersOf(a, def)
	blockable := blockers > 0
	if a.has(kop.KwMenace) {
		blockable = blockers >= 2
	}
	if !blockable {
		t = 2
	} else if cost, ok := cs.killBlockCost(a, def); ok && cost < a.pt() {
		t = 0
	}
	return t
}

// chooseAttackers is chooseAttackersMode(lethalPressure=true,
// combinedLethal=false) over cs.opts; the answer is cs.chosen[:cs.nCh].
func (cs *combatScratch) chooseAttackers(b *board, me uint8) {
	cs.nCh = 0
	if cs.n == 0 {
		return
	}
	lethal := func(def uint8, power int32) bool { return power > 0 && power >= b.life[def] }
	cs.nDef = 0
	threat := false
	for i := 0; i < b.nCre; i++ {
		c := &b.cre[i]
		if c.ctrl == me {
			continue
		}
		cs.def[cs.nDef] = c
		cs.nDef++
		if c.power > 0 {
			threat = true
		}
	}
	// swarm: per defender, in first-seen option order, the subset of its
	// (creature, option) pairs whose damage gets through its best blocks
	for i := 0; i < cs.n; i++ {
		cs.forced[i] = false
	}
	for at := 0; at < cs.na; at++ {
		cs.ar8For[at] = 0
	}
	var seen [dirac.MaxSeats]bool
	for i := 0; i < cs.n; i++ {
		if def := cs.opts[i].def; !seen[def] {
			seen[def] = true
			cs.swarm(b, def)
		}
	}
	for at := 0; at < cs.na; at++ {
		a := b.creGet(cs.atObj[at])
		req := cs.atReq[at]
		if a.power <= 0 && !req {
			continue
		}
		if f := cs.ar8For[at]; f > 0 {
			cs.chosen[cs.nCh] = f - 1
			cs.nCh++
			continue
		}
		best, bestTier := -1, -1
		var bestLife int32
		for j := 0; j < cs.atNOpt[at]; j++ {
			oi := int(cs.atOpts[at][j])
			def := cs.opts[oi].def
			var t int
			if lethal(def, a.power) {
				t = 3
			} else if t = cs.blockRisk(a, def); t <= 0 {
				continue
			}
			l := b.life[def]
			if t > bestTier || (t == bestTier && l < bestLife) {
				best, bestTier, bestLife = oi, t, l
			}
		}
		if best < 0 {
			if !req {
				continue
			}
			best = int(cs.atOpts[at][0])
		}
		cs.chosen[cs.nCh] = best
		cs.nCh++
	}
	// the hold: keep one blocker home when nothing else could block
	canStillBlock := 0
	for i := 0; i < b.nCre; i++ {
		c := &b.cre[i]
		if c.ctrl != me || c.tapped {
			continue
		}
		attacking := false
		for j := 0; j < cs.nCh; j++ {
			if cs.opts[cs.chosen[j]].src == c.obj {
				attacking = true
				break
			}
		}
		if !attacking || c.has(kop.KwVigilance) {
			canStillBlock++
		}
	}
	if canStillBlock == 0 && threat && cs.nCh > 0 {
		hold := -1
		var holdScore int32 = -1
		var holdID dirac.Obj
		for j := 0; j < cs.nCh; j++ {
			oi := cs.chosen[j]
			o := cs.opts[oi].src
			a := b.creGet(o)
			req := false
			for at := 0; at < cs.na; at++ {
				if cs.atObj[at] == o {
					req = cs.atReq[at]
				}
			}
			// blockable by some opponent's creatures (an any-match over
			// the defenders)
			blockable := false
			for q := uint8(0); q < b.n && !blockable; q++ {
				if q == me {
					continue
				}
				blockers := cs.blockersOf(a, q)
				blockable = blockers > 0 && (!a.has(kop.KwMenace) || blockers >= 2)
			}
			if !blockable {
				continue
			}
			if req || lethal(cs.opts[oi].def, a.power) || cs.forced[oi] {
				continue
			}
			if a.pt() > holdScore || (a.pt() == holdScore && o < holdID) {
				hold, holdScore, holdID = j, a.pt(), o
			}
		}
		if hold >= 0 {
			copy(cs.chosen[hold:cs.nCh-1], cs.chosen[hold+1:cs.nCh])
			cs.nCh--
		}
	}
}

// swarm is chooseAttackersMode's swarmLethalSubset for defender def: its
// pairs sorted by power desc, id asc (stable), skipping a creature an
// earlier defender's subset took, until the damage past its untapped
// creatures reaches its life; the subset's options are forced.
func (cs *combatScratch) swarm(b *board, def uint8) {
	cs.nSw = 0
	for at := 0; at < cs.na; at++ {
		for j := 0; j < cs.atNOpt[at]; j++ {
			if oi := int(cs.atOpts[at][j]); cs.opts[oi].def == def {
				cs.swAt[cs.nSw] = at
				cs.swOi[cs.nSw] = oi
				cs.nSw++
			}
		}
	}
	for i := 1; i < cs.nSw; i++ {
		xa, xo := cs.swAt[i], cs.swOi[i]
		px, ix := b.creGet(cs.atObj[xa]).power, cs.atObj[xa]
		j := i
		for j > 0 {
			ya := cs.swAt[j-1]
			py, iy := b.creGet(cs.atObj[ya]).power, cs.atObj[ya]
			if py > px || py == px && iy <= ix {
				break
			}
			cs.swAt[j], cs.swOi[j] = cs.swAt[j-1], cs.swOi[j-1]
			j--
		}
		cs.swAt[j], cs.swOi[j] = xa, xo
	}
	life := b.life[def]
	if life <= 0 {
		return
	}
	k := 0
	for i := 0; i < cs.nDef; i++ {
		if cs.def[i].ctrl == def && !cs.def[i].tapped {
			k++
		}
	}
	out := 0
	var through int64
	hit := -1
	for i := 0; i < cs.nSw; i++ {
		a := b.creGet(cs.atObj[cs.swAt[i]])
		if a.power <= 0 {
			break
		}
		cs.swIn[i] = cs.ar8For[cs.swAt[i]] == 0 // an earlier defender's subset took it
		if !cs.swIn[i] {
			continue
		}
		out++
		if out > k {
			through += int64(a.power)
			if through >= int64(life) {
				hit = i
				break
			}
		}
	}
	for i := 0; i <= hit; i++ {
		if !cs.swIn[i] {
			continue
		}
		cs.ar8For[cs.swAt[i]] = cs.swOi[i] + 1
		cs.forced[cs.swOi[i]] = true
	}
}

// ---- blockers -------------------------------------------------------------

// blockers answers a DkBlockers pick.
func (bt *Bot) blockers(s *dirac.State, d *dirac.Decision) int {
	b := &bt.b
	b.fillCombat(s, bt.f, d.Player)
	cs := &b.cs
	cs.buildBlockOpts(b, s, d.Player)
	cs.chooseBlockers(b, d.Player)
	for j := 0; j < cs.nCh; j++ {
		o := cs.opts[cs.chosen[j]]
		if s.HBotBlocking(o.src) != dirac.NoObj {
			continue // declared already
		}
		if i := optIndex(d, dirac.MakeAction(dirac.AkBlock, dirac.TCode(o.src), dirac.TCode(o.tgt), 0)); i >= 0 {
			return i
		}
	}
	if i := doneIndex(d); i >= 0 {
		return i
	}
	return 0
}

// buildBlockOpts lists gorge's block options: each blocker in the
// defender's battlefield entry order, each attacker in the active player's
// attacking me, a planeswalker I control or a battle I protect (CR 509.1a,
// 802.4a: at more than two seats the others are not mine to block).
func (cs *combatScratch) buildBlockOpts(b *board, s *dirac.State, me uint8) {
	var atks [dirac.MaxPerm]dirac.Obj
	na := 0
	pod := s.NumSeats() > 2
	for _, o := range s.FeatBattlefield(s.ActivePlayer(), b.objs[:0]) {
		if tgt, ok := s.HBotAttackTarget(o); ok && na < len(atks) && (!pod || s.FeatAttackDefender(tgt) == me) {
			atks[na] = o
			na++
		}
	}
	cs.n = 0
	for _, bl := range s.FeatBattlefield(me, b.objs[:0]) {
		for i := 0; i < na; i++ {
			if cs.n < maxCOpts && s.HBotBlockPair(bl, atks[i]) {
				cs.opts[cs.n] = copt{src: bl, tgt: uint8(atks[i])}
				cs.n++
			}
		}
	}
	cs.groupBlockAttackers()
}

// groupBlockAttackers groups cs.opts by attacker, in first-occurrence
// order (chooseBlockers' attackers list before its sort).
func (cs *combatScratch) groupBlockAttackers() {
	cs.na = 0
	for i := 0; i < cs.n; i++ {
		a := dirac.Obj(cs.opts[i].tgt)
		at := -1
		for j := 0; j < cs.na; j++ {
			if cs.atObj[j] == a {
				at = j
				break
			}
		}
		if at < 0 {
			if cs.na >= dirac.MaxPerm {
				continue
			}
			at = cs.na
			cs.atObj[at], cs.atPos[at], cs.atNOpt[at] = a, i, 0
			cs.na++
		}
		if cs.atNOpt[at] < len(cs.atOpts[at]) {
			cs.atOpts[at][cs.atNOpt[at]] = int16(i)
			cs.atNOpt[at]++
		}
	}
}

// chooseBlockers is botpolicy's chooseBlockers then legalBlockChoices'
// menace rule; the answer is cs.chosen[:cs.nCh].
func (cs *combatScratch) chooseBlockers(b *board, me uint8) {
	cs.nCh = 0
	if cs.n == 0 {
		return
	}
	myLife := b.life[me]
	// processing order: power desc, first option position asc
	for i := 0; i < cs.na; i++ {
		cs.atOrd[i] = i
	}
	for i := 1; i < cs.na; i++ {
		x := cs.atOrd[i]
		px := b.creGet(cs.atObj[x]).power
		j := i
		for j > 0 {
			y := cs.atOrd[j-1]
			py := b.creGet(cs.atObj[y]).power
			if py > px || py == px && cs.atPos[y] <= cs.atPos[x] {
				break
			}
			cs.atOrd[j] = y
			j--
		}
		cs.atOrd[j] = x
	}
	var unblocked int32
	for i := 0; i < cs.na; i++ {
		unblocked += b.creGet(cs.atObj[i]).power
	}
	for i := 0; i < cs.n; i++ {
		cs.used[cs.opts[i].src] = false
	}
	support := func(at, first int) int {
		best := -1
		fo := cs.opts[first].src
		for j := 0; j < cs.atNOpt[at]; j++ {
			oi := int(cs.atOpts[at][j])
			obj := cs.opts[oi].src
			if obj == fo || cs.used[obj] {
				continue
			}
			if best == -1 {
				best = oi
				continue
			}
			po, pb := b.creGet(obj).pt(), b.creGet(cs.opts[best].src).pt()
			if po < pb || po == pb && oi < best {
				best = oi
			}
		}
		return best
	}
	add := func(oi int) {
		cs.chosen[cs.nCh] = oi
		cs.nCh++
		cs.used[cs.opts[oi].src] = true
	}
	for k := 0; k < cs.na; k++ {
		at := cs.atOrd[k]
		a := b.creGet(cs.atObj[at])
		if a.power <= 0 || a.remTough() <= 0 {
			continue
		}
		best := -1
		var bestScore int32 = -1
		for j := 0; j < cs.atNOpt[at]; j++ {
			oi := int(cs.atOpts[at][j])
			obj := cs.opts[oi].src
			if cs.used[obj] {
				continue
			}
			bl := b.creGet(obj)
			cs.team[0] = bl
			if !cs.blockCombat(a, cs.team[:1]) {
				continue
			}
			if cs.dead[0] && a.pt() < bl.pt() {
				continue
			}
			if best == -1 || bl.pt() < bestScore || (bl.pt() == bestScore && oi < best) {
				best, bestScore = oi, bl.pt()
			}
		}
		if best >= 0 {
			second := -1
			if a.has(kop.KwMenace) {
				second = support(at, best)
				if second < 0 {
					best = -1
				}
			}
			if best >= 0 {
				add(best)
				if second >= 0 {
					add(second)
				}
				unblocked -= a.power
				continue
			}
		}
		if myLife-unblocked <= 0 {
			chump := -1
			for j := 0; j < cs.atNOpt[at]; j++ {
				oi := int(cs.atOpts[at][j])
				obj := cs.opts[oi].src
				if cs.used[obj] {
					continue
				}
				if chump == -1 || b.creGet(obj).pt() < b.creGet(cs.opts[chump].src).pt() {
					chump = oi
				}
			}
			if chump >= 0 {
				second := -1
				if a.has(kop.KwMenace) {
					second = support(at, chump)
					if second < 0 {
						chump = -1
					}
				}
				if chump >= 0 {
					add(chump)
					if second >= 0 {
						add(second)
					}
					saved := a.power
					if a.has(kop.KwTrample) {
						saved = b.creGet(cs.opts[chump].src).remTough()
						if second >= 0 {
							saved += b.creGet(cs.opts[second].src).remTough()
						}
						if saved > a.power {
							saved = a.power
						}
					}
					unblocked -= saved
				}
			}
		}
	}
	// legalBlockChoices: a menace attacker needs two blockers
	n := 0
	for j := 0; j < cs.nCh; j++ {
		oi := cs.chosen[j]
		a := dirac.Obj(cs.opts[oi].tgt)
		if b.creGet(a).has(kop.KwMenace) {
			cnt := 0
			for k := 0; k < cs.nCh; k++ {
				if dirac.Obj(cs.opts[cs.chosen[k]].tgt) == a {
					cnt++
				}
			}
			if cnt < 2 {
				continue
			}
		}
		cs.chosen[n] = oi
		n++
	}
	cs.nCh = n
}
