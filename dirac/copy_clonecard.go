package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugCloneCardTmplNone is the mutation switch of TestDemoCloneAddMutation:
// an off-battlefield Clone template names no card, so the copy never happens.
var DebugCloneCardTmplNone bool

// CloneCardStats counts the off-battlefield Clone templates read
// (TestDemoCloneAddLockstep).
type CloneCardStats struct {
	Grave uint32 // a graveyard card copied (CloneTmplCard, TgtZone$ Graveyard)
	Cost  uint32 // the card the cost exiled copied (CloneTmplSac, Defined$ Exiled)
	Types uint32 // copies made with an AddTypes$ exception (Saheeli)
	Grant uint32 // copies made that kept a granted trigger (AddTriggers$, Curie)
	Base  uint32 // Count$CardBasePower reads (ExCardBasePower)
}

// DebugCloneCardStats, when set, receives the counts.
var DebugCloneCardStats *CloneCardStats

// cloneCardTmpl is the card an off-battlefield template of ExtCloneCopy names
// (gorge effClone copies the source object's printed face, CR 400.7, whatever
// zone it now is in): the target slot's card (CloneTmplCard), or the card the
// activation's own Exile<> cost exiled, the entry's sacrifice record
// (CloneTmplSac; snapSacPick). False when there is none.
func (s *State) cloneCardTmpl(in *kop.Instr) (CardID, bool) {
	if DebugCloneCardTmplNone {
		return 0, false
	}
	var o Obj
	if in.B.N&kop.CloneTmplSac != 0 {
		o = s.sacrificedObj(s.ctxEntry(&s.res.e))
	} else {
		o = s.objRefObj(kop.RefTarget0 + kop.Ref(in.B.N>>kop.CloneTmplShift&3))
	}
	if o == NoObj {
		return 0, false
	}
	if st := DebugCloneCardStats; st != nil {
		if in.B.N&kop.CloneTmplSac != 0 {
			st.Cost++
		} else {
			st.Grave++
		}
	}
	return s.objs[o].card, true
}

// cloneCardNote counts one resolved ExtCloneCopy's exception kinds
// (CloneCardStats): grant is the record index+1 cloneCopy applied, 0 none.
func (s *State) cloneCardNote(k *prep, in *kop.Instr, grant uint8) {
	st := DebugCloneCardStats
	if st == nil {
		return
	}
	if in.C != 0 && k.t.Overrides[in.C-1].AddTypes != 0 {
		st.Types++
	}
	if grant != 0 {
		st.Grant++
	}
}
