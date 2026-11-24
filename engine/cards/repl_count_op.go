package cards

// CountOp is the head of a ReplaceCount$ operator suffix ("/Plus.1"): the
// word between the slash and the first dot.
type CountOp uint16

const (
	CountOpNone CountOp = iota
	CountOpTwice
	CountOpPlus
	CountOpMinus
	CountOpLimitMax
)

var countOpCodes = NewStrCodes(
	StrEntry[CountOp]{Key: "Twice", Val: CountOpTwice},
	StrEntry[CountOp]{Key: "Plus", Val: CountOpPlus},
	StrEntry[CountOp]{Key: "Minus", Val: CountOpMinus},
	StrEntry[CountOp]{Key: "LimitMax", Val: CountOpLimitMax},
)

// SplitCountOp reads an operator suffix as the rules engine's replaceCountOp
// returns it: the head's code (CountOpNone for a name outside the vocabulary,
// or an argument form written without its argument, or Twice written with
// one) and the text after the dot.
func SplitCountOp(op string) (CountOp, string) {
	if len(op) < 2 || op[0] != '/' {
		return CountOpNone, ""
	}
	head, arg, hasArg := op[1:], "", false
	for i := 1; i < len(op); i++ {
		if op[i] == '.' {
			head, arg, hasArg = op[1:i], op[i+1:], true
			break
		}
	}
	code := countOpCodes.Code(head)
	if hasArg == (code == CountOpTwice) {
		return CountOpNone, ""
	}
	return code, arg
}
