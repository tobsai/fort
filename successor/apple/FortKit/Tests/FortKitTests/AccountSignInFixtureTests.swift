#if DEBUG && os(macOS)
import Foundation
import Testing
@testable import FortKit

@Test @MainActor func accountInspectionFixtureUsesOnlyExplicitSyntheticOperations() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("FortAccountQA-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let model = try await AccountSignInFixture.make(directory:directory)
    #expect(model.runtimeName == "Synthetic account fixture · no provider")
    #expect(model.canManageProviderAccount)
    #expect(model.accountSignInStatus == .idle(0))
    await model.startAccountSignIn()
    let operation = try #require(model.accountSignInStatus?.operation)
    #expect(model.accountSignInStatus?.challenge?.userCode == "FORT-TEST")
    let reopened = try await AccountSignInFixture.make(directory:directory)
    #expect(reopened.accountSignInStatus == .finished(operation,.unconfirmed))
    #expect(!reopened.hasPendingAccountSignIn)
    await reopened.startAccountSignIn()
    #expect(reopened.accountSignInStatus?.operation?.revision == operation.revision + 1)
    await reopened.cancelAccountSignIn()
    #expect(reopened.accountSignInStatus?.finishedResult == .canceled)
    for file in try FileManager.default.contentsOfDirectory(at:directory,includingPropertiesForKeys:nil) where file.pathExtension == "json" {
        let text = String(decoding:try Data(contentsOf:file),as:UTF8.self)
        #expect(!text.contains("FORT-TEST")); #expect(!text.contains("verificationURL")); #expect(!text.contains("synthetic-account-token"))
    }
}
@Test @MainActor func accountInspectionFixtureRefusesUnscopedTemporaryStorage() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("unrelated-" + UUID().uuidString)
    await #expect(throws:(any Error).self) { try await AccountSignInFixture.make(directory:directory) }
    #expect(!FileManager.default.fileExists(atPath:directory.path))
}
@Test @MainActor func accountFixturesCoverFirstExistingMismatchAndProviderAmbiguity() async throws {
    for scenario in [AccountFixtureScenario.first,.existing,.mismatch,.unverified] {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("FortAccountQA-" + UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:directory) }
        let model = try await AccountSignInFixture.make(directory:directory,scenario:scenario)
        if scenario == .first { await model.startAccountSignIn() }
        let expected = scenario == .first || scenario == .existing
        #expect((model.accountSignInStatus?.accountSelected == true) == expected)
        #expect(model.modelCatalog.canSelect("fixture-model") == expected)
        if scenario == .mismatch {
            #expect(model.modelAccountRequirement?.contains("Advanced account management") == true)
            await model.selectReportedAccount()
            #expect(model.accountSignInStatus?.reportedAccount?.accountId == "different-workspace")
            #expect(model.modelCatalog.canSelect("fixture-model"))
        }
        if scenario == .unverified {
            #expect(model.accountSignInStatus?.reportedAccount == nil)
            #expect(model.accountSignInMessage.contains("workspace choice"))
        }
    }
}
#endif
