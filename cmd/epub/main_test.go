package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunCapabilityUsageErrorsHonorJSON(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{name: "missing capability", argv: []string{"--json"}, want: "缺少 capability-id"},
		{name: "malformed key value", argv: []string{"epub.package.nav.audit", "--json", "malformed"}, want: "KEY=VALUE"},
		{name: "json after key value", argv: []string{"epub.package.nav.audit", "mode=x", "--json"}, want: "before KEY=VALUE"},
		{name: "numeric json boolean", argv: []string{"--json=1"}, want: "缺少 capability-id"},
		{name: "flag parse error", argv: []string{"epub.package.nav.audit", "--json", "--unknown"}, want: "flag provided"},
		{name: "duplicate parameter", argv: []string{"epub.typography.optimize", "--json", "preset=one", "preset=two"}, want: "重复参数"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, _ := captureRunCapability(t, tc.argv)
			if code != 3 {
				t.Fatalf("exit = %d, want 3", code)
			}
			var env struct {
				SchemaVersion string `json:"schemaVersion"`
				Status        string `json:"status"`
				Findings      []struct {
					Detail string `json:"detail"`
				} `json:"findings"`
			}
			if err := json.Unmarshal([]byte(stdout), &env); err != nil {
				t.Fatalf("stdout is not an envelope: %v\n%s", err, stdout)
			}
			if env.SchemaVersion != "2" || env.Status != "failed" || len(env.Findings) != 1 {
				t.Fatalf("unexpected envelope: %#v", env)
			}
			if !strings.Contains(env.Findings[0].Detail, tc.want) {
				t.Fatalf("detail = %q, want substring %q", env.Findings[0].Detail, tc.want)
			}
		})
	}
}

func captureRunCapability(t *testing.T, argv []string) (code int, stdout, stderr string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	originalOut, originalErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() {
		os.Stdout, os.Stderr = originalOut, originalErr
	}()

	code = runCapability(argv)
	if err := outW.Close(); err != nil {
		t.Fatal(err)
	}
	if err := errW.Close(); err != nil {
		t.Fatal(err)
	}
	outBytes, err := io.ReadAll(outR)
	if err != nil {
		t.Fatal(err)
	}
	errBytes, err := io.ReadAll(errR)
	if err != nil {
		t.Fatal(err)
	}
	if err := outR.Close(); err != nil {
		t.Fatal(err)
	}
	if err := errR.Close(); err != nil {
		t.Fatal(err)
	}
	return code, string(outBytes), string(errBytes)
}
