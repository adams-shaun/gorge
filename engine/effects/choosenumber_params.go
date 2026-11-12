package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// ChooseNumberAtRandom reports whether an api:ChooseNumber draws its number at
// random (Random$ True). Haktos the Unscarred's "choose 2, 3, or 4 at random"
// is the as-enters carrier: rules' entry ask must not pose a question for it,
// and instead records the engine's own draw so a log-only replay re-derives
// it. This is the ONE reader of that parameter; call sites never call ParamStr.
func ChooseNumberAtRandom(sa *cards.SA) bool {
	return sa != nil && strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKRandom)), "True")
}
