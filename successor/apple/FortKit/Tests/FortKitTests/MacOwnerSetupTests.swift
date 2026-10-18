#if os(macOS)
import Foundation
import Testing
@testable import FortKit

private actor SetupTransportFixture: FortTransport {
    var calls = 0
    func send(_ request: FortRequest) async throws -> FortResponse { calls += 1; return FortResponse(status:200,body:Data()) }
}
private actor SetupControlFixture: OwnerSetupControl {
    let receipt: OwnerSetupReceipt
    let behavior: String
    var calls = 0
    var canceled = false
    init(receipt: OwnerSetupReceipt, behavior: String = "") { self.receipt = receipt; self.behavior = behavior }
    private func action() async throws {
        calls += 1
        if behavior == "throw" { throw NSError(domain:"Synthetic",code:1,userInfo:[NSLocalizedDescriptionKey:"synthetic-secret"]) }
        if behavior == "wait" {
            do { try await Task.sleep(for:.seconds(10)) }
            catch { canceled = true; throw error }
        }
    }
    func beginOwnerSetup(timeout: TimeInterval) async throws -> OwnerSetupState {
        try await action()
        return .available(try OwnerSetupOffer(installationID:"installation",setupID:"setup",expiresAt:"2026-10-18T00:00:00.000Z"))
    }
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) async throws -> OwnerSetupReceipt { try await action(); return receipt }
}
private actor SetupCancellationFixture: OwnerSetupControl, OwnerSetupCancellationControl {
    let proof: OwnerSetupCancellation
    let behavior: String
    var calls = 0
    var canceled = false
    init(proof: OwnerSetupCancellation, behavior: String = "") { self.proof = proof; self.behavior = behavior }
    func beginOwnerSetup(timeout: TimeInterval) async throws -> OwnerSetupState { throw ownerSetupUnavailable() }
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) async throws -> OwnerSetupReceipt { throw ownerSetupUnavailable() }
    func cancelOwnerSetup(_ identity: OwnerSetupIdentity, timeout: TimeInterval) async throws -> OwnerSetupCancellation {
        calls += 1
        if behavior == "throw" { throw NSError(domain:"Synthetic",code:1,userInfo:[NSLocalizedDescriptionKey:"synthetic-secret"]) }
        if behavior == "wait" {
            do { try await Task.sleep(for:.seconds(10)) }
            catch { canceled = true; throw error }
        }
        return proof
    }
}
private func invokeSetup(_ channel: MacHostChannel, bytes: Data) async throws -> Data {
    try await withCheckedThrowingContinuation { continuation in
        channel.setupOwner(bytes) { data,error in
            if let error { continuation.resume(throwing:error) }
            else if let data { continuation.resume(returning:data) }
            else { continuation.resume(throwing:ClientError.invalidResponse) }
        }
    }
}

@Test func nativeXPCOwnerSetupRequiresExplicitCompositionAndConfirmsExactReceipt() async throws {
    let transport = SetupTransportFixture()
    let receipt = try OwnerSetupReceipt(installationID:"installation",deviceID:"device",expiresAt:nil)
    let control = SetupControlFixture(receipt:receipt)
    let begin = Data(#"{"operation":"begin","timeout":3}"#.utf8)
    await #expect(throws:(any Error).self) { try await invokeSetup(MacHostChannel(transport:transport),bytes:begin) }
    let offered = try await invokeSetup(MacHostChannel(transport:transport,ownerSetup:control),bytes:begin)
    #expect(try JSONDecoder().decode(OwnerSetupState.self,from:offered) == .available(OwnerSetupOffer(installationID:"installation",setupID:"setup",expiresAt:"2026-10-18T00:00:00.000Z")))
    let complete = try JSONSerialization.data(withJSONObject:["operation":"complete","timeout":3,"installationId":"installation","setupId":"setup","deviceId":"device","deviceToken":String(repeating:"A",count:43)])
    let accepted = try await invokeSetup(MacHostChannel(transport:transport,ownerSetup:control),bytes:complete)
    #expect(try JSONDecoder().decode(OwnerSetupReceipt.self,from:accepted) == receipt)
    #expect(!String(decoding:accepted,as:UTF8.self).contains("deviceToken"))
    #expect(await control.calls == 2)
    #expect(await transport.calls == 0)
}

@Test func nativeXPCOwnerCancellationRequiresItsSeparateCapabilityAndExactReceipt() async throws {
    let transport = SetupTransportFixture()
    let identity = try OwnerSetupIdentity(installationID:"installation",setupID:"setup",deviceID:"device")
    let bytes = try JSONEncoder().encode(OwnerSetupCommand.cancel(identity,3))
    let beginOnly = SetupControlFixture(receipt:try OwnerSetupReceipt(installationID:"installation",deviceID:"device",expiresAt:nil))
    for channel in [MacHostChannel(transport:transport),MacHostChannel(transport:transport,ownerSetup:beginOnly)] {
        do { _ = try await invokeSetup(channel,bytes:bytes); Issue.record("Cancellation must be unavailable") }
        catch { #expect((error as NSError).domain == "Fort.OwnerSetup" && (error as NSError).code == 1) }
    }
    #expect(await beginOnly.calls == 0)
    for proof in [OwnerSetupCancellation.notPaired(identity),.revoked(identity)] {
        let control = SetupCancellationFixture(proof:proof)
        let result = try await invokeSetup(MacHostChannel(transport:transport,ownerSetup:control),bytes:bytes)
        #expect(try JSONDecoder().decode(OwnerSetupCancellation.self,from:result) == proof)
        #expect(!String(decoding:result,as:UTF8.self).contains("deviceToken"))
        #expect(await control.calls == 1)
    }
    #expect(await transport.calls == 0)
}

@Test func nativeXPCOwnerCancellationRejectsInvalidInputsAndForeignOrFailedProofs() async throws {
    let transport = SetupTransportFixture()
    let identity = try OwnerSetupIdentity(installationID:"installation",setupID:"setup",deviceID:"device")
    let control = SetupCancellationFixture(proof:.notPaired(identity))
    let valid: [String:Any] = ["operation":"cancel","timeout":3,"installationId":"installation","setupId":"setup","deviceId":"device"]
    for changed in [["setupId":""],["deviceId":"wrong/path"],["installationId":String(repeating:"a",count:129)],["timeout":31],["timeout":0],["deviceToken":"forbidden"],["token":"forbidden"],["unexpected":true],["operation":"discard"]] as [[String:Any]] {
        let bytes = try JSONSerialization.data(withJSONObject:valid.merging(changed) { _,new in new })
        await #expect(throws:(any Error).self) { try await invokeSetup(MacHostChannel(transport:transport,ownerSetup:control),bytes:bytes) }
    }
    #expect(await control.calls == 0)
    let bytes = try JSONEncoder().encode(OwnerSetupCommand.cancel(identity,3))
    for foreign in [try OwnerSetupIdentity(installationID:"foreign",setupID:"setup",deviceID:"device"),try OwnerSetupIdentity(installationID:"installation",setupID:"foreign",deviceID:"device"),try OwnerSetupIdentity(installationID:"installation",setupID:"setup",deviceID:"foreign"),identity] {
        let control = SetupCancellationFixture(proof:.revoked(foreign),behavior:foreign == identity ? "throw" : "")
        do { _ = try await invokeSetup(MacHostChannel(transport:transport,ownerSetup:control),bytes:bytes); Issue.record("Cancellation must remain unconfirmed") }
        catch {
            let error = error as NSError
            #expect(error.domain == "Fort.OwnerSetup" && error.code == 2)
            #expect(!error.localizedDescription.contains("synthetic-secret"))
        }
        #expect(await control.calls == 1)
    }
    let peer = try MacXPCTransport(serviceIdentifier:"synthetic.unavailable",signingTeam:"AAAAAAAAAA")
    for timeout in [0,-1,31,Double.infinity,Double.nan] {
        await #expect(throws:ClientError.self) { try await peer.cancelOwnerSetup(identity,timeout:timeout) }
    }
    #expect(await transport.calls == 0)
}

@Test func nativeXPCOwnerCancellationSharesSingleOperationAdmissionAndInvalidation() async throws {
    let transport = SetupTransportFixture()
    let identity = try OwnerSetupIdentity(installationID:"installation",setupID:"setup",deviceID:"device")
    let control = SetupCancellationFixture(proof:.revoked(identity),behavior:"wait")
    let replies = SetupReplies(), channel = MacHostChannel(transport:transport,ownerSetup:control)
    let bytes = try JSONEncoder().encode(OwnerSetupCommand.cancel(identity,3))
    channel.setupOwner(bytes) { _,_ in Task { await replies.received() } }
    for _ in 0..<100 where await control.calls == 0 { try await Task.sleep(for:.milliseconds(5)) }
    #expect(await control.calls == 1)
    await #expect(throws:(any Error).self) { try await invokeSetup(channel,bytes:bytes) }
    channel.cancel()
    for _ in 0..<100 where await !control.canceled { try await Task.sleep(for:.milliseconds(5)) }
    #expect(await control.canceled)
    #expect(await replies.count == 0)
    #expect(await transport.calls == 0)
}

@Test func nativeXPCOwnerSetupRejectsMalformedInputsAndInvalidDeadlinesBeforeCallingControl() async throws {
    let transport = SetupTransportFixture(), control = SetupControlFixture(receipt:try OwnerSetupReceipt(installationID:"installation",deviceID:"device",expiresAt:nil))
    for bytes in [Data(),Data(repeating:65,count:1025),Data(#"{"operation":"begin","timeout":31}"#.utf8),Data(#"{"operation":"renew","timeout":1}"#.utf8),Data(#"{"operation":"complete","timeout":1,"installationId":"wrong/path","setupId":"setup","deviceId":"device","deviceToken":"invalid"}"#.utf8)] {
        await #expect(throws:(any Error).self) { try await invokeSetup(MacHostChannel(transport:transport,ownerSetup:control),bytes:bytes) }
    }
    #expect(await control.calls == 0)
    let peer = try MacXPCTransport(serviceIdentifier:"synthetic.unavailable",signingTeam:"AAAAAAAAAA")
    let request = try OwnerSetupRequest(installationID:"installation",setupID:"setup",deviceID:"device",deviceToken:String(repeating:"A",count:43))
    for timeout in [0,-1,31,Double.infinity,Double.nan] {
        await #expect(throws:ClientError.self) { try await peer.beginOwnerSetup(timeout:timeout) }
        await #expect(throws:ClientError.self) { try await peer.completeOwnerSetup(request,timeout:timeout) }
    }
}

@Test func nativeXPCOwnerSetupSanitizesFailuresAndRejectsForeignReceipts() async throws {
    let transport = SetupTransportFixture()
    let complete = try JSONSerialization.data(withJSONObject:["operation":"complete","timeout":3,"installationId":"installation","setupId":"setup","deviceId":"device","deviceToken":String(repeating:"A",count:43)])
    for identity in [("foreign","device",""),("installation","foreign",""),("installation","device","throw")] {
        let control = SetupControlFixture(receipt:try OwnerSetupReceipt(installationID:identity.0,deviceID:identity.1,expiresAt:nil),behavior:identity.2)
        do { _ = try await invokeSetup(MacHostChannel(transport:transport,ownerSetup:control),bytes:complete); Issue.record("Setup must remain unconfirmed") }
        catch {
            let error = error as NSError
            #expect(error.domain == "Fort.OwnerSetup" && error.code == 2)
            #expect(!error.localizedDescription.contains("synthetic-secret"))
        }
        #expect(await control.calls == 1)
    }
}

private actor SetupReplies { var count = 0; func received() { count += 1 } }
@Test func nativeXPCOwnerSetupAdmitsOneOperationAndCancelsOnInvalidation() async throws {
    let transport = SetupTransportFixture(), control = SetupControlFixture(receipt:try OwnerSetupReceipt(installationID:"installation",deviceID:"device",expiresAt:nil),behavior:"wait")
    let replies = SetupReplies(), channel = MacHostChannel(transport:transport,ownerSetup:control)
    let bytes = Data(#"{"operation":"begin","timeout":3}"#.utf8)
    channel.setupOwner(bytes) { _,_ in Task { await replies.received() } }
    for _ in 0..<100 where await control.calls == 0 { try await Task.sleep(for:.milliseconds(5)) }
    #expect(await control.calls == 1)
    await #expect(throws:(any Error).self) { try await invokeSetup(channel,bytes:bytes) }
    channel.cancel()
    for _ in 0..<100 where await !control.canceled { try await Task.sleep(for:.milliseconds(5)) }
    #expect(await control.canceled)
    #expect(await replies.count == 0)
    #expect(await transport.calls == 0)
}
#endif
