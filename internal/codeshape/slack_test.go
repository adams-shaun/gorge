package codeshape

import "testing"

// TestSlack pins the slack rule and the count ratchets' verdicts at and around it.
func TestSlack(t *testing.T) {
	for _, c := range []struct{ ceiling, slack int }{
		{0, 0}, {1, 2}, {97, 2}, {299, 2}, {1681, 16}, {2756, 27},
	} {
		if got := Slack(c.ceiling); got != c.slack {
			t.Errorf("Slack(%d) = %d, want %d", c.ceiling, got, c.slack)
		}
	}
	for _, c := range []struct {
		name       string
		got, limit int
		fail       bool
	}{
		{"at ceiling", 1681, 1681, false},
		{"shrank", 1680, 1681, false},
		{"within the slack", 1683, 1681, false},
		{"at the slack", 1681 + 16, 1681, false},
		{"beyond the slack", 1681 + 17, 1681, true},
		{"small ceiling within the floor", 99, 97, false},
		{"small ceiling beyond the floor", 100, 97, true},
		{"zero ceiling stays exact", 1, 0, true},
	} {
		if failed, msg := ratchetVerdict(ratchet{name: c.name, got: c.got, limit: c.limit}); failed != c.fail {
			t.Errorf("%s: %d against %d failed = %v, want %v (%s)", c.name, c.got, c.limit, failed, c.fail, msg)
		}
	}
}
