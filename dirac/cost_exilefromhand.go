package dirac

// A printed trigger's own ExileFromHand<N/Card> cost (The Wrath of Khan's
// chapter II "you may exile a card from your hand. If you do, draw a card";
// gorge cumulative.go's triggered-cost window: the trigger_cost pay/decline,
// then one "exile_cost" pick over the payer's hand, unasked with exactly N
// cards). It rides PayExileGrave with B.N kop.PayBSubExileHand (Instr.C's
// nibble is full) and execPay widens it to payKindExileHand. The pick and the
// move are the graveyard exile's (exileFromGrave detaches from any zone: a
// cost exile is no discard, so no discard trigger or madness reads it).

// payKindExileHand is the pay kind (execPay's kind, pickZ) of a trigger's own
// ExileFromHand<N/Card> cost.
const payKindExileHand = 19

// handExileOptions poses the pick (ChGenPayExile): every card in p's hand.
//
//go:noinline
func (s *State) handExileOptions(d *Decision, p uint8) {
	d.Sub = ChGenPayExile
	pl := &s.pl[SeatIx(p)]
	for i := 0; i < int(pl.handN); i++ {
		d.add(MakeAction(AkChoose, uint8(pl.hand[i]), 0, 0))
	}
}
