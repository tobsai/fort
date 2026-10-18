#if os(macOS)
import Foundation
import Testing
@testable import FortKit

private actor OwnerTransportFixture: FortTransport {
    var calls = 0
    func send(_ request: FortRequest) async throws -> FortResponse { calls += 1; return FortResponse(status:200,body:Data()) }
}
private actor OwnerControlFixture: MacOwnerControl {
    let proof: MacOwnerRecoveryReceipt
    let wait: Bool
    var calls = 0
    var canceled = false
    init(proof: MacOwnerRecoveryReceipt, wait: Bool = false) { self.proof = proof; self.wait = wait }
    func recoverPairing(_ request: MacOwnerRecoveryRequest) async throws -> MacOwnerRecoveryReceipt {
        calls += 1
        if wait {
            do { try await Task.sleep(for:.seconds(10)) }
            catch { canceled = true; throw error }
        }
        return proof
    }
}
private func ownerProof(installation: String = "installation", device: String = "device", runtime: Bool = true, revoked: Bool = true, core: Bool = true) -> MacOwnerRecoveryReceipt {
    MacOwnerRecoveryReceipt(installationID:installation,deviceID:device,runtimeStopped:runtime,pairingRevoked:revoked,coreStopped:core)
}
private func invokeOwner(_ channel: MacHostChannel, bytes: Data) async throws -> Data {
    try await withCheckedThrowingContinuation { continuation in
        channel.recoverPairing(bytes) { data,error in
            if let error { continuation.resume(throwing:error) }
            else if let data { continuation.resume(returning:data) }
            else { continuation.resume(throwing:ClientError.invalidResponse) }
        }
    }
}

@Test func nativeOwnerChannelRequiresExplicitControlAndExactCompleteProof() async throws {
    let transport = OwnerTransportFixture()
    let request = try MacOwnerRecoveryRequest(installationID:"installation",deviceID:"device")
    let bytes = try JSONEncoder().encode(request)
    await #expect(throws:(any Error).self) { try await invokeOwner(MacHostChannel(transport:transport),bytes:bytes) }
    for proof in [ownerProof(installation:"foreign"), ownerProof(device:"foreign"), ownerProof(runtime:false), ownerProof(revoked:false), ownerProof(core:false)] {
        let control = OwnerControlFixture(proof:proof)
        await #expect(throws:(any Error).self) { try await invokeOwner(MacHostChannel(transport:transport,ownerControl:control),bytes:bytes) }
        #expect(await control.calls == 1)
    }
    let control = OwnerControlFixture(proof:ownerProof())
    let accepted = try await invokeOwner(MacHostChannel(transport:transport,ownerControl:control),bytes:bytes)
    let receipt = try JSONDecoder().decode(MacOwnerRecoveryReceipt.self,from:accepted)
    try receipt.validate(for:request)
    #expect(await transport.calls == 0)
}

@Test func nativeOwnerChannelRejectsMalformedAndOversizedRequestsBeforeCallingControl() async throws {
    let transport = OwnerTransportFixture(), control = OwnerControlFixture(proof:ownerProof())
    let request = try MacOwnerRecoveryRequest(installationID:"installation",deviceID:"device")
    for bytes in [Data(), Data(repeating:65,count:1025),
        Data(#"{"installationID":"installation","deviceID":"device","timeout":31}"#.utf8),
        Data(#"{"installationID":"installation","deviceID":"device\n","timeout":1}"#.utf8)] {
        await #expect(throws:(any Error).self) { try await invokeOwner(MacHostChannel(transport:transport,ownerControl:control),bytes:bytes) }
    }
    #expect(throws:ClientError.self) { try MacOwnerRecoveryRequest(installationID:"wrong/path",deviceID:"device") }
    #expect(throws:ClientError.self) { try MacOwnerRecoveryRequest(installationID:"installation",deviceID:String(repeating:"a",count:129)) }
    #expect(throws:ClientError.self) { try MacOwnerRecoveryRequest(installationID:"installation",deviceID:"device",timeout:0) }
    #expect(await control.calls == 0)
    _ = try await invokeOwner(MacHostChannel(transport:transport,ownerControl:control),bytes:JSONEncoder().encode(request))
    #expect(await control.calls == 1)
}

private actor OwnerReplies { var count = 0; func received() { count += 1 } }
@Test func nativeOwnerChannelAdmitsOneOperationAndCancelsOnInvalidation() async throws {
    let transport = OwnerTransportFixture(), control = OwnerControlFixture(proof:ownerProof(),wait:true), replies = OwnerReplies()
    let channel = MacHostChannel(transport:transport,ownerControl:control)
    let request = try MacOwnerRecoveryRequest(installationID:"installation",deviceID:"device")
    channel.recoverPairing(try JSONEncoder().encode(request)) { _,_ in Task { await replies.received() } }
    for _ in 0..<100 where await control.calls == 0 { try await Task.sleep(for:.milliseconds(5)) }
    #expect(await control.calls == 1)
    await #expect(throws:(any Error).self) { try await invokeOwner(channel,bytes:JSONEncoder().encode(request)) }
    channel.cancel()
    for _ in 0..<100 where await !control.canceled { try await Task.sleep(for:.milliseconds(5)) }
    #expect(await control.canceled)
    #expect(await replies.count == 0)
    #expect(await transport.calls == 0)
}
#endif
