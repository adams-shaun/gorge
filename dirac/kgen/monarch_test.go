package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// kmonarchETB is a synthetic ETB that makes its controller the monarch.
const kmonarchETB = "T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ ProbeMonarch\nSVar:ProbeMonarch:DB$ BecomeMonarch"

// kmonarchUpkeep is a synthetic upkeep trigger whose draw count reads the
// monarch designation.
const kmonarchUpkeep = "T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ X\nSVar:X:Count$Monarch.2.1"

// kmonarchBuilt lowers and builds body (kcrimeLower's synthetic card),
// failing on a refusal, and reports what the built card carries: an
// ExtBecomeMonarch instruction, an ExMonarch expression (with its N), and a
// TrigBecomeMonarch trigger (with its N).
func kmonarchBuilt(t *testing.T, body string) (op bool, ex, trig int16) {
	t.Helper()
	src := "Name:Monarch Probe\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n" + body + "\n"
	card, diags := cards.ParseBytes("monarch_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Monarch Probe"}, byName: map[string]*cards.Card{"Monarch Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	if r := lw.cards[0].Refusals; len(r) != 0 {
		t.Fatalf("refused: %v\n%s", r, body)
	}
	ex, trig = -1, -1
	for _, a := range bs[0].def.Abilities {
		for i := range a.Prog {
			op = op || a.Prog[i].Ext() == kop.ExtBecomeMonarch
		}
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigBecomeMonarch {
			trig = a.Trig.N
		}
	}
	for _, x := range pt.exprs {
		if x.Kind == kop.ExMonarch {
			ex = x.N
		}
	}
	return op, ex, trig
}

// TestMonarchLowering: api BecomeMonarch, Mode$ BecomeMonarch,
// Count$Monarch and CheckDefinedPlayer$ You|Opponent.isMonarch lower in the
// shapes gorge reads; every other spelling is refused with a named reason.
func TestMonarchLowering(t *testing.T) {
	for _, ok := range []struct {
		body      string
		op        bool
		ex, trig  int16
		checkDesc string
	}{
		{kmonarchETB, true, -1, -1, "an ETB"},
		{strings.Replace(kmonarchETB, "DB$ BecomeMonarch", "DB$ BecomeMonarch | Defined$ You", 1), true, -1, -1, "Defined$ You"},
		{strings.Replace(kmonarchETB, "DB$ BecomeMonarch", "DB$ BecomeMonarch | ValidTgts$ Player", 1), true, -1, -1, "a player target"},
		{strings.Replace(kmonarchETB, "DB$ BecomeMonarch", "DB$ BecomeMonarch | ValidTgts$ Opponent", 1), true, -1, -1, "an opponent target"},
		{"A:AB$ BecomeMonarch | Cost$ 4 T Sac<1/CARDNAME> | Defined$ You", true, -1, -1, "an activated ability"},
		{kmonarchUpkeep, false, 0, -1, "Count$Monarch"},
		{"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | CheckDefinedPlayer$ Opponent.isMonarch | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ 1", false, kop.MonarchOpp, -1, "CheckDefinedPlayer$ Opponent.isMonarch"},
		{"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | CheckDefinedPlayer$ You.isMonarch | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ 1", false, 0, -1, "CheckDefinedPlayer$ You.isMonarch"},
		{"T:Mode$ BecomeMonarch | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ 1", false, -1, 0, "Mode$ BecomeMonarch"},
		{"T:Mode$ BecomeMonarch | ValidPlayer$ Opponent | BeginTurn$ You | TriggerZones$ Battlefield | Execute$ ProbeDrain\nSVar:ProbeDrain:DB$ LoseLife | Defined$ TriggeredPlayer | LifeAmount$ 2", false, -1, kop.TrigNMonarchBeganTurn, "BeginTurn$ You"},
	} {
		op, ex, trig := kmonarchBuilt(t, ok.body)
		if op != ok.op || ex != ok.ex || trig != ok.trig {
			t.Errorf("%s: built op %v ex %d trig %d, want %v %d %d", ok.checkDesc, op, ex, trig, ok.op, ok.ex, ok.trig)
		}
	}
	cases := []struct{ name, body, want string }{
		{"Defined Remembered", strings.Replace(kmonarchETB, "DB$ BecomeMonarch", "DB$ BecomeMonarch | Defined$ Remembered", 1), "BecomeMonarch Defined$"},
		{"creature target", strings.Replace(kmonarchETB, "DB$ BecomeMonarch", "DB$ BecomeMonarch | ValidTgts$ Creature", 1), "BecomeMonarch of a non-player target"},
		{"arms one", strings.Replace(kmonarchUpkeep, "Count$Monarch.2.1", "Count$Monarch.2", 1), "Count$Monarch arms"},
		{"arms svar", strings.Replace(kmonarchUpkeep, "Count$Monarch.2.1", "Count$Monarch.2.Z", 1) + "\nSVar:Z:Count$Morbid.1.0", "Count$Monarch arms"},
		{"CheckDefinedPlayer any player", strings.Replace(kmonarchUpkeep, "Execute$ ProbeDraw", "Execute$ ProbeDraw | CheckDefinedPlayer$ Player.isMonarch", 1), `CheckDefinedPlayer "Player.isMonarch"`},
		{"trigger ValidPlayer", "T:Mode$ BecomeMonarch | ValidPlayer$ Player.OpponentOf Remembered | TriggerZones$ Battlefield | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ 1", "BecomeMonarch ValidPlayer"},
		{"trigger BeginTurn", "T:Mode$ BecomeMonarch | ValidPlayer$ Opponent | BeginTurn$ Opponent | TriggerZones$ Battlefield | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ 1", "BecomeMonarch BeginTurn$"},
		{"trigger zone", "T:Mode$ BecomeMonarch | ValidPlayer$ You | TriggerZones$ Command | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ 1", "BecomeMonarch trigger outside the battlefield"},
		// Palace Jailer's shape (a paraphrase, not the script): an exile whose
		// Effect returns the card when an opponent becomes the monarch
		{"exile until an opponent becomes the monarch", "T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ ProbeExile\n" +
			"SVar:ProbeExile:DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | ValidTgts$ Creature.OppCtrl | SubAbility$ ProbeEffect\n" +
			"SVar:ProbeEffect:DB$ Effect | Triggers$ ProbeBack | RememberObjects$ You & Targeted | Duration$ Permanent | ForgetOnMoved$ Exile\n" +
			"SVar:ProbeBack:Mode$ BecomeMonarch | ValidPlayer$ Player.OpponentOf Remembered | Execute$ ProbeReturn | TriggerZones$ Command | Static$ True\n" +
			"SVar:ProbeReturn:DB$ ChangeZoneAll | Origin$ Exile | Destination$ Battlefield | ChangeType$ Card.IsRemembered", "an exile until an opponent becomes the monarch (Palace Jailer) is not modelled"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rs := kcrimeLower(t, c.body)
			if len(rs) == 0 {
				t.Fatal("accepted, want a refusal")
			}
			for _, r := range rs {
				if strings.Contains(r.Reason, c.want) {
					return
				}
			}
			t.Fatalf("refusals %v lack %q", rs, c.want)
		})
	}
}
