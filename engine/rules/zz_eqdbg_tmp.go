package rules

import (
	"fmt"
	"os"

	"github.com/adams-shaun/gorge/engine/state"
)

// TEMPORARY debug aid (delete before commit): traces the offer gates for the
// traced objects during the offer walk.
func dbgEquip(o *state.Object, format string, args ...any) {
	if o == nil || o.Face() == nil {
		return
	}
	n := o.Face().Name
	if n != "Loxodon Warhammer" && n != "Billiard Room" && n != "Vault of the Archangel" {
		return
	}
	fmt.Fprintf(os.Stderr, "EQDBG "+n+" "+format+"\n", args...)
}

func dbgEquipObj(e *Engine, id state.ObjID, format string, args ...any) {
	dbgEquip(e.G.Obj(id), format, args...)
}
