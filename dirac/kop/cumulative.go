package kop

// ExtCumulative (Mystic Remora; CR 702.24a): the head of a cumulative upkeep
// trigger's body. If the source is still the permanent that triggered on the
// battlefield it gets an age counter (a counter placement like any other, so
// an AddCounter replacement applies, gorge startCumulativeUpkeep's
// CounterChange emit); if it is not, the next C instructions (the payment and
// the sacrifice behind it) are skipped. The UnlessPaid that follows charges
// its generic cost once per age counter now on the source, as the trigger's
// own pay-or-decline ask (gorge's cumulative_pay / cumulative_sac); the
// sacrifice behind it runs on a decline. dirac/trig_cumulative.go.
//
// With B = 1 (EchoNoCounter) it is the head of an echo trigger's body
// instead (CR 702.30a; Cradle Guard): the same gone-source skip, no age
// counter, and the UnlessPaid behind it charges the echo cost once.
const ExtCumulative ExtOp = 312

// EchoNoCounter is ExtCumulative's B for an echo trigger: no age counter.
const EchoNoCounter = 1

func init() { extOpNames[ExtCumulative] = "Cumulative" }
