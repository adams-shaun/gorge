package contracts

// PackKinds is dzg-pack-2's canonical decision-kind table: a shard's kind
// column holds an index into it, and every meta.json lists it whole. The
// spellings are decision.Kind's. The first four are the search's own index
// order (azmcts.KindNames); the rest follow decision's declaration order.
//
// APPEND-ONLY. A new kind goes at the end (an additive change: append it to
// testdata/pack_kinds.txt too and update that golden's hash). Reordering or
// removing an entry renumbers every packed shard on disk and is a breaking
// change to dzg-pack; TestPackKindsAppendOnly fails on it.
var PackKinds = []string{
	"priority",
	"attackers",
	"blockers",
	"target",
	"mulligan",
	"modes",
	"trigger_order",
	"trigger_optional",
	"commander_zone",
	"choose",
	"replacement",
	"arrange",
	"starting_player",
}

// PackKindIndex is kind's index in PackKinds, or -1.
func PackKindIndex(kind string) int {
	for i, k := range PackKinds {
		if k == kind {
			return i
		}
	}
	return -1
}
