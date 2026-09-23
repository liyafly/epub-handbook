package pipeline

import "github.com/liyafly/epub-handbook/internal/report"

// MarshalEnvelope serializes the CLI's public JSON through the report package
// while keeping cmd/epub dependent only on this orchestration layer.
func MarshalEnvelope(envelope any) ([]byte, error) {
	return report.MarshalLegacy(envelope)
}
