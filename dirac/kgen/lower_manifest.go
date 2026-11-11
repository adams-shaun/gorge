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
	if strings.TrimSpace(ps.str("Defined")) == "Remembered" {
		return lc.manifestRemembered(ps)
	}
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

// manifestRemembered lowers Manifest Defined$ Remembered (gorge
// effManifest's manifestObjects, CR 701.40a; Write into Being): each
// remembered card manifests from where it sits under the resolving
// controller. gorge ignores Amount$ for a defined set, so only the default 1
// (or none) is read; DefinedPlayer$, Choices$ and RememberManifested$ beside
// it refuse.
func (lc *lowerCtx) manifestRemembered(ps *params) error {
	for _, k := range []string{"Choices", "DefinedPlayer", "RememberManifested"} {
		if _, ok := ps.get(k); ok {
			return refuse("Manifest Defined$ Remembered with %s$", k)
		}
	}
	if v, ok := ps.get("Amount"); ok {
		switch strings.TrimSpace(v) {
		case "", "1":
		default:
			return refuse("Manifest Defined$ Remembered with Amount$ %q", strings.TrimSpace(v))
		}
	}
	lc.emit(lInstr{Op: "Manifest", Obj: lRef{Kind: rYou}, A: cst(1), C: kop.CfManifestRemembered,
		Note: "manifest each remembered card under the resolving controller"})
	return nil
}

// cloak lowers api:Cloak (gorge effCloak, CR 701.58a): the named player
// cloaks Amount$ (default 1) cards from the top of their library face down
// with ward {2}. Supported shapes: DefinedPlayer$ (searchPlayers) or a bare
// Defined$ TopOfLibrary (the resolving controller); the Choices$ hand pick
// (Vannifar, Evolved Enigma); the ValidLibrary Card.TopLibrary+OwnedBy shape
// (Etrata, Deadly Fugitive); Defined$ Remembered (cloakRemembered);
// Tapped$/Shuffle$/RememberCloaked$ True.
func (lc *lowerCtx) cloak(ps *params) error {
	if strings.TrimSpace(ps.str("Defined")) == "Remembered" {
		return lc.cloakRemembered(ps)
	}
	if v, ok := ps.get("Choices"); ok {
		return lc.cloakChoices(ps, v)
	}
	if v, ok := ps.get("Defined"); ok {
		if spec := strings.TrimSpace(v); strings.HasPrefix(spec, "ValidLibrary ") {
			return lc.cloakValidLibrary(ps, spec)
		}
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
	fl, err := lc.cloakRiders(ps)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Cloak", Obj: ref, A: n, C: fl})
	return nil
}

// cloakRiders reads the shared cloak riders: Tapped$/Shuffle$/
// RememberCloaked$ True set their flag bits, any other value refuses.
func (lc *lowerCtx) cloakRiders(ps *params) (int, error) {
	var fl int
	for _, r := range []struct {
		key string
		bit int
	}{{"Tapped", kop.CfTapped}, {"Shuffle", kop.CfShuffle}, {"RememberCloaked", kop.CfRemember}} {
		if v, ok := ps.get(r.key); ok {
			if !strings.EqualFold(strings.TrimSpace(v), "True") {
				return 0, refuse("Cloak %s$ %q", r.key, v)
			}
			fl |= r.bit
		}
	}
	return fl, nil
}

// cloakRemembered lowers Defined$ Remembered: every card of the resolution's
// remembered set cloaks from where it sits under the resolving controller
// (gorge effCloak's Remembered arm, CR 701.58a, 110.2a). Shuffle$ is gorge's
// pile shuffle, a no-op (no event, no RNG), so the kernel ignores CfShuffle.
// Choices$, DefinedPlayer$ and a non-default Amount$ (ignored by gorge on
// this arm) refuse.
func (lc *lowerCtx) cloakRemembered(ps *params) error {
	if _, ok := ps.get("Choices"); ok {
		return refuse("Cloak Defined$ Remembered with Choices$")
	}
	if _, ok := ps.get("DefinedPlayer"); ok {
		return refuse("Cloak Defined$ Remembered with DefinedPlayer$")
	}
	if v, ok := ps.get("Amount"); ok {
		switch strings.TrimSpace(v) {
		case "", "1":
		default:
			return refuse("Cloak Defined$ Remembered with Amount$ %q", strings.TrimSpace(v))
		}
	}
	fl, err := lc.cloakRiders(ps)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Cloak", Obj: lRef{Kind: rYou}, A: cst(1), C: fl | kop.CfCloakRemembered,
		Note: "cloak each remembered card under the resolving controller"})
	return nil
}

// cloakChoices lowers the Choices$ hand-pick shape (Vannifar, Evolved
// Enigma's "DB$ Cloak | Choices$ Card.YouCtrl" inside its combat trigger's
// Charm): the resolving controller picks one card of their hand that
// Choices$ admits and cloaks it (gorge effCloak's chooseFromZoneForFaceDown +
// cloak(id, c.Controller); CR 701.58a, 110.2a). The ask is the mandatory
// single pick of a hand filter, gorge's option kind "face_down_choice"; an
// empty pool is no ask and no cloak (OpChooseCard's zero-candidate break,
// gorge's n <= 0). A Defined$ beside Choices$ has no corpus carrier
// (gorge's shape check), a ChoiceZone$ other than Hand and an Amount$ other
// than the default 1 refuse.
func (lc *lowerCtx) cloakChoices(ps *params, choices string) error {
	if v, ok := ps.get("Defined"); ok && strings.TrimSpace(v) != "" {
		return refuse("Cloak Choices$ with Defined$ %q", v)
	}
	if v, ok := ps.get("ChoiceZone"); ok && strings.TrimSpace(v) != "Hand" {
		return refuse("Cloak Choices$ with ChoiceZone$ %q", v)
	}
	if v, ok := ps.get("Amount"); ok {
		switch strings.TrimSpace(v) {
		case "X", "Y":
			return refuse("Cloak Amount$ %s", strings.TrimSpace(v))
		case "", "1":
		default:
			return refuse("Cloak Choices$ Amount$ %q", v)
		}
	}
	fl, err := lc.cloakRiders(ps)
	if err != nil {
		return err
	}
	f, err := lc.filter(choices)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "ChooseCard", Obj: lRef{Kind: rYou}, F: f, C: zHand, CS: "FaceDownPick",
		Note: "the cloak-from-hand pick (gorge's face_down_choice)"})
	lc.emit(lInstr{Op: "Cloak", Obj: lRef{Kind: rYou}, A: cst(1), C: fl | kop.CfChosen,
		Note: "cloak the pick from where it sits, under the resolving controller"})
	return nil
}

// cloakValidLibrary lowers the ValidLibrary shape (Etrata, Deadly Fugitive's
// Defined$ "ValidLibrary Card.TopLibrary+OwnedBy TriggeredTarget"): the top
// card of the named player's library cloaks under the resolving controller,
// not the library's owner (gorge effCloak's definedSpec +
// cloak(t.Obj, c.Controller); CR 701.58a, 110.2a). An empty library cloaks
// nothing. Only the TopLibrary+OwnedBy selector has a corpus carrier; any
// other ValidLibrary selector refuses by name, as does any non-default
// Amount$ (gorge resolves but then ignores Amount$ there; the kernel has no
// shape for it).
func (lc *lowerCtx) cloakValidLibrary(ps *params, spec string) error {
	ownedBy, player, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(spec, "ValidLibrary ")), "+OwnedBy ")
	if !ok || ownedBy != "Card.TopLibrary" {
		return refuse("Cloak Defined$ %q", spec)
	}
	ref, err := lc.definedPlayer(strings.TrimSpace(player))
	if err != nil {
		return refuse("Cloak Defined$ %q: %v", spec, err)
	}
	fl, err := lc.cloakRiders(ps)
	if err != nil {
		return err
	}
	if v, ok := ps.get("Amount"); ok {
		switch strings.TrimSpace(v) {
		case "", "1":
		default:
			return refuse("Cloak Defined$ ValidLibrary with Amount$ %q", strings.TrimSpace(v))
		}
	}
	lc.emit(lInstr{Op: "Cloak", Obj: ref, A: cst(1), C: fl | kop.CfCloakCtrl,
		Note: "cloak the library's top card under the resolving controller"})
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
