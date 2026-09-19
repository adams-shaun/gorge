package effects

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The RollDice publication tests (the r2 review's MAJOR finding): the
// multi-roll shapes the primitive claims must actually publish what their
// chained sub-abilities read. Every test resolves a REAL corpus SVar (or a
// real corpus ability) through the askHost double, with a scripted Rand so
// the dice are known.

// scriptedRollHost is an askHost whose Rand replays a scripted sequence (each
// entry is the raw [0,n) draw, so a d6 die value is entry+1) and whose
// Suspended() mirrors what rules.Engine does: true from the moment an ask is
// posed until the test clears it, so Resolve stops the SubAbility$ chain at
// the suspension exactly like the engine does instead of running the subs
// before the answer (the way the bare fakeHost's always-false Suspended would).
type scriptedRollHost struct {
	askHost
	seq       []int
	i         int
	suspended bool
}

func (h *scriptedRollHost) Rand(n int) int {
	if h.i < len(h.seq) {
		v := h.seq[h.i]
		h.i++
		return v % n
	}
	return 0
}

func (h *scriptedRollHost) Ask(d *decision.Decision) bool {
	h.askHost.Ask(d)
	h.suspended = true
	return true
}

func (h *scriptedRollHost) Suspended() bool { return h.suspended }

// rollResults collects the die values the host's roll Notes recorded ("rolls
// a dN: X", the raw die — a Modifier$ would append " + m = r" after it).
func rollResults(t *testing.T, h *scriptedRollHost) []int32 {
	t.Helper()
	var out []int32
	for _, e := range h.log {
		if e.Kind != events.Note {
			continue
		}
		rest, ok := strings.CutPrefix(e.Text, "rolls a d")
		if !ok {
			continue
		}
		if i := strings.IndexByte(rest, ':'); i >= 0 {
			f := strings.TrimSpace(rest[i+1:])
			if j := strings.IndexByte(f, ' '); j >= 0 {
				f = f[:j]
			}
			n, err := strconv.Atoi(f)
			if err != nil {
				t.Fatalf("unparseable roll Note %q", e.Text)
			}
			out = append(out, int32(n))
		}
	}
	return out
}

// TestBoomflingerPublishesTheDifferenceBetweenRolls is the
// UseDifferenceBetweenRolls$ leaf (real corpus Boomflinger): two d6 rolls of
// 4 and 2 publish |4-2| = 2 under the ResultSVar$ name, and the chained
// DBDamage reads NumDmg$ Result through the publication and deals exactly 2
// to the preset target.
func TestBoomflingerPublishesTheDifferenceBetweenRolls(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	bf, ok := reg.Lookup("Boomflinger")
	if !ok {
		t.Fatal("corpus fixture: Boomflinger missing")
	}
	g := state.NewGame([]string{"a", "b"})
	g.Tokens = reg.Tokens
	h := &scriptedRollHost{seq: []int{3, 1}} // Rand draws 3, 1 → dice 4, 2
	h.g = g
	src := g.AddObject(bf, 0)
	src.Zone = state.ZBattlefield
	face := bf.Faces[0]
	saTrig := cards.ResolveSVar(face.SVars, "TrigCrank")
	if saTrig == nil {
		t.Fatal("corpus fixture: Boomflinger has no TrigCrank SVar")
	}
	ctx := &Ctx{Controller: 0, Source: src.ID, SVars: face.SVars,
		Targets:   []state.Target{{Player: 1, IsPlayer: true}},
		OfferedSA: saTrig.Sub}
	life1 := g.Players[1].Life
	Resolve(h, ctx, saTrig)
	if rolls := rollResults(t, h); len(rolls) != 2 || rolls[0] != 4 || rolls[1] != 2 {
		t.Fatalf("roll Notes = %v, want [4 2]", rolls)
	}
	if ctx.LastRollName != "Result" || ctx.LastRoll != 2 {
		t.Fatalf("publication = %s/%d, want Result/2 (|4-2|)", ctx.LastRollName, ctx.LastRoll)
	}
	if got := life1 - g.Players[1].Life; got != 2 {
		t.Fatalf("seat 1 took %d damage, want 2 (the difference, read through NumDmg$ Result)", got)
	}
}

// TestNeverwinterHydraPublishesTheTotalOfXRolls is the multi-roll
// ResultSVar$ leaf (real corpus Neverwinter Hydra): Amount$ X = 3 rolls three
// d6 (5, 2, 6 here) and publishes the TOTAL 13 under Result, which the
// chained DBCounters reads through CounterNum$ Result — the card's own oracle
// wording, "a number of +1/+1 counters on it equal to the total of those
// results". Before the fix no multi-roll resolution published at all, so the
// Hydra entered with zero counters.
func TestNeverwinterHydraPublishesTheTotalOfXRolls(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	nh, ok := reg.Lookup("Neverwinter Hydra")
	if !ok {
		t.Fatal("corpus fixture: Neverwinter Hydra missing")
	}
	g := state.NewGame([]string{"a", "b"})
	g.Tokens = reg.Tokens
	h := &scriptedRollHost{seq: []int{4, 1, 5}} // dice 5, 2, 6
	h.g = g
	src := g.AddObject(nh, 0)
	src.Zone = state.ZBattlefield
	face := nh.Faces[0]
	saRoll := cards.ResolveSVar(face.SVars, "RollCounters")
	if saRoll == nil {
		t.Fatal("corpus fixture: Neverwinter Hydra has no RollCounters SVar")
	}
	ctx := &Ctx{Controller: 0, Source: src.ID, SVars: face.SVars, X: 3}
	Resolve(h, ctx, saRoll)
	if rolls := rollResults(t, h); len(rolls) != 3 || rolls[0] != 5 || rolls[1] != 2 || rolls[2] != 6 {
		t.Fatalf("roll Notes = %v, want [5 2 6]", rolls)
	}
	if ctx.LastRollName != "Result" || ctx.LastRoll != 13 {
		t.Fatalf("publication = %s/%d, want Result/13 (the total of 5+2+6)", ctx.LastRollName, ctx.LastRoll)
	}
	if got := src.Counter("P1P1"); got != 13 {
		t.Fatalf("the Hydra entered with %d +1/+1 counters, want 13", got)
	}
}

// TestBerserkersFrenzyIgnoresItsLowerRoll is the IgnoreLower$ regression
// (real corpus Berserker's Frenzy): rolls 2 then 20 still record both dice,
// but only the retained 20 branch resolves. In particular, the ignored 2
// must not reach MustBlock (an unimplemented ChooseCard); the 20 branch does
// reach its real continuous-effect SVar.
func TestBerserkersFrenzyIgnoresItsLowerRoll(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	bf, ok := reg.Lookup("Berserker's Frenzy")
	if !ok {
		t.Fatal("corpus fixture: Berserker's Frenzy missing")
	}
	g := state.NewGame([]string{"a", "b"})
	h := &scriptedRollHost{seq: []int{1, 19}} // dice 2, 20
	h.g = g
	src := g.AddObject(bf, 0)
	face := bf.Faces[0]
	var saRoll *cards.SA
	for _, a := range face.Abilities {
		if a.API == "RollDice" {
			saRoll = a
			break
		}
	}
	if saRoll == nil {
		t.Fatal("corpus fixture: Berserker's Frenzy has no RollDice ability")
	}
	Resolve(h, &Ctx{Controller: 0, Source: src.ID, SVars: face.SVars}, saRoll)
	if rolls := rollResults(t, h); len(rolls) != 2 || rolls[0] != 2 || rolls[1] != 20 {
		t.Fatalf("roll Notes = %v, want [2 20]", rolls)
	}
	var ranHigh bool
	for _, e := range h.log {
		if e.Kind != events.Note {
			continue
		}
		if e.Text == "unimplemented API ChooseCard" {
			t.Fatal("ignored low roll reached Berserker's Frenzy MustBlock branch")
		}
		if strings.Contains(e.Text, "continuous effect Continuous") {
			ranHigh = true
		}
	}
	if !ranHigh {
		t.Fatal("retained high roll did not reach Berserker's Frenzy ChooseBlock branch")
	}
}

// TestIronMastiffUsesOnlyItsHighestRoll is the UseHighestRoll$ regression
// (real corpus Iron Mastiff): its d20 rolls 20 then 1, but only the retained
// 20 range deals 4 to the opponent. The ignored 1 must not also deal 4 to
// Iron Mastiff's controller.
func TestIronMastiffUsesOnlyItsHighestRoll(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	im, ok := reg.Lookup("Iron Mastiff")
	if !ok {
		t.Fatal("corpus fixture: Iron Mastiff missing")
	}
	// The corpus's NbAttackedPlayers SVar is populated by its attack trigger;
	// supply the real two-player-attacked result here while retaining the real
	// RollDice SA and its result ranges. (The Count evaluator does not yet
	// parse Forge's bare PlayerCountOpponents$... body; see this task's report.)
	g := state.NewGame([]string{"a", "b", "c"})
	h := &scriptedRollHost{seq: []int{19, 0}} // dice 20, 1
	h.g = g
	src := g.AddObject(im, 0)
	src.Zone = state.ZBattlefield
	face := im.Faces[0]
	saRoll := cards.ResolveSVar(face.SVars, "DBTrigRollDice")
	if saRoll == nil {
		t.Fatal("corpus fixture: Iron Mastiff has no DBTrigRollDice SVar")
	}
	svars := make(map[string]string, len(face.SVars))
	for name, body := range face.SVars {
		svars[name] = body
	}
	svars["NbAttackedPlayers"] = "2"
	life0, life1, life2 := g.Players[0].Life, g.Players[1].Life, g.Players[2].Life
	// Defined$ Player.Opponent now resolves for real (context.go's "Opponent"/
	// "Player.Opponent" case): every living seat but the controller, which for
	// the 20 range's "deals damage ... to each opponent" is both seat 1 and
	// seat 2 here, not merely whichever seat the trigger happened to target.
	Resolve(h, &Ctx{Controller: 0, Source: src.ID, SVars: svars,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}}, saRoll)
	if rolls := rollResults(t, h); len(rolls) != 2 || rolls[0] != 20 || rolls[1] != 1 {
		t.Fatalf("roll Notes = %v, want [20 1]", rolls)
	}
	if got := life0 - g.Players[0].Life; got != 0 {
		t.Fatalf("controller took %d damage, want 0: ignored low roll must not run its 1-9 range", got)
	}
	if got := life1 - g.Players[1].Life; got != 4 {
		t.Fatalf("seat 1 took %d damage, want 4 from the retained 20 range", got)
	}
	if got := life2 - g.Players[2].Life; got != 4 {
		t.Fatalf("seat 2 took %d damage, want 4: the 20 range hits every opponent, not just the attacked one", got)
	}
}

// TestLuckBobbleheadRollsEveryControlledBobblehead is the computed-Amount$
// leaf (real corpus Luck Bobblehead): X is the number of Bobbleheads you
// control, so this 257-object battlefield rolls all 257 dice. In particular,
// Amount$ has no semantic cap: token/copy effects can create more objects than
// a deck's card count. Every scripted die is a six, making MaxRolls and
// EvenResults both 257 and driving 257 Treasure creations through the real
// SVar$EvenResults sub-ability.
func TestLuckBobbleheadRollsEveryControlledBobblehead(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lb, ok := reg.Lookup("Luck Bobblehead")
	if !ok {
		t.Fatal("corpus fixture: Luck Bobblehead missing")
	}
	g := state.NewGame([]string{"a", "b"})
	g.Tokens = reg.Tokens
	h := &scriptedRollHost{}
	h.g = g
	var ids []state.ObjID
	for i := 0; i < 257; i++ {
		ids = append(ids, g.AddObject(lb, 0).ID)
	}
	g.SetZone(state.ZBattlefield, 0, ids)
	face := lb.Faces[0]
	var saRoll *cards.SA
	for _, a := range face.Abilities {
		if a.API == "RollDice" {
			saRoll = a
			break
		}
	}
	if saRoll == nil {
		t.Fatal("corpus fixture: Luck Bobblehead has no RollDice ability")
	}
	// Script every die to six. A truncated roll loop would lower every
	// published count and the card's own Treasure total.
	seq := make([]int, 257)
	for i := range seq {
		seq[i] = 5
	}
	h.seq = seq
	ctx := &Ctx{Controller: 0, Source: ids[0], SVars: face.SVars}
	Resolve(h, ctx, saRoll)
	if rolls := rollResults(t, h); len(rolls) != 257 {
		t.Fatalf("only %d dice rolled: a computed Amount$ must not be truncated", len(rolls))
	}
	if v, ok := rollPublished(ctx, "MaxRolls"); !ok || v != 257 {
		t.Fatalf("MaxRolls publication = %d/%v, want 257", v, ok)
	}
	if v, ok := rollPublished(ctx, "EvenResults"); !ok || v != 257 {
		t.Fatalf("EvenResults publication = %d/%v, want 257", v, ok)
	}
	if v, ok := rollPublished(ctx, "OddResults"); !ok || v != 0 {
		t.Fatalf("OddResults publication = %d/%v, want 0", v, ok)
	}
	treasures := 0
	for _, id := range g.Zone(state.ZBattlefield, 0) {
		o := g.Obj(id)
		if o != nil && o.IsToken && o.Face() != nil && strings.Contains(o.Face().Name, "Treasure") {
			treasures++
		}
	}
	if treasures != 257 {
		t.Fatalf("%d Treasure tokens created, want 257 (one per even result, read through TokenAmount$ Y = SVar$EvenResults)", treasures)
	}
}

// TestValiantEndeavorAsksThenPublishesChosenAndOther is the ChosenSVar$/
// OtherSVar$ leaf (real corpus Valiant Endeavor): the two-die roll SUSPENDS
// on a Min==Max==1 KChoose over the rolled results, nothing resolves before
// the answer, and the answered pick publishes ChosenSVar$ (X — the picked
// die's result, consumed by DBDestroy's Creature.powerGEX) and OtherSVar$ (Y
// — the other die's result, consumed by DBToken's TokenAmount$). Picking the
// SECOND die proves the answer, not a first-die default, drives the
// resolution.
func TestValiantEndeavorAsksThenPublishesChosenAndOther(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	ve, ok := reg.Lookup("Valiant Endeavor")
	if !ok {
		t.Fatal("corpus fixture: Valiant Endeavor missing")
	}
	g := state.NewGame([]string{"a", "b"})
	g.Tokens = reg.Tokens
	h := &scriptedRollHost{seq: []int{4, 2}} // dice 5, 3
	h.g = g
	src := g.AddObject(ve, 0)
	src.Zone = state.ZBattlefield
	// Four known-power creatures across both battlefields: powers 1, 2, 4, 6.
	powers := []int32{1, 2, 4, 6}
	seatCre := map[state.PlayerID][]state.ObjID{0: {}, 1: {}}
	for i, p := range powers {
		c := mkCard(t, "Name:C"+strconv.Itoa(i)+"\nTypes:Creature\nPT:"+strconv.FormatInt(int64(p), 10)+"/1\nOracle:x\n")
		o := g.AddObject(c, state.PlayerID(i%2))
		o.Zone = state.ZBattlefield
		seatCre[state.PlayerID(i%2)] = append(seatCre[state.PlayerID(i%2)], o.ID)
	}
	g.SetZone(state.ZBattlefield, 0, append([]state.ObjID{src.ID}, seatCre[0]...))
	g.SetZone(state.ZBattlefield, 1, seatCre[1])
	face := ve.Faces[0]
	var saRoll *cards.SA
	for _, a := range face.Abilities {
		if a.API == "RollDice" {
			saRoll = a
			break
		}
	}
	if saRoll == nil {
		t.Fatal("corpus fixture: Valiant Endeavor has no RollDice ability")
	}
	ctx := &Ctx{Controller: 0, Source: src.ID, SVars: face.SVars}
	Resolve(h, ctx, saRoll)
	d := h.asked
	if d == nil {
		t.Fatal("no decision was posed: a two-die ChosenSVar$ roll must ask")
	}
	if d.Kind != decision.KChoose || d.Player != 0 || d.Min != 1 || d.Max != 1 || d.ResumeKind != "roll" {
		t.Fatalf("decision = %+v, want a Min==Max==1 KChoose with ResumeKind roll for the controller", d)
	}
	if len(d.Options) != 2 || d.Options[0].Kind != "roll" || !strings.Contains(d.Options[0].Label, "5") ||
		!strings.Contains(d.Options[1].Label, "3") {
		t.Fatalf("options = %+v, want one roll option per die naming its result (5, 3)", d.Options)
	}
	if len(d.Rolls) != 2 || d.Rolls[0] != 5 || d.Rolls[1] != 3 {
		t.Fatalf("decision Rolls = %v, want the per-die results [5 3]", d.Rolls)
	}
	alive := func() map[state.ObjID]bool {
		m := map[state.ObjID]bool{}
		for _, p := range g.AliveFrom(0) {
			for _, id := range g.Zone(state.ZBattlefield, p) {
				m[id] = true
			}
		}
		return m
	}
	// The suspension held: nothing resolved before the answer (the four
	// creatures plus Valiant itself are still on their battlefields).
	if len(alive()) != 5 {
		t.Fatalf("creatures died before the answer: %+v", alive())
	}
	for _, id := range g.Zone(state.ZBattlefield, 0) {
		if o := g.Obj(id); o != nil && o.IsToken {
			t.Fatal("a token was created before the answer")
		}
	}
	// Re-entry, the engine's contract: the second die was picked — chosen 3,
	// other 5 — and the chain now runs: every creature with power >= 3 dies,
	// five Knights arrive.
	h.suspended = false
	ctx2 := &Ctx{Controller: 0, Source: src.ID, SVars: face.SVars,
		RollResults: d.Rolls, RollPick: []int{1}, RollDone: true}
	Resolve(h, ctx2, saRoll)
	if ctx2.LastRollName != "X" || ctx2.LastRoll != 3 {
		t.Fatalf("chosen publication = %s/%d, want X/3", ctx2.LastRollName, ctx2.LastRoll)
	}
	if v, ok := rollPublished(ctx2, "Y"); !ok || v != 5 {
		t.Fatalf("other publication Y = %d/%v, want 5", v, ok)
	}
	for i, p := range powers {
		shouldBeDead := p >= 3
		o := findObj(t, g, "C"+strconv.Itoa(i))
		if shouldBeDead && o.Zone == state.ZBattlefield {
			t.Fatalf("creature with power %d survived (powerGEX with X=3)", p)
		}
		if !shouldBeDead && o.Zone != state.ZBattlefield {
			t.Fatalf("creature with power %d died but is below the chosen result X=3", p)
		}
	}
	knights := 0
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if o := g.Obj(id); o != nil && o.IsToken && o.Face() != nil && strings.Contains(o.Face().Name, "Knight") {
				knights++
			}
		}
	}
	if knights != 5 {
		t.Fatalf("%d Knight tokens created, want 5 (the other result, read through TokenAmount$ Y)", knights)
	}
}

// findObj returns the object whose face is named name, scanning every zone
// (a destroyed creature is in its graveyard and must still be inspectable).
func findObj(t *testing.T, g *state.Game, name string) *state.Object {
	t.Helper()
	for i := range g.Objs {
		if o := &g.Objs[i]; o.Face() != nil && o.Face().Name == name {
			return o
		}
	}
	t.Fatalf("no object named %q anywhere in the game", name)
	return nil
}
