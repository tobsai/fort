#if DEBUG && os(macOS)
import Foundation
import SwiftUI

private let phoneFixtureToken = Data(repeating:2,count:32).base64EncodedString().replacingOccurrences(of:"=",with:"")
private let phoneFixtureCode = Data(repeating:1,count:32).base64EncodedString().replacingOccurrences(of:"=",with:"")
private struct PhoneFixtureCredentials: DeviceCredentials {
    func save(_ connection: DeviceConnection, token: String) throws {
        guard token == phoneFixtureToken else { throw ClientError.invalidResponse }
    }
    func token(for connection: DeviceConnection) throws -> String { phoneFixtureToken }
    func remove(_ connection: DeviceConnection) throws {}
}
@MainActor public enum CompanionSetupFixture {
    public static func make(directory: URL, expired: Bool = false) async throws -> MessengerModel {
        let root = directory.standardizedFileURL, temporary = FileManager.default.temporaryDirectory.resolvingSymlinksInPath()
        guard root.deletingLastPathComponent().resolvingSymlinksInPath() == temporary,
              root.lastPathComponent.hasPrefix("FortPhoneQA-"), UUID(uuidString:String(root.lastPathComponent.dropFirst(12))) != nil,
              (try? root.resourceValues(forKeys:[.isSymbolicLinkKey]).isSymbolicLink) != true else { throw ClientError.invalidResponse }
        let storage = try DeviceStorage(directory:root,credentials:PhoneFixtureCredentials())
        let responder = PhoneFixtureResponder(expired:expired)
        let coordinator = CompanionSetupCoordinator(storage:storage,tokenGenerator:{ phoneFixtureToken },transport:{ _ in responder })
        let prepared = try coordinator.pendingSetup()
        let saved = try storage.connection()
        if let connection = prepared?.connection ?? saved {
            guard connection.installationID == "phone-fixture", connection.role == .companion,
                  connection.endpoint == .network(baseURL:URL(string:"https://phone.fixture.invalid")!,fixtureLoopback:false) else { throw ClientError.invalidResponse }
            await responder.restore(deviceID:connection.deviceID)
        }
        let model = MessengerModel(storage:storage,session:.shared,companionSetup:coordinator,connectionTransport:{ _ in responder })
        model.companionAddress = "https://phone.fixture.invalid"; model.companionInstallationID = "phone-fixture"
        model.companionInvitationID = "synthetic-invitation"; model.companionDeviceName = "Synthetic phone"
        if prepared == nil && saved == nil { model.companionInvitationCode = phoneFixtureCode }
        await model.restore()
        return model
    }
}
public struct CompanionSetupFixtureView: View {
    private let model: MessengerModel
    public init(model: MessengerModel) { self.model = model }
    public var body: some View {
        NavigationStack {
            VStack(spacing:0) {
                Text("Synthetic phone fixture · no network or provider").font(.caption).padding(8)
                if model.pendingCompanionSetup != nil || !model.isCompanionPairing {
                    Form { CompanionSetupView(model:model) }.formStyle(.grouped)
                } else { PairingSettingsView(model:model) }
            }.navigationTitle("Fort QA — Phone controls (synthetic)")
        }
    }
}
private actor PhoneFixtureResponder: FortTransport {
    private var deviceID: String?
    private let expired: Bool
    init(expired: Bool) { self.expired = expired }
    func restore(deviceID: String) { self.deviceID = deviceID }
    func send(_ request: FortRequest) throws -> FortResponse {
        guard request.installationID == "phone-fixture" else { throw ClientError.invalidResponse }
        if request.method == .post && request.path == "/v1/devices/join" {
            let input = try JSONDecoder().decode([String:String].self,from:request.body ?? Data())
            guard input["code"] == phoneFixtureCode, input["invitationId"] == "synthetic-invitation",
                  input["deviceName"] == "Synthetic phone", input["deviceToken"] == phoneFixtureToken,
                  let device = input["deviceId"], deviceID == nil || deviceID == device else { throw ClientError.invalidResponse }
            deviceID = device
            throw URLError(.networkConnectionLost)
        }
        guard request.token == phoneFixtureToken, let deviceID else { throw ClientError.invalidResponse }
        if expired && request.path != "/v1/pairing/leave" { return FortResponse(status:401,body:Data()) }
        let body: [String:Any]
        switch (request.method,request.path) {
        case (.get,"/v1/installation"):
            body = ["contractVersion":"1.0","installationId":"phone-fixture","recoveryEpoch":"synthetic-epoch","mode":"local","serviceName":"Synthetic phone fixture","capabilities":["ownerPairing","companionPairing"]]
        case (.get,"/v1/pairing"): body = ["deviceId":deviceID,"mode":"owner-local","expiresAt":NSNull(),"role":"companion"]
        case (.get,"/v1/agents"): body = ["agents":[]]
        case (.get,"/v1/conversations"): body = ["conversations":[]]
        case (.get,"/v1/skills"): body = ["skills":[]]
        case (.post,"/v1/pairing/leave"): body = ["installationId":"phone-fixture","deviceId":deviceID,"revoked":true,"scope":"companion-only"]
        default: throw ClientError.invalidResponse
        }
        return FortResponse(status:200,body:try JSONSerialization.data(withJSONObject:body,options:.sortedKeys))
    }
}
#endif
