package report

import (
	"strings"
	"testing"
)

func TestMarshalLegacyPythonShape(t *testing.T) {
	type inner struct {
		A string  `json:"a"`
		B PyFloat `json:"b"`
	}
	type doc struct {
		Zhi    string   `json:"中文键"`
		Items  []string `json:"items"`
		Nested inner    `json:"nested"`
		None   *string  `json:"none"`
	}
	got, err := MarshalLegacy(doc{
		Zhi:    "值",
		Items:  []string{"x", "y"},
		Nested: inner{A: "s", B: 0.8},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"中文键\": \"值\",\n  \"items\": [\n    \"x\",\n    \"y\"\n  ],\n  \"nested\": {\n    \"a\": \"s\",\n    \"b\": 0.8\n  },\n  \"none\": null\n}\n"
	if string(got) != want {
		t.Errorf("legacy JSON 形状不符:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if !strings.HasSuffix(string(got), "\n") {
		t.Error("应与 Python print 一样以单个换行结尾")
	}
}

func TestPyFloat(t *testing.T) {
	cases := map[PyFloat]string{
		PyFloat(1.0):  "1.0",
		PyFloat(0.8):  "0.8",
		PyFloat(0):    "0.0",
		PyFloat(-2.5): "-2.5",
		PyFloat(1e30): "1e+30",
		PyFloat(1e-7): "1e-07",
	}
	for in, want := range cases {
		got, err := in.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("PyFloat(%v) = %s, want %s", float64(in), got, want)
		}
	}
}
