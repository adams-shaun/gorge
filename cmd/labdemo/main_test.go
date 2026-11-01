package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab/extern/refagent"
)

// TestMain re-executes this test binary as the external agent child when
// GORGE_EXTAGENT_HELPER is set (no helper binary is ever built): the value
// is a JSON refagent.Config subset the served reference agent reads, over
// its stdio. The parent tests set the variable with t.Setenv (the child
// inherits the environment) and dial ext:exec: + os.Args[0].
func TestMain(m *testing.M) {
	v := os.Getenv("GORGE_EXTAGENT_HELPER")
	if v == "" {
		os.Exit(m.Run())
	}
	var c helperConfig
	if err := json.Unmarshal([]byte(v), &c); err != nil {
		fmt.Fprintf(os.Stderr, "GORGE_EXTAGENT_HELPER: not a JSON refagent config: %v\n", err)
		os.Exit(1)
	}
	_ = refagent.Serve(os.Stdin, os.Stdout, c.refagentConfig())
	os.Exit(0)
}

// helperConfig is the JSON shape GORGE_EXTAGENT_HELPER accepts, the
// refagent.Config subset labdemo's tests need.
type helperConfig struct {
	Policy       string   `json:"policy"`
	ModelVersion string   `json:"model_version"`
	Agent        string   `json:"agent"`
	ViewOmit     []string `json:"view_omit"`
	MaxBatch     int      `json:"max_batch"`
}

// refagentConfig maps the helper config onto a served refagent.Config.
func (c helperConfig) refagentConfig() refagent.Config {
	return refagent.Config{
		Policy:       c.Policy,
		ModelVersion: c.ModelVersion,
		Agent:        c.Agent,
		ViewOmit:     c.ViewOmit,
		MaxBatch:     c.MaxBatch,
	}
}
