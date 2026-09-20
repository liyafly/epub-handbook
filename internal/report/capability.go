package report

// Parameter describes KEY=VALUE strings accepted by the public CLI. Default
// and enum values are strings even for typed parameters: they are copyable
// CLI arguments, not decoded JSON values.
type Parameter struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Default     *string  `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Minimum     *int     `json:"minimum,omitempty"`
}

type CapabilityDescription struct {
	Description string               `json:"description"`
	Parameters  map[string]Parameter `json:"parameters"`
}

type ParameterCatalog struct {
	SchemaVersion    string                           `json:"schemaVersion"`
	CommonParameters map[string]Parameter             `json:"commonParameters"`
	Capabilities     map[string]CapabilityDescription `json:"capabilities"`
}

type CapabilityExecution struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

type CapabilityPermissions struct {
	RequiresWriteAccess bool   `json:"requiresWriteAccess"`
	Network             string `json:"network"`
}

// CapabilityInfo preserves the existing capabilities array and adds discovery
// details without changing any v1 capability manifest or schema.
type CapabilityInfo struct {
	ID          string                `json:"id"`
	Kind        string                `json:"kind"`
	Implemented bool                  `json:"implemented"`
	RedLines    []string              `json:"redLines,omitempty"`
	Requires    []string              `json:"requires,omitempty"`
	Description string                `json:"description"`
	Execution   CapabilityExecution   `json:"execution"`
	Permissions CapabilityPermissions `json:"permissions"`
	Parameters  map[string]Parameter  `json:"parameters"`
}
