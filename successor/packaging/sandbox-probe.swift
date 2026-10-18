import Foundation
import Darwin

let files = FileManager.default
let privateRoot = files.temporaryDirectory.appendingPathComponent("fort059-" + UUID().uuidString, isDirectory: true)
do {
    try files.createDirectory(at: privateRoot, withIntermediateDirectories: false)
    defer { try? files.removeItem(at: privateRoot) }
    guard CommandLine.arguments.count == 3 else { throw CocoaError(.fileReadInvalidFileName) }
    #if FORT_XPC_PROBE
    let channelProof = try JSONSerialization.data(withJSONObject:await probeXPC(),options:[.sortedKeys])
    FileHandle.standardError.write(Data("FORT_PROBE_XPC ".utf8) + channelProof + Data("\n".utf8))
    #endif
    let bundle = Bundle.main.bundleURL
    let executable = bundle.appendingPathComponent("Contents/MacOS/node").path
    let arguments = [executable, "--jitless", "--no-experimental-strip-types", "--no-warnings", "--openssl-config=" + bundle.appendingPathComponent("Contents/Resources/node-openssl.cnf").path, bundle.appendingPathComponent("Contents/Resources/runtime-fixture.mjs").path, privateRoot.path, CommandLine.arguments[1], CommandLine.arguments[2]]
    let environment = ["HOME=" + privateRoot.path, "TMPDIR=" + privateRoot.path, "PATH=/usr/bin:/bin"]
    var argv = arguments.map { strdup($0) } + [nil]
    var env = environment.map { strdup($0) } + [nil]
    defer { argv.forEach { free($0) }; env.forEach { free($0) } }
    var attributes: posix_spawnattr_t?
    var actions: posix_spawn_file_actions_t?
    guard posix_spawnattr_init(&attributes) == 0, posix_spawn_file_actions_init(&actions) == 0 else { throw CocoaError(.executableRuntimeMismatch) }
    defer { posix_spawnattr_destroy(&attributes); posix_spawn_file_actions_destroy(&actions) }
    guard posix_spawnattr_setflags(&attributes, Int16(POSIX_SPAWN_SETPGROUP)) == 0,
          posix_spawnattr_setpgroup(&attributes, getpgrp()) == 0,
          posix_spawn_file_actions_addchdir(&actions, privateRoot.path) == 0 else { throw CocoaError(.executableRuntimeMismatch) }
    var pid: pid_t = 0
    let launched = posix_spawn(&pid, executable, &actions, &attributes, &argv, &env)
    guard launched == 0 else { throw NSError(domain:NSPOSIXErrorDomain, code:Int(launched)) }
    FileHandle.standardError.write(Data("FORT_PROBE_CHILD \(pid) \(getpgid(pid))\n".utf8))
    var status: Int32 = 0
    while waitpid(pid, &status, 0) == -1 {
        if errno != EINTR { throw NSError(domain:NSPOSIXErrorDomain, code:Int(errno)) }
    }
    exit(status & 0x7f == 0 ? (status >> 8) & 0xff : 128 + (status & 0x7f))
} catch {
    FileHandle.standardError.write(Data("sandbox parent failed: \(error)\n".utf8))
    exit(1)
}
