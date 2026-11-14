package kop

// ExtGrantKwAll: every battlefield permanent matching F gains the keywords
// C (the low 16 bits) | B.N << 16 until end of turn, read once per permanent
// in OpPumpAll's walk order (gorge effPumpAll's keyword grant over
// ValidCards$, CR 611.2c). It is a keyword-only PumpAll whose mask has a bit
// above 15, which OpPumpAll's C cannot carry (Dirge of Dread's fear, Tortoise
// Formation's shroud); no P/T is added. Obj is RefNone. dirac/act_grantkwall.go.
const ExtGrantKwAll ExtOp = 296

func init() { extOpNames[ExtGrantKwAll] = "GrantKwAll" }
