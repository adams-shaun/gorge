package kop

// ExtKwMark (CR 702.13a intimidate, CR 702.90 infect, granted until end of
// turn): the battlefield objects named by Obj (RefSelf, a target slot, ...),
// or with Obj RefNone every battlefield permanent matching F (PumpAll's
// ValidCards$, read once as it resolves, CR 611.2c), gain the keywords C
// names (KwMarkIntimidate | KwMarkInfect) until end of turn. Neither keyword
// has a KwMask bit (all 32 are taken), so the grant is an arena slot bit per
// keyword (no State byte), cleared in the cleanup step (CR 514.2) and when a
// new permanent takes the slot. It reads as the printed keyword does
// (CardDef.Evasion EvIntimidate, CardDef.Infect) unless the permanent has
// lost its abilities; a grant made after the loss survives it (CR 613.7).
// gorge Pump / PumpAll KW$ Intimidate, KW$ Infect (the derived keyword
// gorge's combat.CanBlock and damage riders read). dirac/kw_kwmark.go.
// Tainted Strike, Vector Asp, Hideous Visage, Triumph of the Hordes.
const ExtKwMark ExtOp = 10

func init() { extOpNames[ExtKwMark] = "KwMark" }

// ExtKwMark's C bits.
const (
	KwMarkIntimidate = 1 << iota // intimidate (CR 702.13a)
	KwMarkInfect                 // infect (CR 702.90)
)
