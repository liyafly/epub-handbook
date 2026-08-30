// Package report 构造并序列化 run-report（SPEC §1 第 3 层）。
//
// 它是唯一允许构造对外 JSON 的地方：
//   - Envelope（schemaVersion 2）：CLI 的统一返回信封（SPEC §8.2）；
//   - V1RunReport：迁移期 golden 报告，受 contracts/schemas/v1 约束（INV-6）；
//   - MarshalLegacy：与 Python json.dumps(ensure_ascii=False, indent=2)
//     逐字节兼容的序列化，供 --legacy-report parity 脚手架使用。
package report

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
)

// 状态值（信封与 v1 报告共用语义）。
const (
	StatusComplete         = "complete"
	StatusFailed           = "failed"
	StatusCancelled        = "cancelled"
	StatusApprovalRequired = "approval-required"
)

// Finding 是 findings[] 的统一收口。旧的 lint 数组与纯文本行都归到这里。
type Finding struct {
	Level    string `json:"level"` // error | warn | info
	ID       string `json:"id"`
	Title    string `json:"title"`
	Detail   string `json:"detail,omitempty"`
	Location string `json:"location,omitempty"`
}

// Event 是一次运行的步骤记录。
type Event struct {
	Step    string `json:"step"`
	Status  string `json:"status"` // started | completed | failed | skipped
	Message string `json:"message,omitempty"`
}

// Artifact 是信封里的输入/输出引用。
type Artifact struct {
	Path   string `json:"path,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

// Envelope 是所有命令的统一返回形状（SPEC §8.2）。
type Envelope struct {
	SchemaVersion string         `json:"schemaVersion"` // "2"
	Capability    string         `json:"capability"`
	Status        string         `json:"status"`
	Input         *Artifact      `json:"input,omitempty"`
	Output        *Artifact      `json:"output,omitempty"`
	Facts         map[string]any `json:"facts,omitempty"`
	Findings      []Finding      `json:"findings,omitempty"`
	Events        []Event        `json:"events,omitempty"`
	NextCommands  []string       `json:"nextCommands,omitempty"`
}

// Result 是一个 capability 的三段式产物（扫描 → 应用 → 报告）的报告段。
// 不落盘：信封与 v1 报告都由 pipeline 借助本包构造。
type Result struct {
	Capability   string
	Status       string
	Facts        map[string]any
	Findings     []Finding
	Events       []Event
	NextCommands []string
	// Renames 记录 entry 改名（from→to），pipeline 汇总后交给 redline 的 path map。
	Renames map[string]string
}

// Envelope 把 Result 装入统一信封。
func (r Result) Envelope() Envelope {
	return Envelope{
		SchemaVersion: "2",
		Capability:    r.Capability,
		Status:        r.Status,
		Facts:         r.Facts,
		Findings:      r.Findings,
		Events:        r.Events,
		NextCommands:  r.NextCommands,
	}
}

// ---- v1 run-report（golden / INV-6）----

// V1ArtifactRef 对齐 contracts/schemas/v1/artifact-reference.schema.json。
type V1ArtifactRef struct {
	URI           string `json:"uri"`
	Kind          string `json:"kind"` // epub | source-directory | markdown | html | pdf | image-set | unknown
	ContentDigest string `json:"contentDigest,omitempty"`
	LogicalPath   string `json:"logicalPath,omitempty"`
}

// V1Event 对齐 run-report.schema.json 的 events 元素。
type V1Event struct {
	Step    string `json:"step"`
	Status  string `json:"status"` // started | completed | failed | skipped
	Message string `json:"message,omitempty"`
}

// V1RunReport 对齐 contracts/schemas/v1/run-report.schema.json。
type V1RunReport struct {
	SchemaVersion string         `json:"schemaVersion"` // 恒为 "1"
	Status        string         `json:"status"`
	Input         V1ArtifactRef  `json:"input"`
	Output        *V1ArtifactRef `json:"output,omitempty"`
	Events        []V1Event      `json:"events"`
}

// ---- Python 兼容序列化 ----

// MarshalLegacy 以 Python `json.dumps(v, ensure_ascii=False, indent=2)` 的
// 形状序列化：UTF-8 原样输出、两空格缩进、不转义 <>&、键按结构体声明序。
func MarshalLegacy(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// json.Encoder 结尾自带一个 \n，与 Python print(dumps(...)) 一致。
	return buf.Bytes(), nil
}

// PyFloat 按 Python 的 float repr 序列化（1.0 → "1.0"，0.8 → "0.8"）。
type PyFloat float64

// MarshalJSON 实现 Python 风格的浮点字面量。
func (f PyFloat) MarshalJSON() ([]byte, error) {
	v := float64(f)
	switch {
	case math.IsNaN(v):
		return []byte("NaN"), nil
	case math.IsInf(v, 1):
		return []byte("Infinity"), nil
	case math.IsInf(v, -1):
		return []byte("-Infinity"), nil
	}
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if !bytes.ContainsAny([]byte(s), ".eE") {
		s += ".0"
	}
	// Python 对大指数用 1e+30 形式，FormatFloat 的 'g' 已一致；负指数同理。
	return []byte(s), nil
}
