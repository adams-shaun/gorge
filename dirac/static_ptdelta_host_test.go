package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestPTDeltaWhileHostUsesTablePT: a host-bound P/T pump record (Zelyon
// Sword's "target creature gets +2/+0 for as long as CARDNAME remains
// tapped", Duration$ UntilUntaps) keeps its host's Obj and version in the
// record's P/T fields (applyWhileHost), so ptDeltaRecs must add the record's
// table SetP/SetT constants, not the host ids, and the host's untap ends the
// record (whileTappedEnd).
func TestPTDeltaWhileHostUsesTablePT(t *testing.T) {
	tbl := *vanilla.Table
	tbl.Exprs = make([]kop.Expr, 1) // force the generated path (k.gen)
	tbl.Overrides = append([]kop.Override(nil), vanilla.Table.Overrides...)
	tbl.Overrides = append(tbl.Overrides, kop.Override{PTDelta: true, WhileHost: true, WhileTapped: true, SetP: 2, SetT: 3})
	ov := len(tbl.Overrides) - 1
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	host := sc.perm(vanilla.GrizzlyBears, 0)
	tgt := sc.perm(vanilla.GrizzlyBears, 0)
	tgtSl := sc.slot(tgt)
	// precondition: the target is a plain 2/2 with no pump records, the
	// values a fix-off sum of the host ids would not reproduce
	if c := sc.s.chars(sc.k, tgtSl, upTo7b); c.bp != 2 || c.bt != 2 {
		t.Fatalf("target base P/T %d/%d, want 2/2", c.bp, c.bt)
	}
	if dp, dt := sc.s.ptDeltaRecs(sc.k, tgtSl); dp != 0 || dt != 0 {
		t.Fatalf("pump records %d/%d before any application, want 0/0", dp, dt)
	}
	hob := &sc.s.objs[host]
	sc.s.perms[hob.slot].flags |= pfTapped
	sc.s.res = resCtx{e: stackEntry{ctrl: 0, obj: host, srcVer: hob.ver, card: vanilla.GrizzlyBears}, active: true}
	sc.s.applyWhileHost(sc.k, tgtSl, ov, &sc.s.res.e)
	var rec *ovRec
	for i := 0; i < int(sc.s.g.nfx); i++ {
		if r := &sc.s.g.fx[i]; r.sl == tgtSl && r.kind == fxOverride {
			rec = &r.r
		}
	}
	if rec == nil {
		t.Fatal("no override record on the target")
	}
	if Obj(rec.p) != host || rec.t != int16(hob.ver) {
		t.Fatalf("record p/t %d/%d, want host %d ver %d", rec.p, rec.t, host, hob.ver)
	}
	if dp, dt := sc.s.ptDeltaRecs(sc.k, tgtSl); dp != 2 || dt != 3 {
		t.Fatalf("pump records %d/%d, want the table's 2/3 (not the host ids %d/%d)", dp, dt, host, hob.ver)
	}
	// the host's untap ends the record
	sc.s.perms[hob.slot].flags &^= pfTapped
	sc.s.untapEnds(sc.k, host)
	if dp, dt := sc.s.ptDeltaRecs(sc.k, tgtSl); dp != 0 || dt != 0 {
		t.Fatalf("pump records %d/%d after the host untapped, want 0/0", dp, dt)
	}
}
