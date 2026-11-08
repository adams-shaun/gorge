package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestBattleGraveSweepLowers pins the Origin$ Battlefield,Graveyard
// ChangeZoneAll arm: Strategic Betrayal lowers and builds one
// OpChangeZoneAllEx sweep with the MvGraveExile flag and the CzBattleGrave B
// pair, and every shape the kernel does not carry refuses by name.
func TestBattleGraveSweepLowers(t *testing.T) {
	kp := kfhBuild(t, []string{"Strategic Betrayal"}, nil)
	kp.accepted(t, "Strategic Betrayal")
	var got []kop.Instr
	for _, a := range kp.built["Strategic Betrayal"].def.Abilities {
		for _, in := range a.Prog {
			if in.Op == kop.OpChangeZoneAllEx && in.C>>3&kop.MvGraveExile != 0 {
				got = append(got, in)
			}
		}
	}
	if len(got) != 1 {
		t.Fatalf("Strategic Betrayal built %d MvGraveExile ChangeZoneAllEx instructions, want 1", len(got))
	}
	in := got[0]
	if in.B.N != kop.CzBattleGrave {
		t.Errorf("B = %d, want kop.CzBattleGrave (%d)", in.B.N, kop.CzBattleGrave)
	}
	if kop.Zone(in.C&7) != kop.ZoneExile {
		t.Errorf("destination zone %d, want Exile", in.C&7)
	}

	// one refusal per shape the arm must not guess at
	for _, tc := range []struct {
		name string
		svar string
		want string
	}{
		{"destination", "DB$ ChangeZoneAll | Origin$ Battlefield,Graveyard | Destination$ Hand | DefinedPlayer$ Targeted | ChangeType$ Card.ChosenCard,Card.TargetedPlayerOwn+inZoneGraveyard",
			"ChangeZoneAll Origin$ Battlefield,Graveyard to Hand"},
		{"defined player", "DB$ ChangeZoneAll | Origin$ Battlefield,Graveyard | Destination$ Exile | DefinedPlayer$ You | ChangeType$ Card.ChosenCard,Card.TargetedPlayerOwn+inZoneGraveyard",
			"ChangeZoneAll Origin$ Battlefield,Graveyard DefinedPlayer$ You"},
		{"unpinned role", "DB$ ChangeZoneAll | Origin$ Battlefield,Graveyard | Destination$ Exile | DefinedPlayer$ Targeted | ChangeType$ Card.ChosenCard,Card.TargetedPlayerOwn",
			"ChangeZoneAll Origin$ Battlefield,Graveyard role clause without inZoneGraveyard"},
		{"permanent base", "DB$ ChangeZoneAll | Origin$ Battlefield,Graveyard | Destination$ Exile | DefinedPlayer$ Targeted | ChangeType$ Permanent",
			"ChangeZoneAll Origin$ Battlefield,Graveyard over Permanent"},
		{"remember", "DB$ ChangeZoneAll | Origin$ Battlefield,Graveyard | Destination$ Exile | DefinedPlayer$ Targeted | ChangeType$ Card.ChosenCard,Card.TargetedPlayerOwn+inZoneGraveyard | RememberChanged$ True",
			"ChangeZoneAll Origin$ Battlefield,Graveyard with RememberChanged$"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kp := kfhBuild(t, []string{"Strategic Betrayal"}, func(p *pool) {
				c := kfhClone(p.byName["Strategic Betrayal"])
				c.Faces[0].SVars["DBChangeZoneAll"] = tc.svar
				c.Faces[0].Abilities[0].Sub = cards.ResolveSVar(c.Faces[0].SVars, "DBChangeZoneAll")
				p.byName["Strategic Betrayal"] = c
			})
			kp.refusedWith(t, "Strategic Betrayal", tc.want)
		})
	}
}
