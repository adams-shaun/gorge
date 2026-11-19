package cards

// ReplWord classifies the replacement-line words the walks dispatch on:
// the Layer$ value "Copy" (a replacement that is part of the permanent's
// copiable values, CR 707.2), the Layer$ value "CantHappen" (the event does
// not happen, CR 614.1a) and the Keyword$ value "ETBReplacement" (the
// K:ETBReplacement expansion). One table, so rules compares codes rather
// than adding `s == "Literal"` sites.
type ReplWord uint16

const (
	ReplWordNone ReplWord = iota
	ReplWordCopyLayer
	ReplWordETBReplacement
	ReplWordCantHappenLayer
)

var replWordCodes = NewStrCodes(
	StrEntry[ReplWord]{Key: "Copy", Val: ReplWordCopyLayer},
	StrEntry[ReplWord]{Key: "ETBReplacement", Val: ReplWordETBReplacement},
	StrEntry[ReplWord]{Key: "CantHappen", Val: ReplWordCantHappenLayer},
)

// IsCopyLayer reports whether r carries Layer$ Copy.
func (r Repl) IsCopyLayer() bool {
	return replWordCodes.Code(r.ParamStr(PKLayer)) == ReplWordCopyLayer
}

// IsETBReplacementKeyword reports whether r carries Keyword$ ETBReplacement.
func (r Repl) IsETBReplacementKeyword() bool {
	return replWordCodes.Code(r.ParamStr(PKKeyword)) == ReplWordETBReplacement
}

// IsCantHappenLayer reports whether r carries Layer$ CantHappen, the
// "this event does not happen" replacement word (CR 614.1a; Fear of Sleep
// Paralysis's stun-counter lock, the GameLoss/GameWin family, the
// effect-created Counter lock). One entry in the shared ReplWord table, so
// the class matchers compare codes rather than adding new
// `EqualFold(..., "CantHappen")` sites.
func (r Repl) IsCantHappenLayer() bool {
	return replWordCodes.Code(r.ParamStr(PKLayer)) == ReplWordCantHappenLayer
}
