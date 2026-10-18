import Foundation
import Darwin

private actor FixtureCore: FortTransport {
    private let directory: URL
    private let bundle: URL
    private var core: MacCoreProcessTransport?
    private var restarting = false
    private var canceledRequests = 0
    private let outerExecutableDenied: Bool
    init(directory: URL, bundle: URL, outerExecutableDenied: Bool) { self.directory = directory; self.bundle = bundle; self.outerExecutableDenied = outerExecutableDenied }
    func start() async throws {
        let resources = bundle.appendingPathComponent("Contents/Resources")
        guard let outside = Bundle.main.object(forInfoDictionaryKey:"FortOutsideMarker") as? String else { throw ClientError.invalidResponse }
        core = try await MacCoreProcessTransport.launch(executable:bundle.appendingPathComponent("Contents/MacOS/node"),arguments:["--jitless","--no-experimental-strip-types","--no-warnings","--openssl-config=" + resources.appendingPathComponent("node-openssl.cnf").path,resources.appendingPathComponent("native-core.mjs").path,directory.path,resources.appendingPathComponent("Core/service/openai.js").path,resources.appendingPathComponent("Core/host/private-channel.js").path,outside],directory:directory,environment:["PATH":"/usr/bin:/bin","HOME":directory.path,"TMPDIR":directory.path])
    }
    func send(_ request: FortRequest) async throws -> FortResponse {
        guard let core, !restarting else { throw URLError(.resourceUnavailable) }
        if request.path == "/v1/fixture-bootstrap" { return try bootstrap() }
        if request.path == "/v1/fixture-restart" {
            restarting = true
            let stopped = try await core.close()
            guard stopped.exitedCleanly && !stopped.forced else { throw ClientError.invalidResponse }
            self.core = nil
            try await start(); restarting = false
            return try bootstrap()
        }
        if request.path == "/v1/fixture-stop" {
            restarting = true
            let stopped = try await core.close(); self.core = nil
            let clean = stopped.exitedCleanly && !stopped.forced && !FileManager.default.fileExists(atPath:directory.appendingPathComponent("core.sock").path)
            let response = FortResponse(status:200,body:try JSONSerialization.data(withJSONObject:["clean":clean,"pid":core.processID]))
            DispatchQueue.global().asyncAfter(deadline:.now() + 0.2) { exit(clean ? 0 : 1) }
            return response
        }
        do { return try await core.send(request) }
        catch is CancellationError { canceledRequests += 1; throw CancellationError() }
    }
    private func bootstrap() throws -> FortResponse {
        guard let core,
              var setup = try JSONSerialization.jsonObject(with:Data(contentsOf:directory.appendingPathComponent("setup.json"))) as? [String:Any] else { throw ClientError.invalidResponse }
        setup["pid"] = core.processID; setup["helperPID"] = getpid()
        setup["canceledRequests"] = canceledRequests
        setup["outerExecutableDenied"] = outerExecutableDenied
        setup["executions"] = try JSONDecoder().decode(Int.self,from:Data(contentsOf:directory.appendingPathComponent("executions.json")))
        return FortResponse(status:200,body:try JSONSerialization.data(withJSONObject:setup))
    }
    func close() async { if let core { _ = try? await core.close() }; core = nil }
}

private struct StartingFixture: FortTransport {
    let ready: Task<FixtureCore,any Error>
    func send(_ request: FortRequest) async throws -> FortResponse { try await ready.value.send(request) }
}

@main private enum ServiceMain {
    static func main() {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-xpc-core-" + UUID().uuidString)
        do {
            guard let client = Bundle.main.object(forInfoDictionaryKey:"FortClientIdentifier") as? String,
                  let team = Bundle.main.object(forInfoDictionaryKey:"FortSigningTeam") as? String else { throw ClientError.invalidResponse }
            try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
            let bundle = Bundle.main.bundleURL
            let outer = bundle.deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            let ready = Task<FixtureCore,any Error> {
                var denied = false
                do {
                    let forbidden = try await MacCoreProcessTransport.launch(executable:outer.appendingPathComponent("Contents/MacOS/node"),arguments:["--version"],directory:directory,environment:["PATH":"/usr/bin:/bin"])
                    _ = try await forbidden.close()
                } catch { let error = error as NSError; denied = error.domain == NSPOSIXErrorDomain && error.code == Int(EPERM) }
                let core = FixtureCore(directory:directory,bundle:bundle,outerExecutableDenied:denied)
                try await core.start(); return core
            }
            let delegate = try MacHostListener(clientIdentifier:client,signingTeam:team,transport:StartingFixture(ready:ready))
            Task {
                try? await Task.sleep(for:.seconds(12))
                if let core = try? await ready.value { await core.close() }; exit(2)
            }
            let listener = NSXPCListener.service(); listener.delegate = delegate
            withExtendedLifetime(delegate) { listener.resume() }
        } catch {
            let error = error as NSError
            if let data = try? JSONSerialization.data(withJSONObject:["domain":error.domain,"code":error.code]) {
                try? data.write(to:directory.appendingPathComponent("failure.json"))
            }
            exit(1)
        }
    }
}
