package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestRepeatEachOppNonTriggeredDefenderUsesCapturedDefender(t *testing.T) {
	h := newHost(t, 5)
	h.g.Players[3].Lost = true
	c := &Ctx{Controller: 0, TriggerContext: TriggerContext{
		// The triggering player is seat 4, while the attacked/defending
		// player is seat 2. The selector must exclude seat 2 and retain 4.
		TriggerPlayer:   state.Target{Player: 4, IsPlayer: true},
		DefendingPlayer: state.Target{Player: 2, IsPlayer: true},
	}}
	got, ok := repeatPlayers(h, c, "OppNonTriggeredDefender")
	if !ok {
		t.Fatal("OppNonTriggeredDefender was not recognized")
	}
	want := []state.PlayerID{1, 4}
	if len(got) != len(want) {
		t.Fatalf("selected players = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("selected players = %v, want %v", got, want)
		}
	}
}

func TestRepeatEachOppNonTriggeredDefenderCopiesAndRegistersImprint(t *testing.T) {
	const script = `Name:Shredder Fixture
Types:Creature
PT:5/5
A:SP$ RepeatEach | RepeatPlayers$ OppNonTriggeredDefender | RepeatSubAbility$ DBCopy | ChangeZoneTable$ True | SubAbility$ DelTrig
SVar:DBCopy:DB$ CopyPermanent | Defined$ TriggeredAttackerLKICopy | TokenAttacking$ Remembered | TokenTapped$ True | ImprintTokens$ True | NonLegendary$ True
SVar:DelTrig:DB$ DelayedTrigger | Mode$ Phase | Phase$ End Of Turn | Execute$ TrigExile | RememberObjects$ ImprintedLKI
SVar:TrigExile:DB$ ChangeZone | Defined$ DelayTriggerRememberedLKI | Origin$ Battlefield | Destination$ Exile
Oracle:x
`
	card, diags := cards.ParseBytes("repeat-each-carrier.txt", []byte(script))
	if len(diags) != 0 {
		t.Fatalf("parse fixture: %v", diags)
	}
	card.Link()
	if len(card.Faces) != 1 || len(card.Faces[0].Abilities) != 1 {
		t.Fatalf("fixture does not contain one carrier ability: %+v", card.Faces)
	}

	h := newHost(t, 3)
	h.g.Tokens = tokenFixtures(t)
	source := h.g.AddObject(card, 0)
	attackerCard := mkCard(t, "Name:Attacker\nTypes:Creature\nPT:4/4\nOracle:x\n")
	attacker := h.g.AddObject(attackerCard, 0)
	defenderPermanent := h.g.AddObject(mkCard(t, "Name:Defender\nTypes:Creature\nPT:2/2\nOracle:x\n"), 1)
	otherOpponentPermanent := h.g.AddObject(mkCard(t, "Name:Other\nTypes:Creature\nPT:3/3\nOracle:x\n"), 2)
	for _, o := range []*state.Object{source, attacker, defenderPermanent, otherOpponentPermanent} {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	if h.g.Obj(source.ID).Zone != state.ZBattlefield || h.g.Obj(attacker.ID).Zone != state.ZBattlefield || attacker.ID == source.ID {
		t.Fatalf("precondition: source/attacker must be distinct battlefield objects; source=%+v attacker=%+v", h.g.Obj(source.ID), h.g.Obj(attacker.ID))
	}
	if h.g.Players[1].Lost || h.g.Players[2].Lost || defenderPermanent.ID == otherOpponentPermanent.ID {
		t.Fatal("precondition: two distinct opponent seats and permanents must be alive")
	}
	ctx := &Ctx{Source: source.ID, Controller: 0, TriggerContext: TriggerContext{
		// For an attack trigger, the attacker/controller and the attacked
		// defending player are distinct roles. Seat 2 is the other opponent.
		TriggerPlayer:   state.Target{Player: 0, IsPlayer: true},
		DefendingPlayer: state.Target{Player: 1, IsPlayer: true},
	}, Remembered: []state.Target{{Obj: attacker.ID}}, SVars: card.Faces[0].SVars}
	if ctx.TriggerPlayer.Player == ctx.DefendingPlayer.Player {
		t.Fatal("precondition: triggering player and defending player must differ")
	}
	Resolve(h, ctx, card.Faces[0].Abilities[0])

	if got := h.g.Obj(source.ID).ImprintTokens; len(got) != 1 {
		t.Fatalf("source imprint ids = %v, want exactly one copy for the non-defending opponent", got)
	}
	copyID := h.g.Obj(source.ID).ImprintTokens[0]
	copy := h.g.Obj(copyID)
	if copy == nil || !copy.IsToken || copy.Zone != state.ZBattlefield || copy.Face() == nil || copy.Face().Name != "Attacker" {
		t.Fatalf("imprinted object %d = %+v, want battlefield token copy of attacker", copyID, copy)
	}
	if copy.ID == attacker.ID || defenderPermanent.Zone != state.ZBattlefield || otherOpponentPermanent.Zone != state.ZBattlefield {
		t.Fatalf("precondition/result: copied original or opponent permanents changed; copy=%d attacker=%d defender=%v other=%v", copy.ID, attacker.ID, defenderPermanent.Zone, otherOpponentPermanent.Zone)
	}

	var reg *events.Event
	for i := range h.log {
		if h.log[i].Kind == events.DelayedRegister {
			reg = &h.log[i]
		}
		if h.log[i].Kind == events.Note && h.log[i].Text == "RepeatEach selector unimplemented" {
			t.Fatalf("carrier selector still unimplemented: %+v", h.log[i])
		}
	}
	if reg == nil || len(reg.IDs) != 1 || reg.IDs[0] != copyID {
		t.Fatalf("delayed registration = %+v, want remembered ids [%d]", reg, copyID)
	}
}
