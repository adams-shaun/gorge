package cards

// ReplWord classifies the two replacement-line words the as-enters walk
// dispatches on: the Layer$ value "Copy" (a replacement that is part of the
// permanent's copiable values, CR 707.2) and the Keyword$ value
// "ETBReplacement" (the K:ETBReplacement expansion). One table, so rules
// compares codes rather than adding `s == "Literal"` sites.
type ReplWord uint16

const (
	ReplWordNone ReplWord = iota
	ReplWordCopyLayer
	ReplWordETBReplacement
)

var replWordCodes = NewStrCodes(
	StrEntry[ReplWord]{Key: "Copy", Val: ReplWordCopyLayer},
	StrEntry[ReplWord]{Key: "ETBReplacement", Val: ReplWordETBReplacement},
)

// IsCopyLayer reports whether r carries Layer$ Copy.
func (r Repl) IsCopyLayer() bool {
	return replWordCodes.Code(r.ParamStr(PKLayer)) == ReplWordCopyLayer
}

// IsETBReplacementKeyword reports whether r carries Keyword$ ETBReplacement.
func (r Repl) IsETBReplacementKeyword() bool {
	return replWordCodes.Code(r.ParamStr(PKKeyword)) == ReplWordETBReplacement
}
