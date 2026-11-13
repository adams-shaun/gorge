package oracle

import (
	"reflect"
	"testing"
)

// TestPinConfigMirrorsConfig keeps pinConfig, the JSON-decodable copy of
// Config a pin document records, from drifting: every Config field has a
// same-named, same-typed pinConfig field, pinConfig adds none of its own, and
// pinConfig.config copies each one across.
func TestPinConfigMirrorsConfig(t *testing.T) {
	ct, pt := reflect.TypeOf(Config{}), reflect.TypeOf(pinConfig{})
	if ct.NumField() == 0 {
		t.Fatal("Config has no fields: the mirror check would be vacuous")
	}
	if ct.NumField() != pt.NumField() {
		t.Fatalf("Config has %d fields, pinConfig %d: a pin document cannot record every lockstep Config field", ct.NumField(), pt.NumField())
	}
	var pc pinConfig
	pv := reflect.ValueOf(&pc).Elem()
	for i := 0; i < ct.NumField(); i++ {
		cf := ct.Field(i)
		pf, ok := pt.FieldByName(cf.Name)
		if !ok {
			t.Fatalf("pinConfig has no field %s mirroring Config.%s", cf.Name, cf.Name)
		}
		if pf.Type != cf.Type || cf.Type.Kind() != reflect.Bool {
			t.Fatalf("Config.%s is %v, pinConfig.%s is %v: the mirror needs matching bool fields", cf.Name, cf.Type, pf.Name, pf.Type)
		}
		if pf.Tag.Get("json") == "" {
			t.Fatalf("pinConfig.%s has no json tag", pf.Name)
		}
		pv.FieldByName(cf.Name).SetBool(true)
	}
	got := reflect.ValueOf(pc.config())
	for i := 0; i < ct.NumField(); i++ {
		if !got.Field(i).Bool() {
			t.Errorf("pinConfig.config() does not copy %s", ct.Field(i).Name)
		}
	}
}
