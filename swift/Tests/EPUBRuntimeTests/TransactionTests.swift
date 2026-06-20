import Foundation
import EPUBContracts
import EPUBRuntime
import Testing

@Test("transaction captures input and refuses commit until every required gate passes")
func transactionRequiresAllGatesBeforeCommit() async throws {
    let root = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString, directoryHint: .isDirectory)
    defer { try? FileManager.default.removeItem(at: root) }
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    let input = root.appending(path: "source.epub")
    let output = root.appending(path: "cleaned.epub")
    try Data("before".utf8).write(to: input)

    let transaction = try Transaction(
        workspace: .init(root: root),
        requiredGateIDs: ["preflight", "redline"]
    )
    let artifact = ArtifactReference(uri: input, kind: .epub)
    _ = try await transaction.captureInput(artifact)
    let staged = try await transaction.stage(Data("after".utf8), filename: "cleaned.epub")

    do {
        _ = try await transaction.commit(to: output)
        Issue.record("commit must be blocked while required gates are pending")
    } catch let error as TransactionError {
        #expect(error == .pendingGates(["preflight", "redline"]))
    }

    try await transaction.passGate("preflight")
    try await transaction.passGate("redline")
    let result = try await transaction.commit(to: output)

    #expect(result.uri == output)
    #expect(try Data(contentsOf: output) == Data("after".utf8))
    #expect(try Data(contentsOf: root.appending(path: "before/source.epub")) == Data("before".utf8))
    #expect(FileManager.default.fileExists(atPath: staged.path))
}

@Test("transaction rollback removes staged output but preserves captured input")
func transactionRollbackPreservesBeforeBaseline() async throws {
    let root = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString, directoryHint: .isDirectory)
    defer { try? FileManager.default.removeItem(at: root) }
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    let input = root.appending(path: "source.epub")
    try Data("before".utf8).write(to: input)

    let transaction = try Transaction(workspace: .init(root: root), requiredGateIDs: [])
    _ = try await transaction.captureInput(.init(uri: input, kind: .epub))
    let staged = try await transaction.stage(Data("after".utf8), filename: "cleaned.epub")
    try await transaction.rollback()

    #expect(!FileManager.default.fileExists(atPath: staged.path))
    #expect(try Data(contentsOf: root.appending(path: "before/source.epub")) == Data("before".utf8))
}
