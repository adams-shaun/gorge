package env

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// goldenIdentity is the fixed engine identity the Protocol v2 goldens pin.
// The live identity carries the IR compiler fingerprint and corpus hash, so
// pinning it would make every IR or corpus change rewrite every transcript.
var goldenIdentity = protocol.Engine{
	Name:             "gorge",
	Version:          "gorge-golden/spellbench-adapter-" + AdapterVersion,
	RulesSnapshotID:  "gorge/golden/ir-golden",
	CardPoolIdentity: "forge-golden/corpus-golden/catalog-golden",
}

// NewForGoldens builds a Server whose engine identity is goldenIdentity, for
// the golden replay and regeneration only.
func NewForGoldens(reg *cards.Registry) *Server {
	id := goldenIdentity
	return newServer(reg, nil, &id)
}
