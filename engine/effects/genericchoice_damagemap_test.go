package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

func genericChoiceDamageFixture(t *testing.T) (*batchSpyHost, *Ctx) {
	t.Helper()
	h := &batchSpyHost{fakeHost: newHost(t, 3)}
	src := battlefield(t, h.fakeHost, damageCarrier)
	for i := range h.g.Players {
		h.g.Players[i].Life = 20
	}
	c := &Ctx{Source: src, Controller: 0, SVars: map[string]string{
		"A": "DB$ DealDamage | NumDmg$ 3 | Defined$ Remembered",
	}}
	return h, c
}

func TestGenericChoiceDamageMapDealsOneBatchAfterEveryChooser(t *testing.T) {
	h, c := genericChoiceDamageFixture(t)
	Resolve(h, c, sa(t, "DB$ GenericChoice | Choices$ A | TempRemember$ Chooser | Defined$ Opponent | DamageMap$ True"))
	if h.g.Players[1].Life != 17 || h.g.Players[2].Life != 17 {
		t.Fatalf("precondition: opponents at %d/%d, want 17/17", h.g.Players[1].Life, h.g.Players[2].Life)
	}
	if h.opens != 1 || len(h.deals) != 1 || h.deals[0] != 6 {
		t.Fatalf("batches opens=%d deals=%v, want one batch [6]", h.opens, h.deals)
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "DamageMap$") {
			t.Fatalf("unread DamageMap$ Note: %q", ev.Text)
		}
	}
	if c.DamageMapOpen || len(c.PendingDamage) != 0 {
		t.Fatalf("map left open: open=%v pending=%d", c.DamageMapOpen, len(c.PendingDamage))
	}
}

func TestGenericChoiceWithoutDamageMapKeepsPerChooserBatches(t *testing.T) {
	h, c := genericChoiceDamageFixture(t)
	Resolve(h, c, sa(t, "DB$ GenericChoice | Choices$ A | TempRemember$ Chooser | Defined$ Opponent"))
	if h.g.Players[1].Life != 17 || h.g.Players[2].Life != 17 {
		t.Fatalf("precondition: opponents at %d/%d, want 17/17", h.g.Players[1].Life, h.g.Players[2].Life)
	}
	if len(h.deals) != 2 || h.deals[0] != 3 || h.deals[1] != 3 {
		t.Fatalf("deals=%v, want [3 3]", h.deals)
	}
}
