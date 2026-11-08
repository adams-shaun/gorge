package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestEvidenceStageIgnoresStaleTriggerReservation: a declined Lamplight
// Phoenix cost leaves pickZ = payKindTrigNewEvidence and res.e (a trigger whose
// card sits in the graveyard) behind. An activation's collect-evidence pick
// (Forensic Researcher, Polygraph Orb; CR 701.59a) built afterwards must list
// every graveyard card: the stale reservation excluded the card, and a
// one-card graveyard posed a decision with no options.
func TestEvidenceStageIgnoresStaleTriggerReservation(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Cackling Prowler", Count: 1}, {Name: "Forest", Count: 8}})
	s := sc.s
	g := sc.grave(id["Cackling Prowler"], 0, 1)
	o := g[0]
	s.g.lki[0] = lkiRec{obj: o, ver: s.objs[o].ver - 1}
	s.res.e = stackEntry{kind: seTrigger, trigObj: o, li: 1}
	s.g.pickZ = payKindTrigNewEvidence

	// precondition: while a trigger's own cost runs, the reservation holds
	s.ctl = ctlResolve
	if s.evExcl(0) != 0 {
		t.Fatalf("precondition: evExcl = %d during a resolution, want 0", s.evExcl(0))
	}

	// the activation being built: the stale reservation must not apply
	s.ctl = ctlBuild
	*s.bld() = build{e: stackEntry{kind: seAbility, ctrl: 0}}
	s.g.evSel = 0
	var d Decision
	s.evidenceOptions(sc.k, &d, 0, 1)
	if d.N != 1 || d.Options[0].Kind() != AkChoose || Obj(d.Options[0].Source()) != o {
		t.Fatalf("evidence pick options = %v, want the one graveyard card", d.Options[:d.N])
	}
	if got, want := s.evAllMask(0), uint64(1); got != want {
		t.Fatalf("evAllMask = %b, want %b", got, want)
	}
}
