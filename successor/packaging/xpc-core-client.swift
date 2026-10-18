import Foundation

func probeCoreXPC() async throws -> [String:Any] {
    guard let identifier = Bundle.main.bundleIdentifier,
          let team = Bundle.main.object(forInfoDictionaryKey:"FortSigningTeam") as? String else { throw ClientError.invalidResponse }
    let transport = try MacXPCTransport(serviceIdentifier:identifier + ".core",signingTeam:team)
    func object(_ value: Any?) throws -> [String:Any] {
        guard let object = value as? [String:Any] else { throw ClientError.invalidResponse }; return object
    }
    func string(_ value: Any?) throws -> String {
        guard let string = value as? String else { throw ClientError.invalidResponse }; return string
    }
    func api(_ path: String, installation: String = "synthetic", token: String? = nil, body: [String:Any]? = nil) async throws -> (Int,[String:Any]) {
        let bytes = try body.map { try JSONSerialization.data(withJSONObject:$0) }
        let result = try await transport.send(FortRequest(path:path,method:body == nil ? .get : .post,installationID:installation,token:token,body:bytes,timeout:3))
        return try (result.status,object(JSONSerialization.jsonObject(with:result.body)))
    }
    let (_,setup) = try await api("/v1/fixture-bootstrap")
    let identity = try object(setup["installation"]), enrollment = try object(setup["enrollment"])
    let installation = try string(identity["installationId"]), epoch = try string(identity["recoveryEpoch"])
    let anonymous = try await api("/v1/installation",installation:installation)
    let enrolled = try await api("/v1/devices/enroll",installation:installation,body:["installationId":installation,"code":try string(enrollment["code"]),"deviceName":"Synthetic signed XPC"])
    let token = try string(enrolled.1["deviceToken"])
    let device = try string(enrolled.1["deviceId"])
    let foreign = try await api("/v1/installation",installation:"foreign",token:token)
    let agent = try await api("/v1/agents",installation:installation,token:token,body:["name":"Signed XPC Home","model":"exact-synthetic","instructions":"Echo."])
    let home = try string(agent.1["homeConversationId"]), agentID = try string(agent.1["agentId"])
    let binding = try string(object(agent.1["binding"])["bindingRevisionId"])
    let text = "  Exact signed cedar 🌲 Café e\u{301}\r\n"
    let command: [String:Any] = ["requestId":"signed-xpc-turn","text":text,"recipientAgentId":agentID,"bindingRevisionId":binding,"recoveryEpoch":epoch,"skills":[]]
    let accepted = try await api("/v1/conversations/" + home + "/messages",installation:installation,token:token,body:command)
    var snapshot = try await api("/v1/conversations/" + home + "/snapshot",installation:installation,token:token)
    for _ in 0..<50 where (snapshot.1["messages"] as? [[String:Any]])?.count != 2 {
        try await Task.sleep(for:.milliseconds(10))
        snapshot = try await api("/v1/conversations/" + home + "/snapshot",installation:installation,token:token)
    }
    let messages = snapshot.1["messages"] as? [[String:Any]]
    let events = try await api("/v1/events?after=0&recoveryEpoch=" + epoch,installation:installation,token:token)
    guard let cursor = events.1["cursor"] as? Int else { throw ClientError.invalidResponse }
    let waiting = Task {
        try await transport.send(FortRequest(path:"/v1/events?after=\(cursor)&recoveryEpoch=\(epoch)&wait=25000",method:.get,installationID:installation,token:token,body:nil,timeout:30))
    }
    try await Task.sleep(for:.milliseconds(50)); waiting.cancel()
    var canceled = false
    do { _ = try await waiting.value } catch is CancellationError { canceled = true }
    var beforeRestart = try await api("/v1/fixture-bootstrap")
    for _ in 0..<50 where beforeRestart.1["canceledRequests"] as? Int != 1 {
        try await Task.sleep(for:.milliseconds(10)); beforeRestart = try await api("/v1/fixture-bootstrap")
    }
    let restarted = try await api("/v1/fixture-restart",body:[:])
    let duplicate = try await api("/v1/conversations/" + home + "/messages",installation:installation,token:token,body:command)
    let afterRestart = try await api("/v1/fixture-bootstrap")
    let noHTTPRecovery = try await api("/v1/pairing/recover",installation:installation,token:token,body:[:])
    var foreignRecoveries = 0
    for identity in [("foreign",device),(installation,"foreign")] {
        do { _ = try await transport.recoverPairing(MacOwnerRecoveryRequest(installationID:identity.0,deviceID:identity.1,timeout:3)) }
        catch { foreignRecoveries += 1 }
    }
    let stillPaired = try await api("/v1/installation",installation:installation,token:token)
    let recovered = try await transport.recoverPairing(MacOwnerRecoveryRequest(installationID:installation,deviceID:device,timeout:3))
    let oldReceipt = try object(accepted.1["receipt"]), newReceipt = try object(duplicate.1["receipt"])
    let duplicateMessages = try object(duplicate.1["snapshot"])["messages"] as? [[String:Any]]
    let restartedInstallation = try string(object(restarted.1["installation"])["installationId"])
    let controls = try object(setup["controls"]), restartedControls = try object(restarted.1["controls"])
    guard let firstPID = setup["pid"] as? Int, let secondPID = restarted.1["pid"] as? Int,
          let helperPID = setup["helperPID"] as? Int else { throw ClientError.invalidResponse }
    return ["xpcCoreAuthentication":anonymous.0 == 401 && enrolled.0 == 201 && foreign.0 == 401,
            "xpcCoreExactMessages":accepted.0 == 201 && messages?.compactMap { $0["text"] as? String } == [text,text],
            "xpcCoreReplay":events.0 == 200 && events.1["semantics"] as? String == "invalidation" && cursor > 0,
            "xpcCoreCancellation":canceled && beforeRestart.1["canceledRequests"] as? Int == 1,
            "xpcCoreRestart":firstPID != secondPID && restartedInstallation == installation,
            "xpcCoreNoDuplicate":NSDictionary(dictionary:oldReceipt).isEqual(to:newReceipt) && duplicateMessages?.compactMap { $0["text"] as? String } == [text,text] && beforeRestart.1["executions"] as? Int == 1 && afterRestart.1["executions"] as? Int == 0,
            "xpcCoreStopped":recovered.coreStopped,"xpcCoreOuterExecutableDenied":setup["outerExecutableDenied"] as? Bool == true,
            "xpcOwnerRecovery":recovered.installationID == installation && recovered.deviceID == device && recovered.runtimeStopped && recovered.pairingRevoked && recovered.coreStopped,
            "xpcOwnerForeignRejected":foreignRecoveries == 2 && stillPaired.0 == 200,
            "xpcOwnerNoHTTPRecovery":noHTTPRecovery.0 == 404,
            "xpcCoreOutsideReadDenied":controls["outsideReadDenied"] as? Bool == true && restartedControls["outsideReadDenied"] as? Bool == true,
            "xpcCoreOutsideWriteDenied":controls["outsideWriteDenied"] as? Bool == true && restartedControls["outsideWriteDenied"] as? Bool == true,
            "xpcCoreTCPDenied":controls["tcpDenied"] as? Bool == true && restartedControls["tcpDenied"] as? Bool == true,
            "xpcCorePids":[firstPID,secondPID],"xpcCoreServicePid":helperPID]
}
