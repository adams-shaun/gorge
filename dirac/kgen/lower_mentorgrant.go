package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/rules/scriptfacts"
)

// splitMentorGrant takes every bare "Mentor" term out of a `&` keyword list:
// the rest of the list and how many there were. Each term is its own instance
// (CR 702.134b, 113.2c): gorge queues one granted Mentor trigger per granting
// static, beside the printed K:Mentor's own.
func splitMentorGrant(kw string) (string, int) {
	var rest []string
	n := 0
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		if k == "Mentor" {
			n++
			continue
		}
		rest = append(rest, k)
	}
	return strings.Join(rest, " & "), n
}

// grantMentor takes the bare Mentor terms out of a static's granted keyword
// list and appends one latent trigger per instance to st.Grant (CR 702.134b,
// 113.2c), returning the rest of the list. Lives here so lowerCtx.continuous
// gains one call, not another per-instance loop.
func (lc *lowerCtx) grantMentor(kw string, st *lStatic) (string, error) {
	rest, n := splitMentorGrant(kw)
	for ; n > 0; n-- {
		idx, err := lc.mentorGrantTrigger()
		if err != nil {
			return "", err
		}
		st.Grant = append(st.Grant, idx)
	}
	return rest, nil
}

// mentorGrantTrigger lowers the trigger a granted Mentor gives its holder
// (CR 702.134b; Nyxborn Unicorn's bestowed "has mentor") as a latent ability
// and returns its index, one per card. It has the printed expansion's shape
// (cards.kwMentor, via scriptfacts): Attacks on Self, body DB$ PutCounter with
// the Mentor$ lesser-power marker. The kernel lists it once per granting
// static among the holder's granted triggers (grantedAbilitiesN), so the
// holder triggers once per instance beside any printed Mentor.
func (lc *lowerCtx) mentorGrantTrigger() (int, error) {
	const name = "GrantedMentor"
	for i, a := range lc.out.Abilities {
		if a.Latent && a.Name == name {
			return i, nil
		}
	}
	// One shared shape: the same synthesizer gorge queues the granted
	// trigger from (scriptfacts.GrantedKeywordTrigger), so the kernel and the
	// engine cannot drift apart on what a granted Mentor is.
	tp := scriptfacts.GrantedKeywordTrigger("Mentor")
	if tp == nil {
		return 0, refuse("granted Mentor lowering")
	}
	t := *tp
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) != idx+1 {
		return 0, refuse("granted Mentor lowering")
	}
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = name
	return idx, nil
}
