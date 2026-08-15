# Capability contracts

`capabilities/v1/` contains the versioned, provider-neutral capability manifests.
`schemas/v1/` contains the input and output JSON Schemas those manifests reference.
Together they define capability identity, safety red lines, dependencies and supported
adapter surfaces; they do not define EPUB policy, which remains in `docs/final/` and
the demo evidence.

`../adapters/` consumes these contracts to publish provider-specific catalogs and
public-entrypoint inventories. Adapters must not redefine a capability or make an
unregistered provider executable.

Validate the contract graph and Python entrypoint inventory with:

```sh
python3 scripts/validate_contracts.py
python3 scripts/validate_python_entrypoint_inventory.py
```
