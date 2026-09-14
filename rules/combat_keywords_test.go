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
		"Emrakul, the World Anew": "e/emrakul_the_world_anew.txt", "Yavimaya Scion": "y/yavimaya_scion.txt", "Karazikar, the Eye Tyrant": "k/karazikar_the_eye_tyrant.txt",
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

func TestProtectionAndGoadUseCorpusScripts(t *testing.T) {
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
	kar := corpusKeywordCard(t, "Karazikar, the Eye Tyrant")
	goad := cards.ResolveSVar(kar.Faces[0].SVars, "DBGoad")
	if goad == nil || goad.API != "Goad" {
		t.Fatal("Karazikar's real Goad subability did not compile")
	}
	victim := onBoard(t, e, 1, "Name:Victim\nTypes:Creature\nPT:1/1\nOracle:x\n")
	effects.Resolve(e, &effects.Ctx{Controller: 0, Targets: []state.Target{{Obj: victim}}}, goad)
	if !e.G.Obj(victim).Goaded || e.G.Obj(victim).Goader != 0 {
		t.Fatal("Karazikar's Goad did not record its attack requirement")
	}
}
