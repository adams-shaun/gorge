package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Myriad (CR 702.116a): "whenever this creature attacks, for each opponent
// other than defending player, you may create a token that's a copy of this
// creature that's tapped and attacking that player". gorge expands K:Myriad
// into an Attacks Self trigger marked Myriad$ True whose body is DB$ Myriad
// (cards.kwMyriad; trigmatch.AttacksMatchedAttackers fires it only when that
// Execute$ body is the Myriad one), and a layer-6 grant (Blade of Selves,
// Legion Loyalty) synthesizes the same trigger without the marker
// (scriptfacts.GrantedKeywordTrigger, the __kwMyriadGranted payload). The
// body walks the opponents other than the defending player
// (effects.myriadOpponents), asking the controller about each (a "yes" /
// "no" KChoose) and making a tapped copy attacking that player on a yes;
// the copies are exiled as the combat ends. One OpExt ExtMyriad over Self
// (dirac/kw_myriad.go) is the whole body (seat-exact: with no opponent but
// the defending player it asks nothing and creates nothing).
// Each instance triggers separately (CR 702.116b): Scurry of Squirrels'
// two printed lines are two triggers, a printed line beside a grant two.

// isMyriadBody reports whether sa is gorge's DB$ Myriad body as the keyword
// expansion writes it, with no sub-ability (the chain lowering refuses any
// param it does not read).
func isMyriadBody(sa *cards.SA) bool {
	return sa != nil && sa.API == "Myriad" && sa.Sub == nil
}

// faceHasMyriadTrigger reports whether f carries the printed Myriad
// expansion's trigger: Attacks, ValidCard$ Card.Self, Myriad$ True, body
// DB$ Myriad.
func faceHasMyriadTrigger(f *cards.Face) bool {
	for _, t := range f.Triggers {
		if t.Mode == "Attacks" && t.Params["Myriad"] == "True" && t.Params["ValidCard"] == "Card.Self" &&
			isMyriadBody(cards.ResolveSVar(f.SVars, t.Params["Execute"])) {
			return true
		}
	}
	return false
}

// myriadMarker reads an Attacks trigger's Myriad$ marker: only K:Myriad's
// expansion (Card.Self, the DB$ Myriad body) lowers; gorge's matcher never
// fires a marker whose Execute$ body is anything else.
func myriadMarker(v, validCard string, body *cards.SA) error {
	if v != "True" || validCard != "Card.Self" || !isMyriadBody(body) {
		return refuse("Attacks Myriad %q", v)
	}
	return nil
}

// myriad lowers the DB$ Myriad body: one ExtMyriad. It lowers
// only as the whole body of an Attacks Self trigger (the printed expansion
// or the granted one), the context gorge's body reads its defending player
// from.
func (lc *lowerCtx) myriad(sa *cards.SA) error {
	if lc.ab == nil || lc.ab.Kind != akTriggered || lc.ab.Trigger == nil || lc.ab.Trigger.Event != "Attacks" ||
		!filterSelfOnly(lc.ab.Trigger.F) || len(lc.ab.Prog) != 0 || !isMyriadBody(sa) {
		return refuse("DB$ Myriad outside an Attacks Self trigger's whole body")
	}
	lc.emit(lInstr{Op: "Myriad", Obj: lRef{Kind: rSelf}, A: cst(1), CS: "Tapped",
		Note: "for each opponent but the defending player, may copy it tapped and attacking that player"})
	return nil
}

// splitMyriadGrant takes every bare "Myriad" term out of a `&` keyword
// list: the rest of the list and how many there were. Each term is its own
// instance (CR 702.116b): gorge's granted walk (rules/trigger_granted.go
// checkGrantedStaticTriggersUsing) queues one trigger per granted line.
func splitMyriadGrant(kw string) (string, int) {
	var rest []string
	n := 0
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		if k == "Myriad" {
			n++
			continue
		}
		rest = append(rest, k)
	}
	return strings.Join(rest, " & "), n
}

// myriadGrantTrigger lowers the trigger a granted Myriad gives its holder as
// a latent Attacks Self ability and returns its index, one per card: gorge's
// synthesized body (scriptfacts grantedMyriadTrigger), the printed one's
// without the Myriad$ marker. The kernel lists it once per granting static
// among the holder's granted triggers (grantedAbilitiesN), where gorge's
// granted-static walk queues it.
func (lc *lowerCtx) myriadGrantTrigger() (int, error) {
	const name = "GrantedMyriad"
	for i, a := range lc.out.Abilities {
		if a.Latent && a.Name == name {
			return i, nil
		}
	}
	t, ok := cards.ParseTriggerLine("Mode$ Attacks | ValidCard$ Card.Self | TriggerDescription$ Myriad")
	if !ok {
		return 0, refuse("granted Myriad trigger line")
	}
	t.Effect = &cards.SA{Kind: "DB", API: "Myriad", Params: map[string]string{}}
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) != idx+1 {
		return 0, refuse("granted Myriad lowering")
	}
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = name
	return idx, nil
}
