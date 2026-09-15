package effects

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// choiceCorpus is the corpus registry opened once for this file's tests:
// each open decodes the whole compiled corpus (~0.25s), and a registry is
// read-only after load.
var choiceCorpus struct {
	sync.Mutex
	reg *cards.Registry
}

func choiceCorpusRegistry(t *testing.T) *cards.Registry {
	t.Helper()
	choiceCorpus.Lock()
	// Deferred so a Skip/Fatal inside CorpusRegistry (runtime.Goexit)
	// cannot leave the lock held for the next test.
	defer choiceCorpus.Unlock()
	if choiceCorpus.reg == nil {
		choiceCorpus.reg = testutil.CorpusRegistry(t)
	}
	return choiceCorpus.reg
}

func corpusSA(t *testing.T, card, name string) (*cards.Card, *cards.SA) {
	t.Helper()
	r := choiceCorpusRegistry(t)
	c, ok := r.Lookup(card)
	if !ok {
		t.Fatalf("missing corpus card %q", card)
	}
	for _, f := range c.Faces {
		if name == "" && len(f.Abilities) > 0 {
			return c, f.Abilities[0]
		}
		if sa := cards.ResolveSVar(f.SVars, name); sa != nil {
			return c, sa
		}
	}
	t.Fatalf("missing %s on %s", name, card)
	return nil, nil
}

func TestChooseCardDauthiCarriesChosenCardIntoEffect(t *testing.T) {
	card, sa := corpusSA(t, "Dauthi Voidwalker", "")
	h := newHost(t, 2)
	src := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	chosen := h.g.AddObject(mkCard(t, "Name:Exiled\nTypes:Sorcery\nOracle:x\n"), 1)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: chosen.ID, From: state.ZLibrary, To: state.ZExile})
	h.Emit(events.Event{Kind: events.CounterChange, Obj: chosen.ID, Counter: "VOID", Amount: 1})
	ctx := &Ctx{Source: src.ID, Controller: 0, SVars: src.Face().SVars}
	Resolve(h, ctx, sa)
	if len(ctx.Chosen) != 1 || ctx.Chosen[0].Obj != chosen.ID {
		t.Fatalf("Dauthi choice = %+v, want %d", ctx.Chosen, chosen.ID)
	}
	db := cards.ResolveSVar(src.Face().SVars, "DBEffect")
	if got := effectRemembered(h, ctx, db); len(got) != 1 || got[0] != chosen.ID {
		t.Fatalf("RememberObjects$ ChosenCard = %v, want [%d]", got, chosen.ID)
	}
}

func TestChooseCardExiledWithCorpusSA(t *testing.T) {
	card, sa := corpusSA(t, "Ore-Rich Stalactite", "")
	if sa.API != "Mana" { // front face; Cosmium Catalyst is alternate face.
		t.Fatalf("unexpected front ability %+v", sa)
	}
	var choose *cards.SA
	for _, f := range card.Faces {
		for _, a := range f.Abilities {
			if a.API == "ChooseCard" && a.Params["DefinedCards"] == "ExiledWith" {
				choose = a
			}
		}
	}
	if choose == nil {
		t.Fatal("Cosmium Catalyst's ExiledWith choice missing")
	}
	h := newHost(t, 2)
	src := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	mine := h.g.AddObject(mkCard(t, "Name:Crafted\nTypes:Instant\nOracle:x\n"), 0)
	other := h.g.AddObject(mkCard(t, "Name:Other\nTypes:Instant\nOracle:x\n"), 1)
	for _, o := range []*state.Object{mine, other} {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZExile})
	}
	h.Emit(events.Event{Kind: events.Imprint, Obj: src.ID, IDs: []state.ObjID{mine.ID}})
	c := &Ctx{Source: src.ID, Controller: 0}
	effChooseCard(h, c, choose)
	if len(c.Chosen) != 1 || c.Chosen[0].Obj != mine.ID {
		t.Fatalf("ExiledWith choice = %+v, want only crafted %d", c.Chosen, mine.ID)
	}
	if got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "Imprinted"}}); len(got) != 1 || got[0].Obj != mine.ID {
		t.Fatalf("Defined Imprinted = %+v, want crafted card", got)
	}
}

func TestGainControlImprintedControllerSuddenSubstitution(t *testing.T) {
	card, gain := corpusSA(t, "Sudden Substitution", "DBGainControl")
	h := newHost(t, 2)
	src := h.g.AddObject(card, 0)
	target := h.g.AddObject(mkCard(t, "Name:Creature\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	imprinted := h.g.AddObject(mkCard(t, "Name:Spell\nTypes:Instant\nOracle:x\n"), 1)
	for _, o := range []*state.Object{src, target, imprinted} {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	h.Emit(events.Event{Kind: events.Imprint, Obj: src.ID, IDs: []state.ObjID{imprinted.ID}})
	effGainControl(h, &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: target.ID}}}, gain)
	if got := h.g.Obj(target.ID).Controller; got != 1 {
		t.Fatalf("Sudden Substitution target controller = %d, want imprinted controller 1", got)
	}
}

func TestChoosePlayerReplacesPlayerChoiceAndKeepsCards(t *testing.T) {
	card, chooseCard := corpusSA(t, "Dauthi Voidwalker", "")
	_, choosePlayer := corpusSA(t, "Sower of Discord", "ChooseP")
	h := newHost(t, 3)
	src := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	exiled := h.g.AddObject(mkCard(t, "Name:Exiled\nTypes:Sorcery\nOracle:x\n"), 1)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: exiled.ID, From: state.ZLibrary, To: state.ZExile})
	h.Emit(events.Event{Kind: events.CounterChange, Obj: exiled.ID, Counter: "VOID", Amount: 1})
	c := &Ctx{Source: src.ID, Controller: 0}
	effChooseCard(h, c, chooseCard)
	if len(c.Chosen) != 1 || c.Chosen[0].Obj != exiled.ID {
		t.Fatalf("card choice = %+v, want exiled card", c.Chosen)
	}
	effChoosePlayer(h, c, choosePlayer)
	effChoosePlayer(h, c, choosePlayer) // a second player choice replaces the first
	cards, players := 0, 0
	for _, chosen := range c.Chosen {
		if chosen.IsPlayer {
			players++
		} else if chosen.Obj == exiled.ID {
			cards++
		}
	}
	if cards != 1 || players != 1 {
		t.Fatalf("ChoosePlayer must retain cards and replace players, got %+v", c.Chosen)
	}
}

func TestChooseCardLastOneStandingCorpusSA(t *testing.T) {
	_, sa := corpusSA(t, "Last One Standing", "")
	if sa.API != "ChooseCard" || sa.Params["AtRandom"] != "True" {
		t.Fatalf("unexpected SA: %+v", sa)
	}
	h := newHost(t, 2)
	creature := h.g.AddObject(mkCard(t, "Name:A\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: creature.ID, From: state.ZLibrary, To: state.ZBattlefield})
	c := &Ctx{Controller: 0}
	effChooseCard(h, c, sa)
	if len(c.Chosen) != 1 || c.Chosen[0].Obj != creature.ID {
		t.Fatalf("Last One Standing chose %+v, want creature %d", c.Chosen, creature.ID)
	}
}

func TestChooseCardMountDoomAllowsZeroOneOrTwo(t *testing.T) {
	card, _ := corpusSA(t, "Mount Doom", "")
	var choose *cards.SA
	for _, sa := range card.Faces[0].Abilities {
		if sa.API == "ChooseCard" {
			choose = sa
			break
		}
	}
	if choose == nil || choose.Params["Amount"] != "2" || choose.Params["Mandatory"] != "" {
		t.Fatalf("Mount Doom ChooseCard fixture changed: %+v", choose)
	}
	h := &askHost{fakeHost: *newHost(t, 2)}
	for i := 0; i < 2; i++ {
		creature := h.g.AddObject(mkCard(t, "Name:Creature\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: creature.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	effChooseCard(h, &Ctx{Controller: 0}, choose)
	d := h.asked
	if d == nil || d.Min != 0 || d.Max != 2 || len(d.Options) != 2 {
		t.Fatalf("Mount Doom choice = %+v, want 0..2 over two creatures", d)
	}
	for _, picks := range [][]int{nil, {0}, {0, 1}} {
		if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: picks}); err != nil {
			t.Errorf("Mount Doom rejected legal %d-card answer: %v", len(picks), err)
		}
	}
}

func TestChooseCardWildSwingUsesOnlyDefinedTargets(t *testing.T) {
	_, sa := corpusSA(t, "Wild Swing", "DBChooseRandom")
	if sa.API != "ChooseCard" || sa.Params["DefinedCards"] != "Targeted" {
		t.Fatalf("Wild Swing fixture changed: %+v", sa)
	}
	h := newHost(t, 2)
	targeted := h.g.AddObject(mkCard(t, "Name:Targeted\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	unrelated := h.g.AddObject(mkCard(t, "Name:Unrelated\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	for _, o := range []*state.Object{targeted, unrelated} {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	c := &Ctx{Controller: 0, Targets: []state.Target{{Obj: targeted.ID}}}
	effChooseCard(h, c, sa)
	if len(c.Chosen) != 1 || c.Chosen[0].Obj != targeted.ID {
		t.Fatalf("Wild Swing chose %+v, want only targeted permanent %d (unrelated %d)", c.Chosen, targeted.ID, unrelated.ID)
	}
}

func TestChooseCardJuggleSelectsRandomAmountWithoutReplacement(t *testing.T) {
	_, sa := corpusSA(t, "Juggle the Performance", "DBChooseCard")
	if sa.API != "ChooseCard" || sa.Params["AtRandom"] != "True" || sa.Params["Amount"] != "7" {
		t.Fatalf("Juggle the Performance fixture changed: %+v", sa)
	}
	h := newHost(t, 2)
	for p := state.PlayerID(0); p < 2; p++ {
		for i := 0; i < 7; i++ {
			h.g.AddObject(mkCard(t, "Name:Library Card\nTypes:Sorcery\nOracle:x\n"), p)
		}
	}
	c := &Ctx{Controller: 0}
	effChooseCard(h, c, sa)
	if len(c.Chosen) != 14 || h.n != 14 {
		t.Fatalf("Juggle chose %d cards with %d random draws, want seven for each of two players", len(c.Chosen), h.n)
	}
	for start := 0; start < len(c.Chosen); start += 7 {
		seen := map[state.ObjID]bool{}
		for _, picked := range c.Chosen[start : start+7] {
			if seen[picked.Obj] {
				t.Fatalf("Juggle chose card %d twice for one player: %+v", picked.Obj, c.Chosen[start:start+7])
			}
			seen[picked.Obj] = true
		}
	}
}

func TestChoosePlayerSowerOfDiscordCorpusSA(t *testing.T) {
	_, sa := corpusSA(t, "Sower of Discord", "ChooseP")
	if sa.API != "ChoosePlayer" || sa.Params["RememberChosen"] != "True" {
		t.Fatalf("unexpected SA: %+v", sa)
	}
	h := newHost(t, 2)
	c := &Ctx{Controller: 0}
	effChoosePlayer(h, c, sa)
	if len(c.Remembered) != 1 || !c.Remembered[0].IsPlayer {
		t.Fatalf("choice did not remember player: %+v", c.Remembered)
	}
}

func TestGainControlEmrakulCorpusSA(t *testing.T) {
	_, sa := corpusSA(t, "Emrakul, the World Anew", "TrigGainControl")
	if sa.API != "GainControl" {
		t.Fatalf("unexpected SA: %+v", sa)
	}
	g, ids := board(t)
	h := &fakeHost{g: g}
	effGainControl(h, &Ctx{Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}, sa)
	if g.Obj(ids["theirBig"]).Controller != 0 {
		t.Fatal("Emrakul did not gain the targeted player's creature")
	}
}

func TestChangeTargetsCommandeerCorpusSA(t *testing.T) {
	_, sa := corpusSA(t, "Commandeer", "DBChooseTargets")
	if sa.API != "ChangeTargets" {
		t.Fatalf("unexpected SA: %+v", sa)
	}
	h := newHost(t, 2)
	spell := h.g.AddObject(mkCard(t, "Name:Target\nTypes:Instant\nManaCost:R\nOracle:x\nA:SP$ DealDamage | ValidTgts$ Player | NumDmg$ 1\n"), 1)
	spell.Zone = state.ZStack
	spell.Targets = []state.Target{{Player: 1, IsPlayer: true}}
	c := &Ctx{Controller: 0, Targets: []state.Target{{Obj: spell.ID}}}
	effChangeTargets(h, c, sa)
	if c.ChoiceDone || len(spell.Targets) != 1 || !spell.Targets[0].IsPlayer || spell.Targets[0].Player != 1 {
		t.Fatalf("hostless ChangeTargets must keep the targets and leave no answer behind: done=%v targets=%+v", c.ChoiceDone, spell.Targets)
	}
}

func TestControlSpellCommandeerCorpusSA(t *testing.T) {
	_, sa := corpusSA(t, "Commandeer", "")
	if sa.API != "ControlSpell" {
		t.Fatalf("unexpected SA: %+v", sa)
	}
	h := newHost(t, 2)
	spell := h.g.AddObject(mkCard(t, "Name:Target\nTypes:Instant\nManaCost:R\nOracle:x\n"), 1)
	spell.Zone = state.ZStack
	effControlSpell(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: spell.ID}}}, sa)
	if spell.Controller != 0 {
		t.Fatal("Commandeer did not control the spell")
	}
}

func TestRepeatEachBraidsKeepsResolvingController(t *testing.T) {
	card, sa := corpusSA(t, "Braids, Arisen Nightmare", "DBRepeatEach")
	if sa.API != "RepeatEach" {
		t.Fatalf("unexpected SA: %+v", sa)
	}
	h := newHost(t, 3)
	braids := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: braids.ID, From: state.ZLibrary, To: state.ZBattlefield})
	// Keep the corpus RepeatEach SA but replace its inner body with the
	// smallest observable Defined$ You effect. The real nested sacrifice path
	// also needs Player.IsRemembered, a separate filter primitive; this fixture
	// isolates RepeatEach's controller/loop-subject contract.
	for i := 0; i < 2; i++ {
		draw := h.g.AddObject(mkCard(t, "Name:Draw\nTypes:Land\nOracle:x\n"), 0)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: draw.ID, From: state.ZLibrary, To: state.ZLibrary})
	}
	c := &Ctx{Source: braids.ID, Controller: 0, SVars: map[string]string{"DBMaySac": "DB$ Draw | Defined$ You | NumCards$ 1"}}
	effRepeatEach(h, c, sa)
	if got := len(h.g.Zone(state.ZHand, 0)); got != 2 {
		t.Fatalf("Braids controller drew %d cards, want 2", got)
	}
	if got := len(h.g.Zone(state.ZHand, 1)) + len(h.g.Zone(state.ZHand, 2)); got != 0 {
		t.Fatalf("Braids opponents drew %d cards", got)
	}
}

func TestRepeatEachRakdosCharmRepeatCards(t *testing.T) {
	card, sa := corpusSA(t, "Rakdos Charm", "CreatureDamage")
	h := newHost(t, 2)
	src := h.g.AddObject(card, 0)
	for p := state.PlayerID(0); p < 2; p++ {
		creature := h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"), p)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: creature.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	Resolve(h, &Ctx{Source: src.ID, Controller: 0, SVars: src.Face().SVars}, sa)
	if h.g.Players[0].Life != 19 || h.g.Players[1].Life != 19 {
		t.Fatalf("RepeatCards life = [%d %d], want [19 19]", h.g.Players[0].Life, h.g.Players[1].Life)
	}
}

func TestRepeatEachWhirlwindDenialRepeatSpellAbilities(t *testing.T) {
	card, sa := corpusSA(t, "Whirlwind Denial", "")
	h := newHost(t, 2)
	src := h.g.AddObject(card, 0)
	for i := 0; i < 2; i++ {
		spell := h.g.AddObject(mkCard(t, "Name:Spell\nTypes:Instant\nManaCost:R\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"), 1)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: spell.ID, From: state.ZLibrary, To: state.ZStack})
	}
	svars := make(map[string]string, len(src.Face().SVars))
	for k, v := range src.Face().SVars {
		svars[k] = v
	}
	// Counter's UnlessCost decision is independent of RepeatSpellAbilities;
	// make each real selected stack subject observable without suspending.
	svars["DBCounterUnless"] = "DB$ LoseLife | Defined$ RememberedController | LifeAmount$ 1"
	Resolve(h, &Ctx{Source: src.ID, Controller: 0, SVars: svars}, sa)
	if h.g.Players[1].Life != 18 {
		t.Fatalf("RepeatSpellAbilities opponent life = %d, want 18", h.g.Players[1].Life)
	}
}

func TestRepeatEachArchfiendCorpusSA(t *testing.T) {
	_, sa := corpusSA(t, "Archfiend of Despair", "RepeatOpps")
	if sa.API != "RepeatEach" {
		t.Fatalf("unexpected SA: %+v", sa)
	}
	h := newHost(t, 2)
	c := &Ctx{Controller: 0, SVars: map[string]string{"TrigLoseLife": "DB$ LoseLife | Defined$ Remembered | LifeAmount$ 1"}}
	effRepeatEach(h, c, sa)
	if h.g.Players[1].Life != 19 {
		t.Fatalf("repeat did not bind opponent, life=%d", h.g.Players[1].Life)
	}
}

func TestBranchUnholyAnnexCorpusSA(t *testing.T) {
	card, sa := corpusSA(t, "Unholy Annex", "DBBranch")
	if sa.API != "Branch" {
		t.Fatalf("unexpected SA: %+v", sa)
	}
	h := newHost(t, 2)
	src := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	demon := h.g.AddObject(mkCard(t, "Name:Demon\nTypes:Creature Demon\nPT:1/1\nOracle:x\n"), 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: demon.ID, From: state.ZLibrary, To: state.ZBattlefield})
	effBranch(h, &Ctx{Controller: 0, Source: src.ID, SVars: src.Face().SVars}, sa)
	if h.g.Players[1].Life != 18 {
		t.Fatalf("true branch did not run, life=%d", h.g.Players[1].Life)
	}

	withoutDemon := newHost(t, 2)
	src = withoutDemon.g.AddObject(card, 0)
	withoutDemon.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	effBranch(withoutDemon, &Ctx{Controller: 0, Source: src.ID, SVars: src.Face().SVars}, sa)
	if withoutDemon.g.Players[0].Life != 18 || withoutDemon.g.Players[1].Life != 20 {
		t.Fatalf("false branch life = [%d %d], want [18 20]", withoutDemon.g.Players[0].Life, withoutDemon.g.Players[1].Life)
	}
}

func TestBranchResolvesNonLiteralComparator(t *testing.T) {
	h := newHost(t, 3)
	sa := &cards.SA{API: "Branch", Params: map[string]string{
		"BranchConditionSVar": "X", "BranchConditionSVarCompare": "EQPlayerCountOpponents$Amount",
		"TrueSubAbility": "Yes", "FalseSubAbility": "No",
	}}
	ctx := &Ctx{Controller: 0, SVars: map[string]string{
		"X": "Count$PlayerCountOpponents", "Yes": "DB$ LoseLife | Defined$ Opponent | LifeAmount$ 1", "No": "DB$ LoseLife | Defined$ You | LifeAmount$ 5",
	}}
	effBranch(h, ctx, sa)
	if h.g.Players[0].Life != 20 || h.g.Players[1].Life != 19 || h.g.Players[2].Life != 19 {
		t.Fatalf("non-literal comparator chose wrong arm: [%d %d %d]", h.g.Players[0].Life, h.g.Players[1].Life, h.g.Players[2].Life)
	}
}

// TestChoosePlayerBillFernyValidTgtsRestrictsPool is the ValidTgts$-only
// ChoosePlayer shape: with no Choices$, ValidTgts$ Opponent is the pool (the
// controller is never offered), and a player already targeted for it is the
// only choice.
func TestChoosePlayerBillFernyValidTgtsRestrictsPool(t *testing.T) {
	_, sa := corpusSA(t, "Bill Ferny, Bree Swindler", "DBChoose")
	if sa.API != "ChoosePlayer" || sa.Params["Choices"] != "" || sa.Params["ValidTgts"] != "Opponent" {
		t.Fatalf("Bill Ferny fixture changed: %+v", sa)
	}
	h := &askHost{fakeHost: *newHost(t, 3)}
	effChoosePlayer(h, &Ctx{Controller: 0}, sa)
	d := h.asked
	if d == nil || d.Player != 0 || d.Min != 1 || d.Max != 1 || len(d.Options) != 2 {
		t.Fatalf("untargeted Bill Ferny choice = %+v, want one of two opponents", d)
	}
	for _, o := range d.Options {
		if o.Player == 0 {
			t.Fatalf("Bill Ferny offered its controller: %+v", d.Options)
		}
	}

	h = &askHost{fakeHost: *newHost(t, 3)}
	effChoosePlayer(h, &Ctx{Controller: 0, Targets: []state.Target{{Player: 2, IsPlayer: true}}}, sa)
	if d = h.asked; d == nil || len(d.Options) != 1 || d.Options[0].Player != 2 {
		t.Fatalf("targeted Bill Ferny choice = %+v, want only targeted opponent 2", d)
	}
}

// TestRepeatEachPriceOfProgressCountsEachPlayersNonbasics proves the loop
// subject reaches the count filter: RememberedPlayerCtrl counts the lands of
// the player being repeated, not of anyone else.
func TestRepeatEachPriceOfProgressCountsEachPlayersNonbasics(t *testing.T) {
	card, sa := corpusSA(t, "Price of Progress", "")
	h := newHost(t, 2)
	src := h.g.AddObject(card, 0)
	for p, n := range []int{1, 2} {
		for i := 0; i < n; i++ {
			l := h.g.AddObject(mkCard(t, "Name:Nonbasic\nTypes:Land\nOracle:x\n"), state.PlayerID(p))
			h.Emit(events.Event{Kind: events.MoveZone, Obj: l.ID, From: state.ZLibrary, To: state.ZBattlefield})
		}
	}
	basic := h.g.AddObject(mkCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"), 1)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: basic.ID, From: state.ZLibrary, To: state.ZBattlefield})
	Resolve(h, &Ctx{Source: src.ID, Controller: 0, SVars: src.Face().SVars}, sa)
	if h.g.Players[0].Life != 18 || h.g.Players[1].Life != 16 {
		t.Fatalf("Price of Progress life = [%d %d], want [18 16]", h.g.Players[0].Life, h.g.Players[1].Life)
	}
}

// TestRepeatEachChaosDefilerKeepsIterationsRemembered proves what an
// iteration remembers outlives it: each opponent's chosen permanent
// (ControlledBy Remembered, RememberChosen$) is still remembered by the
// sub-ability after the loop, which destroys one of them.
func TestRepeatEachChaosDefilerKeepsIterationsRemembered(t *testing.T) {
	card, sa := corpusSA(t, "Chaos Defiler", "TrigRepeat")
	h := newHost(t, 3)
	src := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	perms := map[state.PlayerID]state.ObjID{}
	for _, p := range []state.PlayerID{1, 2} {
		o := h.g.AddObject(mkCard(t, "Name:Relic\nTypes:Artifact\nOracle:x\n"), p)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
		perms[p] = o.ID
	}
	c := &Ctx{Source: src.ID, Controller: 0, SVars: src.Face().SVars}
	Resolve(h, c, sa)
	destroyed := 0
	for _, id := range perms {
		if h.g.Obj(id).Zone == state.ZGraveyard {
			destroyed++
		}
	}
	if destroyed != 1 || h.g.Obj(src.ID).Zone != state.ZBattlefield {
		t.Fatalf("Chaos Defiler destroyed %d opponent permanents (source zone %v), want exactly 1", destroyed, h.g.Obj(src.ID).Zone)
	}
}

// TestBranchGravelighterDefaultsToGE1 is the no-BranchConditionSVarCompare$
// shape (31 corpus lines): Forge's default is GE1, so with no creature dead
// this turn (X=0) Gravelighter takes the false arm -- each player sacrifices
// a creature -- instead of drawing. (Its X head, ThisTurnEntered_..., is not
// modelled, so the X=1 case below holds X at a supported count.)
func TestBranchGravelighterDefaultsToGE1(t *testing.T) {
	card, sa := corpusSA(t, "Gravelighter", "TrigBranch")
	if sa.Params["BranchConditionSVarCompare"] != "" {
		t.Fatalf("Gravelighter fixture changed: %+v", sa)
	}
	h := newHost(t, 2)
	src := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	h.g.AddObject(mkCard(t, "Name:Book\nTypes:Sorcery\nOracle:x\n"), 0)
	bear := h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"), 1)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: bear.ID, From: state.ZLibrary, To: state.ZBattlefield})
	effBranch(h, &Ctx{Source: src.ID, Controller: 0, SVars: src.Face().SVars}, sa)
	// Seat 0's only creature is Gravelighter itself.
	gz, bz := h.g.Obj(src.ID).Zone, h.g.Obj(bear.ID).Zone
	if len(h.g.Zone(state.ZHand, 0)) != 0 || gz != state.ZGraveyard || bz != state.ZGraveyard {
		t.Fatalf("X=0 with no compare: hand=%d Gravelighter=%v bear=%v, want no draw and both creatures sacrificed",
			len(h.g.Zone(state.ZHand, 0)), gz, bz)
	}

	// X=1, still no compare: the true arm (draw, no sacrifice). The real
	// SVar X head is not modelled, so X is held at a supported count that is
	// 1 here -- the Branch SA and both arms are the card's own.
	h = newHost(t, 2)
	src = h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	book := h.g.AddObject(mkCard(t, "Name:Book\nTypes:Sorcery\nOracle:x\n"), 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: book.ID, From: state.ZLibrary, To: state.ZLibrary})
	bear = h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"), 1)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: bear.ID, From: state.ZLibrary, To: state.ZBattlefield})
	svars := make(map[string]string, len(src.Face().SVars))
	for k, v := range src.Face().SVars {
		svars[k] = v
	}
	svars["X"] = "Count$Valid Creature.YouCtrl"
	effBranch(h, &Ctx{Source: src.ID, Controller: 0, SVars: svars}, sa)
	gz, bz = h.g.Obj(src.ID).Zone, h.g.Obj(bear.ID).Zone
	if len(h.g.Zone(state.ZHand, 0)) != 1 || gz != state.ZBattlefield || bz != state.ZBattlefield {
		t.Fatalf("X=1 with no compare: hand=%d Gravelighter=%v bear=%v, want a draw and no sacrifice",
			len(h.g.Zone(state.ZHand, 0)), gz, bz)
	}
}

// TestGainControlNewControllerTriggeredPlayers: NewController$ TriggeredPlayer
// (Karona, False God) and TriggeredActivator (Drooling Ogre) hand control to
// the player the trigger names; an unbound referent changes nothing rather
// than defaulting to the effect's own controller.
func TestGainControlNewControllerTriggeredPlayers(t *testing.T) {
	for _, name := range []string{"Karona, False God", "Drooling Ogre"} {
		card, sa := corpusSA(t, name, "TrigControl")
		h := newHost(t, 3)
		src := h.g.AddObject(card, 0)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
		c := &Ctx{Source: src.ID, Controller: 0, SVars: src.Face().SVars}
		effGainControl(h, c, sa)
		if got := h.g.Obj(src.ID).Controller; got != 0 {
			t.Fatalf("%s with no trigger binding moved to %d", name, got)
		}
		c.TriggerPlayer = state.Target{Player: 2, IsPlayer: true}
		c.TriggerActivator = state.Target{Player: 2, IsPlayer: true}
		effGainControl(h, c, sa)
		if got := h.g.Obj(src.ID).Controller; got != 2 || !containsID(h.g.Zone(state.ZBattlefield, 2), src.ID) {
			t.Fatalf("%s controller = %d, want the triggering player 2", name, got)
		}
	}
}

// TestChooseCardWithNoCandidatesDoesNotAsk: a choice with nothing to choose
// poses no decision (a 0-option KChoose) and records an empty choice.
func TestChooseCardWithNoCandidatesDoesNotAsk(t *testing.T) {
	card, _ := corpusSA(t, "Mount Doom", "")
	var choose *cards.SA
	for _, sa := range card.Faces[0].Abilities {
		if sa.API == "ChooseCard" {
			choose = sa
		}
	}
	h := &askHost{fakeHost: *newHost(t, 2)}
	c := &Ctx{Controller: 0}
	effChooseCard(h, c, choose)
	if h.asked != nil || len(c.Chosen) != 0 {
		t.Fatalf("empty ChooseCard asked %+v chose %+v", h.asked, c.Chosen)
	}
}

// TestRepeatEachPlayerLoopKeepsRememberedCardsButNotTheTriggerObject: a
// player loop's iteration sees the cards the resolution remembered (Forge
// swaps out only remembered players) but not the object a trigger captured:
// Archfiend of Despair's end-step trigger must not make its own controller
// lose life through Defined$ Remembered.
func TestRepeatEachPlayerLoopKeepsRememberedCardsButNotTheTriggerObject(t *testing.T) {
	card, sa := corpusSA(t, "Archfiend of Despair", "RepeatOpps")
	h := newHost(t, 2)
	src := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	captured := []state.Target{{Obj: src.ID}}
	c := &Ctx{Source: src.ID, Controller: 0, Remembered: copyTargets(captured), Captured: copyTargets(captured),
		SVars: map[string]string{"TrigLoseLife": "DB$ LoseLife | Defined$ Remembered | LifeAmount$ 1"}}
	effRepeatEach(h, c, sa)
	if h.g.Players[0].Life != 20 || h.g.Players[1].Life != 19 {
		t.Fatalf("life = [%d %d], want [20 19]", h.g.Players[0].Life, h.g.Players[1].Life)
	}

	// A remembered (not captured) card stays visible to each iteration.
	relic := h.g.AddObject(mkCard(t, "Name:Relic\nTypes:Artifact\nOracle:x\n"), 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: relic.ID, From: state.ZLibrary, To: state.ZBattlefield})
	c = &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: src.ID}, {Obj: relic.ID}}, Captured: copyTargets(captured),
		SVars: map[string]string{"TrigLoseLife": "DB$ Destroy | Defined$ Remembered"}}
	effRepeatEach(h, c, sa)
	if rz, sz := h.g.Obj(relic.ID).Zone, h.g.Obj(src.ID).Zone; rz != state.ZGraveyard || sz != state.ZBattlefield {
		t.Fatalf("relic zone %v source zone %v, want the remembered relic destroyed and the trigger source kept", rz, sz)
	}
}
