#if os(macOS)
import Foundation
import Darwin
import Testing
@testable import FortKit

@MainActor private final class JourneyListener: MacAgentServiceListener {
    let core: MacCoreHost
    var active = false
    init(_ core: MacCoreHost) { self.core = core }
    func resume() { active = true }
    func invalidate() { active = false }
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_TEST_HOST_RESOURCES"] != nil))
@MainActor func nativeBundledHostJourneyPreservesIdentityAndHistoryThenRevokesAndStops() async throws {
    let env = ProcessInfo.processInfo.environment, fixture = try HostBundleFixture()
    defer { try? fixture.remove() }
    let resources = URL(fileURLWithPath:try #require(env["FORT_TEST_HOST_RESOURCES"]))
    let node = URL(fileURLWithPath:try #require(env["FORT_TEST_NODE"])), guardian = URL(fileURLWithPath:try #require(env["FORT_TEST_GUARDIAN"]))
    let source = URL(fileURLWithPath:#filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    let hostResources = fixture.helper.appendingPathComponent("Contents/Resources/host")
    try FileManager.default.copyItem(at:resources,to:hostResources)
    let tools = fixture.helper.appendingPathComponent("Contents/MacOS")
    try FileManager.default.copyItem(at:node.resolvingSymlinksInPath(),to:tools.appendingPathComponent("node"))
    let nodeLibrary = node.resolvingSymlinksInPath().deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("lib/libnode.147.dylib")
    try FileManager.default.copyItem(at:nodeLibrary,to:tools.appendingPathComponent("libnode.147.dylib"))
    try FileManager.default.copyItem(at:guardian,to:tools.appendingPathComponent("FortProcessGuardian"))
    let authored = fixture.root.appendingPathComponent("authored")
    try FileManager.default.createDirectory(at:authored,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    try FileManager.default.copyItem(at:source.appendingPathComponent("runner/codex-preflight-fixture.ts"),to:authored.appendingPathComponent("preflight.mjs"))
    let protocolSource = try String(contentsOf:source.appendingPathComponent("runner/fixtures/codex-executor-runtime.mjs"),encoding:.utf8)
    try protocolSource.replacingOccurrences(of:"'../codex-preflight-fixture.ts'",with:"'./preflight.mjs'").write(to:authored.appendingPathComponent("runtime.mjs"),atomically:false,encoding:.utf8)
    func js(_ text: String) throws -> String { String(decoding:try JSONEncoder().encode(text),as:UTF8.self) }
    let script = """
    #!\(tools.appendingPathComponent("node").path)
    import { appendFileSync } from 'node:fs';
    import { execFileSync } from 'node:child_process';
    let pid = process.pid; const owned = [];
    for (let i = 0; i < 6; i++) {
      const [child,parent,group] = execFileSync('/bin/ps',['-o','pid=,ppid=,pgid=','-p',String(pid)],{env:{},encoding:'utf8'}).trim().split(/\\s+/).map(Number);
      owned.push({pid:child,parent,group}); pid = parent;
    }
    appendFileSync(\(try js(authored.appendingPathComponent("processes.jsonl").path)),JSON.stringify(owned)+'\\n');
    process.argv = [process.execPath, import.meta.filename, 'echo', \(try js(authored.path))];
    await import(\(try js(authored.appendingPathComponent("runtime.mjs").absoluteString)));
    """
    let codex = tools.appendingPathComponent("codex")
    try script.write(to:codex,atomically:false,encoding:.utf8)
    try FileManager.default.setAttributes([.posixPermissions:0o700],ofItemAtPath:codex.path)
    let program = try MacHostProgram.inspect(helperURL:fixture.helper) { _, identifier, _ in
        if ["node","guardian","codex"].contains(where:{ identifier == fixture.endpoint.serviceIdentifier + "." + $0 }) {
            return MacHostCodeSignature(appSandbox:true,inheritsSandbox:true,applicationGroups:[],entitlementKeys:["com.apple.security.app-sandbox","com.apple.security.inherit"])
        }
        return fixture.signature
    }
    let container = fixture.root.appendingPathComponent("group")
    try FileManager.default.createDirectory(at:container,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    var listener: JourneyListener?
    func makeService() -> MacAgentService {
        MacAgentService(program:program,resolveContainer:{ group in
            #expect(group == fixture.endpoint.appGroup); return container
        },makeListener:{ core in let result = JourneyListener(core); listener = result; return result })
    }
    func absent() throws {
        struct Owned: Decodable { let pid: Int32; let parent: Int32; let group: Int32 }
        let lines = try String(contentsOf:authored.appendingPathComponent("processes.jsonl"),encoding:.utf8).split(separator:"\n")
        #expect(!lines.isEmpty)
        for line in lines {
            let records = try JSONDecoder().decode([Owned].self,from:Data(line.utf8))
            #expect(records.count == 6 && records.last?.parent == getpid())
            for record in records {
                #expect(record.pid > 1 && record.group > 1 && record.group != getpgrp())
                #expect(kill(record.pid,0) == -1 && errno == ESRCH)
                #expect(kill(-record.group,0) == -1 && errno == ESRCH)
            }
        }
    }
    let started = ContinuousClock.now
    var service = makeService()
    do {
        #expect(listener == nil && !FileManager.default.fileExists(atPath:container.appendingPathComponent("FortHost").path))
        try await service.start()
        let first = try #require(listener); #expect(first.active)
        #expect(!FileManager.default.fileExists(atPath:authored.appendingPathComponent("processes.jsonl").path))
        guard case .available(let offer) = try await first.core.beginOwnerSetup(timeout:3) else { throw ClientError.invalidResponse }
        let setup = try OwnerSetupRequest(installationID:offer.installationID,setupID:offer.setupID,deviceID:"synthetic-native-owner",deviceToken:String(repeating:"A",count:43))
        let paired = try await first.core.completeOwnerSetup(setup,timeout:3)
        #expect(paired.expiresAt == nil)
        let client = FortClient(installationID:setup.installationID,token:setup.deviceToken,transport:first.core)
        let initial = try await client.installation()
        #expect(initial.runtime?.hostId == "local-codex")
        #expect(try await client.models().canSelect("fixture-exact-model"))
        let agent = try await client.saveAgent(nil,name:"Synthetic native Home",model:"fixture-exact-model",instructions:"Preserve exact text.")
        let memory = try await client.saveMemory(conversationID:agent.homeConversationId,memory:nil,text:"Synthetic context only.",recoveryEpoch:initial.recoveryEpoch)
        let exact = "  native synthetic cedar 🌲\n"
        let pending = PendingSubmission(requestID:"synthetic-native-turn",conversationID:agent.homeConversationId,bindingRevision:agent.binding.bindingRevisionId,recoveryEpoch:initial.recoveryEpoch,text:exact,createdAt:Date(),recipientAgentID:agent.agentId,skills:[],memoryRevisionId:memory.revisionId)
        let accepted = try await client.submit(pending)
        var snapshot = try await client.snapshot(conversationID:agent.homeConversationId)
        for _ in 0..<400 where snapshot.runs.first?.state != "completed" {
            try await Task.sleep(for:.milliseconds(10)); snapshot = try await client.snapshot(conversationID:agent.homeConversationId)
        }
        #expect(snapshot.runs.first?.state == "completed")
        #expect(snapshot.messages.count == 2 && snapshot.messages.allSatisfy { $0.text == exact })
        #expect(try await service.shutdown().exitedCleanly && !first.active)
        try absent()
        let launches = try Data(contentsOf:authored.appendingPathComponent("processes.jsonl"))
        service = makeService(); try await service.start()
        let reopened = try #require(listener); #expect(reopened.active)
        #expect(try await reopened.core.beginOwnerSetup(timeout:3) == .paired(paired))
        let restored = FortClient(installationID:setup.installationID,token:setup.deviceToken,transport:reopened.core)
        let installation = try await restored.installation()
        #expect(installation.installationId == initial.installationId && installation.recoveryEpoch == initial.recoveryEpoch && installation.runtime == initial.runtime)
        #expect(try await restored.snapshot(conversationID:agent.homeConversationId).messages == snapshot.messages)
        #expect(try await restored.submit(pending).receipt == accepted.receipt)
        #expect(try Data(contentsOf:authored.appendingPathComponent("processes.jsonl")) == launches)
        let recovery = try MacOwnerRecoveryRequest(installationID:setup.installationID,deviceID:setup.deviceID,timeout:3)
        try await reopened.core.recoverPairing(recovery).validate(for:recovery)
        await #expect(throws:(any Error).self) { try await restored.installation() }
        #expect(try await service.shutdown().exitedCleanly && !reopened.active)
        try absent()
        #expect(!FileManager.default.fileExists(atPath:container.appendingPathComponent("FortHost/core.sock").path))
        #expect(FileManager.default.fileExists(atPath:container.appendingPathComponent("FortHost/openai-candidate-b.sqlite").path))
        #expect(!FileManager.default.fileExists(atPath:container.appendingPathComponent("FortHost/Runtime/state/auth.json").path))
        let duration = started.duration(to:.now).components
        let milliseconds = Double(duration.seconds) * 1000 + Double(duration.attoseconds) / 1e15
        print("Synthetic native host journey: \(milliseconds) ms; owned groups absent; no registration or provider.")
        #expect(milliseconds <= 5000)
    } catch { _ = try? await service.shutdown(); throw error }
}
#endif
