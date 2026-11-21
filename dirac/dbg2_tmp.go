package dirac

import (
	"fmt"
	"os"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func (s *State) dbgFeeGate(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost, live, ok, known, decided bool) {
	if os.Getenv("DIRAC_DBG2") == "" {
		return
	}
	fmt.Printf("DBG2 feeGate p=%d %s#%d cost=%v live=%v ok=%v known=%v decided=%v availX=%d\n",
		p, k.t.Cards[c].Name, o, cost, live, ok, known, decided, s.g.availX)
}
