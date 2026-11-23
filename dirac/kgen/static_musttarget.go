package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Flagbearer rule (static_mode.go): a printed S:Mode$ MustTarget makes an
// opponent choose at least one target matching ValidTarget$ while casting a
// spell or activating an ability they control. gorge's
// engine/rules/musttarget_static.go is the reference.
func init() {
	staticMode("MustTarget", (*staticCtx).modeMustTarget)
}

func (sc *staticCtx) modeMustTarget() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	if sc.st.Cond.Kind != cNone {
		return nil, true, refuse("MustTarget with a condition")
	}
	// The kernel reads the rule for an opponent's spell or activated ability
	// only, the corpus shape; another ValidSA$ scope is refused by name
	// rather than silently widened.
	sa, ok := sp.get("ValidSA")
	if !ok {
		return nil, true, refuse("MustTarget without ValidSA$")
	}
	spell, activated := false, false
	for _, alt := range strings.Split(sa, ",") {
		switch strings.TrimSpace(alt) {
		case "Spell.OppCtrl":
			spell = true
		case "Activated.OppCtrl":
			activated = true
		default:
			return nil, true, refuse("MustTarget ValidSA %q", strings.TrimSpace(alt))
		}
	}
	if !spell || !activated {
		return nil, true, refuse("MustTarget ValidSA %q (the kernel reads Spell.OppCtrl,Activated.OppCtrl)", sa)
	}
	vt, ok := sp.get("ValidTarget")
	if !ok {
		return nil, true, refuse("MustTarget without ValidTarget$")
	}
	var err error
	if sc.st.F, err = lc.filter(vt); err != nil {
		return nil, true, err
	}
	if sc.st.F.Players != 0 {
		return nil, true, refuse("MustTarget ValidTarget %q names a player", vt)
	}
	if vz, ok := sp.get("ValidZone"); ok && strings.TrimSpace(vz) != "Battlefield" {
		return nil, true, refuse("MustTarget ValidZone %q (the battlefield is the only modelled zone)", vz)
	}
	sp.consume("Secondary")
	return nil, false, nil
}

// targetStaticKind maps the modes lowered through the builder's CantTarget
// case to their kop kinds: CantTarget's own, and MustTarget (the Flagbearer
// rule), which shares its filter-only shape.
func targetStaticKind(mode string) kop.StaticKind {
	if mode == "MustTarget" {
		return kop.StatMustTarget
	}
	return kop.StatCantTarget
}
