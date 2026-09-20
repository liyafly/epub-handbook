# Capability contracts

`capabilities/v1/` contains the versioned, provider-neutral capability manifests.
`schemas/v1/` contains the input and output JSON Schemas those manifests reference.
Together they define capability identity, safety red lines, dependencies and supported
adapter surfaces; they do not define EPUB policy, which remains in `docs/final/` and
the demo evidence.

The Go CLI consumes these contracts directly: `internal/pipeline/contract.go` loads
and resolves the `requires` chain, `internal/archguard` enforces the red-line closure
(INV-5) and validates that SKILL.md files only reference real capability ids (INV-9).
Adapters must not redefine a capability or make an unregistered provider executable.

`parameters/v2/cli.json` describes public `KEY=VALUE` arguments, with its shape in
`schemas/v2/parameter-catalog.schema.json`. It is a companion catalog, not a change
to the frozen v1 manifests. `epub capabilities --id <id> --json` projects the
capability's direct parameters plus common parameters; `requires` identifies the
upstream contracts. Execution accepts the union of parameters along that chain
and rejects unknown keys, malformed types and missing required values before
opening the input. Global flags still control input/output/dry-run. Defaults are
documented CLI strings; runners apply them, and tests validate their types.

Validate the contract graph with:

```sh
go test ./internal/archguard/ -v
go test ./internal/pipeline/
```
