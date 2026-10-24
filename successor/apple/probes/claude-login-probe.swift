// Sandboxed probe: runs Claude Code's own claude.ai login over stream-json, then one fixed test turn.
// Fort never sees a token: the official binary keeps its sign-in inside this sandbox container.
// Usage: login-probe <claude binary> [--turn-only]
import AppKit
import Foundation

let exe = CommandLine.arguments[1], turnOnly = CommandLine.arguments.contains("--turn-only")
let home = ProcessInfo.processInfo.environment["HOME"] ?? ""
try? FileManager.default.createDirectory(atPath: home + "/tmp", withIntermediateDirectories: true)
let p = Process(); p.executableURL = URL(fileURLWithPath: exe)
p.environment = ["PATH": "/usr/bin:/bin", "HOME": home, "TMPDIR": home + "/tmp", "CLAUDE_CODE_TMPDIR": home + "/tmp", "XDG_RUNTIME_DIR": home + "/tmp",
  "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1", "DISABLE_AUTOUPDATER": "1", "CLAUDE_CODE_DISABLE_AUTO_MEMORY": "1"]
p.arguments = ["-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--tools", "", "--strict-mcp-config",
  "--setting-sources", "", "--disable-slash-commands", "--no-session-persistence", "--model", "claude-sonnet-5-5"]
let input = Pipe(), output = Pipe(); p.standardInput = input; p.standardOutput = output; p.standardError = FileHandle.nullDevice
func send(_ object: [String: Any]) {
  let data = try! JSONSerialization.data(withJSONObject: object)
  input.fileHandleForWriting.write(data + Data("\n".utf8))
}
func control(_ id: String, _ request: [String: Any]) { send(["type": "control_request", "request_id": id, "request": request]) }
func turn() { send(["type": "user", "message": ["role": "user", "content": [["type": "text", "text": "Reply with exactly one lowercase word: cedar"]]], "parent_tool_use_id": NSNull(), "session_id": ""]) }

var buffer = Data()
output.fileHandleForReading.readabilityHandler = { handle in
  let chunk = handle.availableData
  if chunk.isEmpty { handle.readabilityHandler = nil; return }
  buffer.append(chunk)
  while let end = buffer.firstIndex(of: 10) {
    let line = buffer[buffer.startIndex..<end]; buffer = Data(buffer[(end + 1)...])
    guard let frame = try? JSONSerialization.jsonObject(with: line) as? [String: Any] else { continue }
    let type = frame["type"] as? String ?? ""
    if type == "control_response", let r = frame["response"] as? [String: Any] {
      let id = r["request_id"] as? String ?? "", ok = r["subtype"] as? String == "success", body = r["response"] as? [String: Any] ?? [:]
      if !ok { print(id, "error:", String(describing: r["error"] ?? "").prefix(300)); exit(1) }
      if id == "init" {
        let account = body["account"] as? [String: Any] ?? [:]
        print("init account: subscriptionType", account["subscriptionType"] ?? "nil", "tokenSource", account["tokenSource"] ?? "nil", "emailPresent", account["email"] != nil)
        if turnOnly { turn() } else { control("auth", ["subtype": "claude_authenticate", "loginWithClaudeAi": true]) }
      } else if id == "auth" {
        guard let url = URL(string: body["automaticUrl"] as? String ?? "") else { print("no automaticUrl"); exit(1) }
        print("opening Claude login in your browser:", url.host ?? "?", url.path)
        NSWorkspace.shared.open(url)
        control("wait", ["subtype": "claude_oauth_wait_for_completion"])
      } else if id == "wait" {
        let account = body["account"] as? [String: Any] ?? [:]
        print("signed in: subscriptionType", account["subscriptionType"] ?? "nil", "tokenSource", account["tokenSource"] ?? "nil")
        turn()
      }
    } else if type == "result" {
      let text = (frame["result"] as? String ?? "").lowercased().filter { $0.isLetter }
      print("result subtype", frame["subtype"] ?? "nil", "is_error", frame["is_error"] ?? "nil", "expectedWord", text == "cedar",
            "error head:", (frame["is_error"] as? Bool ?? false) ? String((frame["result"] as? String ?? "").prefix(200)) : "")
      exit(0)
    } else if type == "control_request" {
      print("unexpected control request:", (frame["request"] as? [String: Any])?["subtype"] ?? "?"); exit(1)
    }
  }
}
do { try p.run() } catch { print("launch error:", error); exit(1) }
control("init", ["subtype": "initialize"])
DispatchQueue.main.asyncAfter(deadline: .now() + 300) { print("timed out after 5 minutes"); p.terminate(); exit(1) }
RunLoop.main.run()
