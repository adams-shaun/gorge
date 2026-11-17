package main

import (
	"testing"
	"testing/fstest"
)

// A registry card that is in no list, no refusal and no sidecar still gets
// its row: unverified when kgen lowers it, not-lowered when it does not.
func TestAuditRowsCoverRegistry(t *testing.T) {
	reg := twoCardCorpus(t)
	fsys := fstest.MapFS{
		"dirac/support/evidence/a.txt": {Data: []byte("Elsewhere\n")},
	}
	lir := func([]string) map[string]string {
		return map[string]string{"Sol Ring": "1111222233334444"}
	}
	rows := audit(fsys, reg, lir, opts{})
	by := map[string]row{}
	for _, r := range rows {
		by[r.Card] = r
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want Elsewhere, Push, Sol Ring: %v", len(rows), rows)
	}
	sol, ok := by["Sol Ring"]
	if !ok {
		t.Fatal("Sol Ring is in the registry but has no row")
	}
	if sol.GClaim == "-" || sol.DClaim != "unverified" || sol.DLists != "-" {
		t.Errorf("Sol Ring = g_claim %q d_claim %q d_lists %q, want a gorge claim, unverified, no lists",
			sol.GClaim, sol.DClaim, sol.DLists)
	}
	push, ok := by["Push"]
	if !ok {
		t.Fatal("Push is in the registry but has no row")
	}
	if push.GClaim == "-" || push.DClaim != "not-lowered" {
		t.Errorf("Push = g_claim %q d_claim %q, want a gorge claim and not-lowered", push.GClaim, push.DClaim)
	}
}

// The hunt status is read by exact token, so "diverged" is not a gorge-only row.
func TestKeySidesExactTokens(t *testing.T) {
	for _, c := range []struct {
		status, ctx string
		g, x        bool
	}{
		{"both", "", true, true},
		{"x", "", false, true},
		{"g", "", true, false},
		{"diverged", "", true, true},
		{"diverged", "xmage", false, true},
		{"open", "gorge", true, false},
	} {
		g, x := keySides(huntRow{Status: c.status, Ctx: c.ctx})
		if g != c.g || x != c.x {
			t.Errorf("keySides(status %q ctx %q) = %v,%v want %v,%v", c.status, c.ctx, g, x, c.g, c.x)
		}
	}
}
