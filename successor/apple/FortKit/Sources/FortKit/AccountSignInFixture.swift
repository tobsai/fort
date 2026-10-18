#if DEBUG && os(macOS)
import Foundation
import SwiftUI

private struct AccountFixtureCredentials: DeviceCredentials {
    func save(_ connection: DeviceConnection, token: String) throws {}
    func token(for connection: DeviceConnection) throws -> String { "synthetic-account-token" }
    func remove(_ connection: DeviceConnection) throws {}
}
@MainActor public enum AccountSignInFixture {
    public static func make(directory: URL) async throws -> MessengerModel {
        let root = directory.standardizedFileURL, temporary = FileManager.default.temporaryDirectory.resolvingSymlinksInPath()
        guard root.deletingLastPathComponent().resolvingSymlinksInPath() == temporary,
              root.lastPathComponent.hasPrefix("FortAccountQA-"), UUID(uuidString:String(root.lastPathComponent.dropFirst(14))) != nil,
              (try? root.resourceValues(forKeys:[.isSymbolicLinkKey]).isSymbolicLink) != true else { throw ClientError.invalidResponse }
        let storage = try DeviceStorage(directory:root,credentials:AccountFixtureCredentials())
        let endpoint = DeviceEndpoint.macAgent(try MacAgentEndpoint(serviceIdentifier:"io.fort.synthetic.account",signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic.account"))
        let connection = try DeviceConnection(endpoint:endpoint,installationID:"account-fixture",deviceID:"synthetic-mac")
        if let existing = try storage.connection() { guard existing == connection else { throw ClientError.invalidResponse } }
        else { try storage.save(connection,token:"synthetic-account-token") }
        let responder = AccountFixtureResponder(restoring:try storage.accountSignInIntent(for:connection)?.request)
        let model = try MessengerModel(storage:storage,session:.shared,ownerEndpoint:endpoint,ownerSetupControl:responder,connectionTransport:{ _ in responder })
        await model.restore()
        return model
    }
}
public struct AccountSignInFixtureView: View {
    private let model: MessengerModel
    public init(model: MessengerModel) { self.model = model }
    public var body: some View {
        NavigationStack {
            PairingSettingsView(model:model)
                .navigationTitle("Fort QA — Account controls (synthetic)")
        }
    }
}
private actor AccountFixtureResponder: FortTransport, OwnerSetupControl {
    private var current: [String:Any] = ["state":"idle","revision":0]
    init(restoring request: AccountSignInRequest?) {
        if let request { current = ["state":"finished","revision":request.expectedRevision + 1,"operationId":request.operationId,"expiresAt":request.expiresAt,"result":["kind":"unconfirmed"]] }
    }
    func beginOwnerSetup(timeout: TimeInterval) throws -> OwnerSetupState { throw ClientError.invalidResponse }
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) throws -> OwnerSetupReceipt { throw ClientError.invalidResponse }
    func send(_ request: FortRequest) throws -> FortResponse {
        guard request.installationID == "account-fixture", request.token == "synthetic-account-token" else { throw ClientError.invalidResponse }
        let body: [String:Any]
        switch (request.method,request.path) {
        case (.get,"/v1/installation"):
            body = ["contractVersion":"1.0","installationId":"account-fixture","recoveryEpoch":"synthetic-epoch","mode":"local","serviceName":"Synthetic account fixture",
                "capabilities":["ownerPairing","companionPairing","accountSignIn"],
                "runtime":["provider":"synthetic","name":"Synthetic account fixture · no provider","runtimeVersion":"fixture","hostId":"in-process","session":"synthetic","capabilities":[]]]
        case (.get,"/v1/pairing"): body = ["deviceId":"synthetic-mac","mode":"owner-local","expiresAt":NSNull()]
        case (.get,"/v1/agents"): body = ["agents":[]]
        case (.get,"/v1/conversations"): body = ["conversations":[]]
        case (.get,"/v1/skills"): body = ["skills":[]]
        case (.get,"/v1/account/sign-in"): body = current
        case (.post,"/v1/account/sign-in/start"):
            guard let data = request.body else { throw ClientError.invalidResponse }
            let input = try JSONDecoder().decode(AccountSignInRequest.self,from:data)
            if current["operationId"] as? String != input.operationId {
                guard input.expectedRevision == current["revision"] as? Int, input.recoveryEpoch == "synthetic-epoch",
                      input.expiresAt > Int64(Date().timeIntervalSince1970 * 1000), current["state"] as? String != "challenge" else { throw ClientError.invalidResponse }
                current = ["state":"challenge","revision":input.expectedRevision + 1,"operationId":input.operationId,"expiresAt":input.expiresAt,
                    "challenge":["kind":"deviceCode","verificationURL":"https://account.fixture.invalid/verify","userCode":"FORT-TEST"]]
            }
            body = current
        case (.post,"/v1/account/sign-in/cancel"):
            guard let data = request.body, let input = try JSONSerialization.jsonObject(with:data) as? [String:Any],
                  input["operationId"] as? String == current["operationId"] as? String, input["revision"] as? Int == current["revision"] as? Int,
                  input["recoveryEpoch"] as? String == "synthetic-epoch" else { throw ClientError.invalidResponse }
            current.removeValue(forKey:"challenge"); current["state"] = "finished"; current["result"] = ["kind":"canceled"]; body = current
        default: throw ClientError.invalidResponse
        }
        return FortResponse(status:200,body:try JSONSerialization.data(withJSONObject:body,options:.sortedKeys))
    }
}
#endif
