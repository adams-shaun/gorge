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
// from the top of their library. Defined$ Remembered (manifestRemembered),
// the Defined$ ValidLibrary Card.TopLibrary+<owner> top-card shape
// (manifestValidLibrary) and the Choices$ hand pick (manifestChoices) are the
// object and chooser shapes; Amount$ resolves through the shared value
// grammar (an X Amount$ still refuses: a manifest count is never the cast's
// own X). RememberManifested$ is the remember-set family and refuses beside
// every shape here.
func (lc *lowerCtx) manifest(ps *params) error {
	if strings.TrimSpace(ps.str("Defined")) == "Remembered" {
		return lc.manifestRemembered(ps)
	}
	if _, ok := ps.get("RememberManifested"); ok {
		return refuse("Manifest RememberManifested$")
	}
	if v, ok := ps.get("Choices"); ok {
		return lc.manifestChoices(ps, v)
	}
	if v, ok := ps.get("Defined"); ok {
		if spec := strings.TrimSpace(v); strings.HasPrefix(spec, "ValidLibrary ") {
			return lc.manifestValidLibrary(ps, spec)
		}
		return refuse("Manifest Defined$ %q", strings.TrimSpace(v))
	}
	if v, ok := ps.get("Amount"); ok && strings.TrimSpace(v) == "X" {
		return refuse("Manifest Amount$ X")
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

// manifestValidLibrary lowers Defined$ ValidLibrary Card.TopLibrary+<owner>
// (Orochi Soul-Reaver's "+ControlledBy TriggeredTarget", Thieving Amalgam's
// "+ActivePlayerCtrl"): the top card of the named player's library manifests
// under the resolving controller, the card's owner staying the library's
// owner (gorge manifestObjects over definedSpec, ControlChange then
// MoveZone; CR 701.40a, 110.2a). ActivePlayerCtrl reads the Phase trigger's
// player, the active player. An empty library manifests nothing. Any other
// selector, DefinedPlayer$ beside it and a non-default Amount$ (gorge ignores
// Amount$ for a defined set) refuse.
func (lc *lowerCtx) manifestValidLibrary(ps *params, spec string) error {
	sel, ok := strings.CutPrefix(strings.TrimSpace(strings.TrimPrefix(spec, "ValidLibrary ")), "Card.TopLibrary+")
	if !ok {
		return refuse("Manifest Defined$ %q", spec)
	}
	var ref lRef
	switch player, has := strings.CutPrefix(sel, "ControlledBy "); {
	case has:
		r, err := lc.definedPlayer(strings.TrimSpace(player))
		if err != nil {
			return refuse("Manifest Defined$ %q: %v", spec, err)
		}
		ref = r
	case sel == "ActivePlayerCtrl":
		if tr := lc.ab.Trigger; tr == nil || tr.Event != "Phase" {
			return refuse("Manifest Defined$ %q outside a Phase trigger", spec)
		}
		r, err := lc.defined("TriggeredPlayer")
		if err != nil {
			return refuse("Manifest Defined$ %q: %v", spec, err)
		}
		ref = r
	default:
		return refuse("Manifest Defined$ %q", spec)
	}
	if _, ok := ps.get("DefinedPlayer"); ok {
		return refuse("Manifest Defined$ ValidLibrary with DefinedPlayer$")
	}
	if v, ok := ps.get("Amount"); ok {
		switch strings.TrimSpace(v) {
		case "", "1":
		default:
			return refuse("Manifest Defined$ ValidLibrary with Amount$ %q", strings.TrimSpace(v))
		}
	}
	lc.emit(lInstr{Op: "Manifest", Obj: ref, A: cst(1), C: kop.CfCloakCtrl,
		Note: "manifest the library's top card under the resolving controller"})
	return nil
}

// manifestChoices lowers the Choices$ hand-pick shape (Scroll of Fate's
// "AB$ Manifest | Choices$ Card.YouCtrl | ChoiceZone$ Hand"): the resolving
// controller picks one card of their hand that Choices$ admits and manifests
// it (gorge effManifest's chooseFromZoneForFaceDown, CR 701.40a). The ask is
// the mandatory single pick, gorge's option kind "face_down_choice"; an
// empty pool is no ask and no manifest. A Defined$ or DefinedPlayer$ beside
// Choices$, a ChoiceZone$ other than Hand and an Amount$ other than the
// default 1 (gorge asks for min(Amount$, pool) cards; the kernel pick is
// one) refuse.
func (lc *lowerCtx) manifestChoices(ps *params, choices string) error {
	if v, ok := ps.get("Defined"); ok && strings.TrimSpace(v) != "" {
		return refuse("Manifest Choices$ with Defined$ %q", v)
	}
	if _, ok := ps.get("DefinedPlayer"); ok {
		return refuse("Manifest Choices$ with DefinedPlayer$")
	}
	if v, ok := ps.get("ChoiceZone"); ok && strings.TrimSpace(v) != "Hand" {
		return refuse("Manifest Choices$ with ChoiceZone$ %q", v)
	}
	if v, ok := ps.get("Amount"); ok {
		switch strings.TrimSpace(v) {
		case "", "1":
		default:
			return refuse("Manifest Choices$ Amount$ %q", strings.TrimSpace(v))
		}
	}
	f, err := lc.filter(choices)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "ChooseCard", Obj: lRef{Kind: rYou}, F: f, C: zHand, CS: "FaceDownPick",
		Note: "the manifest-from-hand pick (gorge's face_down_choice)"})
	lc.emit(lInstr{Op: "Manifest", Obj: lRef{Kind: rYou}, A: cst(1), C: kop.CfChosen,
		Note: "manifest the pick from where it sits, under the resolving controller"})
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
	if v, ok := ps.get("DefinedPlayer"); ok && strings.TrimSpace(v) == "RememberedController" {
		return lc.cloakEachRememberedController(ps)
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

// cloakEachRememberedController lowers Unexplained Absence's
//
//	DB$ Cloak | Amount$ 1 | Defined$ TopOfLibrary | DefinedPlayer$ RememberedController
//
// after a ChangeZone that exiled several targets with RememberLKI$: for each
// exiled permanent, its controller cloaks the top card of their library. gorge
// expands RememberedController over every remembered object (gorge
// rememberedControllersLKI), so the kernel runs one Cloak per remembered
// object with RefEach as its player, the controller its RememberLKI$ capture
// holds (kop.RepeatRemembered; dirac/act_repeatremembered.go). A plain resumable
// frame (RepeatPlain) so a future asking body would park, and one instruction
// so the loop has no resume state.
func (lc *lowerCtx) cloakEachRememberedController(ps *params) error {
	if !lc.remLKIEach {
		return refuse("Cloak DefinedPlayer$ RememberedController without a multi-target RememberLKI$ move")
	}
	for _, k := range []string{"Choices", "ValidLibrary"} {
		if _, ok := ps.m[k]; ok {
			return refuse("Cloak DefinedPlayer$ RememberedController with %s$", k)
		}
	}
	if v, ok := ps.get("Defined"); ok {
		switch strings.TrimSpace(v) {
		case "", "TopOfLibrary":
		default:
			return refuse("Cloak DefinedPlayer$ RememberedController with Defined$ %q", v)
		}
	}
	if v, ok := ps.get("Amount"); ok {
		switch strings.TrimSpace(v) {
		case "", "1":
		default:
			return refuse("Cloak DefinedPlayer$ RememberedController with Amount$ %q", strings.TrimSpace(v))
		}
	}
	fl, err := lc.cloakRiders(ps)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "RepeatEachCard", C: kop.RepeatRemembered | kop.RepeatPlain, Skip: 1,
		Note: "one cloak for each remembered object's controller"})
	lc.emit(lInstr{Op: "Cloak", Obj: lRef{Kind: rEach}, A: cst(1), C: fl})
	ps.consume("Defined", "DefinedPlayer", "Amount")
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
