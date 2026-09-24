package cards

import (
	"strconv"
)

func kwUndaunted(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	for _, st := range f.Statics {
		if st.Params["KeywordLine"] == k {
			return
		}
	}
	sv := "__kwUndaunted" + strconv.Itoa(i)
	f.setSVar(sv, "PlayerCountOpponents")
	p := parseParams("Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | EffectZone$ All | Amount$ " + sv +
		" | Description$ This spell costs {1} less to cast for each opponent you have.")
	p["KeywordLine"] = k
	f.Statics = append(f.Statics, Static{Mode: "ReduceCost", Params: p})
}

func init() { registerKeyword(kwUndaunted, "Undaunted") }
