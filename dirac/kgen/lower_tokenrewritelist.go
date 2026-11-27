package kgen

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// maxTokRewriteList is how many scripts a list rewriter names: the kernel
// buffers them in a fixed array (dirac/repl_toklistrewrite.go).
const maxTokRewriteList = 8

// tokenRewriteList lowers a CreateToken rewriter with a comma-listed
// TokenScript$ (Academy Manufactor: "If you would create a Clue, Food, or
// Treasure token, instead create one of each"): R:Event$ CreateToken |
// ValidPlayer$ You | ValidToken$ <token types> | ReplaceWith$ DB$ ReplaceToken
// | Type$ ReplaceToken. gorge applyTokenReplacementToPlan replaces each
// matching mint with one mint per listed script, in list order
// (knownTokenScripts), so the first script is Repl.C (the rewriter's card) and
// the others ride the replacement ability's program as CreateToken
// instructions the kernel reads for their card only (the matchup remaps each
// like any CreateToken's). ValidToken$ is the filter on the token the
// controller creates (Repl.Filter); only the controller's own tokens are
// read, so the shape needs ValidPlayer$ You (youOnly).
func (lc *lowerCtx) tokenRewriteList(rp *params, w *cards.SA, rl *lRepl, youOnly bool) error {
	raw := strings.TrimSpace(w.Params["TokenScript"])
	if !youOnly {
		return refuse("repl-hard: CreateToken rewriter TokenScript list %q without ValidPlayer$ You", raw)
	}
	if !withIs(w, "ReplaceToken", map[string]string{"Type": "ReplaceToken", "TokenScript": w.Params["TokenScript"]}) {
		return refuse("repl-hard: CreateToken replacement shape")
	}
	var scripts []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" || strings.EqualFold(s, "Chosen") {
			return refuse("repl-hard: CreateToken rewriter TokenScript %q", raw)
		}
		if _, ok := lc.p.reg.Token(s); !ok {
			return refuse("token script %q not in the corpus", s)
		}
		scripts = append(scripts, s)
	}
	if len(scripts) > maxTokRewriteList {
		return refuse("repl-hard: CreateToken rewriter TokenScript list of %d", len(scripts))
	}
	vt := rp.str("ValidToken")
	f, err := lc.filter(vt)
	if err != nil {
		return err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("repl-hard: CreateToken rewriter ValidToken %q", vt)
	}
	for _, c := range f.Clauses {
		// the kernel reads the token's card and its creator's own battlefield
		if c.Flags&fSelf != 0 || (c.Ctrl != whoYou && c.Ctrl != whoAny) {
			return refuse("repl-hard: CreateToken rewriter ValidToken %q", vt)
		}
	}
	for _, s := range scripts {
		if !slices.Contains(lc.out.Tokens, s) {
			lc.out.Tokens = append(lc.out.Tokens, s)
		}
	}
	for _, s := range scripts[1:] {
		rl.Prog = append(rl.Prog, lInstr{Op: "CreateToken", Obj: lRef{Kind: rYou}, A: cst(1), CS: s})
	}
	rl.F, rl.Kind, rl.S = f, "RewriteTokens", scripts[0]
	return nil
}
