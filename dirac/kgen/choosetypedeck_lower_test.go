package kgen

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestChooseTypeRandomDeckLowering pins the AtRandom$ Type$
// CreatureInTargetedDeck lowering (Aswan Jaguar's enters trigger, "choose a
// random creature type from those in target opponent's deck"). gorge
// effChooseType's AtRandom arm poses no ask: it draws over the sorted creature
// subtypes of the resolution's player target's library and stores the answer
// on the source. The trigger body therefore lowers to its player-target ask
// (ValidTgts$ Opponent) and one OpChooseType marked B = ChooseTypeRandomDeck
// with C naming that slot; the destroy ability's Creature.ChosenType target
// reads the answer. IsCurse$ is a note gorge never reads; AtRandom$ other
// than True and a deck category without AtRandom$ True keep the card refused.
func TestChooseTypeRandomDeckLowering(t *testing.T) {
	kp := kfhBuild(t, []string{"Aswan Jaguar", "Grizzly Bears"}, nil)
	if got := refusalText(kp.cards["Aswan Jaguar"]); got != "" {
		t.Fatalf("Aswan Jaguar: refused: %s", got)
	}
	lir, _ := lowerCarriers(t, []string{"Aswan Jaguar"})
	sec := section(lir, "Aswan Jaguar")
	// the trigger carries the deck's owner as its own target slot (gorge asks
	// a trigger's targets as it resolves: ValidTgts$ Opponent, one player)
	if n := strings.Count(sec, "F={players=2}"); n != 1 {
		t.Errorf("Aswan Jaguar: want one player target slot, got %d:\n%s", n, sec)
	}
	// the random-deck choose: B = ChooseTypeRandomDeck, C names target slot 0
	if n := strings.Count(sec, "ChooseType obj=You B="+strconv.Itoa(int(kop.ChooseTypeRandomDeck))+" C="+strconv.Itoa(int(kop.RefTarget0))); n != 1 {
		t.Errorf("Aswan Jaguar: want one ChooseType B=%d C=%d, got %d:\n%s",
			kop.ChooseTypeRandomDeck, kop.RefTarget0, n, sec)
	}
	// the destroy half keeps its ChosenType target and NoRegen destroy
	if !strings.Contains(sec, "ChosenType") {
		t.Errorf("Aswan Jaguar: the destroy target's filter lost ChosenType:\n%s", sec)
	}
	if !strings.Contains(sec, "Destroy obj=Target0 C=1") {
		t.Errorf("Aswan Jaguar: want the no-regen Destroy of Target0:\n%s", sec)
	}
}
