package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kopusScript is a synthetic battlefield SpellCast listener whose pump reads
// SVar Spent; trig overrides the trigger line, x the SVar Spent.
func kopusLower(t *testing.T, trig, x, extra string) []refusal {
	t.Helper()
	sub := ""
	if extra != "" {
		sub = " | SubAbility$ DBY"
	}
	src := "Name:Opus Probe\nManaCost:1 R\nTypes:Creature Human\nPT:1/1\n" + trig + "\n" +
		"SVar:ProbePump:DB$ Pump | Defined$ Self | NumAtt$ +Spent" + sub + "\n" + x + "\n" + extra
	card, diags := cards.ParseBytes("opus_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Opus Probe"}, byName: map[string]*cards.Card{"Opus Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	return lw.cards[0].Refusals
}

const (
	kopusTrig = "T:Mode$ SpellCast | ValidCard$ Instant,Sorcery | ValidActivatingPlayer$ You | TriggerZones$ Battlefield | Execute$ ProbePump"
	kopusX    = "SVar:Spent:TriggeredCard$CastTotalManaSpent"
)

// TestOpusRefusals: TriggeredCard$CastTotalManaSpent is lowered only in a
// battlefield SpellCast trigger that is not the card's own cast, with the
// bare total; every other shape is refused with a named reason.
func TestOpusRefusals(t *testing.T) {
	if r := kopusLower(t, kopusTrig, kopusX, ""); len(r) != 0 {
		t.Fatalf("the plain Opus shape is refused: %v", r)
	}
	cases := []struct{ name, trig, x, extra, want string }{
		{"outside SpellCast", "T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ ProbePump", kopusX, "", "outside a SpellCast trigger"},
		{"off the battlefield", strings.Replace(kopusTrig, "TriggerZones$ Battlefield", "TriggerZones$ Graveyard", 1), kopusX, "", "off the battlefield"},
		{"self cast", strings.Replace(kopusTrig, "ValidCard$ Instant,Sorcery", "ValidCard$ Card.Self", 1), kopusX, "", "self cast trigger"},
		{"or copy", strings.Replace(kopusTrig, "Mode$ SpellCast", "Mode$ SpellCastOrCopy", 1), kopusX, "", "beside OrCopy"},
		{"cast-count snapshot", kopusTrig, kopusX, "SVar:DBY:DB$ Pump | Defined$ Self | NumDef$ +Y\nSVar:Y:TriggerObjectsCurrentCastSpells$Valid Sorcery.YouCtrl,Instant.YouCtrl", "beside a cast-count snapshot"},
		{"producer filter", kopusTrig, "SVar:Spent:TriggeredCard$CastTotalManaSpent Snow", "", "with a producer filter"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rs := kopusLower(t, c.trig, c.x, c.extra)
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
