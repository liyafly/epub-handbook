package pipeline

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/liyafly/epub-handbook/internal/report"
)

func loadParameterCatalog(root string) (report.ParameterCatalog, error) {
	var catalog report.ParameterCatalog
	raw, err := os.ReadFile(filepath.Join(root, "contracts/parameters/v2/cli.json"))
	if err != nil {
		return catalog, fmt.Errorf("parameter catalog: %w", err)
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return catalog, err
	}
	if catalog.SchemaVersion != "2" || catalog.Capabilities == nil {
		return catalog, fmt.Errorf("invalid parameter catalog")
	}
	return catalog, nil
}

// DescribeCapabilities resolves implementation status and public contracts.
// An optional id keeps agent discovery small while retaining the array shape.
func DescribeCapabilities(root, id string) ([]report.CapabilityInfo, error) {
	contracts, err := AllContracts(root)
	if err != nil {
		return nil, err
	}
	catalog, err := loadParameterCatalog(root)
	if err != nil {
		return nil, err
	}
	out := make([]report.CapabilityInfo, 0, len(contracts))
	for _, c := range contracts {
		if id != "" && c.ID != id {
			continue
		}
		desc, ok := catalog.Capabilities[c.ID]
		if !ok {
			return nil, fmt.Errorf("missing parameter contract for %s", c.ID)
		}
		parameters := maps.Clone(catalog.CommonParameters)
		if parameters == nil {
			parameters = map[string]report.Parameter{}
		}
		maps.Copy(parameters, desc.Parameters)
		out = append(out, report.CapabilityInfo{ID: c.ID, Kind: c.Kind, Implemented: Implemented(c.ID), RedLines: c.RedLines, Requires: c.Requires, Description: desc.Description, Execution: c.Execution, Permissions: c.Permissions, Parameters: parameters})
	}
	if id != "" && len(out) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrUnknownCapability, id)
	}
	return out, nil
}

func validateArguments(root string, chain []Contract, args Args) error {
	catalog, err := loadParameterCatalog(root)
	if err != nil {
		return err
	}
	parameters := maps.Clone(catalog.CommonParameters)
	if parameters == nil {
		parameters = map[string]report.Parameter{}
	}
	for _, c := range chain {
		desc, ok := catalog.Capabilities[c.ID]
		if !ok {
			return fmt.Errorf("missing parameter contract for %s", c.ID)
		}
		maps.Copy(parameters, desc.Parameters)
	}
	for _, key := range slices.Sorted(maps.Keys(args)) {
		// These legacy keys cannot override global flags (Run overwrites them).
		if key == "input" || key == "output" || key == "dry_run" {
			continue
		}
		spec, ok := parameters[key]
		if !ok {
			return fmt.Errorf("unknown parameter %q; inspect epub capabilities --id %s --json", key, chain[len(chain)-1].ID)
		}
		if err := validateParameter(spec, args[key]); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}
	for _, key := range slices.Sorted(maps.Keys(parameters)) {
		if parameters[key].Required && strings.TrimSpace(args[key]) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	return nil
}

func validateParameter(spec report.Parameter, value string) error {
	if len(spec.Enum) > 0 && !slices.Contains(spec.Enum, value) {
		return fmt.Errorf("must be one of %v", spec.Enum)
	}
	checkInt := func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("must be an integer, got %q", v)
		}
		if spec.Minimum != nil && n < *spec.Minimum {
			return fmt.Errorf("must be >= %d", *spec.Minimum)
		}
		return nil
	}
	switch spec.Type {
	case "string":
		return nil
	case "boolean":
		switch strings.ToLower(value) {
		case "true", "false", "1", "0", "yes", "no", "on", "off":
			return nil
		}
		return fmt.Errorf("must be a boolean, got %q", value)
	case "integer":
		return checkInt(value)
	case "integer-list":
		for _, part := range strings.Split(value, ",") {
			if err := checkInt(strings.TrimSpace(part)); err != nil {
				return err
			}
		}
		return nil
	case "json-string-array":
		var values []string
		if err := json.Unmarshal([]byte(value), &values); err != nil || len(values) == 0 {
			return fmt.Errorf("must be a nonempty JSON string array")
		}
		for _, v := range values {
			if v == "" {
				return fmt.Errorf("array entries must not be empty")
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown parameter contract type %q", spec.Type)
	}
}
