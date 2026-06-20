import EPUBContracts
import EPUBRuntime
import Testing

@Test("provider policy keeps agents on Python and GUI on Swift")
func providerPolicySelectsSurfaceSpecificImplementations() async throws {
    let capability = CapabilityID(rawValue: "epub.notes.popup-normalize")
    let registry = CapabilityRegistry()

    await registry.register(.init(capability: capability, provider: .python, version: "1.0.0"))
    await registry.register(.init(capability: capability, provider: .swift, version: "1.0.0"))

    #expect(await registry.resolve(capability, for: .agent)?.provider == .python)
    #expect(await registry.resolve(capability, for: .legacyCLI)?.provider == .python)
    #expect(await registry.resolve(capability, for: .swiftCLI)?.provider == .swift)
    #expect(await registry.resolve(capability, for: .gui)?.provider == .swift)
    #expect(await registry.resolveAll(capability, for: .parity).map(\.provider) == [.python, .swift])
}
