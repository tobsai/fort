import Foundation
import Testing
@testable import FortKit

private func sharedSnapshotObject() throws -> [String:Any] {
    let file = URL(fileURLWithPath:#filePath).deletingLastPathComponent().appendingPathComponent("../../../../contracts/fixtures/snapshot.json")
    return try #require(JSONSerialization.jsonObject(with:Data(contentsOf:file)) as? [String:Any])
}
private func decodeSnapshot(_ values: [String:Any]) throws -> Snapshot {
    try JSONDecoder().decode(Snapshot.self,from:JSONSerialization.data(withJSONObject:values))
}
@Test func unsupportedContractAndRuntimeRemainReadableButCannotSubmit() throws {
    for (version,mode) in [("2.0","fixture"),("1.1","fixture"),("1.0","future-provider")] {
        let json = #"{"contractVersion":"VERSION","installationId":"fixture","recoveryEpoch":"epoch","mode":"MODE","serviceName":"Future Fort","extraCapability":{"ignored":true}}"#.replacingOccurrences(of:"VERSION",with:version).replacingOccurrences(of:"MODE",with:mode)
        let installation = try JSONDecoder().decode(Installation.self,from:Data(json.utf8))
        #expect(!installation.supportsCommands)
        #expect(installation.installationId == "fixture")
    }
}
@Test func additiveWireFieldsAndUnknownRunStatePreserveCanonicalText() throws {
    var values = try sharedSnapshotObject()
    values["futureProjection"] = ["unsupported":true]
    var runs = try #require(values["runs"] as? [[String:Any]])
    runs[0]["state"] = "future-approval-state"; runs[0]["futureAction"] = "do-not-execute"
    values["runs"] = runs
    let snapshot = try decodeSnapshot(values)
    #expect(snapshot.runs[0].displayState.contains("Unsupported"))
    #expect(snapshot.messages[0].text == "  /hello\r\nCafé e\u{301} 👋  ")
    var transcript = Transcript(installationID:snapshot.installationId,conversationID:snapshot.conversationId)
    try transcript.apply(snapshot)
    #expect(transcript.snapshot == snapshot)
}
@Test func malformedMessageSnapshotsCannotReplaceLastValidProjection() throws {
    let valid = try decodeSnapshot(sharedSnapshotObject())
    let mutations: [(inout [String:Any]) -> Void] = [
        { $0["cursor"] = -1 },
        { values in var messages = values["messages"] as! [[String:Any]]; messages.append(messages[0]); values["messages"] = messages },
        { values in var messages = values["messages"] as! [[String:Any]]; messages.reverse(); values["messages"] = messages },
        { values in var messages = values["messages"] as! [[String:Any]]; messages[0]["conversationId"] = "another-home"; values["messages"] = messages },
        { values in var messages = values["messages"] as! [[String:Any]]; messages[0]["sequence"] = 0; values["messages"] = messages },
        { values in var messages = values["messages"] as! [[String:Any]]; messages[0]["sequence"] = 99_999; values["messages"] = messages }
    ]
    for mutation in mutations {
        var values = try sharedSnapshotObject(); mutation(&values)
        let malformed = try decodeSnapshot(values)
        var transcript = Transcript(installationID:valid.installationId,conversationID:valid.conversationId)
        try transcript.apply(valid)
        #expect(throws:ContractError.invalidSnapshot) { try transcript.apply(malformed) }
        #expect(transcript.snapshot == valid)
    }
}
@Test func incompatibleRequiredWireTypesFailDecodingWithoutReplacingCache() throws {
    var values = try sharedSnapshotObject()
    values["cursor"] = "future-cursor-format"
    #expect(throws:DecodingError.self) { try decodeSnapshot(values) }
    values = try sharedSnapshotObject(); values.removeValue(forKey:"recoveryEpoch")
    #expect(throws:DecodingError.self) { try decodeSnapshot(values) }
}
