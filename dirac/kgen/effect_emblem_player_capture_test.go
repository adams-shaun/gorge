package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestGreatTrainHeistPlayerCaptureEmblem: Great Train Heist's third Spree
// mode (an Effect with RememberObjects$ TargetedPlayer over a DamageDone
// trigger whose ValidTarget$ is Player.IsRemembered) lowers to a real
// player-capture emblem rather than a no-op; the first mode's AddPhase still
// lowers; and the shapes the kernel's player capture cannot carry refuse by
// name.
func TestGreatTrainHeistPlayerCaptureEmblem(t *testing.T) {
	t.Run("lowers to a player-capture emblem", func(t *testing.T) {
		checkForms(t, []formCase{
			{card: "Great Train Heist", form: "RememberObjects$ TargetedPlayer over ValidTarget$ Player.IsRemembered", lir: "remembers-player=Target0"},
			{card: "Great Train Heist", form: "mode 1's AddPhase gate still lowers", lir: "StepIn"},
		})
	})

	t.Run("builds CapturePlayer, not Capture", func(t *testing.T) {
		kp := kfhBuild(t, []string{"Great Train Heist"}, nil)
		kp.accepted(t, "Great Train Heist")
		got := kp.pt.emblems // the pool holds this one card
		if len(got) != 1 {
			t.Fatalf("built %d emblems, want 1", len(got))
		}
		e := got[0]
		if e.CapturePlayer != kop.RefTarget0 || e.Capture != kop.RefNone || !e.UntilEOT {
			t.Fatalf("emblem CapturePlayer=%v Capture=%v UntilEOT=%v, want Target0 / none / EOT", e.CapturePlayer, e.Capture, e.UntilEOT)
		}
		if len(e.Abilities) != 1 || e.Abilities[0].Trig.Event != kop.TrigDamageDone || e.Abilities[0].Trig.Flags&kop.TfToPlayer == 0 {
			t.Fatalf("emblem abilities %+v, want one DamageDone trigger to a player", e.Abilities)
		}
	})

	for _, tc := range []struct {
		name string
		mod  func(f *cards.Face)
		want string
	}{
		{"ValidTarget$ Player is not the capture", func(f *cards.Face) {
			f.SVars["TrigDamage"] = strings.Replace(f.SVars["TrigDamage"], "Player.IsRemembered", "Player", 1)
		}, "Effect RememberObjects$ TargetedPlayer with ValidTgts$ \"Opponent\" and ValidTarget$ \"Player\""},
		{"StaticAbilities$ beside the player capture", func(f *cards.Face) {
			f.SVars["DBCombatTrig"] += " | StaticAbilities$ STFoo"
			f.SVars["STFoo"] = "Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ Trample"
		}, "Effect StaticAbilities$ \"STFoo\" beside Triggers$"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kp := kfhBuild(t, []string{"Great Train Heist"}, func(p *pool) {
				c := kfhClone(p.byName["Great Train Heist"])
				tc.mod(c.Faces[0])
				p.byName["Great Train Heist"] = c
			})
			c := kp.cards["Great Train Heist"]
			if len(c.Refusals) == 0 {
				t.Fatal("not refused")
			}
			kp.refusedWith(t, "Great Train Heist", tc.want)
		})
	}
}
