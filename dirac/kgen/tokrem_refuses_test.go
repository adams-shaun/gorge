package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTokenRememberSourceRefuses pins the two over-acceptances the r2 review
// named on tokenRememberSource (both latent: no corpus card hits either):
//
//   - a static-created token (the RepeatEach Role body; the kernel mints it
//     from a static site, dirac/static_roletokens.go) has no Token-effect
//     resolution for the kernel's tokenMemos to read the creator from, so
//     the smRem record it would write is wrong or absent while the bit tells
//     the static read to expect one;
//   - TokenRemembered$ beside RememberSource$: gorge's tokenMemoryOf appends
//     the TokenRemembered$ group AND the source to the token's memory, so the
//     same source twice counts its counters twice in the CDA's
//     Remembered$CardCounters read, while the kernel keeps one smRem record.
//
// The plain Saproling Burst shape must still lower.
func TestTokenRememberSourceRefuses(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p := &pool{nDecks: map[string]int{}, reg: reg}
	script := "g_x_x_saproling_burst"
	if _, ok := reg.Token(script); !ok {
		t.Fatalf("%s not in the corpus", script)
	}
	for _, tc := range []struct {
		name   string
		params map[string]string
		rc     int
		want   string
	}{
		{"TokenRemembered beside", map[string]string{"RememberSource": "True", "TokenRemembered": "Self"}, 0, "TokenRemembered"},
		{"static-created mint", map[string]string{"RememberSource": "True"}, 1, "static-created"},
	} {
		lc := &lowerCtx{p: p, out: &lCard{Name: script}, repeatCard: tc.rc}
		err := lc.tokenRememberSource(newParams(tc.params), script, &lInstr{Op: "CreateToken"})
		if err == nil {
			t.Errorf("%s lowered; want a refusal", tc.name)
		} else if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s refused %q, want it to name %q", tc.name, err, tc.want)
		}
	}
	// the plain shape still lowers onto the src-mem bit
	lc := &lowerCtx{p: p, out: &lCard{Name: script}}
	in := &lInstr{Op: "CreateToken"}
	if err := lc.tokenRememberSource(newParams(map[string]string{"RememberSource": "True"}), script, in); err != nil {
		t.Fatalf("plain shape refused: %v", err)
	}
	if in.C&tokRemSrcBit == 0 || !lc.out.SrcMem {
		t.Errorf("plain shape: C %d SrcMem %v, want the TokRememberSrc bit and SrcMem", in.C, lc.out.SrcMem)
	}
}
