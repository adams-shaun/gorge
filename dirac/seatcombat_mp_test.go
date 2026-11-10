//go:build dirac_mp

package dirac

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// The N-seat combat tests play one seeded green game in gorge and the kernel
// through every combat: each attacker is sent at an opponent by a policy
// both engines apply alike, every defending player is asked for its blocks,
// and the two engines' decisions, offers and life totals are compared.

// A combat-game event. det carries what the decision offered or the board
// it was asked over (life totals at priority).
type combatEv struct {
	kind                     string // "priority", "mulligan", "attack", "block"
	turn, active, player, sp int
	det                      string
}

func (e combatEv) String() string {
	return fmt.Sprintf("%s t%d a%d p%d sp%d %s", e.kind, e.turn, e.active, e.player, e.sp, e.det)
}

// combatDeck is 40 cards: Forests, with Grizzly Bears, Giant Spiders and
// Colossal Dreadmaws spread through it.
func combatDeck() []CardID {
	d := make([]CardID, 40)
	for i := range d {
		switch {
		case i%13 == 7:
			d[i] = v.ColossalDreadmaw
		case i%10 == 3:
			d[i] = v.GiantSpider
		case i%5 == 0:
			d[i] = v.GrizzlyBears
		default:
			d[i] = v.Forest
		}
	}
	return d
}

// combatThrough plays the games on past the first declare-blockers step
// with attackers.
const combatThrough = false

// castRank orders the spells the policy casts: the dearest first.
var castRank = map[string]int{"Colossal Dreadmaw": 0, "Giant Spider": 1, "Grizzly Bears": 2}

// attackAt is the policy's (which attacks only with two or more able
// creatures, so a declaration splits) defender for an attacker: the i-th attacker
// (in the engine's own order; the attackers are interchangeable cards of a
// kind, so only the count per defender matters) goes at the opponent
// (i + turn) mod the opponent count.
func attackAt(opps []int, i, turn int) int { return opps[(i+turn)%len(opps)] }

// blockQuota is how many blocks a defender declares: one blocker on each of
// the first half (rounded up) of the attackers attacking it.
func blockQuota(attackers int) int { return (attackers + 1) / 2 }

// gorgeAttacking reports whether a creature is attacking in gorge.
func gorgeAttacking(e *rules.Engine) bool {
	for _, id := range e.G.Zone(state.ZBattlefield, e.G.Active) {
		if o := e.G.Obj(id); o != nil && o.IsAttacking {
			return true
		}
	}
	return false
}

func lifeDet(lives []int) string {
	s := make([]string, len(lives))
	for i, l := range lives {
		s[i] = fmt.Sprint(l)
	}
	return "life " + strings.Join(s, "/")
}

// offerDet summarises an attackers decision: per attacker the sorted
// defender seats it is offered, attackers sorted.
func offerDet(per map[int][]int) string {
	var rows []string
	for _, defs := range per {
		sort.Ints(defs)
		rows = append(rows, fmt.Sprint(defs))
	}
	sort.Strings(rows)
	return fmt.Sprintf("offers %d %v", len(rows), rows)
}

func gorgeCombatGame(t *testing.T, n int, seed uint64, limit int) []combatEv {
	t.Helper()
	src := map[CardID]string{
		v.Forest:           "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n",
		v.GrizzlyBears:     "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n",
		v.GiantSpider:      "Name:Giant Spider\nManaCost:3 G\nTypes:Creature Spider\nPT:2/4\nK:Reach\nOracle:x\n",
		v.ColossalDreadmaw: "Name:Colossal Dreadmaw\nManaCost:4 G G\nTypes:Creature Dinosaur\nPT:6/6\nK:Trample\nOracle:x\n",
	}
	parsed := map[CardID]*cards.Card{}
	for id, s := range src {
		parsed[id] = cmdTestCard(t, s)
	}
	names := make([]string, n)
	decks := make([][]*cards.Card, n)
	for p := range decks {
		names[p] = fmt.Sprintf("p%d", p)
		for _, c := range combatDeck() {
			decks[p] = append(decks[p], parsed[c])
		}
	}
	e := rules.New(rules.Config{Seed: seed, Names: names, Decks: decks})
	name := func(id state.ObjID) string {
		if o := e.G.Obj(id); o != nil && o.Card != nil {
			return o.Card.Faces[0].Name
		}
		return ""
	}
	lives := func() []int {
		l := make([]int, n)
		for i := range l {
			l[i] = int(e.G.Players[i].Life)
		}
		return l
	}
	var out []combatEv
	for len(out) < limit && !e.G.Over {
		e.Advance()
		d := e.Pending()
		if d == nil || !combatThrough && e.G.Step == state.StepDeclareBlockers && d.Kind == decision.KPriority && gorgeAttacking(e) {
			break
		}
		ev := combatEv{turn: int(e.G.Turn), active: int(e.G.Active), player: int(d.Player), sp: len(e.G.Stack)}
		var ch []int
		switch d.Kind {
		case decision.KPriority:
			ev.kind, ev.det = "priority", lifeDet(lives())
			main := d.Player == e.G.Active && len(e.G.Stack) == 0 && (e.G.Step == state.StepMain1 || e.G.Step == state.StepMain2)
			pick := -1
			for _, o := range d.Options {
				if o.Kind == "play_land" && pick < 0 {
					pick = o.Index
				}
			}
			if pick < 0 && main {
				// gorge casts from a floated pool: float every land first,
				// as the kernel's cast pays from them; a mana activation
				// is no kernel decision
				for _, o := range d.Options {
					if o.Kind == "activate" && pick < 0 {
						pick, ev.kind = o.Index, ""
					}
				}
			}
			for _, o := range d.Options {
				if r, ok := castRank[name(o.Obj)]; o.Kind == "cast" && ok && (pick < 0 || d.Options[pick].Kind == "cast" && r < castRank[name(d.Options[pick].Obj)]) {
					pick = o.Index
				}
			}
			if pick < 0 {
				for _, o := range d.Options {
					if o.Kind == "pass" {
						pick = o.Index
					}
				}
			}
			ch = []int{pick}
		case decision.KMulligan:
			ev.kind = "mulligan"
			ch = []int{0}
		case decision.KAttackers:
			ev.kind = "attack"
			per := map[int][]int{}
			var order []state.ObjID
			for _, o := range d.Options {
				if _, ok := per[int(o.Obj)]; !ok {
					order = append(order, o.Obj)
				}
				per[int(o.Obj)] = append(per[int(o.Obj)], int(o.Player))
			}
			var opps []int
			for p := 0; p < n; p++ {
				if p != int(e.G.Active) {
					opps = append(opps, p)
				}
			}
			sort.SliceStable(order, func(i, j int) bool { return castRank[name(order[i])] < castRank[name(order[j])] })
			cnt := make([]int, n)
			if len(order) < 2 {
				order = nil // the policy attacks with two or more only
			}
			for i, id := range order {
				want := attackAt(opps, i, int(e.G.Turn))
				for _, o := range d.Options {
					if o.Obj == id && int(o.Player) == want {
						ch = append(ch, o.Index)
						cnt[want]++
					}
				}
			}
			ev.det = offerDet(per) + fmt.Sprintf(" declared %v", cnt)
		case decision.KBlockers:
			ev.kind = "block"
			atk := map[state.ObjID]bool{}
			for _, o := range d.Options {
				atk[o.Attacker] = true
			}
			nAtk := 0
			for _, id := range e.G.Zone(state.ZBattlefield, e.G.Active) {
				if o := e.G.Obj(id); o != nil && o.IsAttacking && o.Attacking == d.Player {
					nAtk++
				}
			}
			q := blockQuota(nAtk)
			usedB, usedA := map[state.ObjID]bool{}, map[state.ObjID]bool{}
			opts := append([]decision.Option(nil), d.Options...)
			sort.SliceStable(opts, func(i, j int) bool {
				return pairRank(name(opts[i].Attacker), name(opts[i].Obj)) < pairRank(name(opts[j].Attacker), name(opts[j].Obj))
			})
			for _, o := range opts {
				if len(ch) < q && !usedB[o.Obj] && !usedA[o.Attacker] {
					ch = append(ch, o.Index)
					usedB[o.Obj], usedA[o.Attacker] = true, true
				}
			}
			ev.det = fmt.Sprintf("attackers %d blockable %d pairs %d blocks %d", nAtk, len(atk), len(d.Options), len(ch))
		default:
			t.Fatalf("n=%d seed %d: unexpected gorge decision %v", n, seed, d.Kind)
		}
		if ev.kind != "" {
			out = append(out, ev)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}); err != nil {
			t.Fatalf("gorge submit %v: %v", d.Kind, err)
		}
	}
	return out
}

func kernelCombatGame(t *testing.T, n int, seed uint64, limit int) []combatEv {
	t.Helper()
	var decks [MaxSeats][]CardID
	for p := 0; p < n; p++ {
		decks[p] = combatDeck()
	}
	s := new(State)
	s.InitN(v.Table, decks, uint8(n), seed, Config{NoElide: true})
	lives := func() []int {
		l := make([]int, n)
		for i := range l {
			l[i] = int(s.pl[i].life)
		}
		return l
	}
	cardName := func(o uint8) string { return v.Table.Cards[s.objs[o].card].Name }
	var out []combatEv
	var d Decision
	// the declaration in progress: the attackers' picks queued at the
	// decision's first pose, the defender's blocks left
	var plan []Action
	inDecl := false
	usedB, usedA := map[uint8]bool{}, map[uint8]bool{}
	quota := 0
	for len(out) < limit && !s.Over() {
		s.StepOne(&d)
		if d.Kind == DkNone || !combatThrough && s.step == kop.StepBlockers && d.Kind == DkPriority && s.nAttackers > 0 {
			break
		}
		ev := combatEv{turn: int(s.turn), active: int(s.active), player: int(d.Player), sp: int(s.sp)}
		a := d.Options[0]
		switch d.Kind {
		case DkPriority:
			inDecl = false
			ev.kind, ev.det = "priority", lifeDet(lives())
			best := -1
			rank := func(x Action) int {
				switch x.Kind() {
				case AkPlayLand:
					return -1
				case AkCast:
					return castRank[cardName(x.Source())]
				case AkPass:
					return 10
				}
				return 100
			}
			for i := 0; i < int(d.N); i++ {
				if best < 0 || rank(d.Options[i]) < rank(d.Options[best]) {
					best = i
				}
			}
			a = d.Options[best]
		case DkMulligan:
			ev.kind = "mulligan"
			for i := 0; i < int(d.N); i++ {
				if d.Options[i].Kind() == AkKeep {
					a = d.Options[i]
				}
			}
		case DkAttackers:
			if !inDecl {
				inDecl = true
				ev.kind = "attack"
				per := map[int][]int{}
				var order []uint8
				for i := 0; i < int(d.N); i++ {
					x := d.Options[i]
					if x.Kind() != AkAttack {
						continue
					}
					if _, ok := per[int(x.Source())]; !ok {
						order = append(order, x.Source())
					}
					per[int(x.Source())] = append(per[int(x.Source())], int(CodeSeat(x.Target())))
				}
				var opps []int
				for p := 0; p < n; p++ {
					if p != int(s.active) {
						opps = append(opps, p)
					}
				}
				sort.SliceStable(order, func(i, j int) bool { return castRank[cardName(order[i])] < castRank[cardName(order[j])] })
				cnt := make([]int, n)
				plan = plan[:0]
				if len(order) < 2 {
					order = nil // the policy attacks with two or more only
				}
				for i, o := range order {
					want := attackAt(opps, i, int(s.turn))
					plan = append(plan, MakeAction(AkAttack, o, PlayerCode(uint8(want)), 0))
					cnt[want]++
				}
				ev.det = offerDet(per) + fmt.Sprintf(" declared %v", cnt)
			}
			a = MakeAction(AkDone, 0, 0, 0)
			if len(plan) > 0 {
				a, plan = plan[0], plan[1:]
			}
		case DkBlockers:
			if !inDecl {
				// a defender's declaration opens
				inDecl = true
				usedB, usedA = map[uint8]bool{}, map[uint8]bool{}
				atk := map[uint8]bool{}
				for i := 0; i < int(d.N); i++ {
					if x := d.Options[i]; x.Kind() == AkBlock {
						atk[x.Target()] = true
					}
				}
				nAtk := 0
				for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
					if sl := m.first(); s.perms[sl].flags&pfAttacking != 0 && s.atkSeat(s.k(), sl) == d.Player {
						nAtk++
					}
				}
				quota = blockQuota(nAtk)
				ev.kind = "block"
				ev.det = fmt.Sprintf("attackers %d blockable %d pairs %d blocks %d", nAtk, len(atk), int(d.N)-1, blockable(&d, quota, cardName))
			}
			a = MakeAction(AkDone, 0, 0, 0)
			if len(usedB) < quota {
				if x, ok := nextBlock(&d, usedB, usedA, cardName); ok {
					a = x
					usedB[x.Source()], usedA[x.Target()] = true, true
				}
			}
			if a.Kind() == AkDone {
				inDecl = false // the next blockers decision is the next defender's
			}
		default:
			t.Fatalf("n=%d seed %d: unexpected kernel decision %v", n, seed, d.Kind)
		}
		if ev.kind != "" {
			out = append(out, ev)
		}
		s.Apply(a)
	}
	return out
}

// pairRank orders the block policy's pairs: the attacker by castRank,
// then the blocker.
func pairRank(attacker, blocker string) int { return castRank[attacker]*10 + castRank[blocker] }

// nextBlock is the policy's next block among d's offers: the best-ranked
// pair of a blocker and an attacker neither used yet.
func nextBlock(d *Decision, usedB, usedA map[uint8]bool, name func(uint8) string) (Action, bool) {
	best, ok := Action(0), false
	for i := 0; i < int(d.N); i++ {
		x := d.Options[i]
		if x.Kind() != AkBlock || usedB[x.Source()] || usedA[x.Target()] {
			continue
		}
		if !ok || pairRank(name(x.Target()), name(x.Source())) < pairRank(name(best.Target()), name(best.Source())) {
			best, ok = x, true
		}
	}
	return best, ok
}

// blockable is how many blocks the policy makes from the first pose of a
// blockers decision, at most limit.
func blockable(d *Decision, limit int, name func(uint8) string) int {
	usedB, usedA := map[uint8]bool{}, map[uint8]bool{}
	c := 0
	for c < limit {
		x, ok := nextBlock(d, usedB, usedA, name)
		if !ok {
			break
		}
		usedB[x.Source()], usedA[x.Target()] = true, true
		c++
	}
	return c
}

func compareCombatGames(t *testing.T, n int, seed uint64, limit int) []combatEv {
	t.Helper()
	g := gorgeCombatGame(t, n, seed, limit)
	k := kernelCombatGame(t, n, seed, limit)
	for i := 0; i < len(g) && i < len(k); i++ {
		if g[i] != k[i] {
			lo := max(0, i-6)
			t.Fatalf("n=%d seed %d: event %d: gorge %v, kernel %v\n gorge  %v\n kernel %v", n, seed, i, g[i], k[i], g[lo:i+1], k[lo:i+1])
		}
	}
	if len(g) != len(k) {
		t.Fatalf("n=%d seed %d: gorge %d events, kernel %d", n, seed, len(g), len(k))
	}
	return g
}

// TestNSeatAttackOffers: every able attacker is offered at every opponent
// (CR 506.2, 508.1b), the declaration splits across defenders, and the
// game plays on through combat as gorge's does.
func TestNSeatAttackOffers(t *testing.T) {
	for _, n := range []int{2, 3, 4} {
		split := 0
		for seed := uint64(1); seed <= 4; seed++ {
			for _, e := range compareCombatGames(t, n, seed, 700) {
				if e.kind != "attack" {
					continue
				}
				var c [MaxSeats]int
				fmt.Sscan(strings.Trim(e.det[strings.Index(e.det, "declared")+len("declared "):], "[]"), &c[0], &c[1], &c[2], &c[3])
				hit := 0
				for _, x := range c {
					if x > 0 {
						hit++
					}
				}
				if hit >= 2 {
					split++
				}
			}
		}
		if n > 2 && split == 0 {
			t.Fatalf("n=%d: no declaration attacked two defenders", n)
		}
	}
}

// TestNSeatBlockerRounds: each defending player with a legal block declares
// its own blocks, in turn order from the active player (CR 509.1, 802.4),
// against only the attackers attacking it, as gorge asks them.
func TestNSeatBlockerRounds(t *testing.T) {
	for _, n := range []int{3, 4} {
		multi := 0
		for seed := uint64(1); seed <= 4; seed++ {
			evs := compareCombatGames(t, n, seed, 700)
			for i := 1; i < len(evs); i++ {
				if evs[i].kind == "block" && evs[i-1].kind == "block" && evs[i].turn == evs[i-1].turn {
					multi++
					if (evs[i].player-evs[i].active+n)%n <= (evs[i-1].player-evs[i].active+n)%n {
						t.Fatalf("n=%d seed %d: defenders out of turn order: %v then %v", n, seed, evs[i-1], evs[i])
					}
				}
			}
		}
		if multi == 0 {
			t.Fatalf("n=%d: no combat asked two defenders", n)
		}
	}
}
