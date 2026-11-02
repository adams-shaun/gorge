package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A creature that left the battlefield deals the damage of its abilities with
// the deathtouch and lifelink it last had (CR 608.2h, 702.2b, 702.15a): a
// granted keyword is not on its card, and the ability is on the stack, pending
// or resolving when the source goes.
func TestSourceLeftKeepsGrantedKeywords(t *testing.T) {
	const both = kop.KwDeathtouch | kop.KwLifelink
	for _, where := range []string{"stack", "pending", "resolving", "trigger snapshot"} {
		t.Run(where, func(t *testing.T) {
			sc := newScen(t, Config{NoElide: true})
			g := sc.perm(v.GrizzlyBears, 0)
			sl := sc.slot(g)
			sc.s.perms[sl].eotKw |= both
			e := stackEntry{kind: seTrigger, obj: g, srcVer: sc.s.objs[g].ver, card: sc.s.objs[g].card, trigObj: g}
			switch where {
			case "stack":
				sc.s.stack[0], sc.s.sp = e, 1
			case "pending":
				sc.s.pend[0] = pendingTrig{obj: g, srcVer: e.srcVer, card: e.card, trigObj: g}
				sc.s.npend = 1
			case "resolving":
				sc.s.res.e, sc.s.res.active = e, true
			case "trigger snapshot": // a dies trigger names its source as the triggering object
				e.li = sc.s.snapLKI(g)
				sc.s.res.e, sc.s.res.active = e, true
			}
			k := sc.k
			sc.s.removePerm(k, sl, kop.ZoneGraveyard, false)
			switch where {
			case "stack":
				e = sc.s.stack[0]
			case "pending":
				e = stackEntry{kind: seTrigger, obj: g, srcVer: e.srcVer, card: e.card, skw: sc.s.pend[0].skw}
			case "resolving", "trigger snapshot":
				e = sc.s.res.e
			}
			sc.s.res.e, sc.s.res.active = e, true
			if got := sc.s.srcKw(k) & both; got != both {
				t.Fatalf("a %s ability of a departed source deals damage with keywords %b, want deathtouch and lifelink %b", where, got, both)
			}
		})
	}
}
