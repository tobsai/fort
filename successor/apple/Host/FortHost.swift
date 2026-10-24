import Foundation
import Darwin
import FortKit

@main struct FortHost {
    @MainActor static func main() async {
        do { try await serve(try await MacHostProgram.inspect()) }
        catch {
            FileHandle.standardError.write(Data("Fort host startup or shutdown could not be confirmed.\n".utf8))
            Darwin.exit(EXIT_FAILURE)
        }
    }

    @MainActor private static func serve(_ program: MacHostProgram) async throws {
        let service = program.makeService()
        let events = AsyncStream<Void>.makeStream(bufferingPolicy:.bufferingNewest(1))
        Darwin.signal(SIGTERM,SIG_IGN)
        let source = DispatchSource.makeSignalSource(signal:SIGTERM,queue:.main)
        let starting = Task { try await service.start() }
        source.setEventHandler { starting.cancel(); events.continuation.yield(()) }
        source.resume()
        defer { source.cancel(); events.continuation.finish() }
        do {
            try await starting.value
            // When the core ends by itself (the owner turned Claude on or off), this helper exits too. macOS starts a
            // fresh one on the app's next request, so no Terminal command is needed.
            let ended = Task { await service.waitUntilCoreEnds(); events.continuation.yield(()) }
            defer { ended.cancel() }
            for await _ in events.stream { break }
            _ = try await service.shutdown()
        } catch {
            _ = try await service.shutdown()
            throw error
        }
    }
}
