package rules

import "github.com/adams-shaun/gorge/effects"

// Cipher's runtime encoded-card association and combat-damage trigger are
// implemented in trigger_granted.go; the printed expansion lives in cards.
func init() { effects.RegisterNonAPI("kw:Cipher") }
