package cards

// DayTime is a Forge DayTime$ / DB$ DayTime Value$ word as a code: the CR 731
// designation a replacement requires (Brimstone Vandal's DayTime$ Neither)
// or the one an effect sets, plus Switch (day becomes night and night
// becomes day). Zero is a word outside the vocabulary.
type DayTime uint16

const (
	DayTimeUnknown DayTime = iota
	DayTimeNeither
	DayTimeDay
	DayTimeNight
	DayTimeSwitch
)

var dayTimeCodes = NewStrCodes(
	StrEntry[DayTime]{"Neither", DayTimeNeither},
	StrEntry[DayTime]{"Day", DayTimeDay},
	StrEntry[DayTime]{"Night", DayTimeNight},
	StrEntry[DayTime]{"Switch", DayTimeSwitch},
)

// DayTimeOf is word's code, DayTimeUnknown outside the vocabulary.
func DayTimeOf(word string) DayTime { return dayTimeCodes.Code(word) }
