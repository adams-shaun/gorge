package oracle

import (
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// FocusSpec is what a scripted scenario (docs/design/scripted-scenario-starts.md,
// sections 1 and 4) asks of the focus seat: which seat plays for the focus
// card, which instances are the focus card, which ability keys the run wants
// used, and the (turn, phase) the seat only passes toward.
type FocusSpec struct {
	// Seat is the focus seat; every other seat's decision is base's answer
	// (the scenario's opponent policy is the base of the seat it plays).
	Seat uint8
	// Refs are the focus instances' canonical refs (Matchup.InstanceCard's
	// layout: the same instance on both engines).
	Refs []Ref
	// Keys are the wanted coverage keys, "f<face>/a<i>" or "f<face>/t<i>"
	// (CoverageKeys). Only an a key is picked directly: an activate option
	// whose mode is the ability index i. A t key is reached through the
	// scenario's design (phase, attacking, a board that meets the condition),
	// and a key this does not parse is ignored.
	Keys []string
	// Turn and Phase are the pass-through point: until the game reaches
	// Phase (one of scenarioPhases) of turn Turn, the focus seat only passes.
	// Turn 0 means no pass-through.
	Turn  int
	Phase string
}

// focusKeyMaxPicks is how many times one wanted key is picked in a game. The
// policy then leaves the key to base, which stops an activation that does not
// change the game (a loop the engines would play out to the turn cap).
const focusKeyMaxPicks = 3

// focusPhaseSteps is the first step of each scenarioPhases entry, in the same
// order: upkeep, main1, combat (beginning of combat), main2, end.
var focusPhaseSteps = [...]state.Step{
	state.StepUpkeep, state.StepMain1, state.StepBeginCombat, state.StepMain2, state.StepEnd,
}

// focusClock reads the game's turn and step at a decision. ok is false when
// the run cannot tell.
type focusClock func(c *PickCtx) (turn int, step state.Step, ok bool)

// gorgeClock is the clock of a gorge-led run: PickCtx.G is the gorge side.
// A kernel-led run (replay, PickCtx.G nil) has none.
func gorgeClock(c *PickCtx) (int, state.Step, bool) {
	if c.G == nil || c.G.E == nil || c.G.E.G == nil {
		return 0, 0, false
	}
	return int(c.G.E.G.Turn), c.G.E.G.Step, true
}

// FocusPolicy wraps base for a scripted scenario's focus seat (design section
// 4). It keeps Policy's contract: a pure function of its own counters and the
// context, so a run replays from its action list.
//
// At a focus-seat decision it picks the first of these that is offered:
//
//  1. while (Turn, Phase) has not been reached: ActPass, or ActDone, or the
//     first option when passing is not offered;
//  2. ActActivate of a focus source whose mode is the ability index of a
//     wanted a key (at most focusKeyMaxPicks times a game per key);
//  3. ActSpecial of a focus source (turn face up);
//  4. ActCast of a focus card from hand (a plain cast: the cast modes name
//     the other zones), or ActCastFace of one, a face-down morph cast included;
//  5. ActPlayLand of a focus land;
//  6. at an attackers decision, ActAttack of a focus creature;
//  7. ActYes of a focus subject (an optional trigger).
//
// Otherwise it defers to base. Whether (Turn, Phase) is reached is read from
// PickCtx.G; a run with no gorge side (a kernel-led replay) cannot tell, and
// counts it as reached, so rules 2 to 7 apply at once.
//
// The returned policy holds per-game counters: build one for each game.
func FocusPolicy(base Policy, f FocusSpec) Policy {
	return newFocusPolicy(base, f, gorgeClock)
}

func newFocusPolicy(base Policy, f FocusSpec, clock focusClock) *focusPolicy {
	p := &focusPolicy{base: base, f: f, clock: clock, used: map[string]int{}}
	if f.Turn > 0 {
		i := slices.Index(scenarioPhases, f.Phase)
		if i < 0 {
			panic("oracle: FocusPolicy: phase " + strconv.Quote(f.Phase) + " is not one of " + strings.Join(scenarioPhases, "|"))
		}
		p.goal = focusPhaseSteps[i]
	}
	for _, k := range f.Keys {
		if i, ok := focusAbilityIndex(k); ok {
			p.keys = append(p.keys, focusKey{key: k, ability: i})
		}
	}
	return p
}

// focusKey is a wanted a key and its ability index.
type focusKey struct {
	key     string
	ability uint8
}

// focusAbilityIndex is the ability index of an "f<face>/a<i>" key. A t key,
// or anything else, is not an activation to pick.
func focusAbilityIndex(key string) (uint8, bool) {
	face, ab, ok := strings.Cut(key, "/")
	if !ok || !strings.HasPrefix(face, "f") || !strings.HasPrefix(ab, "a") {
		return 0, false
	}
	if _, err := strconv.ParseUint(face[1:], 10, 8); err != nil {
		return 0, false
	}
	i, err := strconv.ParseUint(ab[1:], 10, 7) // an ActActivate mode holds an ability index below 0x80
	if err != nil {
		return 0, false
	}
	return uint8(i), true
}

type focusPolicy struct {
	base  Policy
	f     FocusSpec
	clock focusClock
	goal  state.Step     // the first step of the pass-through phase
	keys  []focusKey     // the wanted a keys, in Keys order
	used  map[string]int // key -> times picked this game
}

func (p *focusPolicy) Pick(c *PickCtx) Canon {
	d := c.D
	if d == nil || d.Player != p.f.Seat || len(d.Options) == 0 {
		return p.base.Pick(c)
	}
	if !p.reached(c) {
		return focusPass(d)
	}
	if a, ok := p.pickActivate(d); ok {
		return a
	}
	if a, ok := p.pickFirst(d, func(a Canon) bool { return a.Kind() == ActSpecial }); ok {
		return a
	}
	if a, ok := p.pickFirst(d, focusCast); ok {
		return a
	}
	if a, ok := p.pickFirst(d, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
		return a
	}
	if d.Kind == DAttackers {
		if a, ok := p.pickFirst(d, func(a Canon) bool { return a.Kind() == ActAttack }); ok {
			return a
		}
	}
	if a, ok := p.pickFirst(d, func(a Canon) bool { return a.Kind() == ActYes }); ok {
		return a
	}
	return p.base.Pick(c)
}

// reached reports whether the game is at or past (Turn, Phase). It is true
// when there is no pass-through, or the clock cannot tell.
func (p *focusPolicy) reached(c *PickCtx) bool {
	if p.f.Turn <= 0 {
		return true
	}
	turn, step, ok := p.clock(c)
	if !ok {
		return true
	}
	return turn > p.f.Turn || turn == p.f.Turn && step >= p.goal
}

// focusPass is the pass-through answer: Pass, else Done, else the first option.
func focusPass(d *CDecision) Canon {
	for _, kind := range [...]CanonKind{ActPass, ActDone} {
		for _, a := range d.Options {
			if a.Kind() == kind {
				return a
			}
		}
	}
	return d.Options[0]
}

// isFocus reports whether r is a focus instance.
func (p *focusPolicy) isFocus(r Ref) bool { return slices.Contains(p.f.Refs, r) }

// pickFirst is the first option (the options are sorted) that is of a focus
// source and passes ok.
func (p *focusPolicy) pickFirst(d *CDecision, ok func(Canon) bool) (Canon, bool) {
	for _, a := range d.Options {
		if ok(a) && p.isFocus(a.Src()) {
			return a, true
		}
	}
	return 0, false
}

// focusCast is a cast of a card from hand: a plain ActCast (a nonzero cast
// mode names flashback, mayplay and the other zones and routes), or an
// ActCastFace.
func focusCast(a Canon) bool {
	return a.Kind() == ActCastFace || a.Kind() == ActCast && a.Mode()&0xf == 0
}

// pickActivate picks an activation of a focus source for a wanted a key whose
// count is below focusKeyMaxPicks, and counts it.
func (p *focusPolicy) pickActivate(d *CDecision) (Canon, bool) {
	for _, a := range d.Options {
		if a.Kind() != ActActivate || !p.isFocus(a.Src()) {
			continue
		}
		for _, k := range p.keys {
			if k.ability == a.Mode() && p.used[k.key] < focusKeyMaxPicks {
				p.used[k.key]++
				return a, true
			}
		}
	}
	return 0, false
}
