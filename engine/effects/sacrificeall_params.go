package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// sacrificeAllParams compiles the SacrificeAll-specific Controller$ rider:
// Forge's SacrificeAllEffect keeps only the objects the named players control
// ("Controller$ You" on Crumbling Colossus's "sacrifice it" means only its
// controller's copy).
type sacrificeAllParams struct{ Controller string }

func compileSacrificeAllParams(sa *cards.SA) sacrificeAllParams {
	return sacrificeAllParams{Controller: strings.TrimSpace(sa.ParamStr(cards.PKController))}
}
