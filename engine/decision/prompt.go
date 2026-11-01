package decision

import (
	"strconv"

	"github.com/adams-shaun/gorge/engine/state"
)

// PriorityPrompt is a priority decision's Prompt for the seat named name.
func PriorityPrompt(turn int32, step state.Step, name string) string {
	verb := " has priority"
	if name == "You" {
		verb = " have priority"
	}
	return "turn " + strconv.Itoa(int(turn)) + ", " + step.String() + " — " + name + verb
}
