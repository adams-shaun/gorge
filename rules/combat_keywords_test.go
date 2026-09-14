package rules

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// corpusKeywordCard keeps these proofs tied to the actual Forge scripts whose
// coverage entries this ticket retires, rather than lookalike handwritten cards.
func corpusKeywordCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	// The checked-in IR is intentionally a cache. Parse the live GPL corpus
	// at test time so keyword expansion added by this change is exercised.
	paths := map[string]string{
		"Vein Ripper": "v/vein_ripper.txt", "Artisan of Kozilek": "a/artisan_of_kozilek.txt",
		"Fury": "f/fury.txt", "Shriekmaw": "s/shriekmaw.txt", "Dauthi Voidwalker": "d/dauthi_voidwalker.txt",
		"Emrakul, the World Anew": "e/emrakul_the_world_anew.txt", "Yavimaya Scion": "y/yavimaya_scion.txt", "Guardian of the Guildpact": "g/guardian_of_the_guildpact.txt", "Frenemy of the Guildpact": "f/frenemy_of_the_guildpact.txt", "Kitesail Larcenist": "k/kitesail_larcenist.txt", "Karazikar, the Eye Tyrant": "k/karazikar_the_eye_tyrant.txt",
	}
	path, ok := paths[name]
	if !ok {
		t.Fatalf("no corpus path for %q", name)
	}
	c, ds := cards.Parse(filepath.Join("..", ".cards", "cardsfolder", path))
	if len(ds) != 0 {
		t.Fatalf("parse %s: %v", name, ds)
	}
	if ds = c.Link(); len(ds) != 0 {
		t.Fatalf("link %s: %v", name, ds)
	}
	return c
}

func TestWardVeinRipperCountersAnUnpaidTargetingSpell(t *testing.T) {
	e := combatEngine(t)
	warded := onBoardCard(t, e, 0, corpusKeywordCard(t, "Vein Ripper"))
	cause := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.PutOnStack, Obj: cause, Player: 1, From: state.ZLibrary, To: state.ZStack})
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: cause, IDs: []state.ObjID{warded}})
	e.putTriggersOnStack()
	e.resolveTop()
	if e.Pending() == nil {
		t.Fatal("Ward did not ask the targeting spell's controller to pay")
	}
	// The second option is decline.
	if err := e.Submit(decision.Intent{Seq: e.Pending().Seq, Player: 1, Choices: []int{1}}); err != nil {
		t.Fatal(err)
	}
	if got := e.G.Obj(cause).Zone; got != state.ZGraveyard {
		t.Fatalf("unpaid ward spell zone = %s, want graveyard", got)
	}
}

func TestWardKitesailLarcenistChargesTheNonzeroPayer(t *testing.T) {
	e := combatEngine(t)
	warded := onBoardCard(t, e, 0, corpusKeywordCard(t, "Kitesail Larcenist"))
	cause := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.PutOnStack, Obj: cause, Player: 1, From: state.ZLibrary, To: state.ZStack})
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: cause, IDs: []state.ObjID{warded}})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 1, Amount: 1})
	e.putTriggersOnStack()
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.ResumeKind != "unless_pay" {
		t.Fatalf("Kitesail ward did not ask to pay: %+v", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{0}}); err != nil {
		t.Fatal(err)
	}
	if got := e.G.Players[1].Pool.Total(); got != 0 {
		t.Fatalf("ward payer pool = %d, want 0 after paying", got)
	}
	if got := e.G.Obj(cause).Zone; got != state.ZStack {
		t.Fatalf("paid ward moved targeting spell to %s, want stack", got)
	}
}

func TestAnnihilatorArtisanSacrificesThePrintedAmount(t *testing.T) {
	e := combatEngine(t)
	a := onBoardCard(t, e, 0, corpusKeywordCard(t, "Artisan of Kozilek"))
	e.G.Obj(a).SummonSick = false
	p1 := onBoard(t, e, 1, "Name:One\nTypes:Artifact\nOracle:x\n")
	p2 := onBoard(t, e, 1, "Name:Two\nTypes:Artifact\nOracle:x\n")
	p3 := onBoard(t, e, 1, "Name:Three\nTypes:Artifact\nOracle:x\n")
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{a}})
	e.putTriggersOnStack()
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("annihilator did not ask its defending player to choose sacrifices: %+v", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{1, 2}}); err != nil {
		t.Fatal(err)
	}
	if e.G.Obj(p1).Zone != state.ZBattlefield || e.G.Obj(p2).Zone != state.ZGraveyard || e.G.Obj(p3).Zone != state.ZGraveyard {
		t.Fatalf("Artisan of Kozilek did not sacrifice the chosen two permanents: %s %s %s", e.G.Obj(p1).Zone, e.G.Obj(p2).Zone, e.G.Obj(p3).Zone)
	}
}

func TestDoubleStrikeFearAndShadowUseCorpusCombatKeywords(t *testing.T) {
	e := combatEngine(t)
	fury := onBoardCard(t, e, 0, corpusKeywordCard(t, "Fury"))
	fearBlocker := onBoard(t, e, 1, "Name:White\nManaCost:W\nTypes:Creature\nPT:1/1\nOracle:x\n")
	if e.actsThisDamageStep(fury, true) != true || e.actsThisDamageStep(fury, false) != true {
		t.Fatal("Fury's double strike must deal damage in both steps")
	}
	shriek := onBoardCard(t, e, 0, corpusKeywordCard(t, "Shriekmaw"))
	e.G.Obj(shriek).IsAttacking = true
	e.G.Obj(shriek).Attacking = 1
	if e.canBlock(fearBlocker, shriek) {
		t.Fatal("nonblack nonartifact creature blocked Shriekmaw with fear")
	}
	voidwalker := onBoardCard(t, e, 0, corpusKeywordCard(t, "Dauthi Voidwalker"))
	e.G.Obj(voidwalker).IsAttacking, e.G.Obj(voidwalker).Attacking = true, 1
	if e.canBlock(fearBlocker, voidwalker) {
		t.Fatal("non-Shadow creature blocked Dauthi Voidwalker")
	}
}

func TestProtectionUsesAllLiveColourQualities(t *testing.T) {
	e := combatEngine(t)
	emrakul := onBoardCard(t, e, 0, corpusKeywordCard(t, "Emrakul, the World Anew"))
	spell := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.PutOnStack, Obj: spell, Player: 1, From: state.ZLibrary, To: state.ZStack})
	if !e.protectedFrom(emrakul, spell) {
		t.Fatal("Emrakul's protection from spells did not recognize a spell")
	}
	scion := onBoardCard(t, e, 0, corpusKeywordCard(t, "Yavimaya Scion"))
	artifact := onBoard(t, e, 1, "Name:Artifact source\nTypes:Artifact\nOracle:x\n")
	if !e.protectedFrom(scion, artifact) {
		t.Fatal("parameterized Protection:Artifact did not use the shared object-spec grammar")
	}
	guardian := onBoardCard(t, e, 0, corpusKeywordCard(t, "Guardian of the Guildpact"))
	mono := onBoard(t, e, 1, "Name:Mono\nManaCost:G\nTypes:Creature\nPT:1/1\nOracle:x\n")
	if !e.protectedFrom(guardian, mono) {
		t.Fatal("Guardian's Protection:Card.MonoColor did not match a monocolored source")
	}
	frenemy := onBoardCard(t, e, 0, corpusKeywordCard(t, "Frenemy of the Guildpact"))
	enemy := onBoard(t, e, 1, "Name:Enemy pair\nManaCost:U G\nTypes:Creature\nPT:1/1\nOracle:x\n")
	ally := onBoard(t, e, 1, "Name:Ally pair\nManaCost:W U\nTypes:Creature\nPT:1/1\nOracle:x\n")
	if !e.protectedFrom(frenemy, enemy) || e.protectedFrom(frenemy, ally) {
		t.Fatal("Frenemy's Protection:Card.EnemyColor did not distinguish enemy and allied pairs")
	}
}

func TestGoadKarazikarEnforcesEveryGoaderAtDeclaration(t *testing.T) {
	e := New(seatZeroStart(Config{Seed: 1, Names: []string{"a", "b", "c", "d"}, Decks: [][]*cards.Card{
		mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40),
	}}))
	e.G.Active = 2
	e.G.Step = state.StepDeclareAttackers
	kar := corpusKeywordCard(t, "Karazikar, the Eye Tyrant")
	goad := cards.ResolveSVar(kar.Faces[0].SVars, "DBGoad")
	if goad == nil || goad.API != "Goad" {
		t.Fatal("Karazikar's real Goad subability did not compile")
	}
	victim := onBoardReady(t, e, 2, "Name:Victim\nTypes:Creature\nPT:1/1\nOracle:x\n")
	for _, goader := range []state.PlayerID{0, 1} {
		effects.Resolve(e, &effects.Ctx{Controller: goader, Targets: []state.Target{{Obj: victim}}}, goad)
	}
	if got := e.G.Obj(victim).Goaders; len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("goad relationships = %v, want [0 1]", got)
	}
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers || len(d.Options) != 1 || d.Options[0].Obj != victim || d.Options[0].Player != 3 {
		t.Fatalf("two-goader attack options = %+v, want only victim attacking player 3", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 2}); err == nil {
		t.Fatal("goaded creature was allowed to skip its required attack")
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 2, Choices: []int{d.Options[0].Index}}); err != nil {
		t.Fatalf("legal non-goader attack rejected: %v", err)
	}
	if !e.G.Obj(victim).IsAttacking || e.G.Obj(victim).Attacking != 3 {
		t.Fatal("goaded creature did not attack the only non-goader")
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 1})
	if got := e.G.Obj(victim).Goaders; len(got) != 1 || got[0] != 1 {
		t.Fatalf("first goader expiry = %v, want [1]", got)
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 1, Amount: 2})
	if got := e.G.Obj(victim).Goaders; len(got) != 0 {
		t.Fatalf("second goader expiry = %v, want none", got)
	}
}
