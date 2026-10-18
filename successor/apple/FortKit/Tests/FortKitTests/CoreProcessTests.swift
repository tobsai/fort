#if os(macOS)
import Foundation
import Darwin
import Testing
@testable import FortKit

private let successor = URL(fileURLWithPath:#filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
private func nodeExecutable() throws -> URL {
    let environment = ProcessInfo.processInfo.environment
    if let path = environment["FORT_TEST_NODE"] { return URL(fileURLWithPath:path) }
    for directory in (environment["PATH"] ?? "").split(separator:":") {
        let path = String(directory) + "/node"
        if FileManager.default.isExecutableFile(atPath:path) { return URL(fileURLWithPath:path) }
    }
    throw CocoaError(.fileNoSuchFile)
}
private func privateDirectory() throws -> URL {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-native-core-" + UUID().uuidString)
    try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    return directory
}
private func startCore(_ directory: URL) async throws -> MacCoreProcessTransport {
    let artifact = ProcessInfo.processInfo.environment["FORT_TEST_CORE_ARTIFACT"].map { URL(fileURLWithPath:$0) }
    let core = artifact?.appendingPathComponent("service/openai.js") ?? successor.appendingPathComponent("service/openai.ts")
    let channel = artifact?.appendingPathComponent("host/private-channel.js") ?? successor.appendingPathComponent("host/private-channel.ts")
    let flags = artifact == nil ? ["--no-warnings"] : ["--jitless","--no-experimental-strip-types","--no-warnings"]
    return try await MacCoreProcessTransport.launch(executable:nodeExecutable(),arguments:flags + [successor.appendingPathComponent("host/fixtures/native-core.mjs").path,directory.path,core.path,channel.path],directory:directory,environment:["PATH":"/usr/bin:/bin","HOME":directory.path,"TMPDIR":directory.path],lifecycle:.processGroup)
}
private struct Setup: Decodable {
    struct Identity: Decodable { let installationId: String; let recoveryEpoch: String }
    struct Enrollment: Decodable { let code: String }
    let installation: Identity
    let enrollment: Enrollment
}
private func setup(_ directory: URL) throws -> Setup { try JSONDecoder().decode(Setup.self,from:Data(contentsOf:directory.appendingPathComponent("setup.json"))) }
private func assertAbsent(_ pid: Int32) {
    #expect(kill(pid,0) == -1 && errno == ESRCH)
    #expect(kill(-pid,0) == -1 && errno == ESRCH)
}

@Test func ownedNativeCoreCarriesAuthenticatedExactMessagesAndRestartReceipts() async throws {
    let directory = try privateDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let first = try await startCore(directory)
    do {
        let bootstrap = try setup(directory)
        let anonymous = FortClient(installationID:bootstrap.installation.installationId,transport:first)
        await #expect(throws:ClientError.self) { try await anonymous.installation() }
        let enrollment = try await anonymous.enroll(code:bootstrap.enrollment.code,deviceName:"Synthetic Swift pipe")
        let client = FortClient(installationID:bootstrap.installation.installationId,token:enrollment.deviceToken,transport:first)
        let foreign = FortClient(installationID:"foreign",token:enrollment.deviceToken,transport:first)
        await #expect(throws:ClientError.self) { try await foreign.installation() }
        let agent = try await client.saveAgent(nil,name:"Native pipe Home",model:"exact-synthetic",instructions:"Echo.")
        let text = "  Exact cedar 🌲 Café e\u{301}\r\n"
        let pending = PendingSubmission(requestID:"native-pipe-turn",conversationID:agent.homeConversationId,bindingRevision:agent.binding.bindingRevisionId,recoveryEpoch:bootstrap.installation.recoveryEpoch,text:text,createdAt:Date(),recipientAgentID:agent.agentId,skills:[])
        let accepted = try await client.submit(pending)
        var snapshot = try await client.snapshot(conversationID:agent.homeConversationId)
        for _ in 0..<50 where snapshot.messages.count != 2 {
            try await Task.sleep(for:.milliseconds(10)); snapshot = try await client.snapshot(conversationID:agent.homeConversationId)
        }
        #expect(snapshot.messages.map(\.text) == [text,text])
        let page = try await client.events(after:0,recoveryEpoch:bootstrap.installation.recoveryEpoch)
        let wait = Task { try await client.events(after:page.cursor,recoveryEpoch:bootstrap.installation.recoveryEpoch,waitMilliseconds:25_000) }
        try await Task.sleep(for:.milliseconds(20)); wait.cancel()
        await #expect(throws:CancellationError.self) { try await wait.value }
        #expect(try await client.installation().installationId == bootstrap.installation.installationId)
        let timed = FortRequest(path:"/v1/events?after=\(page.cursor)&recoveryEpoch=\(bootstrap.installation.recoveryEpoch)&wait=25000",method:.get,installationID:bootstrap.installation.installationId,token:enrollment.deviceToken,body:nil,timeout:0.05)
        do { _ = try await first.send(timed); Issue.record("Expected timeout") }
        catch let error as URLError { #expect(error.code == .timedOut) }
        #expect(try await client.installation().installationId == bootstrap.installation.installationId)
        #expect(try String(contentsOf:directory.appendingPathComponent("executions.json"),encoding:.utf8) == "1")
        let stopped = try await first.close()
        #expect(stopped.exitedCleanly && !stopped.forced)
        assertAbsent(first.processID)
        #expect(!FileManager.default.fileExists(atPath:directory.appendingPathComponent("core.sock").path))
        let second = try await startCore(directory)
        do {
            #expect(try setup(directory).installation.installationId == bootstrap.installation.installationId)
            let restarted = FortClient(installationID:bootstrap.installation.installationId,token:enrollment.deviceToken,transport:second)
            let duplicate = try await restarted.submit(pending)
            #expect(duplicate.receipt.requestId == accepted.receipt.requestId)
            #expect(duplicate.snapshot.messages.map(\.text) == [text,text])
            #expect(try String(contentsOf:directory.appendingPathComponent("executions.json"),encoding:.utf8) == "0")
            let stopped = try await second.close()
            #expect(stopped.exitedCleanly && !stopped.forced)
            assertAbsent(second.processID)
        } catch { _ = try? await second.close(); throw error }
    } catch { _ = try? await first.close(); throw error }
}

private func startChannel(_ directory: URL, mode: String) async throws -> MacCoreProcessTransport {
    try await MacCoreProcessTransport.launch(executable:nodeExecutable(),arguments:["--jitless","--no-warnings",successor.appendingPathComponent("host/fixtures/native-channel.mjs").path,mode],directory:directory,environment:["PATH":"/usr/bin:/bin"],lifecycle:.processGroup)
}
private func channelRequest(timeout: TimeInterval = 1, body: Data? = nil) -> FortRequest {
    FortRequest(path:"/v1/probe",method:.post,installationID:"synthetic",token:nil,body:body,timeout:timeout)
}

@Test(arguments:["malformed","oversized-frame","oversized-body","wrong-id","wrong-version","invalid-status","exit"])
func ownedNativeCoreRejectsInvalidEvidence(mode: String) async throws {
    let directory = try privateDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let host = try await startChannel(directory,mode:mode)
    await #expect(throws:(any Error).self) { try await host.send(channelRequest()) }
    let stop = try await host.close()
    #expect(!stop.exitedCleanly); assertAbsent(host.processID)
}

@Test func ownedNativeCoreCorrelatesConcurrentRepliesAndRecoversAdmissionAfterCancellation() async throws {
    let directory = try privateDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let reordered = try await startChannel(directory,mode:"out-of-order")
    let results = try await withThrowingTaskGroup(of:Bool.self,returning:[Bool].self) { group in
        for index in 0..<8 {
            group.addTask {
                let body = Data("response-\(index)".utf8)
                return try await reordered.send(channelRequest(body:body)).body == body
            }
        }
        var values: [Bool] = []; for try await value in group { values.append(value) }; return values
    }
    #expect(results.count == 8 && results.allSatisfy({ $0 }))
    #expect(try await reordered.close().exitedCleanly); assertAbsent(reordered.processID)
    let gated = try await startChannel(directory,mode:"gate")
    let hold = FortRequest(path:"/v1/hold",method:.get,installationID:"synthetic",token:nil,body:nil,timeout:30)
    let waiting = (0..<8).map { _ in Task { try await gated.send(hold) } }
    var held = 0
    for _ in 0..<100 where held < 8 {
        if let data = try? Data(contentsOf:directory.appendingPathComponent("held.json")) { held = (try? JSONDecoder().decode(Int.self,from:data)) ?? 0 }
        try await Task.sleep(for:.milliseconds(10))
    }
    #expect(held == 8)
    do { _ = try await gated.send(channelRequest()); Issue.record("Expected bounded admission") }
    catch let error as URLError { #expect(error.code == .resourceUnavailable) }
    waiting.forEach { $0.cancel() }
    for task in waiting { await #expect(throws:CancellationError.self) { try await task.value } }
    try await Task.sleep(for:.milliseconds(30))
    #expect(try await gated.send(channelRequest()).body == Data("exact".utf8))
    #expect(try await gated.close().exitedCleanly); assertAbsent(gated.processID)
}

@Test func ownedNativeCoreRejectsDuplicateResponseWithoutReusingTheChannel() async throws {
    let directory = try privateDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let host = try await startChannel(directory,mode:"duplicate")
    _ = try await host.send(channelRequest())
    try await Task.sleep(for:.milliseconds(30))
    await #expect(throws:(any Error).self) { try await host.send(channelRequest()) }
    let stop = try await host.close()
    #expect(!stop.exitedCleanly); assertAbsent(host.processID)
}

@Test func ownedNativeCoreRequiresReadinessAndReapsTheFailedLaunch() async throws {
    let directory = try privateDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let start = ContinuousClock.now
    await #expect(throws:(any Error).self) { try await startChannel(directory,mode:"no-ready") }
    let pid = try JSONDecoder().decode(Int32.self,from:Data(contentsOf:directory.appendingPathComponent("fixture-pid.json")))
    assertAbsent(pid)
    #expect(start.duration(to:.now) < .seconds(8))
}

@Test(arguments:["ignore-cancel","blocked-input","hang"])
func ownedNativeCoreBoundsCancellationBackpressureAndHungDescendants(mode: String) async throws {
    let directory = try privateDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let host = try await startChannel(directory,mode:mode)
    let start = ContinuousClock.now
    if mode == "ignore-cancel" {
        let task = Task { try await host.send(channelRequest(timeout:30)) }
        try await Task.sleep(for:.milliseconds(30)); task.cancel()
        await #expect(throws:CancellationError.self) { try await task.value }
        try await Task.sleep(for:.milliseconds(5200))
    } else if mode == "blocked-input" {
        await #expect(throws:(any Error).self) { try await host.send(channelRequest(timeout:30,body:Data(repeating:65,count:1024 * 1024))) }
    }
    let stop = try await host.close()
    #expect(!stop.exitedCleanly)
    #expect(stop.forced == (mode != "ignore-cancel"))
    assertAbsent(host.processID)
    if mode == "hang" {
        let pid = try JSONDecoder().decode(Int32.self,from:Data(contentsOf:directory.appendingPathComponent("descendant-pid.json")))
        #expect(kill(pid,0) == -1 && errno == ESRCH)
    }
    #expect(start.duration(to:.now) < .seconds(9))
}
#endif
