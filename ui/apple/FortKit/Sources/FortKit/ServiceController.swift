//
//  ServiceController.swift
//  FortKit
//
//  Drives the Fort daemon's launchd lifecycle by shelling out to the
//  `fort service <verb>` subcommand (spec 032). The macOS app binds its
//  install / start / stop / restart / uninstall controls to this and reads
//  `status.running` for the running/stopped indicator.
//
//  Available on macOS only — `Process` is not present on iOS/watchOS.
//

#if os(macOS)

import Foundation

#if canImport(Combine)
import Combine
#endif

/// Runs `fort service <verb>` against a `fort` binary (bundled in the app, or a
/// PATH/Homebrew fallback) and publishes the parsed daemon status. `@MainActor`
/// so SwiftUI can drive it directly as a `@StateObject` / `@EnvironmentObject`.
@MainActor
public final class ServiceController: ObservableObject {

    /// The parsed result of `fort service status`.
    public struct Status: Sendable {
        public var running: Bool
        public var detail: String

        public init(running: Bool, detail: String) {
            self.running = running
            self.detail = detail
        }
    }

    @Published public private(set) var status = Status(running: false, detail: "unknown")

    /// The `fort` binary to invoke; bundled in the app, else a Homebrew fallback.
    public var fortBinaryURL: URL

    public init(fortBinaryURL: URL) {
        self.fortBinaryURL = fortBinaryURL
    }

    /// Runs `fort <args>` and returns its exit code and combined stdout+stderr.
    @discardableResult
    public func run(_ args: [String]) async -> (Int32, String) {
        let binary = fortBinaryURL
        return await withCheckedContinuation { cont in
            let p = Process()
            p.executableURL = binary
            p.arguments = args
            let pipe = Pipe()
            p.standardOutput = pipe
            p.standardError = pipe
            p.terminationHandler = { proc in
                let d = pipe.fileHandleForReading.readDataToEndOfFile()
                cont.resume(returning: (proc.terminationStatus, String(decoding: d, as: UTF8.self)))
            }
            do {
                try p.run()
            } catch {
                cont.resume(returning: (-1, "\(error)"))
            }
        }
    }

    public func install() async { _ = await run(["service", "install"]);   await refresh() }
    public func start()   async { _ = await run(["service", "start"]);     await refresh() }
    public func stop()    async { _ = await run(["service", "stop"]);      await refresh() }
    public func restart() async { _ = await run(["service", "restart"]);   await refresh() }
    public func uninstall() async { _ = await run(["service", "uninstall"]); await refresh() }

    /// Refreshes `status` from `fort service status` — running iff the command
    /// exits 0 and its output mentions "running".
    public func refresh() async {
        let (code, out) = await run(["service", "status"])
        status = Status(
            running: code == 0 && out.contains("running"),
            detail: out.trimmingCharacters(in: .whitespacesAndNewlines)
        )
    }
}

#endif
