package events

import (
	"encoding/json"
	"strconv"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// CopyExceptions is a CopyExcept event's payload: the FINAL values a copy
// effect's exceptions give the copy (CR 707.9a/b), settled by the emitter
// (engine/effects/copy_except.go) against the copied face. The fold writes
// them onto the copy's layer-1 basis face, so they are the copy's copiable
// values and a later copy of the copy inherits them (CR 707.3). It rides the
// event's Text as JSON (Text/ParseCopyExceptions); encoding/json writes
// struct fields in declaration order, so the bytes are deterministic.
type CopyExceptions struct {
	// Types, when TypesSet, is the copy's whole type line.
	Types    []string `json:"types,omitempty"`
	TypesSet bool     `json:"types_set,omitempty"`
	// Power/Toughness, when PTSet, are the copy's P/T.
	Power     int32 `json:"power,omitempty"`
	Toughness int32 `json:"toughness,omitempty"`
	PTSet     bool  `json:"pt_set,omitempty"`
	// Keywords are keyword lines the copy has (CR 707.9a), in grant order.
	Keywords []string `json:"keywords,omitempty"`
}

// Empty reports whether x carries no exception.
func (x CopyExceptions) Empty() bool {
	return !x.TypesSet && !x.PTSet && len(x.Keywords) == 0
}

// Text is the CopyExcept event's Text for x.
func (x CopyExceptions) Text() string {
	b, _ := json.Marshal(x) // a plain struct of strings, ints and bools: never fails
	return string(b)
}

// ParseCopyExceptions reads Text's encoding. A malformed payload fails
// closed: ok is false and the fold applies nothing.
func ParseCopyExceptions(s string) (x CopyExceptions, ok bool) {
	if json.Unmarshal([]byte(s), &x) != nil {
		return CopyExceptions{}, false
	}
	return x, true
}

// foldCopyExcept folds Kind CopyExcept into state. The copy's current face
// (its CopyFace basis, or a just-minted token copy's printed face) is
// value-copied and edited -- never mutated in place, since it may be the
// shared corpus face or another copy's basis (CR 707.2b) -- and installed
// as Obj's CopyFace. No derive() runs (the cards setters re-parse only the
// field they write), so a replay rebuilds a FaceEqual twin.
func foldCopyExcept(g *state.Game, e *Event) {
	o := g.Obj(e.Obj)
	if o == nil || o.Face() == nil {
		return
	}
	x, ok := ParseCopyExceptions(e.Text)
	if !ok {
		return
	}
	f := *o.Face()
	// The edited face is no longer the catalog row it may be bound to (a
	// compiled TypeMask or trigger-interest prefilter would answer for the
	// unedited text), so it takes the computed paths a token face takes.
	f.ForgetCompiled()
	if x.TypesSet {
		f.Types = append([]string(nil), x.Types...)
	}
	if x.PTSet {
		f.SetPT(strconv.Itoa(int(x.Power)) + "/" + strconv.Itoa(int(x.Toughness)))
		// CR 707.9d: a copy effect that provides a specific P/T does not copy
		// a characteristic-defining ability that defines P/T (Quicksilver
		// Gargantuan copying Tarmogoyf is 7/7).
		kept := make([]cards.Static, 0, len(f.Statics))
		for _, s := range f.Statics {
			_, setP := s.Param(cards.PKSetPower)
			_, setT := s.Param(cards.PKSetToughness)
			if s.ParamStr(cards.PKCharacteristicDefining) == "True" && (setP || setT) {
				continue
			}
			kept = append(kept, s)
		}
		f.Statics = kept
	}
	if len(x.Keywords) > 0 {
		// Deep-copy every container a keyword expansion appends to or writes
		// (the ClonePermanent "ifnew" discipline): f aliases the source face.
		svars := make(map[string]string, len(f.SVars))
		for k, v := range f.SVars {
			svars[k] = v
		}
		f.SVars = svars
		f.Keywords = append([]string(nil), f.Keywords...)
		f.Abilities = append([]*cards.SA(nil), f.Abilities...)
		f.Triggers = append([]cards.Trigger(nil), f.Triggers...)
		f.Repls = append([]cards.Repl(nil), f.Repls...)
		f.Statics = append([]cards.Static(nil), f.Statics...)
		for _, k := range x.Keywords {
			if !f.ApplyKeywordExpansion(k) {
				f.Keywords = append(f.Keywords, k)
			}
		}
	}
	o.SetCopyFace(&f)
}
