package rules

import "github.com/adams-shaun/gorge/engine/effects"

// stat:CantRegenerate is the Effect-delivered "can't be regenerated"
// restriction (Incinerate's StaticAbilities$ body with Mode$ CantRegenerate),
// honoured by RegenerationDisallowed on the destroy-replacement path. No
// printed S:Mode$ CantRegenerate static exists in the corpus; only the
// Effect-delivered shape is claimed. Its own init keeps the registration out
// of the shared statics.go list.
func init() { effects.RegisterNonAPI("stat:CantRegenerate") }
