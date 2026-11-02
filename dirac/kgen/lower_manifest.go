package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// manifest, cloak and manifest dread. The
// supported shapes are exactly the ones gorge's effManifest / effCloak /
// effManifestDread resolve: a top-card move for a named player (or the
// resolving controller), a literal/SVar Amount$, and the cloak/dread riders.
// Everything else refuses.

// manifest lowers api:Manifest (gorge effManifest, CR 701.40a): the named
// player (DefinedPlayer$, default You) manifests Amount$ (default 1) cards
// from the top of their library. Defined$ object manifests, Choices$ and
// RememberManifested$ are gorge's unimplemented shapes and refuse.
func (lc *lowerCtx) manifest(ps *params) error {
	for _, k := range []string{"Defined", "Choices", "RememberManifested"} {
		if _, ok := ps.get(k); ok {
			return refuse("Manifest %s$", k)
		}
	}
	if v, ok := ps.get("Amount"); ok {
		// gorge's effManifest refuses X/Y (and any body Num cannot resolve)
		// loudly: a manifest count is never the cast's own X value.
		switch strings.TrimSpace(v) {
		case "X", "Y":
			return refuse("Manifest Amount$ %s", strings.TrimSpace(v))
		}
	}
	ref := lRef{Kind: rYou}
	if v, ok := ps.get("DefinedPlayer"); ok {
		r, err := lc.definedPlayer(strings.TrimSpace(v))
		if err != nil {
			return err
		}
		ref = r
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Manifest", Obj: ref, A: n})
	return nil
}

// cloak lowers api:Cloak (gorge effCloak, CR 701.58a): the named player
// cloaks Amount$ (default 1) cards from the top of their library face down
// with ward {2}. Supported shapes: DefinedPlayer$ (searchPlayers) or a bare
// Defined$ TopOfLibrary (the resolving controller); Tapped$/Shuffle$/
// RememberCloaked$ True. Choices$, the Remembered object shape and the
// ValidLibrary shape refuse.
func (lc *lowerCtx) cloak(ps *params) error {
	if _, ok := ps.get("Choices"); ok {
		return refuse("Cloak Choices$")
	}
	if v, ok := ps.get("Amount"); ok {
		// gorge's effCloak refuses X/Y loudly (effCloak's Amount$ scope).
		switch strings.TrimSpace(v) {
		case "X", "Y":
			return refuse("Cloak Amount$ %s", strings.TrimSpace(v))
		}
	}
	ref := lRef{Kind: rYou}
	if v, ok := ps.get("DefinedPlayer"); ok {
		r, err := lc.definedPlayer(strings.TrimSpace(v))
		if err != nil {
			return err
		}
		ref = r
	}
	if v, ok := ps.get("Defined"); ok {
		switch strings.TrimSpace(v) {
		case "", "TopOfLibrary":
		default:
			return refuse("Cloak Defined$ %q", v)
		}
	}
	var fl int
	for _, r := range []struct {
		key string
		bit int
	}{{"Tapped", kop.CfTapped}, {"Shuffle", kop.CfShuffle}, {"RememberCloaked", kop.CfRemember}} {
		if v, ok := ps.get(r.key); ok {
			if !strings.EqualFold(strings.TrimSpace(v), "True") {
				return refuse("Cloak %s$ %q", r.key, v)
			}
			fl |= r.bit
		}
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Cloak", Obj: ref, A: n, C: fl})
	return nil
}

// manifestDread lowers api:ManifestDread (gorge effManifestDread, CR
// 701.62a): the named player manifests dread Amount$ times (default 1).
// RememberManifested$ True joins each manifested card to the remembered set.
// Choices$ is gorge's unimplemented shape and refuses.
func (lc *lowerCtx) manifestDread(ps *params) error {
	if _, ok := ps.get("Choices"); ok {
		return refuse("ManifestDread Choices$")
	}
	ref := lRef{Kind: rYou}
	if v, ok := ps.get("DefinedPlayer"); ok {
		r, err := lc.definedPlayer(strings.TrimSpace(v))
		if err != nil {
			return err
		}
		ref = r
	}
	var fl int
	if v, ok := ps.get("RememberManifested"); ok {
		if !strings.EqualFold(strings.TrimSpace(v), "True") {
			return refuse("ManifestDread RememberManifested$ %q", v)
		}
		fl |= kop.CfRemember
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "ManifestDread", Obj: ref, A: n, C: fl})
	return nil
}
