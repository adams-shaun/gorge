package oracle

import (
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/dirac/kop"
)

func init() {
	NewAtomLowerer = func(t *kop.Table) AtomLowerer { return kgen.NewAtomCompilerOver(t) }
}
