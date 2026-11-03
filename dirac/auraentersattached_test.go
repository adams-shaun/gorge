package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAuraEntersAttachedBeforeItsEntersCheck: an Aura that enters attached
// (CR 303.4f) is attached when the "whenever an enchantment enters" watchers
// look (CR 603.6a), as gorge folds the move and the attach before its trigger
// scan. Witness Protection returned from the graveyard onto Savannah Lions
// strips the Lions of the watcher line they carry, so only the Ambush Wolf's
// copy of it fires (Duskmourn's Domination put onto the battlefield under
// Erratic Apparition's Stalked Researcher, which it silences, left only the
// Apparition's Eerie trigger).
func TestAuraEntersAttachedBeforeItsEntersCheck(t *testing.T) {
	l := newL7(t)
	f := l.filter(kop.Filter{Types: kop.TEnchantment})
	watch := kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigETB, Filter: f},
		Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}
	for _, name := range []string{"Savannah Lions", "Ambush Wolf"} {
		cd := &l.tbl.Cards[l.id[name]]
		cd.Abilities = append(append([]kop.Ability(nil), cd.Abilities...), watch)
	}
	l.auraOverride("Witness Protection", kop.Override{LoseAbilities: true})
	l.start(t)
	lions := l.perm("Savannah Lions")
	l.perm("Ambush Wolf")
	aura := l.sc.obj(l.id["Witness Protection"], 0)
	l.sc.s.place(aura, kop.ZoneGraveyard, false)
	l.sc.s.npend = 0
	l.sc.s.returnAura(l.sc.k, aura, l.sc.slot(lions))
	if got := l.sc.s.npend; got != 1 {
		t.Fatalf("%d enters checks queued, want 1 (the Ambush Wolf's; the Lions lost its abilities to the Aura as it entered)", got)
	}
	if got := l.sc.s.perms[l.sc.slot(aura)].attached; got != l.sc.slot(lions) {
		t.Fatalf("the Aura is attached to slot %d, want the Lions' %d", got, l.sc.slot(lions))
	}
}
