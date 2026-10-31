package rules

// engine_gaps_test.go pins three engine gaps, each through the real corpus
// card that names it.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func runLB5Oracle(t *testing.T, raw string) OracleResult {
	t.Helper()
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	return res
}

// CR 708.2a: a face-down permanent has no abilities of its own, but CR 613.1f
// still applies a layer-6 grant to it. Etrata, Deadly Fugitive's "Face-down
// creatures you control have '{2}{U}{B}: Turn this creature face up...'" was
// never offered on the face-down creature (the granted-ability walk skipped
// every face-down permanent), so the card did nothing. The offer must exist,
// name no printed face (CR 708.8), and resolve turning the creature face up;
// its stack entry names no source card while the source is face down.
const etrataFaceDownGrant = `{"name":"etrata-face-down-grant","cr":["708.2a","613.1f"],"why":"inline","setup":{"p0":{"battlefield":["Etrata, Deadly Fugitive"],"hand":["Bolrac-Clan Basher"]}},"steps":[` +
	`{"op":"cast","seat":0,"card":"p0:Bolrac-Clan Basher","mana":"CCC","cast_mode":"disguised"},{"op":"resolve"},` +
	`{"op":"activate","seat":0,"card":"p0:Bolrac-Clan Basher","mana":"CCUB","ability":"Face-down creature: Turn this creature face up"},` +
	`{"op":"resolve"}]}`

func TestFaceDownCreatureIsOfferedItsGrantedAbility(t *testing.T) {
	res := runLB5Oracle(t, etrataFaceDownGrant)
	// After the activation (checkpoint 3: setup, cast, resolve, activate) the
	// ability is on the stack and its source, still face down, is unnamed.
	act := res.Snapshots[3]
	if len(act.Stack) != 1 || act.Stack[0].Kind != "ability" || act.Stack[0].Source != "p0:" {
		t.Fatalf("activation stack = %+v, want one ability whose face-down source names no card", act.Stack)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	p, ok := snapPerm(last, "p0:Bolrac-Clan Basher")
	if !ok || p.FaceDown || p.Name != "Bolrac-Clan Basher" {
		t.Fatalf("Basher after the granted ability = %+v (found %v), want it face up", p, ok)
	}
	// Control: without Etrata the face-down creature has no ability at all.
	control := strings.Replace(etrataFaceDownGrant, `"battlefield":["Etrata, Deadly Fugitive"],`, "", 1)
	cres, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(control))
	if err != nil {
		t.Fatal(err)
	}
	if len(cres.Fails) == 0 {
		t.Fatal("control: the face-down creature was offered the ability with no Etrata granting it")
	}
}

// CR 603.12 / 608.2c: Aang, Master of Elements' "At the beginning of each
// upkeep, you may transform Aang. If you do, you gain 4 life, draw four cards,
// put four +1/+1 counters on him, and he deals 4 damage to each opponent."
// The transform remembers the card (RememberChanged$) and each rider is gated
// on that memory. A phase trigger's capture is its own source (a step change
// names no object), and that capture was kept as the resolution's Captured
// while its Remembered started from the source's durable list -- so the
// RememberChanged entry cancelled against the capture and the transform did
// nothing else. The riders must all happen.
const aangUpkeepTransform = `{"name":"aang-upkeep-transform","cr":["603.12","608.2c"],"why":"inline","setup":{"p0":{"battlefield":["Avatar Aang"],"back_face":["Avatar Aang"]},"p1":{"battlefield":["Grizzly Bears"]}},` +
	`"setup_answers":[{"kind":"trigger_optional","pick":["no"]}],"steps":[` +
	`{"op":"pass_to","seat":0,"step":"upkeep","active":"p0","answers":[{"kind":"trigger_optional","pick":["no"]}]},` +
	`{"op":"resolve","answers":[{"kind":"trigger_optional","pick":["yes"]}]}]}`

func TestPhaseTriggerIfYouDoReadsItsOwnTransform(t *testing.T) {
	res := runLB5Oracle(t, aangUpkeepTransform)
	last := res.Snapshots[len(res.Snapshots)-1]
	if p, ok := snapPerm(last, "p0:Avatar Aang"); !ok || p.Name != "Avatar Aang" || p.Counters["P1P1"] != 4 {
		t.Fatalf("Aang after the trigger = %+v (found %v), want front face with four +1/+1 counters", p, ok)
	}
	for _, pl := range last.Players {
		switch pl.Seat {
		case 0:
			if pl.Life != 24 || len(pl.Hand) != 4 {
				t.Fatalf("p0 life %d hand %d, want 24 and 4", pl.Life, len(pl.Hand))
			}
		case 1:
			if pl.Life != 16 {
				t.Fatalf("p1 life %d, want 16", pl.Life)
			}
		}
	}
}

// CR 101.2 / 508.1: Sandswirl Wanderglyph's "Each opponent who attacked you or
// a planeswalker you control this turn can't cast spells." The Caster$ spec's
// attackedYouTheirCurrentTurn qualifier was unknown to the player filter, so
// it matched nobody and the restriction never applied. After p1 attacks p0,
// p1 may not cast; the control turn without the attack offers the spell.
const wanderglyphAttackedYou = `{"name":"wanderglyph-attacked-you","turn":2,"cr":["101.2","508.1"],"why":"inline","setup":{"p0":{"battlefield":["Unstable Glyphbridge"],"back_face":["Unstable Glyphbridge"]},"p1":{"battlefield":["Grizzly Bears"],"hand":["Gut Shot"]}},"steps":[` +
	`{"op":"attack","seat":1,"defender":"p0","attackers":["p1:Grizzly Bears"]},` +
	`{"op":"pass_to","seat":1,"step":"main2","expect":[{"offered":{"seat":1,"kind":"cast","card":"p1:Gut Shot"},"want":false}]}]}`

const wanderglyphNoAttack = `{"name":"wanderglyph-no-attack","turn":2,"cr":["101.2","508.1"],"why":"inline","setup":{"p0":{"battlefield":["Unstable Glyphbridge"],"back_face":["Unstable Glyphbridge"]},"p1":{"battlefield":["Grizzly Bears"],"hand":["Gut Shot"]}},"steps":[` +
	`{"op":"pass_to","seat":1,"step":"main2","expect":[{"offered":{"seat":1,"kind":"cast","card":"p1:Gut Shot"},"want":true}]}]}`

func TestAttackedYouThisTurnOpponentCantCast(t *testing.T) {
	runLB5Oracle(t, wanderglyphAttackedYou)
	runLB5Oracle(t, wanderglyphNoAttack)
}
