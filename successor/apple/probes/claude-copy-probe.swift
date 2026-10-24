// Sandboxed probe: copies the owner's claude binary into this sandbox container, as Fort will,
// then runs the copy's `--version` and `auth status --json`. No prompt is sent and no token is read.
// Usage: claude-copy-probe <claude binary>
import Foundation

let source = URL(fileURLWithPath:CommandLine.arguments[1]).resolvingSymlinksInPath()
let home = ProcessInfo.processInfo.environment["HOME"] ?? ""
let directory = URL(fileURLWithPath:home + "/ClaudeCode"), copy = directory.appendingPathComponent("claude"), temporary = home + "/tmp"
try? FileManager.default.createDirectory(atPath:temporary,withIntermediateDirectories:true)
try? FileManager.default.removeItem(at:copy)
try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:true,attributes:[.posixPermissions:0o700])
try Data(contentsOf:source).write(to:copy)
try FileManager.default.setAttributes([.posixPermissions:0o700],ofItemAtPath:copy.path)
let quarantine = getxattr(copy.path,"com.apple.quarantine",nil,0,0,0) >= 0
print("copied bytes:", (try? FileManager.default.attributesOfItem(atPath:copy.path)[.size]) ?? "?", "quarantined:", quarantine)

func run(_ arguments: [String]) {
    let p = Process(), output = Pipe()
    p.executableURL = copy; p.arguments = arguments; p.standardOutput = output; p.standardError = output
    p.environment = ["PATH":"/usr/bin:/bin","HOME":home,"TMPDIR":temporary,"XDG_RUNTIME_DIR":temporary,"DISABLE_AUTOUPDATER":"1","CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC":"1"]
    do { try p.run(); p.waitUntilExit() } catch { print(arguments.joined(separator:" "), "launch error:", error); return }
    let text = String(decoding:output.fileHandleForReading.readDataToEndOfFile(),as:UTF8.self)
    // Print only non-identifying fields of the status.
    if arguments.first == "auth", let object = try? JSONSerialization.jsonObject(with:Data(text.utf8)) as? [String:Any] {
        print("auth status exit", p.terminationStatus, "loggedIn", object["loggedIn"] ?? "nil", "authMethod", object["authMethod"] ?? "nil", "subscriptionType", object["subscriptionType"] ?? "nil")
    } else { print(arguments.joined(separator:" "), "exit", p.terminationStatus, "output:", text.prefix(200)) }
}
run(["--version"])
run(["auth","status","--json"])
