package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestArcbondEmblemFormLowers: the two sides together (this ticket's
// recipient-captured DamageDoneOnce trigger, ticket A's sweep whose source
// is the emblem's captured object) lower Arcbond's whole Effect to the
// CreateEmblem the kernel fires on the damaged recipient.
func TestArcbondEmblemFormLowers(t *testing.T) {
	checkForms(t, []formCase{{card: "Arcbond", form: "recipient-captured DamageDoneOnce + captured-source sweep", lir: "CreateEmblem"}})
}

// TestEmblemDamageDoneOnceRecipientCapture: an end-of-turn Effect whose
// DamageDoneOnce line captures the damage RECIPIENT (ValidTarget$
// Card.IsRemembered) lowers to a CreateEmblem with the capture and its
// ExileOnMoved$ Battlefield lapse, not refused as a trigger without a
// permanent duration. The synthetic face is The Inner Light's chain with its
// source-side DamageDone line rewritten to the recipient-side DamageDoneOnce
// shape (Arcbond's trigger) and an ExileOnMoved$ Battlefield rider.
func TestEmblemDamageDoneOnceRecipientCapture(t *testing.T) {
	kp := kfhBuild(t, []string{"The Inner Light"}, func(p *pool) {
		c := kfhClone(p.byName["The Inner Light"])
		f := c.Faces[0]
		f.SVars["DBEffect"] += " | ExileOnMoved$ Battlefield"
		f.SVars["DamageTrig"] = "Mode$ DamageDoneOnce | ValidTarget$ Card.IsRemembered | Execute$ TrigDraw | TriggerDescription$ test"
		p.byName["The Inner Light"] = c
	})
	kp.accepted(t, "The Inner Light")
	if len(kp.pt.emblems) != 1 {
		t.Fatalf("built %d emblems, want 1", len(kp.pt.emblems))
	}
	e := kp.pt.emblems[0]
	if e.Capture != kop.RefTarget0 || e.CapturePlayer != kop.RefNone || !e.UntilEOT {
		t.Fatalf("emblem Capture=%v CapturePlayer=%v UntilEOT=%v, want Target0 / none / EOT", e.Capture, e.CapturePlayer, e.UntilEOT)
	}
	if !e.CaptureLapse {
		t.Fatal("emblem CaptureLapse is false; ExileOnMoved$ Battlefield must lapse the capture")
	}
	if len(e.Abilities) != 1 || e.Abilities[0].Trig.Event != kop.TrigDamageDone || e.Abilities[0].Trig.Filter2 == 0 {
		t.Fatalf("emblem abilities %+v, want one DamageDone trigger with the recipient filter", e.Abilities)
	}
}

// TestEmblemDamageDoneRecipientSourceReadRefused: on a plain DamageDone
// recipient-captured emblem line (ValidTarget$ Card.IsRemembered, the
// recipient branch of kgen's emblemCaptureFilter) the kernel emblem hook
// queues the DAMAGED object as the trigger object
// (dirac/trig_effectuntileot.go emblemTriggerObj over embRecipientCapture),
// so a body read of the damage source would name the damaged creature, not
// the source gorge reads. The read refuses instead.
func TestEmblemDamageDoneRecipientSourceReadRefused(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"TriggeredSource", "DB$ Destroy | Defined$ TriggeredSource"},
		{"TriggeredSourceController", "DB$ Draw | Defined$ TriggeredSourceController | NumCards$ 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kp := kfhBuild(t, []string{"The Inner Light"}, func(p *pool) {
				c := kfhClone(p.byName["The Inner Light"])
				f := c.Faces[0]
				f.SVars["DamageTrig"] = "Mode$ DamageDone | ValidTarget$ Card.IsRemembered | Execute$ TrigDestroySrc | TriggerDescription$ test"
				f.SVars["TrigDestroySrc"] = tc.body
				p.byName["The Inner Light"] = c
			})
			kp.refusedWith(t, "The Inner Light", "beside an emblem trigger that reads the damaged recipient as its trigger object")
		})
	}
}

// TestEmblemDamageDoneRecipientRememberedSourceRefused: on the
// recipient-captured shape a ValidSource$ that still carries IsRemembered
// (here as one clause of two) refuses: the emblem's capture is matched on
// the RECIPIENT side, and a source filter keeping the Remembered flag would
// never match, so the emblem would silently never fire.
func TestEmblemDamageDoneRecipientRememberedSourceRefused(t *testing.T) {
	kp := kfhBuild(t, []string{"The Inner Light"}, func(p *pool) {
		c := kfhClone(p.byName["The Inner Light"])
		f := c.Faces[0]
		f.SVars["DamageTrig"] = "Mode$ DamageDone | ValidSource$ Card.IsRemembered,Creature | ValidTarget$ Card.IsRemembered | Execute$ TrigDraw | TriggerDescription$ test"
		p.byName["The Inner Light"] = c
	})
	kp.refusedWith(t, "The Inner Light", "Effect RememberObjects$ Targeted beside a remembered ValidSource$ and a remembered object ValidTarget$")
}
