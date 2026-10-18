import Foundation
import Darwin

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
    func api(_ path: String, installation: String = "synthetic", token: String? = nil, body: [String:Any]? = nil, timeout: TimeInterval = 3) async throws -> (Int,[String:Any]) {
        let bytes = try body.map { try JSONSerialization.data(withJSONObject:$0) }
        let result = try await transport.send(FortRequest(path:path,method:body == nil ? .get : .post,installationID:installation,token:token,body:bytes,timeout:timeout))
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
    guard let crashedHelper = afterRestart.1["helperPID"] as? Int32,
          let crashedGroup = afterRestart.1["pid"] as? Int32,
          let crashedRuntime = afterRestart.1["runtimePID"] as? Int32 else { throw ClientError.invalidResponse }
    let crashStarted = ContinuousClock.now
    var crashObserved = false
    do { _ = try await api("/v1/fixture-crash",body:[:]) } catch { crashObserved = true }
    func absent(_ pid: Int32) -> Bool { kill(pid,0) == -1 && errno == ESRCH }
    let crashedProcesses = [crashedHelper,crashedGroup,crashedRuntime,-crashedGroup]
    while !crashedProcesses.allSatisfy(absent) && ContinuousClock.now < crashStarted.advanced(by:.seconds(3)) {
        try await Task.sleep(for:.milliseconds(10))
    }
    let cleanup = crashStarted.duration(to:.now).components
    let crashCleanupMs = Double(cleanup.seconds) * 1000 + Double(cleanup.attoseconds) / 1e15
    guard crashObserved, crashedProcesses.allSatisfy(absent), crashCleanupMs < 3000 else { throw ClientError.invalidResponse }
    let restartStarted = ContinuousClock.now
    let afterCrash = try await api("/v1/fixture-bootstrap",timeout:12)
    let restart = restartStarted.duration(to:.now).components
    let restartMs = Double(restart.seconds) * 1000 + Double(restart.attoseconds) / 1e15
    let crashDuplicate = try await api("/v1/conversations/" + home + "/messages",installation:installation,token:token,body:command)
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
    let controls = try object(setup["controls"]), restartedControls = try object(restarted.1["controls"]), crashControls = try object(afterCrash.1["controls"])
    guard let firstPID = setup["pid"] as? Int, let secondPID = restarted.1["pid"] as? Int,
          let firstRuntime = setup["runtimePID"] as? Int, let secondRuntime = restarted.1["runtimePID"] as? Int,
          let thirdPID = afterCrash.1["pid"] as? Int, let thirdRuntime = afterCrash.1["runtimePID"] as? Int,
          let helperPID = setup["helperPID"] as? Int, let recoveredHelper = afterCrash.1["helperPID"] as? Int else { throw ClientError.invalidResponse }
    let crashReceipt = try object(crashDuplicate.1["receipt"])
    let crashMessages = try object(crashDuplicate.1["snapshot"])["messages"] as? [[String:Any]]
    let crashInstallation = try string(object(afterCrash.1["installation"])["installationId"])
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
            "xpcCoreAbruptRecovery":helperPID != recoveredHelper && thirdPID != firstPID && thirdPID != secondPID && crashInstallation == installation && crashDuplicate.0 == 201 && NSDictionary(dictionary:oldReceipt).isEqual(to:crashReceipt) && crashMessages?.compactMap { $0["text"] as? String } == [text,text] && afterCrash.1["executions"] as? Int == 0,
            "xpcCoreCrashCleanupMs":crashCleanupMs,
            "xpcCoreCrashRestartMs":restartMs,
            "xpcCoreOutsideReadDenied":[controls,restartedControls,crashControls].allSatisfy { $0["outsideReadDenied"] as? Bool == true },
            "xpcCoreOutsideWriteDenied":[controls,restartedControls,crashControls].allSatisfy { $0["outsideWriteDenied"] as? Bool == true },
            "xpcCoreTCPDenied":[controls,restartedControls,crashControls].allSatisfy { $0["tcpDenied"] as? Bool == true },
            "xpcCoreGuarded":[setup,restarted.1,afterCrash.1].allSatisfy { $0["guardedGroup"] as? Bool == true },
            "xpcCorePids":[firstPID,secondPID,thirdPID],"xpcCoreRuntimePids":[firstRuntime,secondRuntime,thirdRuntime],"xpcCoreServicePid":helperPID,"xpcCoreServicePids":[helperPID,recoveredHelper]]
}
