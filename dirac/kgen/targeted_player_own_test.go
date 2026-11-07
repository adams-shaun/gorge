package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TargetedPlayerOwn is the one-token spelling of OwnedBy Targeted: the
// candidate's owner is a player target of the resolving entry (gorge
// matchTargetedPlayerOwn through controlReferentPlayers, which reads
// "Targeted" and "TargetedPlayer" as one code, so the spellings resolve to
// the same player set). Off the battlefield the kernel reads the owner
// through the same F2TgtPlayerCtrl bit a controller role sets (ownerRoleF2);
// on the battlefield or stack the role refuses.

func TestTargetedPlayerOwnLowers(t *testing.T) {
	// the word lowers where the owner role already has a home: a count over
	// graveyard cards (the twin OwnedBy Targeted's shape)
	kp := kfhBuild(t, []string{"Villainous Wrath"}, func(p *pool) {
		c := kfhClone(p.byName["Villainous Wrath"])
		c.Faces[0].SVars["X"] = "Count$ValidGraveyard Card.TargetedPlayerOwn"
		p.byName["Villainous Wrath"] = c
	})
	kp.accepted(t, "Villainous Wrath")
	fs := kp.filtersWith(kop.F2TgtPlayerCtrl)
	if len(fs) != 1 || fs[0].Ctrl != kop.CtrlAny || fs[0].Owner != kop.CtrlAny || fs[0].Zone != kop.ZoneGraveyard {
		t.Fatalf("TargetedPlayerOwn filters %+v, want one graveyard filter reading the owner through F2TgtPlayerCtrl", fs)
	}

	// the word's corpus carriers parse it. Each card still refuses on a
	// later shape of its own instruction, never on the word; the refusals
	// below are those later blockers, pinned so a fix to either is made
	// against this test:
	//   - Binding Negotiation's DB$ ChangeZone Exile->Graveyard pick
	//   - Strategic Betrayal's ChangeZoneAll Origin$ Battlefield,Graveyard
	for _, tc := range []struct {
		name, blocker string
	}{
		{"Binding Negotiation", "DB$ ChangeZone: ChangeType$ move Exile->Graveyard"},
		{"Strategic Betrayal", `DB$ ChangeZoneAll: Origin "Battlefield,Graveyard"`},
	} {
		kp := kfhBuild(t, []string{tc.name}, nil)
		c := kp.cards[tc.name]
		var got []string
		for _, r := range c.Refusals {
			got = append(got, r.Reason)
			if strings.Contains(r.Reason, `unknown property "TargetedPlayerOwn"`) {
				t.Fatalf("%s: the word still refuses: %s", tc.name, r.Reason)
			}
		}
		if all := strings.Join(got, " | "); !strings.Contains(all, tc.blocker) {
			t.Fatalf("%s refusals %q, want the later blocker %q", tc.name, all, tc.blocker)
		}
	}
}
