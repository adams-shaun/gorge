package dirac

import (
	"fmt"
	"os"
)

// TEMP PROBE (delete before commit): env-gated trace of the kernel's filter
// prefix search (filtSearch) for the Bash to Bits flashback-cast divergence.
// Run the oracle repro with TESTENV='GORGE_FILT_TRACE=1'.

var zzFilt = os.Getenv("GORGE_FILT_TRACE") != ""
var zzOn bool

var zzW *os.File

// ZzFiltOn turns the temp trace on (TEMP PROBE).
func ZzFiltOn() {
	zzFilt = true
	if zzW == nil {
		zzW, _ = os.Create("../../.ds4/scratch/filt.trace")
	}
}

func zzPrint(args ...any) {
	if zzW != nil {
		zzW.WriteString(fmt.Sprint(args...))
	}
}

func (s *State) zzN(k *prep, sl Slot) string {
	if sl == NoSlot {
		return "-"
	}
	return k.t.CardName(s.cardAt(sl))
}

func (s *State) zzM(k *prep, m slotMask) string {
	out := "["
	for b := m; b.any(); b = b.rest() {
		out += s.zzN(k, b.first()) + " "
	}
	return out + "]"
}

func zzPool(t *State, p uint8) string {
	pl := &t.pl[SeatIx(p)]
	out := "pool("
	for c, n := range pl.pool {
		if n != 0 {
			out += string("WUBRGCAg"[c]) + itoaZ(int(n))
		}
	}
	return out + ")"
}

func itoaZ(n int) string {
	if n == 0 {
		return "0"
	}
	out := ""
	for n > 0 {
		out = string(rune('0'+n%10)) + out
		n /= 10
	}
	return out
}

func (fs *filtSearch) zzStepDesc(s *State, k *prep, t *State, st filtStep) string {
	out := "{sl=" + s.zzN(k, st.sl) + " ans=" + itoaZ(int(st.ans))
	if st.gap {
		out += " gap"
	}
	if st.restr {
		out += " restr"
	}
	if st.sacOnly {
		out += " sacOnly"
	}
	if st.wide {
		out += " wide"
	}
	if st.other.kind != 0 {
		out += " otherkind=" + itoaZ(int(st.other.kind)) + " col=" + itoaZ(int(st.other.col)) + " plain=" + itoaZ(b2iZ(st.other.plain))
	}
	return out + "}"
}

func b2iZ(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (fs *filtSearch) zzPath(s *State, k *prep) string {
	out := "["
	for i := 0; i < fs.n; i++ {
		out += fs.zzStepDesc(s, k, nil, fs.path[i]) + " "
	}
	return out + "]"
}
