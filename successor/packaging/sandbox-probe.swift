import Foundation

let files = FileManager.default
let privateRoot = files.temporaryDirectory.appendingPathComponent("fort059-" + UUID().uuidString, isDirectory: true)
do {
    try files.createDirectory(at: privateRoot, withIntermediateDirectories: false)
    defer { try? files.removeItem(at: privateRoot) }
    guard CommandLine.arguments.count == 2 else { throw CocoaError(.fileReadInvalidFileName) }
    let bundle = Bundle.main.bundleURL
    let child = Process()
    child.executableURL = bundle.appendingPathComponent("Contents/MacOS/node")
    child.arguments = ["--jitless", "--no-experimental-strip-types", "--no-warnings", "--openssl-config=" + bundle.appendingPathComponent("Contents/Resources/node-openssl.cnf").path, bundle.appendingPathComponent("Contents/Resources/runtime-fixture.mjs").path, privateRoot.path, CommandLine.arguments[1]]
    child.environment = ["HOME": privateRoot.path, "TMPDIR": privateRoot.path, "PATH": "/usr/bin:/bin"]
    child.currentDirectoryURL = privateRoot
    child.standardInput = FileHandle.nullDevice
    child.standardOutput = FileHandle.standardOutput
    child.standardError = FileHandle.standardError
    try child.run()
    child.waitUntilExit()
    exit(child.terminationStatus)
} catch {
    FileHandle.standardError.write(Data("sandbox parent failed: \(error)\n".utf8))
    exit(1)
}
