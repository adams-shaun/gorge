package mtgish

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// LoadAll reads every card of an mtgish.lines.json file in file order, with
// no filter and no dedupe. The index needs the order, which Load's map loses.
func LoadAll(path string) ([]*Card, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadAll(f)
}

// ReadAll is LoadAll over a reader.
func ReadAll(r io.Reader) ([]*Card, error) {
	br := bufio.NewReaderSize(r, 1<<20)
	var out []*Card
	lineNo := 0
	for {
		line, err := br.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			lineNo++
			var m map[string]any
			if jerr := json.Unmarshal(line, &m); jerr != nil {
				return nil, fmt.Errorf("mtgish: line %d: %v", lineNo, jerr)
			}
			if c, ok := decodeCard(m); ok {
				out = append(out, c)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
