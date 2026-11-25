package rules

import (
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func TestZZProbeDecree(t *testing.T) {
	var buf []byte
	logf := func(format string, args ...any) {
		buf = append(buf, []byte(fmt.Sprintf(format+"\n", args...))...)
	}
	e := handEngine(t, corpusAlternativeCard(t, "Decree of Savagery"))
	kobold := onBoard(t, e, 1, "Name:Kobold\nManaCost:R\nTypes:Creature Kobold\nPT:0/1\nOracle:x\n")
	id := e.G.Zone(state.ZHand, 0)[0]
	addMana(t, e, 0, "CCCCGG")
	e.beginActivation(0, cyclingAbilityOption(t, e, id))
	logf("pendingTriggers=%d", len(e.pendingTriggers))
	for i, pt := range e.pendingTriggers {
		o := e.G.Obj(pt.Source)
		var fcards int
		var ptSA, faceSA string
		if o != nil && o.Face() != nil {
			fcards = len(o.Face().Triggers)
			if fcards > 0 {
				faceSA = fmt.Sprintf("%p", o.Face().Triggers[0].Effect)
			}
		}
		if pt.SA != nil {
			ptSA = fmt.Sprintf("%p", pt.SA)
		}
		tr, ok := e.triggerOf(pt)
		spec := ""
		if ok {
			spec = triggerOptionalSpec(tr)
		}
		logf("pt[%d] src=%d o-nil=%v faceTriggers=%d ptIdx=%d ptMode=%q triggerOf-ok=%v mode=%q spec=%q ptSA=%s faceSA=%s params=%v",
			i, pt.Source, o == nil, fcards, pt.Idx, pt.Trigger.Mode, ok, tr.Mode, spec, ptSA, faceSA, tr.Params)
		dec, opt, askable := e.optionalDecider(pt)
		logf("pt[%d] optionalDecider who=%d optional=%v askable=%v", i, dec, opt, askable)
	}
	if e.Pending() == nil {
		e.Advance()
	}
	d := e.Pending()
	logf("pending kind=%v stack=%d", d.Kind, len(e.G.Stack))
	for _, sid := range e.G.Stack {
		so := e.G.Obj(sid)
		if so == nil || so.Ability == nil {
			continue
		}
		tr, ok := e.triggerForAbilityObject(sid, so)
		srcFace := ""
		if so.Source != 0 {
			if so2 := e.G.Obj(so.Source); so2 != nil && so2.Face() != nil && len(so2.Face().Triggers) > 0 {
				srcFace = fmt.Sprintf("%p", so2.Face().Triggers[0].Effect)
			}
		}
		logf("stack %d: triggerForAbilityObject ok=%v mode=%q spec=%q zone=%v ability=%s srcFaceSA=%s leftCopy=%v",
			sid, ok, tr.Mode, triggerOptionalSpec(tr), so.Zone, fmt.Sprintf("%p", so.Ability), srcFace, so.LeftCopyFace != nil)
	}
	if err := os.WriteFile("/home/sadams/projects/gorge/.worktrees/cli-20261125T144959Z-f494cc98/.ds4/scratch/probe.txt", buf, 0o644); err != nil {
		t.Fatal(err)
	}
	_ = kobold
}
