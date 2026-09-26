package main

import (
	jsonv2 "encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
)

var (
	version = "0.4.0-dev"
	commit  = "unknown"
	builtAt = "unknown"
)

type versionInfo struct {
	Version  string `json:"version"`
	Commit   string `json:"commit"`
	BuiltAt  string `json:"builtAt"`
	Go       string `json:"go"`
	GOOS     string `json:"goos"`
	GOARCH   string `json:"goarch"`
	Modified *bool  `json:"modified,omitempty"`
}

func currentVersionInfo() versionInfo {
	info := versionInfo{
		Version: version, Commit: commit, BuiltAt: builtAt,
		Go: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
	}
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "unknown" && setting.Value != "" {
					info.Commit = setting.Value
				}
			case "vcs.time":
				if info.BuiltAt == "unknown" && setting.Value != "" {
					info.BuiltAt = setting.Value
				}
			case "vcs.modified":
				info.Modified = new(setting.Value == "true")
			}
		}
	}
	return info
}

func runVersion(argv []string) int {
	fs := flag.NewFlagSet("epub version", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOutput := fs.Bool("json", false, "以 JSON 输出版本信息")
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, "epub version:", err)
		return 3
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "epub version: unexpected positional arguments")
		return 3
	}
	info := currentVersionInfo()
	if *jsonOutput {
		data, err := jsonv2.Marshal(info)
		if err != nil {
			fmt.Fprintln(os.Stderr, "epub version:", err)
			return 1
		}
		_, _ = os.Stdout.Write(append(data, '\n'))
		return 0
	}
	fmt.Printf("version:  %s\ncommit:   %s\nbuilt at: %s\ngo:       %s\nplatform: %s/%s",
		info.Version, info.Commit, info.BuiltAt, info.Go, info.GOOS, info.GOARCH)
	if info.Modified != nil {
		fmt.Printf("\nmodified: %t", *info.Modified)
	}
	fmt.Println()
	return 0
}
