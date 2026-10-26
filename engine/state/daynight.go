package state

// DayNight is the CR 731 day/night designation. The game has exactly one of
// three states: neither (CR 731.1, the start of every game), day or night.
// Once it is day or night it stays one of the two; only a CR 727 restart puts
// it back to neither. The zero value is neither.
type DayNight uint8

const (
	DayNightNeither DayNight = iota
	DayNightDay
	DayNightNight
)

// String is "" for neither, "day" or "night": the event text and the view
// field, both omitted while the game has neither.
func (d DayNight) String() string {
	switch d {
	case DayNightDay:
		return "day"
	case DayNightNight:
		return "night"
	}
	return ""
}
