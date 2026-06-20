import Foundation
import EPUBContracts

public enum TransactionGateState: String, CaseIterable, Codable, Sendable {
    case pending
    case passed
    case failed
}

public enum TransactionError: Error, Equatable, Sendable {
    case inputMissing
    case inputAlreadyCaptured
    case captureInputRequired
    case unsafeFilename
    case stagedOutputMissing
    case pendingGates([String])
    case failedGates([String])
    case unknownGate(String)
    case destinationAlreadyExists
    case finished
}

/// A staged filesystem transaction for a single artifact transformation.
/// Inputs are copied to `before/`; generated data is staged separately and
/// copied to a new destination only after every required gate has passed.
public actor Transaction {
    public let workspace: Workspace

    private let beforeDirectory: URL
    private let stagingDirectory: URL
    private var gateStates: [String: TransactionGateState]
    private let gateOrder: [String]
    private var inputArtifact: ArtifactReference?
    private var stagedOutput: URL?
    private var isFinished = false
    private var auditEvents: [RunEvent] = []

    public init(workspace: Workspace, requiredGateIDs: [String]) throws {
        self.workspace = workspace
        beforeDirectory = workspace.before ?? workspace.root.appending(path: "before", directoryHint: .isDirectory)
        stagingDirectory = workspace.staging ?? workspace.root.appending(path: "staging", directoryHint: .isDirectory)
        gateOrder = requiredGateIDs
        gateStates = Dictionary(uniqueKeysWithValues: requiredGateIDs.map { ($0, .pending) })
        try FileManager.default.createDirectory(at: beforeDirectory, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: stagingDirectory, withIntermediateDirectories: true)
    }

    public func captureInput(_ artifact: ArtifactReference) throws -> ArtifactReference {
        try assertActive()
        guard artifact.uri.isFileURL, FileManager.default.fileExists(atPath: artifact.uri.path) else {
            throw TransactionError.inputMissing
        }
        guard inputArtifact == nil else {
            throw TransactionError.inputAlreadyCaptured
        }
        let ext = artifact.uri.pathExtension
        let baselineName = ext.isEmpty ? "source" : "source.\(ext)"
        let baseline = beforeDirectory.appending(path: baselineName)
        try FileManager.default.copyItem(at: artifact.uri, to: baseline)
        inputArtifact = artifact
        auditEvents.append(.init(step: "capture-input", status: .completed, message: baseline.path))
        return ArtifactReference(
            uri: baseline,
            kind: artifact.kind,
            contentDigest: artifact.contentDigest,
            logicalPath: baselineName
        )
    }

    public func stage(_ data: Data, filename: String) throws -> URL {
        try assertActive()
        guard isSafeFilename(filename) else {
            throw TransactionError.unsafeFilename
        }
        let target = stagingDirectory.appending(path: filename)
        try data.write(to: target, options: .atomic)
        stagedOutput = target
        auditEvents.append(.init(step: "stage-output", status: .completed, message: target.path))
        return target
    }

    public func passGate(_ gateID: String) throws {
        try assertActive()
        guard gateStates[gateID] != nil else {
            throw TransactionError.unknownGate(gateID)
        }
        gateStates[gateID] = .passed
        auditEvents.append(.init(step: gateID, status: .completed, message: "gate passed"))
    }

    public func failGate(_ gateID: String, message: String? = nil) throws {
        try assertActive()
        guard gateStates[gateID] != nil else {
            throw TransactionError.unknownGate(gateID)
        }
        gateStates[gateID] = .failed
        auditEvents.append(.init(step: gateID, status: .failed, message: message ?? "gate failed"))
    }

    public func commit(to destination: URL) throws -> ArtifactReference {
        try assertActive()
        guard inputArtifact != nil else {
            throw TransactionError.captureInputRequired
        }
        let failed = gateOrder.filter { gateStates[$0] == .failed }
        guard failed.isEmpty else {
            throw TransactionError.failedGates(failed)
        }
        let pending = gateOrder.filter { gateStates[$0] != .passed }
        guard pending.isEmpty else {
            throw TransactionError.pendingGates(pending)
        }
        guard let stagedOutput else {
            throw TransactionError.stagedOutputMissing
        }
        guard !FileManager.default.fileExists(atPath: destination.path) else {
            throw TransactionError.destinationAlreadyExists
        }
        try FileManager.default.createDirectory(at: destination.deletingLastPathComponent(), withIntermediateDirectories: true)
        try FileManager.default.copyItem(at: stagedOutput, to: destination)
        isFinished = true
        auditEvents.append(.init(step: "commit", status: .completed, message: destination.path))
        return ArtifactReference(uri: destination, kind: inputArtifact?.kind ?? .unknown)
    }

    public func rollback() throws {
        try assertActive()
        if FileManager.default.fileExists(atPath: stagingDirectory.path) {
            try FileManager.default.removeItem(at: stagingDirectory)
        }
        isFinished = true
        auditEvents.append(.init(step: "rollback", status: .completed, message: "staged output removed"))
    }

    public func gateState(for gateID: String) -> TransactionGateState? {
        gateStates[gateID]
    }

    public func events() -> [RunEvent] {
        auditEvents
    }

    private func assertActive() throws {
        if isFinished {
            throw TransactionError.finished
        }
    }

    private func isSafeFilename(_ filename: String) -> Bool {
        !filename.isEmpty &&
            !filename.contains("/") &&
            !filename.contains("\\") &&
            filename != "." &&
            filename != ".."
    }
}
