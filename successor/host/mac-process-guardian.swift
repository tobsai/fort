import Foundation
import Darwin

private func check(_ result: Int32) throws {
    if result != 0 { throw NSError(domain:NSPOSIXErrorDomain,code:Int(result)) }
}

private func terminateOwnedGroup() -> Never {
    _ = kill(-getpid(),SIGKILL)
    _exit(125)
}

private func run() throws -> Int32 {
    let arguments = CommandLine.arguments
    guard arguments.count >= 3, let parent = pid_t(arguments[1]), parent > 1,
          getppid() == parent, getpgrp() == getpid(),
          arguments[2].hasPrefix("/"), !arguments[2].utf8.contains(0) else { return 125 }
    let events = kqueue()
    guard events >= 0 else { return 125 }
    guard fcntl(events,F_SETFD,FD_CLOEXEC) == 0 else { return 125 }
    var watch = kevent(ident:UInt(parent),filter:Int16(EVFILT_PROC),
                       flags:UInt16(EV_ADD | EV_ENABLE | EV_ONESHOT),
                       fflags:UInt32(NOTE_EXIT),data:0,udata:nil)
    guard kevent(events,&watch,1,nil,0,nil) == 0, getppid() == parent else { return 125 }
    Thread.detachNewThread {
        var event = kevent()
        while kevent(events,nil,0,&event,1,nil) == -1 && errno == EINTR {}
        terminateOwnedGroup()
    }
    var attributes: posix_spawnattr_t?
    var actions: posix_spawn_file_actions_t?
    try check(posix_spawnattr_init(&attributes))
    defer { posix_spawnattr_destroy(&attributes) }
    try check(posix_spawn_file_actions_init(&actions))
    defer { posix_spawn_file_actions_destroy(&actions) }
    try check(posix_spawnattr_setflags(&attributes,Int16(POSIX_SPAWN_CLOEXEC_DEFAULT)))
    for descriptor in [STDIN_FILENO,STDOUT_FILENO,STDERR_FILENO] {
        try check(posix_spawn_file_actions_addinherit_np(&actions,descriptor))
    }
    var argv = arguments.dropFirst(2).map { strdup($0) } + [nil]
    var environment = ProcessInfo.processInfo.environment.sorted(by:{ $0.key < $1.key })
        .map { strdup($0.key + "=" + $0.value) } + [nil]
    defer { argv.forEach { free($0) }; environment.forEach { free($0) } }
    var child: pid_t = 0
    try check(posix_spawn(&child,arguments[2],&actions,&attributes,&argv,&environment))
    for descriptor in [STDIN_FILENO,STDOUT_FILENO,STDERR_FILENO] { close(descriptor) }
    var status: Int32 = 0
    while waitpid(child,&status,0) == -1 {
        if errno != EINTR { terminateOwnedGroup() }
    }
    return status & 0x7f == 0 ? (status >> 8) & 0xff : 128 + (status & 0x7f)
}

do { _exit(try run()) } catch { _exit(125) }
