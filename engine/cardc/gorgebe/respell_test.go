package gorgebe

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestRespellDefaults holds the closed default table: every row cites the
// engine reader (file:line, text on that line) that reads the param absent and
// written with its default alike, the list keeps its required rows, and
// followForge adds or removes only a listed param with its default value.
func TestRespellDefaults(t *testing.T) {
	need := map[[2]string]string{
		{"Draw", "NumCards"}: "1", {"ChangeZone", "ChangeNum"}: "1",
		{"Draw", "Defined"}: "You", {"Discard", "Defined"}: "You", {"GainLife", "Defined"}: "You",
		{"LoseLife", "Defined"}: "You", {"Mill", "Defined"}: "You",
	}
	seen := map[[2]string]bool{}
	for _, r := range respellDefaults {
		k := [2]string{r.API, r.Param}
		if seen[k] {
			t.Errorf("%s %s listed twice", r.API, r.Param)
		}
		seen[k] = true
		delete(need, k)
		file, line, ok := strings.Cut(r.Reader, ":")
		n, err := strconv.Atoi(line)
		if !ok || err != nil {
			t.Errorf("%s %s: reader %q is not file:line", r.API, r.Param, r.Reader)
			continue
		}
		b, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(file)))
		if err != nil {
			t.Errorf("%s %s: %v", r.API, r.Param, err)
			continue
		}
		ls := strings.Split(string(b), "\n")
		if n < 1 || n > len(ls) || !strings.Contains(ls[n-1], r.Marker) {
			t.Errorf("%s %s: %s does not hold %q", r.API, r.Param, r.Reader, r.Marker)
		}
	}
	for k := range need {
		t.Errorf("required default %v missing", k)
	}
}

func respellForge(t *testing.T, line string) *forgeCard {
	t.Helper()
	c, diags := ParseScript("forge.txt", []byte("Name:T\nManaCost:no cost\nTypes:Instant\nA:"+line+"\nOracle:x\n"))
	if len(diags) > 0 {
		t.Fatalf("%v", diags)
	}
	return c
}

type forgeCard = cards.Card

func TestRespellFollow(t *testing.T) {
	run := func(cardc, forge string) string {
		out := followForge([]byte("Name:T\nManaCost:no cost\nTypes:Instant\nA:"+cardc+"\nOracle:x\n"), respellForge(t, forge))
		return strings.TrimSuffix(strings.SplitN(strings.SplitN(string(out), "A:", 2)[1], "\n", 2)[0], "")
	}
	for _, c := range []struct{ name, cardc, forge, want string }{
		{"drops an explicit default", "SP$ Draw | Cost$ 0 | NumCards$ 1", "SP$ Draw | Cost$ 0", "SP$ Draw | Cost$ 0"},
		{"adds an omitted default", "SP$ Draw | Cost$ 0", "SP$ Draw | Cost$ 0 | Defined$ You | NumCards$ 1", "SP$ Draw | Cost$ 0 | Defined$ You | NumCards$ 1"},
		{"keeps a non-default value", "SP$ Draw | Cost$ 0 | NumCards$ 2", "SP$ Draw | Cost$ 0", "SP$ Draw | Cost$ 0 | NumCards$ 2"},
		{"leaves an unlisted param", "SP$ Draw | Cost$ 0 | Upto$ True", "SP$ Draw | Cost$ 0", "SP$ Draw | Cost$ 0 | Upto$ True"},
		{"leaves an unlisted API", "SP$ Scry | Cost$ 0 | ScryNum$ 1", "SP$ Scry | Cost$ 0", "SP$ Scry | Cost$ 0 | ScryNum$ 1"},
		{"never touches a target", "SP$ PutCounter | Cost$ 0 | ValidTgts$ Creature | Defined$ Self", "SP$ PutCounter | Cost$ 0 | ValidTgts$ Creature", "SP$ PutCounter | Cost$ 0 | ValidTgts$ Creature | Defined$ Self"},
	} {
		if got := run(c.cardc, c.forge); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
