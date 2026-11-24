package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// warRoomScript is the carrier's script, read from the corpus.
func warRoomScript(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(tableCorpus(t), "cardsfolder", "w", "war_room.txt"))
	if err != nil {
		t.Skip("no gorge corpus: ", err)
	}
	return string(src)
}

// TestLifeIdentCarrier: War Room's draw ability ("{3}, {T}, Pay life equal to
// the number of colors in your commanders' color identity: Draw a card.
// Activate only in a game of Commander") lowers to a LifeIdent cost on a
// Commander-only ability. The life part is neither CostPayLife nor a count
// in Cost.N: its amount belongs to the payer, and with N 0 the flag would
// read as the announced PayLife<X>. The mana ability is untouched.
func TestLifeIdentCarrier(t *testing.T) {
	d, rs := kwardpowBuild(t, "War Room", warRoomScript(t))
	if len(rs) != 0 {
		t.Fatalf("War Room refused: %v", rs)
	}
	var draw, mana *kop.Ability
	for i := range d.Abilities {
		switch a := &d.Abilities[i]; a.Kind {
		case kop.AbActivated:
			draw = a
		case kop.AbMana:
			mana = a
		}
	}
	if draw == nil || mana == nil {
		t.Fatalf("abilities %v: want one activated and one mana ability", d.Abilities)
	}
	if c := draw.Cost; !c.LifeIdent || c.Flags != kop.CostTap || c.N != 0 || c.Mana.Generic != 3 {
		t.Errorf("draw cost %+v: want {3}, {T} and LifeIdent with no CostPayLife and N 0", c)
	}
	if !draw.CmdOnly {
		t.Error("the draw ability is not Commander-only")
	}
	if mana.Cost.LifeIdent || mana.CmdOnly {
		t.Errorf("the mana ability carries the draw's cost or gate: %+v", mana.Cost)
	}
}

// TestLifeIdentRefusals: the count lowers only on a battlefield activated
// ability's own cost, and a format list lowers only when it names Commander.
func TestLifeIdentRefusals(t *testing.T) {
	const head = "Name:Ident Probe\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\n"
	// the count's SVar line is the carrier's own, taken from the corpus: no
	// script line is spelled out here
	x := ""
	for _, line := range strings.Split(warRoomScript(t), "\n") {
		if strings.HasPrefix(line, "SVar:X:") {
			x = strings.TrimSpace(line) + "\n"
		}
	}
	if x == "" {
		t.Fatal("War Room's script has no SVar X")
	}
	for _, p := range []struct{ name, src, want string }{
		{"a mana ability", "A:AB$ Mana | Cost$ T PayLife<X> | Produced$ B\n" + x,
			"life cost PayLife<X> with SVar X"},
		{"a graveyard ability", "A:AB$ Draw | Cost$ 2 PayLife<X> | ActivationZone$ Graveyard | NumCards$ 1\n" + x,
			"commander colour identity life cost outside a battlefield activated ability"},
		{"beside a fixed life part", "A:AB$ Draw | Cost$ 2 PayLife<1> PayLife<X> | NumCards$ 1\n" + x,
			"life cost PayLife<X> with SVar X"},
		{"a format list without Commander", "A:AB$ Draw | Cost$ 2 | ActivationGameTypes$ Brawl,Oathbreaker | NumCards$ 1\n",
			"names no modelled format"},
	} {
		_, rs := kwardpowBuild(t, "Ident Probe", head+p.src)
		if all := kpaylifexReasons(rs); !strings.Contains(all, p.want) {
			t.Errorf("%s: refusals %q, want %q", p.name, all, p.want)
		}
	}
	// the plain shape, without the format gate, lowers: outside a Commander
	// game no seat has a commander and both engines charge 0
	d, rs := kwardpowBuild(t, "Ident Probe", head+"A:AB$ Draw | Cost$ 2 PayLife<X> | NumCards$ 1\n"+x)
	if len(rs) != 0 || len(d.Abilities) != 1 || !d.Abilities[0].Cost.LifeIdent || d.Abilities[0].CmdOnly {
		t.Errorf("the ungated shape: refusals %v, abilities %+v", rs, d.Abilities)
	}
}
