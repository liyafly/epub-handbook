import EPUBContracts

public enum ProviderKind: String, CaseIterable, Codable, Sendable {
    case python
    case swift
}

public enum ProviderSurface: String, CaseIterable, Codable, Sendable {
    case agent
    case legacyCLI = "legacy-cli"
    case swiftCLI = "swift-cli"
    case gui
    case parity
}

public struct CapabilityProvider: Hashable, Codable, Sendable {
    public let capability: CapabilityID
    public let provider: ProviderKind
    public let version: String

    public init(capability: CapabilityID, provider: ProviderKind, version: String) {
        self.capability = capability
        self.provider = provider
        self.version = version
    }
}

public actor CapabilityRegistry {
    private var providers: [CapabilityID: Set<CapabilityProvider>] = [:]

    public init() {}

    public func register(_ provider: CapabilityProvider) {
        providers[provider.capability, default: []].insert(provider)
    }

    public func resolve(_ capability: CapabilityID, for surface: ProviderSurface) -> CapabilityProvider? {
        resolveAll(capability, for: surface).first
    }

    public func resolveAll(_ capability: CapabilityID, for surface: ProviderSurface) -> [CapabilityProvider] {
        let available = providers[capability, default: []]
        return providerPreference(for: surface).compactMap { kind in
            available
                .filter { $0.provider == kind }
                .sorted { lhs, rhs in lhs.version > rhs.version }
                .first
        }
    }

    private func providerPreference(for surface: ProviderSurface) -> [ProviderKind] {
        switch surface {
        case .agent, .legacyCLI:
            [.python, .swift]
        case .swiftCLI, .gui:
            [.swift]
        case .parity:
            [.python, .swift]
        }
    }
}
