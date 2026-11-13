package search

import (
	"errors"
	"fmt"
	"math"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The seat-level self-play options (gorge bots/azmcts seat.go, pcr.go,
// decided.go): a per-kind simulation budget (Options.SimsByKind),
// KataGo's playout-cap randomisation (Options.FullProb, CheapSims,
// FullProbByKind) and its fewer visits once a game is decided
// (Options.DecidedHigh, DecidedN, DecidedSims, DecidedWeight). They pick
// each searched decision's budget and whether its record is written, so
// they live in SeatSearch, around Search: the knobs are read once per
// decision, never per simulation. With every one of them 0, SeatSearch is
// Search.

// NumKinds is the number of searched-kind codes (points.go kindOf), the
// no-kind code 0 included.
const NumKinds = int(kindChoose) + 1

// KindNames are the searched kinds' names by kind code (index 0, no kind,
// is ""): azmcts's :kinds= names, as ksims= and pcrk= spell them. The
// dirac poses a multi-target declaration one pick at a time, so azmcts's
// "multitarget" is dirac's "target".
var KindNames = [NumKinds]string{"", "priority", "attackers", "blockers", "target", "modes", "optional", "choose"}

// KindCode is the searched kind named name (KindNames; gorge's decision
// kind trigger_optional reads as optional), false for any other name.
func KindCode(name string) (int, bool) {
	if name == "trigger_optional" {
		name = "optional"
	}
	for k := 1; k < NumKinds; k++ {
		if KindNames[k] == name {
			return k, true
		}
	}
	return 0, false
}

// pcrSalt is azmcts's: it separates the playout-cap draw from every stream
// Search derives from the same decision seed.
const pcrSalt = 0x7063722d66756c6c // "pcr-full"

// PCRFull is azmcts PCRFull: the playout-cap draw at a decision whose seed
// is decisionSeed (DecisionSeed), true -- a full search -- with probability
// p; p >= 1 is always full, p <= 0 (or NaN) never. Bit-identical to gorge's.
func PCRFull(decisionSeed uint64, p float64) bool {
	switch {
	case p >= 1:
		return true
	case !(p > 0):
		return false
	}
	u := float64(splitmix(decisionSeed^pcrSalt)>>11) / (1 << 53)
	return u < p
}

// SeatState is one seat's state across a game's searched decisions: the
// decided-game run (azmcts decidedState) and the seat's counters. The zero
// value is a new game's.
type SeatState struct {
	// side is the run's side: +1 above DecidedHigh, -1 below 1-DecidedHigh,
	// 0 no run; run counts its consecutive completed searches.
	side int8
	run  int
	// Full and Cheap count the searched decisions playout-cap randomisation
	// made full and cheap (both 0 with FullProb 0; a decided search counts
	// as full). Decided and Winning count the decided searches and those on
	// the winning side.
	Full, Cheap      int
	Decided, Winning int
	// react is the seat's InstWinReact turn tracker (ObserveReact).
	react reactTrack
	// Replan's last pass and Reuse's kept pass line (instkeys.go)
	rp    replanState
	reuse reuseState
}

// decidedSide is azmcts decidedSide: +1 above hi, -1 below 1-hi, 0 between.
func decidedSide(v, hi float64) int8 {
	switch {
	case v > hi:
		return 1
	case v < 1-hi:
		return -1
	}
	return 0
}

// observe is azmcts decidedState.observe: a value on the run's side extends
// it, one on the other side starts a new run, one inside the band ends it.
func (st *SeatState) observe(v, hi float64) {
	switch sd := decidedSide(v, hi); {
	case sd == 0:
		st.side, st.run = 0, 0
	case sd == st.side:
		st.run++
	default:
		st.side, st.run = sd, 1
	}
}

// SeatSearch is Search at root's pending decision d for a seat whose state
// is st (azmcts Seat.DecideSearch's budget, recording and run rules). A
// playout-cap cheap decision runs CheapSims with no noise, no sampling and
// no forced playouts and is never recorded; else the budget is d's kind's
// SimsByKind entry (or Sims), and once the game is decided a full search
// runs DecidedSims with no noise, sampling or forced playouts, recorded at
// DecidedWeight. The result's Cheap, Decided, Weight and Record say which;
// sr.Opts is left as it was. Allocation-free.
func (sr *Searcher) SeatSearch(root *dirac.State, d *dirac.Decision, seatSeed uint64, st *SeatState) *Result {
	if sr.Opts.Replan || sr.Opts.Reuse > 0 {
		return sr.seatSearchKeys(root, d, seatSeed, st) // instkeys.go
	}
	return sr.seatSearch(root, d, seatSeed, st)
}

// seatSearch is SeatSearch without Replan and Reuse.
func (sr *Searcher) seatSearch(root *dirac.State, d *dirac.Decision, seatSeed uint64, st *SeatState) *Result {
	o := &sr.Opts
	if o.FullProb <= 0 && o.DecidedHigh <= 0 && o.SimsByKind == ([NumKinds]int32{}) {
		sr.rootReact = st.react
		res := sr.Search(root, seatSeed)
		res.Weight, res.Record = 1, (res.Stats.Searched == 1 || res.HeldSingle) && res.Stats.ChainSteps == 0
		return res
	}
	saved := *o
	kind := sr.kindAt(root, d)
	cheap := false
	if o.FullProb > 0 {
		p := o.FullProb
		if kind != kindNone && o.FullProbKinds&(1<<kind) != 0 {
			p = o.FullProbByKind[kind]
		}
		cheap = !PCRFull(DecisionSeed(seatSeed, uint64(root.SeqNo())), p)
	}
	decided := !cheap && o.DecidedHigh > 0 && st.side != 0 && st.run >= o.DecidedN
	switch {
	case cheap:
		o.Sims, o.Noise, o.Sample, o.ForcedK = o.CheapSims, false, false, 0
	case decided:
		o.Sims, o.Noise, o.Sample, o.ForcedK = o.DecidedSims, false, false, 0
	case kind != kindNone && o.SimsByKind[kind] > 0:
		o.Sims = int(o.SimsByKind[kind])
	}
	sr.rootReact = st.react
	res := sr.Search(root, seatSeed)
	*o = saved
	searched := res.Stats.Searched == 1
	if searched && o.FullProb > 0 {
		if cheap {
			st.Cheap++
		} else {
			st.Full++
		}
	}
	if searched && decided {
		st.Decided++
		if st.side > 0 {
			st.Winning++
		}
	}
	complete := searched && res.Stats.Completed > 0
	if o.DecidedHigh > 0 && complete {
		st.observe(res.RootValue, o.DecidedHigh)
	}
	res.Cheap, res.Decided = cheap && searched, decided && searched
	res.Weight = 1
	if decided {
		res.Weight = o.DecidedWeight
	}
	// recorded as a plain search is (searched), unless cheap or at weight 0
	res.Record = (searched || res.HeldSingle) && !cheap && res.Weight > 0 && res.Stats.ChainSteps == 0
	return res
}

// MaxSims is the most simulations one of opts' searches can run (the tree's
// capacity, and the size a CutTrace needs to record every search): Sims,
// every SimsByKind entry, CheapSims and DecidedSims.
func MaxSims(o *Options) int {
	n := o.Sims
	for _, k := range o.SimsByKind {
		n = max(n, int(k))
	}
	if o.FullProb > 0 {
		n = max(n, o.CheapSims)
	}
	if o.DecidedHigh > 0 {
		n = max(n, o.DecidedSims)
	}
	for _, c := range o.CardSims { // cardsims.go: Search's per-card budget
		n = max(n, int(c.Sims))
	}
	return n
}

// Validate refuses option values a search must not run with
// (azmcts Options.Validate, validateSimsByKind, validatePCR,
// validateDecided): negative PriorRankMaxOptions or SimsByKind entries; a
// ForcedK that is negative, NaN or infinite; FullProb or a FullProbByKind
// entry outside [0,1]; per-kind entries without FullProb > 0; CheapSims < 1
// when a decision can be cheap; and with DecidedHigh > 0 (0 is off)
// DecidedHigh outside (0.5, 1), DecidedN or DecidedSims < 1, or
// DecidedWeight outside [0, 1].
func (o *Options) Validate() error {
	switch {
	case o.PriorRankMaxOptions < 0:
		return fmt.Errorf("search: prior rank max options %d must be >= 0 (0 is off)", o.PriorRankMaxOptions)
	case !(o.ForcedK >= 0) || math.IsInf(o.ForcedK, 1):
		return fmt.Errorf("search: forced-playout k %g must be finite and >= 0 (0 is off)", o.ForcedK)
	case o.LandPick && !o.LandSearch, o.DedupSearch && !o.Kinds.Choose: // landsearch.go
		return errors.New("search: LandPick needs LandSearch, DedupSearch needs Kinds.Choose")
	case o.MaxN && (o.NodeCache > 0 || o.Batch > 0): // maxn.go
		return errors.New("search: MaxN does not combine with NodeCache or Batch (their backups carry the scalar value alone)")
	}
	if err := o.validateTiming(); err != nil { // timing.go
		return err
	}
	if err := o.validateInstKeys(); err != nil { // instkeys.go
		return err
	}
	if err := o.validatePerCreature(); err != nil { // percreature.go
		return err
	}
	for k := 1; k < NumKinds; k++ {
		if o.SimsByKind[k] < 0 {
			return fmt.Errorf("search: SimsByKind[%s] %d must be >= 0 (0 uses Sims)", KindNames[k], o.SimsByKind[k])
		}
	}
	if o.FullProbKinds&1 != 0 || o.FullProbKinds>>NumKinds != 0 {
		return errors.New("search: FullProbKinds names a code that is no searched kind")
	}
	if o.FullProb == 0 {
		if o.FullProbKinds != 0 {
			return errors.New("search: FullProbByKind needs FullProb > 0 (FullProb 0 turns playout-cap randomisation off)")
		}
	} else {
		if !(o.FullProb > 0 && o.FullProb <= 1) {
			return fmt.Errorf("search: FullProb %g must be in [0,1] (0 is off)", o.FullProb)
		}
		cheap := o.FullProb < 1
		for k := 1; k < NumKinds; k++ {
			if o.FullProbKinds&(1<<k) == 0 {
				continue
			}
			p := o.FullProbByKind[k]
			if !(p >= 0 && p <= 1) {
				return fmt.Errorf("search: FullProbByKind[%s] %g must be in [0,1]", KindNames[k], p)
			}
			cheap = cheap || p < 1
		}
		if cheap && o.CheapSims < 1 {
			return fmt.Errorf("search: CheapSims %d must be >= 1 when a decision can be cheap (FullProb %g)", o.CheapSims, o.FullProb)
		}
	}
	switch {
	case o.Batch < 0 || o.Batch > MaxBatch:
		return fmt.Errorf("search: batch %d must be in [0,%d] (0 is off)", o.Batch, MaxBatch)
	case !(o.VirtualLoss >= 0) || math.IsInf(o.VirtualLoss, 1):
		return fmt.Errorf("search: virtual loss %g must be finite and >= 0 (0 is off)", o.VirtualLoss)
	case o.VirtualLoss > 0 && o.Batch == 0:
		return fmt.Errorf("search: virtual loss %g needs batch > 0", o.VirtualLoss)
	case o.Batch > 0 && o.NodeCache > 0:
		return fmt.Errorf("search: batch %d is incompatible with node cache %d (the cache keeps one world)", o.Batch, o.NodeCache)
	case o.Batch > 0 && (o.EarlyCutGap || o.EarlyCutShare > 0):
		return fmt.Errorf("search: batch %d is incompatible with the early cut (it needs one record per simulation)", o.Batch)
	}
	if err := o.validateInfo(); err != nil {
		return err
	}
	if o.DecidedHigh == 0 {
		return nil
	}
	switch {
	case !(o.DecidedHigh > 0.5 && o.DecidedHigh < 1):
		return fmt.Errorf("search: DecidedHigh %g must be in (0.5, 1) (0 is off)", o.DecidedHigh)
	case o.DecidedN < 1:
		return fmt.Errorf("search: DecidedN %d must be >= 1 with DecidedHigh %g", o.DecidedN, o.DecidedHigh)
	case o.DecidedSims < 1:
		return fmt.Errorf("search: DecidedSims %d must be >= 1 with DecidedHigh %g", o.DecidedSims, o.DecidedHigh)
	case !(o.DecidedWeight >= 0 && o.DecidedWeight <= 1):
		return fmt.Errorf("search: DecidedWeight %g must be in [0,1] (0 records nothing)", o.DecidedWeight)
	}
	return nil
}

// ranks reports whether a point at d under a top-k cut of topK reads the
// network prior: false when the decision has more options than
// PriorRankMaxOptions (azmcts rankSkips), counted as a fallback -- the
// point then keeps the bot's answer and the enumeration's next topK-1
// under a uniform prior. buildPoint calls it only with the cap set.
func (sr *Searcher) ranks(d *dirac.Decision, topK int) bool {
	if topK > 0 && int(d.N) > sr.Opts.PriorRankMaxOptions {
		sr.St.PriorFallbacks++
		return false
	}
	return true
}

// lazyPoint is buildPoint's lazy-prior skip without the bot's answer
// (advance calls it before Bot.Pick when sr.hint >= 0). Under the skip the
// candidate order is never read: the walk adds no edge, selection and the
// availability marks walk the node's edge list, and play finds the
// selected candidate by its key. So when the candidate SET does not depend
// on the bot's answer, the point is built from a stand-in answer and the
// bot is not asked:
//
//   - priority: every option a pass, cast or activation (no land play,
//     which would leave the point unsearched; nothing outside the
//     vocabulary), so the set is every option;
//   - target / modes / optional / choose: the set is every option;
//   - attackers: one creature (every attack option shares a source) and
//     a Done option: the set is no attack plus each of its defenders,
//     whatever the bot declares (Done stands in: botDeclaration copies
//     nothing);
//   - blockers: one blocker, likewise (a blocking creature is never
//     offered again, so the bot's declaration is at most one pair);
//
// and in every case no truncation (the option count within topK, else
// Limit) and every key distinct (no opponent dedup or key lookup to
// resolve by order). It reports false, with no side effect but pt's
// contents, whenever buildPoint might differ; advance then asks the bot
// and calls buildPoint as before.
func (sr *Searcher) lazyPoint(s *dirac.State, d *dirac.Decision, pt *point) bool {
	o := &sr.Opts
	if !sr.netPrior || d.N < 2 {
		return false
	}
	// Only a pure bot may have its Pick skipped: a stateful bot's later
	// answers (and its own counters) would depend on whether this call
	// happened, which the stand-in-answer argument does not cover.
	if _, ok := sr.Bot.(PureBot); !ok {
		return false
	}
	// The builders cap at MaxCands, so a larger decision is truncated
	// before the c guard below can see it; the stand-in set could then
	// omit options the real bot's answer would keep.
	if int(d.N) > MaxCands {
		return false
	}
	kind := kindOf(d, &o.Kinds, o.Limit)
	if kind == kindNone {
		return false
	}
	opp := d.Player != sr.seat
	if opp && !o.OpponentNodes {
		return false
	}
	topK := 0
	if !opp || o.OpponentMirror {
		topK = o.PriorTopK
	}
	// rank (buildPoint), without ranks' fallback count
	if o.PriorRankMaxOptions > 0 && topK > 0 && int(d.N) > o.PriorRankMaxOptions {
		return false
	}
	c, limit := o.Limit, o.Limit
	if topK > 0 {
		c, limit = topK, MaxCands
	}
	if int(d.N) > c {
		return false
	}
	pt.n, pt.opp, pt.seat, pt.off[0] = 0, opp, relSeat(d.Player, sr.seat, s.NumSeats()), 0
	switch kind {
	case kindPriority:
		// instwin.go: the instwin / endfire filter drops options from
		// the seat's own set by the state, and the bot's answer is first
		// replaced (instBot, endFireBot), so the stand-in set would keep
		// casts (or a pass) the real point never offers.
		if sr.tmWalk || !opp && (o.InstWin != InstWinOff || o.EndFire || o.Defer > 0) {
			return false
		}
		for i := 0; i < int(d.N); i++ {
			switch d.Options[i].Kind() {
			case dirac.AkPass, dirac.AkCast, dirac.AkActivate:
			default:
				return false
			}
		}
		if !sr.priorityCands(s, d, 0, limit, pt) {
			return false
		}
	case kindAttackers, kindBlockers:
		if sr.chainOn {
			return false
		}
		done := lazyOneSource(d)
		if done < 0 {
			return false
		}
		if kind == kindAttackers {
			sr.attackCands(s, d, done, limit, pt)
		} else {
			sr.blockCands(s, d, done, limit, pt)
		}
	default:
		if o.ReactAim && kind == kindTarget && !opp {
			return false // instkeys.go: ReactAim may narrow the seat's targets
		}
		sr.single(d, 0, limit, pt)
	}
	if pt.n < 2 || pt.n > c {
		return false
	}
	for i := 0; i < pt.n; i++ {
		k := uint64(kind)
		for j := pt.off[i]; j < pt.off[i+1]; j++ {
			k = mix(k, sr.actKey(s, dirac.Action(pt.acts[j]), opp))
		}
		if opp {
			k = mix(k, 0x6f70703a) // "opp:"
		}
		key := Key(k)
		for j := 0; j < i; j++ {
			if pt.keys[j] == key {
				return false
			}
		}
		if sr.t.child(sr.hint, key) < 0 {
			return false
		}
		pt.keys[i] = key
	}
	for i := 0; i < pt.n; i++ {
		pt.prior[i] = 1 / float64(pt.n) // placeholder: nothing reads it
	}
	sr.St.PriorSkips++
	return true
}

// lazyOneSource is the Done option's index when every other option of the
// declaration d shares one source, else -1.
func lazyOneSource(d *dirac.Decision) int {
	done, src := -1, -1
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		switch a.Kind() {
		case dirac.AkDone:
			done = i
		case dirac.AkAttack, dirac.AkBlock:
			if src >= 0 && int(a.Source()) != src {
				return -1
			}
			src = int(a.Source())
		default:
			return -1
		}
	}
	return done
}

// Declarer is a Bot that answers a whole attackers or blockers declaration
// in one call (hbot.Bot.Declaration). Its contract: at every step of the
// declaration, Pick answers the list's first pair that is not yet declared
// and is offered, else AkDone (else option 0), so the list computed at
// the declaration's opening decision serves every later pick.
type Declarer interface {
	Declaration(s *dirac.State, d *dirac.Decision, out []dirac.Action) []dirac.Action
}

// pickBot is Bot.Pick, except that a Declarer at an attackers or blockers
// decision is asked for its whole declaration (kept for botDeclaration at
// the same decision) and the pick served from it: one board fill per
// declaration instead of one per pick.
func (sr *Searcher) pickBot(s *dirac.State, d *dirac.Decision) int {
	sr.declS = nil
	if d.Kind == dirac.DkAttackers || d.Kind == dirac.DkBlockers {
		if dc, ok := sr.Bot.(Declarer); ok {
			sr.declL = dc.Declaration(s, d, sr.declBuf[:0])
			sr.declS, sr.declD = s, d
			return declIndex(s, d, sr.declL)
		}
	}
	return sr.Bot.Pick(s, d)
}

// declWalk is botDeclaration for a Declarer: the bot's list from the
// opening decision (pickBot's, when it was asked at this decision), served
// pick by pick through dirac's walk (a pre-declared must-attacker is
// skipped; a dirac-forced pick the walk never offers is never played).
func (sr *Searcher) declWalk(dc Declarer, s *dirac.State, d *dirac.Decision, a dirac.Action, out *[MaxPerm]uint32) int {
	l := sr.declL
	if sr.declS != s || sr.declD != d {
		l = dc.Declaration(s, d, sr.declBuf[:0])
	}
	sr.declS = nil
	skip := declSkippable(s, d, l)
	n := 0
	sr.tmp.CopyFrom(s)
	kind := d.Kind
	for n < len(out) {
		out[n] = uint32(a)
		n++
		sr.tmp.Apply(a)
		if skip && declDone(&sr.tmp, kind, l) {
			break // every pair of l declared: the next answer is AkDone
		}
		sr.tmp.StepInto(&sr.td)
		if sr.td.Kind != kind || sr.td.Player != d.Player || sr.tmp.Turn() != s.Turn() {
			break
		}
		a = sr.td.Options[declIndex(&sr.tmp, &sr.td, l)]
		if a.Kind() == dirac.AkDone {
			break
		}
	}
	return n
}

// declSkippable reports that the walk may end without stepping once every
// pair of l is declared (declDone), because dirac then offers AkDone,
// Pick's answer: d offers Done, so (attackers) no must-attacker is
// undeclared under Config.GorgeMustAttack, which declaring more cannot
// change, and (blockers) no menace attacker is left with a lone blocker;
// one can be only through a block on a menace attacker, so no pair of l
// may target one.
func declSkippable(s *dirac.State, d *dirac.Decision, l []dirac.Action) bool {
	done := false
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkDone {
			done = true
			break
		}
	}
	if !done {
		return false
	}
	if d.Kind == dirac.DkBlockers {
		for _, x := range l {
			if s.HasKeyword(dirac.Obj(x.Target()), kop.KwMenace) {
				return false
			}
		}
	}
	return true
}

// declDone reports that every pair of l is declared in s (the walk is
// mid-declaration, not yet stepped): Pick's next answer is then AkDone
// whenever declSkippable held at the opening decision.
func declDone(s *dirac.State, kind dirac.DecisionKind, l []dirac.Action) bool {
	for _, x := range l {
		if kind == dirac.DkAttackers {
			if _, ok := s.HBotAttackTarget(dirac.Obj(x.Source())); !ok {
				return false
			}
		} else if s.HBotBlocking(dirac.Obj(x.Source())) == dirac.NoObj {
			return false
		}
	}
	return true
}

// declIndex is Pick's answer at td given the opening list l: the first
// pair of l not declared and offered, else AkDone, else option 0.
func declIndex(s *dirac.State, td *dirac.Decision, l []dirac.Action) int {
	for _, x := range l {
		if td.Kind == dirac.DkAttackers {
			if _, ok := s.HBotAttackTarget(dirac.Obj(x.Source())); ok {
				continue
			}
		} else if s.HBotBlocking(dirac.Obj(x.Source())) != dirac.NoObj {
			continue
		}
		for i := 0; i < int(td.N); i++ {
			if td.Options[i] == x {
				return i
			}
		}
	}
	for i := 0; i < int(td.N); i++ {
		if td.Options[i].Kind() == dirac.AkDone {
			return i
		}
	}
	return 0
}
