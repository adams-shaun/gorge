package oracle

import (
	"fmt"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The four-seat combat-referent lockstep. Preacher of the Schism's
// Attacked$ Player.withMostLife gate, Cloak of Confusion's
// TriggeredDefendingPlayer discard and Contested Game Ball's
// TriggeredAttackingPlayer control change all name a seat the two-player
// kernel read as the one opponent. Each now reads the event's defended seat
// (Preacher, Cloak) or the damage source's controller (Contested Game Ball).
// Preacher's second trigger (CheckDefinedPlayer$ You.withMostLife) reads the
// same event's player group as every living opponent, not the next seat.
var preacherCombatDeck = deckOf(4, "Preacher of the Schism",
	deckOf(4, "Raging Goblin", deckOf(16, "Swamp", deckOf(16, "Mountain")...)...)...)

// cloakCombatDeck makes Cloak's unblocked trigger reachable (Invisible
// Stalker) and still carries Contested Game Ball for its own subtest.
// ballCombatDeck is Contested Game Ball plus a cheap attacker and lands: no
// other card of the two-seat ddoshapes pool (which is a two-seat test set).
var ballCombatDeck = deckOf(4, "Contested Game Ball",
	deckOf(4, "Raging Goblin", deckOf(32, "Mountain")...)...)

var cloakCombatDeck = deckOf(3, "Cloak of Confusion",
	deckOf(3, "Invisible Stalker",
		deckOf(2, "Contested Game Ball",
			deckOf(4, "Raging Goblin",
				deckOf(10, "Island", deckOf(10, "Swamp", deckOf(8, "Mountain")...)...)...)...)...)...)

// preacherCombatTally reads the most-life gate from gorge's event log: each
// Preacher attack names its defender, and the gate's trigger push fires iff
// the defender's life is not less than every living player's (ties included).
type preacherCombatTally struct {
	inited        bool
	last          int
	lives         []int32
	lost          []bool
	pending       []preacherAtk
	firedMax      int // a pushed trigger on a (tied) max-life defender
	skippedNonMax int // no push on a defender another living player outranks
	wrong         int // a push/skip that disagrees with the life totals
	drift         int // the event log's life reconstruction missed a change
	wrongDetail   []string
}

type preacherAtk struct {
	obj    state.ObjID
	def    state.PlayerID
	max    bool
	pushed bool
}

// reset starts a game at its first decision, before the log's later events.
func (p *preacherCombatTally) reset(ref *GorgeSide) {
	g := ref.E.G
	p.inited = true
	p.last = len(ref.E.L.Events)
	p.pending = p.pending[:0]
	p.lives = make([]int32, len(g.Players))
	p.lost = make([]bool, len(g.Players))
	for i := range g.Players {
		p.lives[i], p.lost[i] = g.Players[i].Life, g.Players[i].Lost
	}
}

func (p *preacherCombatTally) check(ref *GorgeSide) {
	if !p.inited {
		p.reset(ref)
		return
	}
	g := ref.E.G
	evs := ref.E.L.Events
	for ; p.last < len(evs); p.last++ {
		e := &evs[p.last]
		switch e.Kind {
		case events.Damage:
			// a player recipient (Obj 0): combat damage and burn. Infect
			// replaces the life loss with poison, and a wither counter is
			// creature-only.
			if e.Obj == 0 && e.Amount > 0 && e.Counter != "infect" && int(e.Player) < len(p.lives) {
				p.lives[e.Player] -= e.Amount
			}
		case events.LifeChange:
			if int(e.Player) < len(p.lives) {
				p.lives[e.Player] += e.Amount
			}
		case events.PlayerLost:
			if int(e.Player) < len(p.lost) {
				p.lost[e.Player] = true
			}
		case events.DeclareAttackers:
			if e.Obj != 0 {
				continue // a planeswalker/battle attack, never Preacher's player gate
			}
			max := mostLife(p.lives, p.lost, e.Player)
			for _, id := range e.IDs {
				o := g.Obj(id)
				if o == nil || o.Face() == nil || o.Face().Name != "Preacher of the Schism" {
					continue
				}
				// a new attack of the same Preacher: the previous attack's
				// trigger has had its whole combat to be pushed.
				for i := 0; i < len(p.pending); i++ {
					if p.pending[i].obj == id {
						p.finalizeOne(p.pending[i])
						p.pending = append(p.pending[:i], p.pending[i+1:]...)
						i--
					}
				}
				p.pending = append(p.pending, preacherAtk{obj: id, def: e.Player, max: max})
			}
		case events.TriggerPush:
			o := g.Obj(e.Obj)
			if o == nil || o.Face() == nil || o.Face().Name != "Preacher of the Schism" {
				continue
			}
			_, tr := kddoMode(g, e.Obj, e.Amount)
			if tr == nil || tr.Params["Attacked"] != "Player.withMostLife" {
				continue
			}
			for i := range p.pending {
				if p.pending[i].obj == e.Obj && !p.pending[i].pushed {
					p.pending[i].pushed = true
					break
				}
			}
		}
	}
	for i := range g.Players {
		if g.Players[i].Life != p.lives[i] || g.Players[i].Lost != p.lost[i] {
			p.drift++
			break
		}
	}
}

// finalizeOne scores one attack; finish scores the ones whose trigger push
// only arrived after the last decision.
func (p *preacherCombatTally) finalizeOne(a preacherAtk) {
	switch {
	case a.pushed && a.max:
		p.firedMax++
	case !a.pushed && !a.max:
		p.skippedNonMax++
	default:
		p.wrong++
		if len(p.wrongDetail) < 12 {
			p.wrongDetail = append(p.wrongDetail, fmt.Sprintf("obj=%d def=%d max=%v pushed=%v", a.obj, a.def, a.max, a.pushed))
		}
	}
}

// finish processes the game's last events and scores every pending attack.
func (p *preacherCombatTally) finish(ref *GorgeSide) {
	p.check(ref)
	for _, a := range p.pending {
		p.finalizeOne(a)
	}
	p.pending = p.pending[:0]
}

// mostLife is gorge's trigmatch.PlayerHasMostLife over a life snapshot.
func mostLife(lives []int32, lost []bool, p state.PlayerID) bool {
	if int(p) >= len(lives) || lost[p] {
		return false
	}
	for i := range lives {
		if !lost[i] && lives[i] > lives[p] {
			return false
		}
	}
	return true
}

// fourSeatSetPlay builds a four-seat pod from decks on an existing set's
// table, with an optional table mutation and a per-decision check hook.
func fourSeatSetPlay(t *testing.T, set cardset.Set, decks [4][]string, mutate func(*kop.Table), check func(*GorgeSide, Side) *Divergence, finish func(*GorgeSide), pol func(uint64) Policy) func(uint64) Result {
	t.Helper()
	return fourSeatSetPlayCmd(t, set, decks, nil, mutate, check, finish, pol)
}

// fourSeatSetPlayCmd is fourSeatSetPlay as a Commander game when commanders
// is set: seat s's commander is card commanders[s] of its deck.
func fourSeatSetPlayCmd(t *testing.T, set cardset.Set, decks [4][]string, commanders []int, mutate func(*kop.Table), check func(*GorgeSide, Side) *Divergence, finish func(*GorgeSide), pol func(uint64) Policy) func(uint64) Result {
	t.Helper()
	reg := synthReg(t)
	var lists [][]cardset.Entry
	for _, d := range decks {
		es := make([]cardset.Entry, len(d))
		for i, n := range d {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	return func(seed uint64) Result {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full",
			Commander: commanders != nil, Commanders: commanders}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 30
		rc.Check, rc.Finish = check, finish
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		defer CloseSide(k)
		return Lockstep(m, seed, k, pol(seed), rc)
	}
}

// rawCombatSet is a tracked pool's set.
func rawCombatSet(t *testing.T, name string) cardset.Set {
	t.Helper()
	set, err := cardset.RawByName(name)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// combatPolicy casts and activates, answers yes to optional triggers, attacks
// with every offered creature, and lets choosePreacher force a Preacher
// attack at a chosen defender.
func combatPolicy(seed uint64, choosePreacher func(*PickCtx) (Canon, bool)) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, o := range c.D.Options {
			if o.Kind() == ActYes {
				return o
			}
		}
		if choosePreacher != nil {
			if a, ok := choosePreacher(c); ok {
				return a
			}
		}
		buf = buf[:0]
		for _, o := range c.D.Options {
			switch o.Kind() {
			case ActCast, ActCastFace, ActActivate, ActPlayLand, ActAttack:
				buf = append(buf, o)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// preacherChooser alternates Preacher attacks between the (tied) most-life
// defender and the least-life defender, so one seed exercises the firing and
// the suppressing arm of the gate.
func preacherChooser(n *int) func(*PickCtx) (Canon, bool) {
	return func(c *PickCtx) (Canon, bool) {
		if c.G == nil || c.G.E.G == nil {
			return 0, false
		}
		g := c.G.E.G
		var opts []Canon
		var lives []int
		var defs []state.PlayerID
		for _, o := range c.D.Options {
			if o.Kind() != ActAttack {
				continue
			}
			opt, ok := c.G.GorgeOption(o)
			if !ok || opt.Obj == 0 || opt.Battle != 0 {
				continue
			}
			ob := g.Obj(opt.Obj)
			if ob == nil || ob.Face() == nil || ob.Face().Name != "Preacher of the Schism" {
				continue
			}
			if int(opt.Player) >= len(g.Players) || g.Players[opt.Player].Lost {
				continue
			}
			opts = append(opts, o)
			lives = append(lives, int(g.Players[opt.Player].Life))
			defs = append(defs, opt.Player)
		}
		if len(opts) == 0 {
			return 0, false
		}
		order := make([]int, len(opts))
		for i := range order {
			order[i] = i
		}
		slices.SortFunc(order, func(a, b int) int {
			if lives[a] != lives[b] {
				return lives[a] - lives[b]
			}
			return int(defs[a]) - int(defs[b])
		})
		pick := order[len(order)-1] // max life
		if *n%2 == 1 && lives[order[0]] < lives[order[len(order)-1]] {
			pick = order[0] // a strictly lower defender: the gate must not fire
		}
		*n++
		return opts[pick], true
	}
}

func TestFourSeatCombatReferentsLockstep(t *testing.T) {
	corpusDirOrSkip(t)
	t.Run("Preacher", func(t *testing.T) {
		tally := &preacherCombatTally{}
		base := fourSeatSetPlay(t, rawCombatSet(t, "fp"), [4][]string{preacherCombatDeck, preacherCombatDeck, preacherCombatDeck, preacherCombatDeck},
			nil, func(g *GorgeSide, _ Side) *Divergence { tally.check(g); return nil }, tally.finish,
			func(seed uint64) Policy {
				n := 0
				return combatPolicy(seed, preacherChooser(&n))
			})
		play := func(seed uint64) Result {
			tally.resetFirst()
			return base(seed)
		}
		const seeds = 32
		fourSeatAtkClean(t, play, seeds)
		t.Logf("Preacher: %d triggered on a (tied) max-life defender, %d suppressed on a lower defender, %d wrong, %d life drift",
			tally.firedMax, tally.skippedNonMax, tally.wrong, tally.drift)
		if tally.drift != 0 {
			t.Errorf("%d checks' reconstructed life totals disagree with the game state", tally.drift)
		}
		if tally.wrong != 0 {
			for _, d := range tally.wrongDetail {
				t.Logf("wrong: %s", d)
			}
			t.Errorf("%d Preacher gate decisions disagree with the life totals", tally.wrong)
		}
		if tally.firedMax == 0 {
			t.Error("no Preacher attack fired the most-life trigger: the firing arm is unexercised")
		}
		if tally.skippedNonMax == 0 {
			t.Error("no Preacher attack was suppressed on a lower-life defender: the suppressing arm is unexercised")
		}
	})

	t.Run("Cloak", func(t *testing.T) {
		var triggers, discards int
		last := 0
		check := func(g *GorgeSide, _ Side) *Divergence {
			evs := g.E.L.Events
			for ; last < len(evs); last++ {
				e := &evs[last]
				switch e.Kind {
				case events.TriggerPush:
					o := g.E.G.Obj(e.Obj)
					if o == nil || o.Face() == nil || o.Face().Name != "Cloak of Confusion" {
						continue
					}
					_, tr := kddoMode(g.E.G, e.Obj, e.Amount)
					if tr != nil && tr.Mode == "AttackerUnblocked" {
						triggers++
					}
				case events.MoveZone:
					if e.From == state.ZHand && e.To == state.ZGraveyard {
						discards++
					}
				}
			}
			return nil
		}
		set := printedDelveSet(t, [2][]string{cloakCombatDeck, cloakCombatDeck})
		base := fourSeatSetPlay(t, set, [4][]string{cloakCombatDeck, cloakCombatDeck, cloakCombatDeck, cloakCombatDeck},
			nil, check, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
		play := func(seed uint64) Result {
			last = 0
			return base(seed)
		}
		const seeds = 32
		fourSeatAtkClean(t, play, seeds)
		t.Logf("Cloak: %d unblocked triggers, %d hand->graveyard discards", triggers, discards)
		if triggers == 0 {
			t.Error("no Cloak of Confusion unblocked trigger fired")
		}
		if discards == 0 {
			t.Error("no discard was made")
		}
	})

	t.Run("ContestedGameBall", func(t *testing.T) {
		var controls int
		last := 0
		check := func(g *GorgeSide, _ Side) *Divergence {
			evs := g.E.L.Events
			for ; last < len(evs); last++ {
				e := &evs[last]
				if e.Kind != events.ControlChange {
					continue
				}
				o := g.E.G.Obj(e.Obj)
				if o != nil && o.Face() != nil && o.Face().Name == "Contested Game Ball" {
					controls++
				}
			}
			return nil
		}
		set := printedDelveSet(t, [2][]string{ballCombatDeck, ballCombatDeck})
		base := fourSeatSetPlay(t, set, [4][]string{ballCombatDeck, ballCombatDeck, ballCombatDeck, ballCombatDeck},
			nil, check, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
		play := func(seed uint64) Result {
			last = 0
			return base(seed)
		}
		const seeds = 32
		fourSeatAtkClean(t, play, seeds)
		t.Logf("Contested Game Ball: %d control changes", controls)
		if controls == 0 {
			t.Error("no Contested Game Ball control changed")
		}
	})

	t.Run("Mutation", func(t *testing.T) {
		// Restore the two-seat reading of Preacher's most-life gate (the one
		// opponent's life >= yours) and Cloak/Ball's one-opponent refs. The
		// same games then diverge.
		play := fourSeatSetPlay(t, rawCombatSet(t, "fp"), [4][]string{preacherCombatDeck, preacherCombatDeck, preacherCombatDeck, preacherCombatDeck},
			mutateCombatReferentsToOpponent, nil, nil, func(seed uint64) Policy {
				n := 0
				return combatPolicy(seed, preacherChooser(&n))
			})
		var dropped bool
		if !fourSeatAtkMutated(t, play, 32, &dropped) {
			t.Fatal("restoring the EachOpp reading produced no divergence")
		}
	})
}

// resetFirst marks the tally to re-read its life snapshot from the next check.
func (p *preacherCombatTally) resetFirst() { p.inited = false }

// mutateCombatReferentsToOpponent rewrites the seat-exact reads back to the
// one-opponent reading: Preacher's most-life gate becomes EachOpp >= You,
// Cloak's discard and Contested Game Ball's new controller read EachOpp.
func mutateCombatReferentsToOpponent(t *kop.Table) {
	mutated := 0
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			ab := &t.Cards[i].Abilities[j]
			if ab.Kind != kop.AbTriggered {
				continue
			}
			if t.Cards[i].Name == "Preacher of the Schism" && len(ab.Prog) > 0 && ab.Prog[0].Op == kop.OpCreateToken {
				eOpp := len(t.Exprs)
				t.Exprs = append(t.Exprs, kop.Expr{Kind: kop.ExLifeOf, Ref: kop.RefEachOpponent})
				eYou := len(t.Exprs)
				t.Exprs = append(t.Exprs, kop.Expr{Kind: kop.ExLifeOf, Ref: kop.RefController})
				cid := len(t.Conds)
				t.Conds = append(t.Conds, kop.Cond{Kind: kop.CondValue, Op: kop.CmpGE,
					A: kop.ValueRef{K: kop.VExpr, N: int16(eOpp)}, B: kop.ValueRef{K: kop.VExpr, N: int16(eYou)}})
				ab.Trig.Cond = kop.CondID(cid)
				mutated++
			}
			for k := range ab.Prog {
				in := &ab.Prog[k]
				if in.Obj == kop.RefDefendingPlayer {
					in.Obj = kop.RefEachOpponent
					mutated++
				}
				if in.Op == kop.OpGainControl && int(in.C>>8) == int(kop.RefTriggerTgtCtrl) {
					in.C = int16(kop.RefEachOpponent) << 8
					mutated++
				}
			}
		}
	}
	if mutated == 0 {
		panic("no combat referent to mutate")
	}
}
