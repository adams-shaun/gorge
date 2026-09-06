package botpolicy

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// Chars is the derived-characteristics surface the game-shaped half of the
// adapter pair reads a state.Game through. *rules.Engine satisfies it (the
// view package's Chars interface already pins Engine.Power/Toughness/
// Keywords with a compile-time assertion), so BoardFromGame derives exactly
// the post-effects P/T and keyword set the view-shaped half reads off the
// projected View — without botpolicy importing rules or view (Ruling F7).
type Chars interface {
	Power(state.ObjID) int32
	Toughness(state.ObjID) int32
	Keywords(state.ObjID) []string
}

// Creature is one battlefield creature's combat-relevant facts, in the
// plain-data shape that keeps the adapter pair in step: the view-shaped
// half (seat/bot.go's boardFromView) reads Power/Toughness/Keywords off the
// projected View (which the engine derives post-effects) and the rest off
// public CardView fields; the game-shaped half (BoardFromGame) reads the
// same facts off the engine and state.Game. seat/integration_test.go's
// TestBotAdaptersAgreeOverWholeGame pins the two halves to identical facts
// over a whole game. Fields exist only because a heuristic branch reads
// them; a fact no branch reads would be untested surface.
//
// Keywords carries the engine's derived keyword list, parameter-stripped
// and case-insensitively comparable by hasKeyword, so a keyword that means
// something in rules (Flying gates blocking, Deathtouch kills at any
// amount, ...) means the same thing here on both halves; keywords whose
// combat effect is not visible in the board facts (an attack trigger, a
// can't-be-blocked static) are deliberately not guessed at.
type Creature struct {
	Power      int32
	Toughness  int32
	Damage     int32
	Keywords   []string
	Tapped     bool
	Controller state.PlayerID
}

// hasKeyword is the engine's own keyword match (rules/engine.go's
// cardsKeywordHead) re-expressed for a Creature, so botpolicy and rules
// agree on what a derived keyword is without botpolicy importing rules.
func (c Creature) hasKeyword(kw string) bool {
	for _, k := range c.Keywords {
		if strings.EqualFold(cards.KeywordHead(k), kw) {
			return true
		}
	}
	return false
}

// remTough is how much more damage this creature can take this turn before
// combat damage is lethal to it (CR 119.6: damage accumulates).
func (c Creature) remTough() int32 { return c.Toughness - c.Damage }

// pt is the policy's proxy for a creature's worth: power plus toughness.
// It is deliberately crude — no cost, no ability value — and it is only
// ever used to decide which of two trades is favourable; the rules in
// chooseAttackers/chooseBlockers say exactly where.
func (c Creature) pt() int32 { return c.Power + c.Toughness }

// BoardFromGame is the game-shaped half of the adapter pair: the Board a
// decision is answered from, derived from the engine's own state.Game the
// way seat/bot.go's boardFromView derives it from the projected View a real
// client would receive. me is the player the decision that will be answered
// is asked of — the same seat whose hand the view-shaped half projects — so
// the casting Card facts below come from exactly the zones that seat may
// legally see. Both halves read public facts (every player's battlefield,
// each player's life) plus the deciding seat's own private ones (its hand
// and graveyard), so agreeing on them is agreeing on what that seat can
// see, not cheating that happens to be consistent. The creature census
// matches the View's exactly: every object the View lists in each
// battlefield (it already drops cardless ability objects and
// off-battlefield ephemerals), further filtered to creatures the same way
// boardFromView filters on the joined type list (cards/face.go's hasType is
// an EqualFold membership check on exactly those words).
func BoardFromGame(g *state.Game, ch Chars, me state.PlayerID) Board {
	b := Board{
		IsMain:    g.Step.IsMain(),
		Creatures: make(map[state.ObjID]Creature, 32),
		Life:      make(map[state.PlayerID]int32, len(g.Players)),
		Cards:     make(map[state.ObjID]Card, 16),
	}
	for i := range g.Players {
		p := &g.Players[i]
		b.Life[p.ID] = p.Life
		for _, id := range g.Zone(state.ZBattlefield, p.ID) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil || o.Ephemeral() || !o.Face().IsCreature() {
				continue
			}
			b.Creatures[id] = Creature{
				Power:      ch.Power(id),
				Toughness:  ch.Toughness(id),
				Damage:     o.Damage,
				Keywords:   append([]string(nil), ch.Keywords(id)...),
				Tapped:     o.Tapped,
				Controller: o.Controller,
			}
		}
	}
	// The casting Card census: every object in the deciding seat's own hand,
	// graveyard and battlefield — exactly the zones boardFromView fills from
	// the viewer's own Hand/Graveyard/Battlefield CardViews. Reading the
	// face's Types and ManaCost and the engine's derived Power here, and the
	// CardView's matching fields on the view side, fills the same fact with
	// the same function (CmcOf, hasTypeWord), so a card ranks identically on
	// both halves.
	for _, z := range [...]state.Zone{state.ZHand, state.ZGraveyard, state.ZBattlefield} {
		for _, id := range g.Zone(z, me) {
			o := g.Obj(id)
			if o == nil || o.Ephemeral() {
				continue
			}
			f := o.Face()
			if f == nil {
				continue
			}
			b.Cards[id] = Card{
				Creature: f.IsCreature(),
				Power:    ch.Power(id),
				CMC:      CmcOf(f.ManaCost),
				Basic:    hasTypeWord(f.Types, "Basic"),
			}
		}
	}
	return b
}

// canBlockLike is the policy's approximation of the engine's canBlock for
// creatures it is only evaluating (the defender's options during declare
// attackers), not deciding over: the blocker must be untapped and, against
// a flying attacker, itself fly or have Reach (CR 702.9b). The engine's
// offered option list is the authority for actual block choices; here the
// gates the bot cannot see (protection, Can't-Block statics) make it
// overestimate the block risk, which errs toward staying home.
func canBlockLike(a, b Creature) bool {
	if b.Tapped {
		return false
	}
	if a.hasKeyword("Flying") && !b.hasKeyword("Flying") && !b.hasKeyword("Reach") {
		return false
	}
	return true
}

// blockCombat simulates the engine's combat-damage model for one attacker a
// against an ordered block list: a deals its power to the blockers in
// order, lethal damage to each in turn, the last absorbing the remainder;
// every surviving blocker deals its power back; First Strike runs as its
// own earlier step and a creature dead after that step does not act in the
// regular one (rules/combat.go's damageStep, including Deathtouch's
// kill-at-any-amount). The simulation is only ever used to ORDER a
// decision, never to mutate game state, so a small divergence from the
// engine's exact resolution would cost a slightly off bot move, not a
// replay violation.
func blockCombat(a Creature, block []Creature) (aDead bool, dead []bool) {
	dead = make([]bool, len(block))
	aRem := a.remTough()
	taken := make([]int32, len(block))
	dt := make([]bool, len(block)) // the caller marked these with Deathtouch damage
	var aTaken int32
	aDT := false // a took Deathtouch damage from some blocker
	rem := func(i int) int32 { return block[i].remTough() }
	fsA := a.hasKeyword("First Strike")

	// aAssign is one full attack round: a's power to the blockers in order,
	// every blocker but the last capped at its own need, the last absorbing
	// what remains — the engine's bounded loop verbatim.
	aAssign := func() {
		remaining := a.Power
		for i := range block {
			if remaining <= 0 {
				break
			}
			need := rem(i)
			give := remaining
			if i < len(block)-1 && give > need {
				give = need
			}
			if give > 0 {
				taken[i] += give
				if a.hasKeyword("Deathtouch") {
					dt[i] = true
				}
			}
			remaining -= give
		}
	}
	// aHit is one blocker's hit-back: its power (a 0-power creature deals
	// nothing), plus a Deathtouch mark when it has the keyword.
	aHit := func(i int) {
		if b := block[i]; b.Power > 0 {
			aTaken += b.Power
			if b.hasKeyword("Deathtouch") {
				aDT = true
			}
		}
	}
	aLethal := func() bool { return aTaken >= aRem || aDT }

	anyFS := fsA
	for i := range block {
		anyFS = anyFS || block[i].hasKeyword("First Strike")
	}
	if anyFS {
		// First-strike step: participants with First Strike deal, then the
		// state-based actions between the two steps resolve: a dead a deals
		// nothing in the regular step, and a blocker a already killed does
		// not hit back.
		if fsA {
			aAssign()
		}
		for i := range block {
			if block[i].hasKeyword("First Strike") {
				aHit(i)
			}
		}
		if aLethal() {
			return true, dead
		}
		// Regular step: a (unless it is first-strike, which already dealt)
		// and every surviving blocker without First Strike deal, all
		// simultaneous.
		if !fsA {
			aAssign()
		}
		for i := range block {
			if block[i].hasKeyword("First Strike") {
				continue
			}
			if taken[i] >= rem(i) || dt[i] {
				continue // a's first-strike step already killed this one
			}
			aHit(i)
		}
	} else {
		aAssign()
		for i := range block {
			aHit(i)
		}
	}
	for i := range block {
		dead[i] = taken[i] >= rem(i) || dt[i]
	}
	return aLethal(), dead
}

// blocker is a defender creature carrying its object id, so the kill-block
// search and the sorting below stay deterministic (ties break on ObjID,
// never on map iteration order).
type blocker struct {
	id state.ObjID
	c  Creature
}

// killBlockCost returns the defender's cheapest block — smallest total pt
// of the creatures it loses — that kills a, and whether any block can at
// all. It evaluates every lone legal blocker against the full damage
// simulation, then every prefix of the blockers sorted by ascending pt
// (the defender leads with its cheapest creatures so the valuable ones
// absorb only the remainder), whose combined power is at least a's
// remaining toughness. A team is never cheaper than its cheapest killing
// member for pure cost counting, but a larger prefix can be: {2/2, 4/4}
// kills a 5/5 for one 2/2, cheaper than the {4/4, 4/4} team — so every
// killing prefix is simulated, not just the first.
func killBlockCost(def []blocker, a Creature) (int32, bool) {
	var can []int
	for i := range def {
		if canBlockLike(a, def[i].c) {
			can = append(can, i)
		}
	}
	if len(can) == 0 {
		return 0, false
	}
	best := int32(-1)
	consider := func(cost int32) {
		if best == -1 || cost < best {
			best = cost
		}
	}
	// Lone blockers that kill a outright (a 3/3 stops a 2/2 for free; a
	// 1/1 Deathtouch stops a 5/5 for one 1/1; a First-Strike 2/2 kills a
	// 2/2 before it deals).
	for _, i := range can {
		aDead, dead := blockCombat(a, []Creature{def[i].c})
		if aDead && dead[0] {
			consider(def[i].c.pt())
		}
	}
	// Teams: cheapest-first prefixes whose power sums high enough.
	sort.SliceStable(can, func(i, j int) bool {
		pi, pj := def[can[i]].c.pt(), def[can[j]].c.pt()
		if pi != pj {
			return pi < pj
		}
		return def[can[i]].id < def[can[j]].id
	})
	var power int32
	for k := 1; k <= len(can); k++ {
		power += def[can[k-1]].c.Power
		if power < a.remTough() {
			continue // this team cannot kill a yet
		}
		team := make([]Creature, k)
		for i := 0; i < k; i++ {
			team[i] = def[can[i]].c
		}
		aDead, dead := blockCombat(a, team)
		if !aDead {
			continue
		}
		var cost int32
		for i := range dead {
			if dead[i] {
				cost += def[can[i]].c.pt()
			}
		}
		consider(cost)
	}
	return best, best != -1
}

// chooseAttackers is the KAttackers policy: it picks which of the offered
// attacker options to declare. The rules, each stated for what it reads:
//
//   - AR1 (nothing to gain): a creature with power <= 0 deals no damage and
//     trades nothing; it stays home.
//   - AR2 (unblockable): if no defender creature can block it (Flying
//     against a defender with no Flying/Reach; no untapped defenders), it
//     attacks — the damage is guaranteed and no block can punish it.
//   - AR3 (deadly block): if the defender has a block that kills it while
//     costing the defender less than the creature's own pt (its power plus
//     toughness) — a free kill, a chump-plus-finisher team, a First-Strike
//     or Deathtouch ambush — it stays home; it only attacks when every way
//     the defender can kill it is a trade the attacker wins or survives.
//   - AR4 (leave a blocker): if attacking with the chosen set would leave
//     the board with no untapped creature that can block (an attacker with
//     Vigilance never taps and still blocks) while the defender has a
//     creature of its own, the best blockable attacker is held back, so an
//     attack never leaves the board undefended.
//
// The decision is purely a function of the offered options and the board
// facts both adapters supply; no rng is consumed, and no map iteration
// order reaches the answer (ties break on ObjID or option index).
func (b Board) chooseAttackers(d *decision.Decision) []int {
	if len(d.Options) == 0 {
		return nil
	}
	me := d.Player
	defender := d.Options[0].Player // M1: every attacker swings at the same seat.

	var theirBlockers []blocker
	defHasThreat := false
	for id, c := range b.Creatures {
		if c.Controller != defender {
			continue
		}
		theirBlockers = append(theirBlockers, blocker{id, c})
		if c.Power > 0 {
			defHasThreat = true
		}
	}

	var chosen []int
	for _, o := range d.Options {
		a := b.Creatures[o.Obj] // zero facts (not on a battlefield) read as a 0/0 — never an attacker
		if a.Power <= 0 {
			continue // AR1
		}
		blockable := false
		for _, db := range theirBlockers {
			if canBlockLike(a, db.c) {
				blockable = true
				break
			}
		}
		if !blockable {
			chosen = append(chosen, o.Index) // AR2: nothing can block it
			continue
		}
		if cost, ok := killBlockCost(theirBlockers, a); ok && cost < a.pt() {
			continue // AR3: it dies for less than it is worth
		}
		chosen = append(chosen, o.Index)
	}

	// AR4: hold back a best blocker if attacking leaves nobody to block
	// with. Zero remaining after the attack (attackers tap without
	// Vigilance) plus a defender that could attack back is the trigger;
	// only blockable attackers are candidates, since an unblockable one is
	// the way a defensive board still wins.
	canStillBlock := 0
	for id, c := range b.Creatures {
		if c.Controller != me || c.Tapped {
			continue
		}
		attacking := false
		for _, oi := range chosen {
			if d.Options[oi].Obj == id {
				attacking = true
				break
			}
		}
		if !attacking || c.hasKeyword("Vigilance") {
			canStillBlock++
		}
	}
	if canStillBlock == 0 && defHasThreat && len(chosen) > 0 {
		hold := -1
		var holdScore int32 = -1
		var holdID state.ObjID
		for _, oi := range chosen {
			a := b.Creatures[d.Options[oi].Obj]
			blockable := false
			for _, db := range theirBlockers {
				if canBlockLike(a, db.c) {
					blockable = true
					break
				}
			}
			if !blockable {
				continue
			}
			if a.pt() > holdScore || (a.pt() == holdScore && d.Options[oi].Obj < holdID) {
				hold, holdScore, holdID = oi, a.pt(), d.Options[oi].Obj
			}
		}
		if hold >= 0 {
			chosen = append(chosen[:hold:hold], chosen[hold+1:]...)
		}
	}
	return chosen
}

// chooseBlockers is the KBlockers policy: it picks which of the offered
// (blocker, attacker) pairs to declare. The rules:
//
//   - BR1 (favourable and even trades): a blocker blocks when it kills the
//     attacker and either survives the return swing or trades even-or-up
//     (the blocker's pt is no more than the attacker's). Among the viable
//     blockers for one attacker the cheapest is spent, preserving the big
//     creatures for the rest of the combat.
//   - BR2 (chump only for lethal): a blocker that dies without killing
//     blocks only when the still-unblocked damage this combat would
//     otherwise be lethal (life - damage <= 0). A chump against a trample
//     attacker saves only the blocker's own toughness, which is exactly
//     what the bookkeeping subtracts.
//   - Nothing else blocks: a creature that dies holding the line while the
//     attacker survives is thrown away for nothing.
//
// Attackers are processed biggest-power-first, so the biggest threat takes
// the first pick of blockers and the first chump when one is needed. One
// blocker is never given to two attackers (used is a membership set, as
// before). The choice is deterministic: no rng, ties on option index.
func (b Board) chooseBlockers(d *decision.Decision) []int {
	if len(d.Options) == 0 {
		return nil
	}
	me := d.Player
	myLife := b.Life[me]

	// Group the offered options by attacker, preserving the engine's
	// enumeration order (first-seen position is the deterministic tiebreak).
	type atk struct {
		id   state.ObjID
		a    Creature
		pos  int // first option index for this attacker
		opts []int
	}
	var attackers []*atk
	byID := make(map[state.ObjID]*atk)
	for i := range d.Options {
		o := &d.Options[i]
		at, ok := byID[o.Attacker]
		if !ok {
			at = &atk{id: o.Attacker, a: b.Creatures[o.Attacker], pos: i}
			byID[o.Attacker] = at
			attackers = append(attackers, at)
		}
		at.opts = append(at.opts, i)
	}
	sort.SliceStable(attackers, func(i, j int) bool {
		pi, pj := attackers[i].a.Power, attackers[j].a.Power
		if pi != pj {
			return pi > pj // biggest threat first
		}
		return attackers[i].pos < attackers[j].pos
	})

	unblocked := int32(0) // damage that will reach me if nothing more blocks
	for _, at := range attackers {
		unblocked += at.a.Power
	}

	used := map[state.ObjID]bool{}
	var ch []int
	for _, at := range attackers {
		// An attacker with nothing to block (no power to stop, no lethal
		// damage to save — including a factless option that reads as a 0/0)
		// is passed over entirely; BR1 and BR2 both exist to change damage
		// going to a player, and this one deals none.
		if at.a.Power <= 0 || at.a.remTough() <= 0 {
			continue
		}
		// BR1: cheapest unused blocker that kills the attacker on a
		// trade that is at least even (the blocker survives, or the two
		// trade with the attacker worth no less).
		best := -1
		var bestScore int32 = -1
		for _, oi := range at.opts {
			ob := &d.Options[oi]
			if used[ob.Obj] {
				continue
			}
			bl := b.Creatures[ob.Obj]
			aDead, dead := blockCombat(at.a, []Creature{bl})
			if !aDead {
				continue // this block does not kill the attacker
			}
			if dead[0] && at.a.pt() < bl.pt() {
				continue // it dies trading down; not even
			}
			if best == -1 || bl.pt() < bestScore || (bl.pt() == bestScore && oi < best) {
				best, bestScore = oi, bl.pt()
			}
		}
		if best >= 0 {
			ch = append(ch, best)
			used[d.Options[best].Obj] = true
			unblocked -= at.a.Power // a dead attacker deals nothing, trample or not
			continue
		}
		// BR2: chump only when the unblocked damage would otherwise kill.
		if myLife-unblocked <= 0 {
			chump := -1
			for _, oi := range at.opts {
				if used[d.Options[oi].Obj] {
					continue
				}
				if chump == -1 || b.Creatures[d.Options[oi].Obj].pt() < b.Creatures[d.Options[chump].Obj].pt() {
					chump = oi
				}
			}
			if chump >= 0 {
				ch = append(ch, chump)
				used[d.Options[chump].Obj] = true
				saved := at.a.Power
				if at.a.hasKeyword("Trample") {
					if b := b.Creatures[d.Options[chump].Obj]; saved > b.remTough() {
						saved = b.remTough()
					}
				}
				unblocked -= saved
			}
		}
	}
	return ch
}
