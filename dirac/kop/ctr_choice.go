package kop

// ExtPutCounterChoice's operands (dirac/ctr_choice.go): up to four counter
// kinds, one byte each in B's low byte, B's high byte, C's low byte and C's
// high byte, in the card's printed order. A byte of PutCtrChoiceNone is no
// kind (CtrP1P1 is 0, so 0 cannot be the filler).
const (
	PutCtrChoiceMax  = 4
	PutCtrChoiceNone = 0xff
)

var _ = [1]struct{}{}[int(NumCounterKinds3)>>8] // every kind fits a byte below the filler

// PutCtrChoiceOperands packs kinds (at most PutCtrChoiceMax) into the B and C
// operands of an ExtPutCounterChoice instruction.
func PutCtrChoiceOperands(kinds []CounterKind) (b ValueRef, c int16) {
	var w [PutCtrChoiceMax]uint8
	for i := range w {
		w[i] = PutCtrChoiceNone
		if i < len(kinds) {
			w[i] = uint8(kinds[i])
		}
	}
	return Const(int(int16(uint16(w[0]) | uint16(w[1])<<8))), int16(uint16(w[2]) | uint16(w[3])<<8)
}

// PutCtrChoiceKinds returns the kinds an ExtPutCounterChoice instruction
// offers, in order.
func (in *Instr) PutCtrChoiceKinds() (kinds [PutCtrChoiceMax]uint8, n int) {
	w := [PutCtrChoiceMax]uint8{uint8(in.B.N), uint8(uint16(in.B.N) >> 8), uint8(in.C), uint8(uint16(in.C) >> 8)}
	for _, k := range w {
		if k == PutCtrChoiceNone {
			break
		}
		kinds[n] = k
		n++
	}
	return kinds, n
}
