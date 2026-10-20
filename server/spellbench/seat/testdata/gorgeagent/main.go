// Command gorgeagent is a test-only SpellBench v2 agent. It records every
// choose request when SPELLBENCH_RECORD is set and always picks candidate 0.
package main

import (
	"bufio"
	"encoding/json"
	"os"
)

func main() {
	record := os.Getenv("SPELLBENCH_RECORD")
	var f *os.File
	if record != "" {
		f, _ = os.OpenFile(record, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if f != nil {
			defer f.Close()
		}
	}
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for {
		line, err := in.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			return
		}
		var req struct {
			RequestType string `json:"request_type"`
			RequestID   string `json:"request_id"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		if f != nil && req.RequestType == "choose" {
			_, _ = f.Write(append(line, '\n'))
			_ = f.Sync()
		}
		reply := map[string]any{"protocol": "spellbench/v2", "request_id": req.RequestID}
		switch req.RequestType {
		case "hello":
			reply["response_type"] = "hello_ok"
			reply["bot"] = map[string]string{"name": "gorge-test-agent", "version": "1.0.0"}
			reply["extensions_accepted"] = []string{"x_gorge_view_v1"}
		case "choose":
			reply["response_type"] = "choice"
			reply["selection"] = map[string]uint32{"candidate_id": 0}
		default:
			reply["response_type"] = "ack"
		}
		b, _ := json.Marshal(reply)
		_, _ = out.Write(append(b, '\n'))
		_ = out.Flush()
	}
}
