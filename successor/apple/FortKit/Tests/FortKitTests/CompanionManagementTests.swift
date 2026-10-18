import Foundation
import Testing
@testable import FortKit

private let managementCode = Data(repeating:7,count:32).base64EncodedString().replacingOccurrences(of:"=",with:"")
private let invitationReceipt = #"{"installationId":"one","invitationId":"invite","expiresAt":"2100-01-01T00:00:00.000Z"}"#
private let availableOverview = #"{"installationId":"one","hostDeviceId":"mac","pairing":{"state":"available"}}"#
private let revokedPhone = #"{"installationId":"one","deviceId":"phone","revoked":true,"scope":"companion-only"}"#
private actor ManagementTransport: FortTransport {
    private let replies: [String]
    private let lostReply: Bool
    private(set) var requests: [FortRequest] = []
    init(_ replies: [String], lostReply: Bool = false) { self.replies = replies; self.lostReply = lostReply }
    func send(_ request: FortRequest) throws -> FortResponse {
        requests.append(request)
        if lostReply { throw URLError(.networkConnectionLost) }
        guard replies.indices.contains(requests.count - 1) else { throw ClientError.invalidResponse }
        return FortResponse(status:200,body:Data(replies[requests.count - 1].utf8))
    }
}
@Test func nativeCompanionManagementUsesExactPublicRequestsAndReceipts() async throws {
    let transport = ManagementTransport([availableOverview,invitationReceipt,revokedPhone])
    let client = FortClient(installationID:"one",token:"synthetic-primary-only",transport:transport)
    #expect(await transport.requests.isEmpty)
    let overview = try await client.companionOverview(hostDeviceID:"mac")
    #expect(overview.installationId == "one" && overview.hostDeviceId == "mac" && overview.pairing == .available)
    let invitation = try await client.inviteCompanion(invitationID:"invite",code:managementCode)
    #expect(invitation.invitationId == "invite" && invitation.expiresAt == "2100-01-01T00:00:00.000Z")
    let revocation = try await client.revokeCompanion(deviceID:"phone")
    #expect(revocation.deviceId == "phone" && revocation.revoked && revocation.scope == "companion-only")
    let requests = await transport.requests
    #expect(requests.count == 3)
    #expect(requests.allSatisfy { $0.installationID == "one" && $0.token == "synthetic-primary-only" })
    #expect(requests[0].method == .get && requests[0].path == "/v1/pairing/companion" && requests[0].body == nil)
    #expect(requests[1].method == .post && requests[1].path == "/v1/pairing/invitation")
    #expect(try JSONDecoder().decode([String:String].self,from:requests[1].body ?? Data()) == ["invitationId":"invite","code":managementCode])
    #expect(requests[2].method == .post && requests[2].path == "/v1/pairing/revoke-companion")
    #expect(try JSONDecoder().decode([String:String].self,from:requests[2].body ?? Data()) == ["deviceId":"phone"])
}

@Test(arguments:[
    #"{"state":"available","deviceId":"unexpected"}"#,
    #"{"state":"unknown"}"#,
    #"{"state":"invited","invitationId":"bad/id","expiresAt":"2100-01-01T00:00:00.000Z"}"#,
    #"{"state":"invited","invitationId":"invite","expiresAt":"not-a-date"}"#,
    #"{"state":"invited","invitationId":"invite","expiresAt":"2100-02-30T00:00:00.000Z"}"#,
    #"{"state":"invited","invitationId":"invite","expiresAt":"2100-01-01T00:00:00.000Z","code":"unexpected"}"#,
    #"{"state":"paired","deviceId":"phone","deviceName":"Phone","active":true}"#,
    #"{"state":"paired","deviceId":"phone","deviceName":"Phone","expiresAt":null,"active":false}"#,
    #"{"state":"paired","deviceId":"mac","deviceName":"Phone","expiresAt":null,"active":true}"#,
    #"{"state":"paired","deviceId":"bad/id","deviceName":"Phone","expiresAt":null,"active":true}"#,
    #"{"state":"paired","deviceId":"phone","deviceName":" ","expiresAt":null,"active":true}"#,
    #"{"state":"paired","deviceId":"phone","deviceName":"Phone","expiresAt":"invalid","active":true}"#,
    #"{"state":"paired","deviceId":"phone","deviceName":"Phone","expiresAt":null,"active":"true"}"#
]) func nativeCompanionManagementRejectsMalformedOverviewStates(pairing: String) async throws {
    let transport = ManagementTransport(["{\"installationId\":\"one\",\"hostDeviceId\":\"mac\",\"pairing\":\(pairing)}"])
    let client = FortClient(installationID:"one",token:"synthetic-primary-only",transport:transport)
    await #expect(throws:(any Error).self) { try await client.companionOverview(hostDeviceID:"mac") }
    #expect(await transport.requests.count == 1)
}
@Test(arguments:[
    #"{"installationId":"other","hostDeviceId":"mac","pairing":{"state":"available"}}"#,
    #"{"installationId":"one","hostDeviceId":"other","pairing":{"state":"available"}}"#,
    #"{"installationId":"one","hostDeviceId":"mac","pairing":{"state":"available"},"code":"unexpected"}"#
]) func nativeCompanionManagementRejectsForeignOrExpandedOverview(reply: String) async throws {
    let transport = ManagementTransport([reply])
    let selected = FortClient(installationID:"one",token:"synthetic-primary-only",transport:transport)
    await #expect(throws:(any Error).self) { try await selected.companionOverview(hostDeviceID:"mac") }
}
@Test(arguments:[
    #"{"installationId":"other","invitationId":"invite","expiresAt":"2100-01-01T00:00:00.000Z"}"#,
    #"{"installationId":"one","invitationId":"other","expiresAt":"2100-01-01T00:00:00.000Z"}"#,
    #"{"installationId":"one","invitationId":"invite","expiresAt":"tomorrow"}"#,
    #"{"installationId":"one","invitationId":"invite","expiresAt":"2100-02-30T00:00:00.000Z"}"#,
    #"{"installationId":"one","invitationId":"invite","expiresAt":"2100-01-01T00:00:00.000Z","code":"unexpected"}"#
]) func nativeCompanionManagementRejectsWrongInvitationReceipts(reply: String) async throws {
    let transport = ManagementTransport([reply])
    let selected = FortClient(installationID:"one",token:"synthetic-primary-only",transport:transport)
    await #expect(throws:(any Error).self) { try await selected.inviteCompanion(invitationID:"invite",code:managementCode) }
}
@Test(arguments:[
    #"{"installationId":"other","deviceId":"phone","revoked":true,"scope":"companion-only"}"#,
    #"{"installationId":"one","deviceId":"other","revoked":true,"scope":"companion-only"}"#,
    #"{"installationId":"one","deviceId":"phone","revoked":false,"scope":"companion-only"}"#,
    #"{"installationId":"one","deviceId":"phone","revoked":true,"scope":"all-devices"}"#
]) func nativeCompanionManagementRequiresExactPhoneRevocation(reply: String) async throws {
    let transport = ManagementTransport([reply])
    let client = FortClient(installationID:"one",token:"synthetic-primary-only",transport:transport)
    await #expect(throws:OwnerLifecycleError.self) { try await client.revokeCompanion(deviceID:"phone") }
}
@Test(arguments:["", "bad/id", "white space", "é", String(repeating:"a",count:129)])
func nativeCompanionManagementRejectsInvalidIdentifiersBeforeTransport(id: String) async throws {
    let transport = ManagementTransport([])
    let selected = FortClient(installationID:"one",token:"synthetic-primary-only",transport:transport)
    await #expect(throws:ClientError.self) { try await selected.companionOverview(hostDeviceID:id) }
    await #expect(throws:ClientError.self) { try await selected.inviteCompanion(invitationID:id,code:managementCode) }
    await #expect(throws:ClientError.self) { try await selected.revokeCompanion(deviceID:id) }
    #expect(await transport.requests.isEmpty)
}
@Test func nativeCompanionManagementRejectsInvalidCodesAndInstallationBeforeTransport() async throws {
    let transport = ManagementTransport([])
    let client = FortClient(installationID:"one",token:"synthetic-primary-only",transport:transport)
    await #expect(throws:ClientError.self) { try await client.inviteCompanion(invitationID:"invite",code:"short") }
    let foreign = FortClient(installationID:"bad/id",token:"synthetic-primary-only",transport:transport)
    await #expect(throws:ClientError.self) { try await foreign.companionOverview(hostDeviceID:"mac") }
    await #expect(throws:ClientError.self) { try await foreign.inviteCompanion(invitationID:"invite",code:managementCode) }
    await #expect(throws:ClientError.self) { try await foreign.revokeCompanion(deviceID:"phone") }
    #expect(await transport.requests.isEmpty)
}
@Test func nativeCompanionManagementKeepsLostResponsesUnconfirmedWithoutRetry() async throws {
    let transport = ManagementTransport([],lostReply:true)
    let client = FortClient(installationID:"one",token:"synthetic-primary-only",transport:transport)
    await #expect(throws:URLError.self) { try await client.companionOverview(hostDeviceID:"mac") }
    await #expect(throws:URLError.self) { try await client.inviteCompanion(invitationID:"invite",code:managementCode) }
    await #expect(throws:URLError.self) { try await client.revokeCompanion(deviceID:"phone") }
    #expect(await transport.requests.count == 3)
}
@Test func nativeCompanionManagementDecodesInvitationAndActiveOrExpiredPhone() async throws {
    let invited = #"{"installationId":"one","hostDeviceId":"mac","pairing":{"state":"invited","invitationId":"invite","expiresAt":"2100-01-01T00:00:00.000Z"}}"#
    let active = #"{"installationId":"one","hostDeviceId":"mac","pairing":{"state":"paired","deviceId":"phone","deviceName":"My phone","expiresAt":null,"active":true}}"#
    let expired = #"{"installationId":"one","hostDeviceId":"mac","pairing":{"state":"paired","deviceId":"phone","deviceName":"My phone","expiresAt":"2020-01-01T00:00:00.000Z","active":false}}"#
    let transport = ManagementTransport([invited,active,expired])
    let client = FortClient(installationID:"one",token:"synthetic-primary-only",transport:transport)
    #expect(try await client.companionOverview(hostDeviceID:"mac").pairing == .invited(invitationID:"invite",expiresAt:"2100-01-01T00:00:00.000Z"))
    #expect(try await client.companionOverview(hostDeviceID:"mac").pairing == .paired(deviceID:"phone",deviceName:"My phone",expiresAt:nil,active:true))
    #expect(try await client.companionOverview(hostDeviceID:"mac").pairing == .paired(deviceID:"phone",deviceName:"My phone",expiresAt:"2020-01-01T00:00:00.000Z",active:false))
}

@Test(arguments:[64,65]) func nativeCompanionManagementChecksNameBytesWithoutChangingUnicode(count: Int) async throws {
    let name = String(repeating:"é",count:count)
    let reply = "{\"installationId\":\"one\",\"hostDeviceId\":\"mac\",\"pairing\":{\"state\":\"paired\",\"deviceId\":\"phone\",\"deviceName\":\"\(name)\",\"expiresAt\":null,\"active\":true}}"
    let transport = ManagementTransport([reply])
    let selected = FortClient(installationID:"one",token:"synthetic-primary-only",transport:transport)
    if count == 64 {
        #expect(try await selected.companionOverview(hostDeviceID:"mac").pairing == .paired(deviceID:"phone",deviceName:name,expiresAt:nil,active:true))
    } else { await #expect(throws:ClientError.self) { try await selected.companionOverview(hostDeviceID:"mac") } }
}
@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_NATIVE_COMPANION_MANAGEMENT_BENCHMARK"] != nil))
func nativeCompanionManagementMeetsItsIsolatedBudget() async throws {
    var samples: [Double] = []
    for index in 0..<25 {
        let transport = ManagementTransport([availableOverview,invitationReceipt,revokedPhone])
        let client = FortClient(installationID:"one",token:"synthetic-primary-only",transport:transport)
        let started = ContinuousClock.now
        _ = try await client.companionOverview(hostDeviceID:"mac")
        _ = try await client.inviteCompanion(invitationID:"invite",code:managementCode)
        _ = try await client.revokeCompanion(deviceID:"phone")
        #expect(await transport.requests.count == 3)
        let elapsed = started.duration(to:.now).components
        if index >= 5 { samples.append(Double(elapsed.seconds) * 1000 + Double(elapsed.attoseconds) / 1e15) }
    }
    samples.sort(); #expect(samples[18] < 100)
    let result: [String:Any] = ["warmups":5,"samples":samples,"p50":samples[9],"p95":samples[18],"maximum":samples[19],"budgetMs":100]
    let destination = try #require(ProcessInfo.processInfo.environment["FORT_NATIVE_COMPANION_MANAGEMENT_BENCHMARK"])
    try JSONSerialization.data(withJSONObject:result,options:.sortedKeys).write(to:URL(fileURLWithPath:destination),options:.withoutOverwriting)
}
