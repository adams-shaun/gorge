package gonet_test

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
)

func TestGonetRefusesExtNet(t *testing.T) {
	sh := gonettest.Small
	p, err := gonettest.WriteExt(t.TempDir(), "ext.dzgw", sh, 512, make([]float32, sh.D*512))
	if err != nil {
		t.Fatal(err)
	}
	_, err = gonet.Load(p)
	if err == nil || !strings.Contains(err.Error(), "ext nets (EXT_W=512) need netsimd") {
		t.Fatalf("Load = %v, want the ext refusal", err)
	}
}
