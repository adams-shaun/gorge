package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExtortAskIsNotTheMayMillAsk pins the Extort election's ask value off the
// may-mill family's. resolveOptionsGen dispatches on res.ask and tries the
// Extort options before the may-mill ones, so a shared value presents the
// {W/B} pay/decline election for a mill's yes/no; applyAskExtort would then
// charge the pip and drain instead of milling. The two neighbouring
// raGenManifestDread offsets (modal activation) are pinned for the same
// reason: they were the values the iota block handed out before Extort moved
// out of it.
func TestExtortAskIsNotTheMayMillAsk(t *testing.T) {
	if raGenExtort == raGenMayMill {
		t.Fatalf("raGenExtort (%d) aliases raGenMayMill", raGenExtort)
	}
	if raGenExtort == raGenModalMode || raGenExtort == raGenModalTarget {
		t.Fatalf("raGenExtort (%d) aliases a modal-activation ask (%d/%d)",
			raGenExtort, raGenModalMode, raGenModalTarget)
	}
	if raGenDigAllOpt == raGenModalMode {
		t.Fatalf("raGenDigAllOpt (%d) aliases raGenModalMode", raGenDigAllOpt)
	}
	if raGenCmdZone == raGenModalTarget {
		t.Fatalf("raGenCmdZone (%d) aliases raGenModalTarget", raGenCmdZone)
	}

	// The behavioural half: a real may-mill ask (OpMill with MvMayMill, the
	// one lower_effect.go's MvMayMill would emit) must reach the confirm
	// election, not Extort's.
	prog := []kop.Instr{{Op: kop.OpMill, Obj: kop.RefController, A: kop.Const(2), C: kop.MvMayMill}}
	sc := czScen(t, prog)
	if sc.s.exec(sc.k, &prog[0]) || sc.s.res.ask != raGenMayMill {
		t.Fatalf("precondition: no may-mill ask, res.ask %d", sc.s.res.ask)
	}
	var d Decision
	sc.s.resolveOptionsGen(sc.k, &d)
	if d.Sub == ChGenPayMode {
		t.Fatal("the may-mill ask resolved as Extort's {W/B} pay/decline election")
	}
	if d.Sub != ChGenConfirm || d.N != 2 {
		t.Fatalf("may-mill options: sub %d n %d, want confirm yes/no", d.Sub, d.N)
	}
}
