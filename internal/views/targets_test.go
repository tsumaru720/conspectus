package views

import (
	"reflect"
	"testing"
)

func TestParseTargetsInput(t *testing.T) {
	for in, want := range map[string][]float64{
		"":                nil,
		"   ":             nil,
		"1000":            {1000},
		"1000, 2000":      {1000, 2000},
		" 1000 ,  2000  ": {1000, 2000},
		"[1000, 2000]":    {1000, 2000},
		"1000,":           {1000},
		"1000,,2000":      {1000, 2000},
	} {
		got, err := parseTargetsInput(in)
		if err != nil {
			t.Errorf("parseTargetsInput(%q): %v", in, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("parseTargetsInput(%q) = %v, want %v", in, got, want)
		}
	}
	for _, bad := range []string{"abc", "1000, x", "0", "-5", "1000, 0"} {
		if _, err := parseTargetsInput(bad); err != errInvalidTargets {
			t.Errorf("parseTargetsInput(%q) error = %v, want errInvalidTargets", bad, err)
		}
	}
}

func TestMergeAndMarshalTargets(t *testing.T) {
	got := mergeTargets([]float64{5000, 1000}, []float64{2000, 1000, -3})
	if want := []float64{1000, 2000, 5000}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mergeTargets = %v, want %v", got, want)
	}
	s, err := marshalTargets([]float64{2000, 1000})
	if err != nil {
		t.Fatal(err)
	}
	if s != "[1000,2000]" {
		t.Fatalf("marshalTargets = %q, want %q", s, "[1000,2000]")
	}
	if s, _ := marshalTargets(nil); s != "" {
		t.Fatalf("empty list must marshal to \"\", got %q", s)
	}
	if got := parseStoredTargets("[1000,2000]"); !reflect.DeepEqual(got, []float64{1000, 2000}) {
		t.Fatalf("parseStoredTargets = %v", got)
	}
	if got := parseStoredTargets(""); got != nil {
		t.Fatalf("empty stored value must parse to nil, got %v", got)
	}
}
