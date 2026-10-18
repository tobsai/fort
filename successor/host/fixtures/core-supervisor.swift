import Foundation
import Darwin

@main private enum SupervisorFixture {
    static func main() async {
        do {
            let arguments = CommandLine.arguments
            guard arguments.count == 6 else { exit(2) }
            let directory = URL(fileURLWithPath:arguments[3])
            let started = ContinuousClock.now
            let transport = try await MacCoreProcessTransport.launch(
                executable:URL(fileURLWithPath:arguments[1]),
                arguments:["--jitless","--no-warnings",arguments[2],arguments[4]],
                directory:directory,environment:["PATH":"/usr/bin:/bin"],
                lifecycle:arguments[5] == "unguarded" ? .processGroup : .parentExit(guardian:URL(fileURLWithPath:arguments[5])))
            let elapsed = started.duration(to:.now).components
            let startupMs = Double(elapsed.seconds) * 1000 + Double(elapsed.attoseconds) / 1e15
            try JSONSerialization.data(withJSONObject:["supervisor":getpid(),"group":transport.processID,"startupMs":startupMs])
                .write(to:directory.appendingPathComponent("supervisor-ready.json"))
            if arguments[4] == "normal" {
                let response = try await transport.send(FortRequest(path:"/v1/echo",method:.get,installationID:"fixture",token:nil,body:nil,timeout:2))
                let stopped = try await transport.close()
                try JSONSerialization.data(withJSONObject:["text":String(decoding:response.body,as:UTF8.self),"clean":stopped.exitedCleanly,"forced":stopped.forced])
                    .write(to:directory.appendingPathComponent("closed.json"))
                return
            }
            try await Task.sleep(for:.seconds(15))
            _ = try await transport.close()
            exit(3)
        } catch { exit(1) }
    }
}
