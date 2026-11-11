package oracle

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// Coverage is which deck cards and own abilities a set of lockstep games
// exercised, keyed exactly as gorge's cmd/cardfuzz keys them: per card a
// cast or land play, plus one key per own ability, "f<face>/a<index>" for
// each activated ability (Kind AB, mana abilities included) and
// "f<face>/t<index>" for each trigger that is not bookkeeping. Spell
// abilities, statics, replacements and granted or copied abilities earn no
// key. It is reporting only: nothing in it feeds a policy.
//
// The inventory and crediting rules are a port of gorge cmd/cardfuzz
// (abilities.go: abilityInventory, bookkeepingTrigger, abilitiesUsed,
// creditRefs, realCardName, ownCardName; main.go: played, playedLand),
// which is package main and cannot be imported.
type Coverage struct {
	Cards map[string]*CardCoverage // by card name (Faces[0].Name)
	// Games, Decisions and Fallbacks are run totals the caller adds:
	// games covered, canonical decisions, bot-policy fallbacks
	// (PolicyFallbacks).
	Games, Decisions, Fallbacks int
}

// CardCoverage is one card's coverage.
type CardCoverage struct {
	Cast int            // times put on the stack as a spell
	Land int            // times played as a land
	Used map[string]int // ability key -> uses
	Keys []string       // the card's full key list (CoverageKeys)
}

// NewCoverage is an empty Coverage.
func NewCoverage() *Coverage { return &Coverage{Cards: map[string]*CardCoverage{}} }

func coverageName(c *cards.Card) string {
	if c == nil || len(c.Faces) == 0 || c.Faces[0] == nil {
		return ""
	}
	return c.Faces[0].Name
}

// covSlot is one inventory entry: its key and compiled ability.
type covSlot struct {
	key string
	sa  *cards.SA
}

// coverageInventory is cardfuzz's abilityInventory: every face's AB
// abilities, then its non-bookkeeping triggers, in face then index order.
func coverageInventory(c *cards.Card) []covSlot {
	var out []covSlot
	for fi, f := range c.Faces {
		if f == nil {
			continue
		}
		for i, a := range f.Abilities {
			if a == nil || a.Kind != "AB" {
				continue
			}
			out = append(out, covSlot{key: fmt.Sprintf("f%d/a%d", fi, i), sa: a})
		}
		for i, t := range f.Triggers {
			if t.Effect == nil || bookkeepingTrigger(t) {
				continue
			}
			out = append(out, covSlot{key: fmt.Sprintf("f%d/t%d", fi, i), sa: t.Effect})
		}
	}
	return out
}

// execKeys maps each inventory trigger's Execute$ name to its key. A name two
// triggers of the card share maps to "", so it credits neither.
func execKeys(c *cards.Card) map[string]string {
	out := map[string]string{}
	for fi, f := range c.Faces {
		if f == nil {
			continue
		}
		for i, t := range f.Triggers {
			x := t.ParamStr(cards.PKExecute)
			if t.Effect == nil || bookkeepingTrigger(t) || x == "" {
				continue
			}
			if _, dup := out[x]; dup {
				out[x] = ""
				continue
			}
			out[x] = fmt.Sprintf("f%d/t%d", fi, i)
		}
	}
	return out
}

// bookkeepingTrigger is cardfuzz's: a Static$ True trigger with no
// TriggerDescription$ (a DBForget/DBCleanup rider), unless TapsForMana.
func bookkeepingTrigger(t cards.Trigger) bool {
	return strings.EqualFold(strings.TrimSpace(t.Params["Static"]), "True") &&
		strings.TrimSpace(t.Params["TriggerDescription"]) == "" && t.Mode != "TapsForMana"
}

// CoverageKeys is c's ability keys in inventory order (nil: none).
func CoverageKeys(c *cards.Card) []string {
	var out []string
	for _, s := range coverageInventory(c) {
		out = append(out, s.key)
	}
	return out
}

// manaUse is one resolved mana ability: its source object and compiled SA.
type manaUse struct {
	src state.ObjID
	sa  *cards.SA
}

// ArmCoverage installs the mana-ability probe on g's engine (chaining any
// hook already there): mana abilities never use the stack and ManaAdd
// carries no source, so CoverageOf credits them from this record, as
// cardfuzz's useProbe does. The static-trigger probe credits a Static$ True
// trigger the same way: it resolves off the stack, leaving no ability
// object. Idempotent. The hooks emit nothing, so the game is unchanged.
func (g *GorgeSide) ArmCoverage() {
	if g.manaArmed {
		return
	}
	g.manaArmed = true
	prev := g.E.ManaAbilityHook
	g.E.ManaAbilityHook = func(p state.PlayerID, src state.ObjID, sa *cards.SA) {
		g.manaUses = append(g.manaUses, manaUse{src: src, sa: sa})
		if prev != nil {
			prev(p, src, sa)
		}
	}
	// a Static$ True trigger resolves off the stack and leaves no ability
	// object: credited from the same record (Storied Veteran's rider)
	prevStatic := g.E.StaticTriggerHook
	g.E.StaticTriggerHook = func(p state.PlayerID, src state.ObjID, sa *cards.SA) {
		g.manaUses = append(g.manaUses, manaUse{src: src, sa: sa})
		if prevStatic != nil {
			prevStatic(p, src, sa)
		}
	}
}

// CoverageOf is the coverage of g's finished game: cardfuzz's played over
// the event log (a spell put on the stack credits its card's cast, a land
// play its land), cardfuzz's abilitiesUsed over the object arena (an
// ability object whose compiled SA is an inventory slot credits that slot
// when its source is the owning card) and, when g was armed (ArmCoverage),
// the resolved mana abilities. Only the two decks' cards (m.Decks) are
// credited, never tokens or copies; every deck card gets a row.
func CoverageOf(g *GorgeSide, m *Matchup) *Coverage {
	c := NewCoverage()
	e := g.E
	bySA := map[*cards.SA][]string{} // SA -> owning card names
	keyOf := map[*cards.SA]string{}
	byExec := map[string]map[string]string{} // card -> Execute$ -> key; "" when two triggers share it
	for s := range m.Decks {
		for _, cd := range m.Decks[s] {
			n := coverageName(cd)
			if n == "" || c.Cards[n] != nil {
				continue
			}
			cc := &CardCoverage{Used: map[string]int{}}
			c.Cards[n] = cc
			for _, sl := range coverageInventory(cd) {
				cc.Keys = append(cc.Keys, sl.key)
				bySA[sl.sa] = append(bySA[sl.sa], n)
				keyOf[sl.sa] = sl.key
			}
			byExec[n] = execKeys(cd)
		}
	}
	credit := func(n string, sa *cards.SA) {
		for _, owner := range bySA[sa] {
			if owner == n {
				c.Cards[n].Used[keyOf[sa]]++
			}
		}
	}
	evs := e.L.Events
	for i, ev := range evs {
		switch ev.Kind {
		case events.PutOnStack:
			if ev.To == state.ZStack {
				if cc := c.Cards[ownCardName(e, ev.Obj)]; cc != nil {
					cc.Cast++
				}
			}
		case events.DelayedPush:
			// a trigger queued in the delayed shape (a Room's unlock, a Saga
			// chapter) mints its ability object from a fresh SA resolved by
			// its Execute$ name, which no inventory slot is: credit the one
			// trigger of the source card that names that body
			n := ownCardName(e, ev.Obj)
			if k := byExec[n][ev.Counter]; k != "" {
				c.Cards[n].Used[k]++
			}
		case events.LandPlayed:
			if cc := c.Cards[ownCardName(e, playedLand(evs, i))]; cc != nil {
				cc.Land++
			}
		}
	}
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Ability == nil || o.Card != nil || o.IsCopy || o.Source == 0 || len(bySA[o.Ability]) == 0 {
			continue
		}
		credit(realCardName(e, o.Source), o.Ability)
	}
	for _, u := range g.manaUses {
		if len(bySA[u.sa]) == 0 {
			continue
		}
		if n := ownCardName(e, u.src); n != "" {
			credit(n, u.sa)
		}
	}
	return c
}

// playedLand is cardfuzz's: a LandPlayed event carries only the player, so
// the land is the latest MoveZone before it, up to the play's Priority.
func playedLand(evs []events.Event, i int) state.ObjID {
	for j := i - 1; j >= 0; j-- {
		switch evs[j].Kind {
		case events.MoveZone:
			return evs[j].Obj
		case events.Priority:
			return 0
		}
	}
	return 0
}

// ownCardName is the object's card name when it is a real card, not a
// token, copy or copy-effect permanent; else "".
func ownCardName(e *rules.Engine, id state.ObjID) string {
	if id == 0 {
		return ""
	}
	o := e.G.Obj(id)
	if o == nil || o.Card == nil || o.IsToken || o.IsCopy || o.CopyFace != nil {
		return ""
	}
	return coverageName(o.Card)
}

// realCardName walks an object's Source chain (tokens, copies, ability
// objects) to the real card it came from, or "".
func realCardName(e *rules.Engine, id state.ObjID) string {
	for hops := 0; hops < 3; hops++ {
		o := e.G.Obj(id)
		if o == nil {
			return ""
		}
		if o.Card != nil && !o.IsToken && !o.IsCopy {
			return coverageName(o.Card)
		}
		if o.Source == 0 || o.Source == id {
			return ""
		}
		id = o.Source
	}
	return ""
}

// WithCoverage arms rc for one game of m: the gorge side's mana probe is
// installed at the first agreed decision (a Check that never reports) and
// the game's coverage is written to *out when it ends (Finish). Any Check or
// Finish already in rc still runs.
func WithCoverage(rc RunConfig, m *Matchup, out **Coverage) RunConfig {
	check, finish := rc.Check, rc.Finish
	rc.Check = func(g *GorgeSide, k Side) *Divergence {
		g.ArmCoverage()
		if check != nil {
			return check(g, k)
		}
		return nil
	}
	rc.Finish = func(g *GorgeSide) {
		*out = CoverageOf(g, m)
		if finish != nil {
			finish(g)
		}
	}
	return rc
}

// Merge adds o's counts into c (a card's key list is taken from whichever
// side has it).
func (c *Coverage) Merge(o *Coverage) {
	if o == nil {
		return
	}
	if c.Cards == nil {
		c.Cards = map[string]*CardCoverage{}
	}
	c.Games += o.Games
	c.Decisions += o.Decisions
	c.Fallbacks += o.Fallbacks
	for n, oc := range o.Cards {
		cc := c.Cards[n]
		if cc == nil {
			cc = &CardCoverage{Used: map[string]int{}, Keys: append([]string(nil), oc.Keys...)}
			c.Cards[n] = cc
		}
		cc.Cast += oc.Cast
		cc.Land += oc.Land
		for k, v := range oc.Used {
			cc.Used[k] += v
		}
	}
}

// MergeClean is Merge for one finished game: only a clean game (Result.Clean: no
// divergence, no unmodelled stall) earns coverage, so a card cannot look exercised
// on the strength of a game that went wrong. A hung game has no Result and is never
// merged.
func (c *Coverage) MergeClean(o *Coverage, res *Result) {
	if res.Clean() {
		c.Merge(o)
	}
}

func (c *Coverage) names() []string {
	ns := make([]string, 0, len(c.Cards))
	for n := range c.Cards {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	return ns
}

// Unused is cc's keys never used, sorted.
func (cc *CardCoverage) Unused() []string {
	var out []string
	for _, k := range cc.Keys {
		if cc.Used[k] == 0 {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// CoverageTotals is a Coverage's summary.
type CoverageTotals struct {
	Cards, Played, Full int // cards; cast or played as a land; played with every key used
	Keys, KeysUsed      int
}

// Totals summarises c.
func (c *Coverage) Totals() CoverageTotals {
	var t CoverageTotals
	for _, cc := range c.Cards {
		t.Cards++
		t.Keys += len(cc.Keys)
		u := len(cc.Keys) - len(cc.Unused())
		t.KeysUsed += u
		if cc.Cast+cc.Land > 0 {
			t.Played++
			if u == len(cc.Keys) {
				t.Full++
			}
		}
	}
	return t
}

// WriteTSV writes one row per card, sorted by name:
// card, cast, land, keys_used, keys_total, unused_keys (comma-joined, sorted).
func (c *Coverage) WriteTSV(w io.Writer) error {
	if _, err := fmt.Fprintln(w, "card\tcast\tland\tkeys_used\tkeys_total\tunused_keys"); err != nil {
		return err
	}
	for _, n := range c.names() {
		cc := c.Cards[n]
		un := cc.Unused()
		if _, err := fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%s\n", n, cc.Cast, cc.Land, len(cc.Keys)-len(un), len(cc.Keys), strings.Join(un, ",")); err != nil {
			return err
		}
	}
	return nil
}
