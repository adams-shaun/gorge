package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestAttackedBySourceRef: kop.RefAttackedBySource (gorge
// playerStateQualifierAttackedBySourceThisTurn, Defined$
// Player.attackedBySourceThisTurn) names the seat the resolving source
// DECLARED an attack at this turn, nobody before a declaration, nobody on a
// later turn, and nobody once the source has left the battlefield. A
// permanent put onto the battlefield attacking is not a declaration and must
// not set the ledger.
func TestAttackedBySourceRef(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	s := sc.s
	src := sc.perm(v.GrizzlyBears, 0)
	p := &s.perms[s.objs[src].slot]
	s.res.e = stackEntry{kind: seAbility, obj: src, ctrl: 0}

	// precondition: no declaration yet, so the ref names nobody.
	if got := s.playerRef(sc.k, kop.RefAttackedBySource); got != 255 {
		t.Fatalf("before a declaration: playerRef = %d, want none", got)
	}

	// an entry "attacking" (setAttackTarget, not a declaration) is not a
	// DeclareAttackers fold: the ledger stays empty.
	s.setAttackTarget(p, PlayerCode(1))
	if got := s.playerRef(sc.k, kop.RefAttackedBySource); got != 255 {
		t.Fatalf("put onto the battlefield attacking: playerRef = %d, want none", got)
	}

	// a declaration at seat 1 this turn names it.
	s.setDeclaredAttackTarget(p, PlayerCode(1))
	if got := s.playerRef(sc.k, kop.RefAttackedBySource); got != 1 {
		t.Fatalf("declared at seat 1: playerRef = %d, want 1", got)
	}
	if got, want := s.refCode(sc.k, kop.RefAttackedBySource), PlayerCode(1); got != want {
		t.Fatalf("refCode = %d, want %d", got, want)
	}

	// the declaration is this turn's only: the turn change names nobody.
	s.beginTurn(1)
	if got := s.playerRef(sc.k, kop.RefAttackedBySource); got != 255 {
		t.Fatalf("next turn: playerRef = %d, want none", got)
	}

	// a source that has left the battlefield names nobody.
	s.setDeclaredAttackTarget(p, PlayerCode(1))
	s.objs[src].zone = kop.ZoneGraveyard
	if got := s.playerRef(sc.k, kop.RefAttackedBySource); got != 255 {
		t.Fatalf("source left: playerRef = %d, want none", got)
	}
}
