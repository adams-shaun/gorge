package main

import "fmt"

// Alert thresholds (brief: traindash, view 4).
const (
	// belowControlMargin: an eval win rate below control minus this fires.
	belowControlMargin = 0.05
	// dropMargin: an eval win rate this far below the previous round's fires.
	dropMargin = 0.08
	// klMultiple: a kind's final_kl above this multiple of -ppo-kl fires.
	klMultiple = 3.0
)

// Alert is one rule firing on one round of one run.
type Alert struct {
	Run       string  `json:"run"`
	RunStatus string  `json:"run_status"`
	Round     int     `json:"round"`
	Rule      string  `json:"rule"` // below_control, drop, kl
	Kind      string  `json:"kind,omitempty"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Message   string  `json:"message"`
	// Latest marks an alert on the run's newest evaluated/trained round, i.e.
	// one that is still the run's current state rather than history.
	Latest bool `json:"latest"`
}

// RunAlerts evaluates every alert rule over every round of r. r.ID and
// r.Status must already be set.
// An adhoc run has no win rate or KL target to alert on and never alerts.
func RunAlerts(r *Run) []Alert {
	if r.Kind == KindAdhoc {
		return nil
	}
	var out []Alert
	latest := -1
	for _, rd := range r.Rounds {
		if rd.Eval != nil || rd.Stats != nil {
			latest = rd.N
		}
	}
	add := func(a Alert) {
		a.Run, a.RunStatus, a.Latest = r.ID, r.Status, a.Round == latest
		out = append(out, a)
	}
	var control *float64
	if r.Control != nil {
		control = r.Control.WinRate
	}
	var prev *float64
	for _, rd := range r.Rounds {
		if rd.Stats != nil && r.KLTarget != nil {
			lim := klMultiple * *r.KLTarget
			for _, k := range rd.Stats.Kinds {
				if k.FinalKL != nil && *k.FinalKL > lim {
					add(Alert{Round: rd.N, Rule: "kl", Kind: k.Kind, Value: *k.FinalKL, Threshold: lim,
						Message: fmt.Sprintf("r%d %s final_kl %.3f > %.0f× target %.3g", rd.N, k.Kind, *k.FinalKL, klMultiple, *r.KLTarget)})
				}
			}
		}
		if rd.Eval == nil || rd.Eval.WinRate == nil {
			continue
		}
		wr := *rd.Eval.WinRate
		if control != nil && wr < *control-belowControlMargin {
			add(Alert{Round: rd.N, Rule: "below_control", Value: wr, Threshold: *control - belowControlMargin,
				Message: fmt.Sprintf("r%d eval %.3f < control %.3f − %.2f", rd.N, wr, *control, belowControlMargin)})
		}
		if prev != nil && *prev-wr > dropMargin {
			add(Alert{Round: rd.N, Rule: "drop", Value: wr, Threshold: *prev - dropMargin,
				Message: fmt.Sprintf("r%d eval %.3f dropped %.3f from %.3f (> %.2f)", rd.N, wr, *prev-wr, *prev, dropMargin)})
		}
		w := wr
		prev = &w
	}
	return out
}
