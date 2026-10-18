#if os(macOS)
import Foundation
import Security
import CryptoKit
import Darwin
import Testing
@testable import FortKit

private final class HistoryCredentials: DeviceCredentials {
    var present = false
    var reads = 0
    var onRead: ((Int) throws -> Void)?
    func save(_ connection: DeviceConnection, token: String) throws { present = true }
    func token(for connection: DeviceConnection) throws -> String {
        reads += 1; try onRead?(reads)
        guard present else { throw DeviceStorageError.keychain(errSecItemNotFound) }
        return "synthetic"
    }
    func remove(_ connection: DeviceConnection) throws { present = false }
}
private func historyFixture() throws -> (URL, DeviceStorage, DeviceConnection, HistoryCredentials) {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("fort-history-delete-" + UUID().uuidString)
    let credentials = HistoryCredentials(), storage = try DeviceStorage(directory:root,credentials:credentials)
    let endpoint = DeviceEndpoint.macAgent(try MacAgentEndpoint(serviceIdentifier:"io.fort.synthetic.agent",signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic"))
    let connection = try DeviceConnection(endpoint:endpoint,installationID:"one",deviceID:"device")
    try storage.save(connection,token:"synthetic")
    for id in ["one","other"] {
        let installation = try JSONDecoder().decode(Installation.self,from:Data("{\"contractVersion\":\"1.0\",\"installationId\":\"\(id)\",\"recoveryEpoch\":\"epoch\",\"mode\":\"fixture\",\"serviceName\":\"Synthetic\"}".utf8))
        try storage.saveSnapshot(Snapshot(installationId:id,recoveryEpoch:"epoch",conversationId:"home",cursor:0,messages:[],runs:[],lastReadMessageSequence:0))
        try storage.saveNavigation(SavedNavigation(installation:installation,agents:[],selectedAgentID:nil))
        let pending = try PendingStore(directory:root,installationID:id)
        try pending.enqueue(conversationID:"home",bindingRevision:"binding",recoveryEpoch:"epoch",text:"synthetic history")
    }
    try Data("preserve".utf8).write(to:root.appendingPathComponent("unrelated.txt"))
    let imageRoot = root.appendingPathComponent("local-images")
    for id in ["one","other"] {
        let key = SHA256.hash(data:Data((id+"\0home").utf8)).map { String(format:"%02x",$0) }.joined()
        let folder = imageRoot.appendingPathComponent(key)
        try FileManager.default.createDirectory(at:folder,withIntermediateDirectories:true,attributes:[.posixPermissions:0o700])
        let image = LocalChatImage(id:UUID().uuidString,installationID:id,conversationID:"home",afterSequence:0,createdAt:Date(),name:"synthetic",width:1,height:1,png:Data())
        let file = folder.appendingPathComponent(image.id+".json")
        try JSONEncoder().encode(image).write(to:file)
        try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:file.path)
    }
    try storage.removePairing(connection)
    return (root,storage,connection,credentials)
}
@Test func savedHistoryDeletionRemovesOnlyConfirmedInstallationAndCanRetry() throws {
    let (root,storage,connection,_) = try historyFixture()
    defer { try? FileManager.default.removeItem(at:root) }
    try storage.deleteSavedHistory(for:connection)
    #expect(try storage.navigation(installationID:"one") == nil)
    #expect(try storage.snapshot(installationID:"one",conversationID:"home") == nil)
    #expect(try storage.snapshot(installationID:"other",conversationID:"home") != nil)
    let images = FileManager.default.enumerator(at:root.appendingPathComponent("local-images"),includingPropertiesForKeys:nil)!.allObjects as! [URL]
    #expect(images.filter { $0.pathExtension == "json" }.count == 1)
    #expect(try PendingStore(directory:root,installationID:"one").pending().isEmpty)
    #expect(try storage.navigation(installationID:"other") != nil)
    #expect(try PendingStore(directory:root,installationID:"other").pending().count == 1)
    #expect(try String(contentsOf:root.appendingPathComponent("unrelated.txt"),encoding:.utf8) == "preserve")
    #expect(try storage.disconnectedConnection() == connection)
    #expect(try storage.pairingRemoval()?.state == .confirmed)
    try storage.deleteSavedHistory(for:connection)
}
@Test(arguments:["active", "pending", "forgotten", "foreign", "credential", "malformed", "symlink", "hardlink", "permissions", "directoryLink", "locked", "oversized", "entryLimit", "missingRevocation", "pathMismatch"])
func savedHistoryDeletionRefusesUnsafeStateBeforeRemovingAnyHistory(scenario: String) throws {
    let (root,storage,connection,credentials) = try historyFixture()
    defer { try? FileManager.default.removeItem(at:root) }
    let files = try FileManager.default.contentsOfDirectory(at:root,includingPropertiesForKeys:nil)
    let snapshot = try #require(files.first { $0.lastPathComponent.hasPrefix("snapshot-") })
    let original = try Data(contentsOf:snapshot)
    let sentinel = root.appendingPathComponent("sentinel")
    try original.write(to:sentinel); try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:sentinel.path)
    var lease: OwnerSetupFiles?
    switch scenario {
    case "active": try storage.save(connection,token:"synthetic")
    case "pending": try Data("{}".utf8).write(to:root.appendingPathComponent("owner-setup.json"))
    case "forgotten": try storage.removePairing(connection,revocationConfirmed:false); try JSONEncoder().encode(PairingRemovalIntent(connection:connection,state:.locallyForgotten)).write(to:root.appendingPathComponent("pairing-removal.json"))
    case "foreign": try JSONEncoder().encode(DeviceConnection(endpoint:connection.endpoint,installationID:"other",deviceID:"device")).write(to:root.appendingPathComponent("disconnected.json"))
    case "credential": credentials.present = true
    case "malformed": try Data("broken".utf8).write(to:snapshot)
    case "symlink": try FileManager.default.removeItem(at:snapshot); try FileManager.default.createSymbolicLink(at:snapshot,withDestinationURL:sentinel)
    case "hardlink": try FileManager.default.removeItem(at:snapshot); try FileManager.default.linkItem(at:sentinel,to:snapshot)
    case "permissions": try FileManager.default.setAttributes([.posixPermissions:0o644],ofItemAtPath:snapshot.path)
    case "oversized":
        let handle = try FileHandle(forWritingTo:snapshot)
        try handle.truncate(atOffset:16 * 1024 * 1024 + 1); try handle.close()
    case "entryLimit":
        for index in 0..<4097 { FileManager.default.createFile(atPath:root.appendingPathComponent("extra-\(index)").path,contents:Data()) }
    case "missingRevocation": try FileManager.default.removeItem(at:root.appendingPathComponent("pairing-removal.json"))
    case "pathMismatch": try original.write(to:root.appendingPathComponent("snapshot-" + String(repeating:"0",count:64) + ".json"))
    case "directoryLink":
        let images = root.appendingPathComponent("local-images"), retained = root.appendingPathComponent("retained-images")
        try FileManager.default.moveItem(at:images,to:retained); try FileManager.default.createSymbolicLink(at:images,withDestinationURL:retained)
    default: lease = try OwnerSetupFiles(directory:root)
    }
    defer { lease?.close() }
    #expect(throws:(any Error).self) { try storage.deleteSavedHistory(for:connection) }
    #expect(try String(contentsOf:root.appendingPathComponent("unrelated.txt"),encoding:.utf8) == "preserve")
    #expect(try Data(contentsOf:sentinel) == original)
    for file in files where file != snapshot && file.lastPathComponent.hasPrefix("snapshot-") { #expect(FileManager.default.fileExists(atPath:file.path)) }
}
@Test @MainActor func savedHistoryNativeActionClearsOnlyAfterConfirmedDeletion() async throws {
    let (root,storage,connection,credentials) = try historyFixture()
    defer { try? FileManager.default.removeItem(at:root) }
    let model = MessengerModel(storage:storage,session:.shared)
    await model.restore()
    #expect(model.canDeleteSavedHistory && model.installation != nil && !model.pending.isEmpty)
    let stale = try DeviceConnection(endpoint:connection.endpoint,installationID:"other",deviceID:"device")
    model.deleteSavedHistory(stale)
    #expect(try storage.navigation(installationID:"one") != nil)
    #expect(model.savedHistoryDeletionTarget == connection)
    credentials.present = true
    model.deleteSavedHistory(connection)
    #expect(model.installation != nil && !model.pending.isEmpty)
    credentials.present = false
    model.deleteSavedHistory(connection)
    #expect(model.installation == nil && model.pending.isEmpty && model.localImages.isEmpty)
    #expect(model.status == "Saved history deleted on this Mac")
    let reopened = MessengerModel(storage:storage,session:.shared)
    await reopened.restore()
    #expect(reopened.installation == nil && reopened.pending.isEmpty)
    #expect(reopened.status == "Disconnected")
    #expect(reopened.pairingMessage.contains("No saved conversation list"))
    #expect(try storage.navigation(installationID:"other") != nil)
}
@Test(arguments:["root", "file", "interrupted"])
func savedHistoryDeletionRejectsChangesAndRetriesInterruptedCleanup(scenario: String) throws {
    let (root,storage,connection,credentials) = try historyFixture()
    let moved = root.appendingPathExtension("moved")
    defer { try? FileManager.default.removeItem(at:root); try? FileManager.default.removeItem(at:moved) }
    let files = try FileManager.default.contentsOfDirectory(at:root,includingPropertiesForKeys:nil).filter { $0.lastPathComponent.hasPrefix("snapshot-") }
    let bytes = try files.map { try Data(contentsOf:$0) }
    credentials.onRead = { count in
        if scenario == "root", count == 2 {
            try FileManager.default.moveItem(at:root,to:moved)
            try FileManager.default.createDirectory(at:root,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
        } else if scenario == "file", count == 2 {
            let key = SHA256.hash(data:Data("one/home".utf8)).map { String(format:"%02x",$0) }.joined()
            try Data("changed".utf8).write(to:root.appendingPathComponent("snapshot-" + key + ".json"))
        } else if scenario == "interrupted", count == 4 { throw URLError(.cancelled) }
    }
    #expect(throws:(any Error).self) { try storage.deleteSavedHistory(for:connection) }
    if scenario == "root" {
        #expect(try FileManager.default.contentsOfDirectory(atPath:root.path).isEmpty)
        for (file,bytes) in zip(files,bytes) { #expect(try Data(contentsOf:moved.appendingPathComponent(file.lastPathComponent)) == bytes) }
    } else if scenario == "file" {
        for file in files { #expect(FileManager.default.fileExists(atPath:file.path)) }
    } else {
        #expect(try storage.navigation(installationID:"one") == nil || storage.snapshot(installationID:"one",conversationID:"home") == nil)
        credentials.onRead = nil
        try storage.deleteSavedHistory(for:connection)
        #expect(try storage.navigation(installationID:"one") == nil && storage.snapshot(installationID:"one",conversationID:"home") == nil)
        #expect(try storage.navigation(installationID:"other") != nil)
    }
}
@Test func savedHistoryDeletionBudget() throws {
    guard let output = ProcessInfo.processInfo.environment["FORT_HISTORY_BENCHMARK"] else { return }
    var samples: [Double] = []
    for _ in 0..<20 {
        let (root,storage,connection,_) = try historyFixture()
        defer { try? FileManager.default.removeItem(at:root) }
        for index in 0..<100 { try storage.saveSnapshot(Snapshot(installationId:"one",recoveryEpoch:"epoch",conversationId:"home-\(index)",cursor:0,messages:[],runs:[],lastReadMessageSequence:0)) }
        let start = ContinuousClock.now
        try storage.deleteSavedHistory(for:connection)
        let elapsed = start.duration(to:.now)
        samples.append(Double(elapsed.components.seconds)*1000 + Double(elapsed.components.attoseconds)/1e15)
    }
    try JSONEncoder().encode(samples).write(to:URL(fileURLWithPath:output))
    #expect(samples.sorted()[18] <= 500)
}
#endif
