#if os(macOS)
import Foundation

extension MacHostProgram {
    func deleteHistory(_ request: MacHistoryDeletionRequest, storage: MacAgentStorage) async throws -> MacHistoryDeletionReceipt {
        try Task.checkCancellation()
        let directory = try storage.validatedDirectory()
        let worker = try await MacCoreProcessTransport.launch(executable:executable,
            arguments:["--jitless","--no-experimental-strip-types","--no-warnings","--openssl-config=" + opensslConfiguration.path,historyMaintenance.path],
            directory:directory,environment:[:],lifecycle:.parentExit(guardian:guardian))
        do {
            let receipt = try await worker.deleteHistory(request)
            let stop = try await worker.close()
            guard stop.exitedCleanly, !stop.forced else { throw OwnerLifecycleError.stopUnconfirmed }
            try Task.checkCancellation()
            guard try storage.validatedDirectory() == directory else { throw MacAgentStorageError.changedDirectory }
            return receipt
        } catch {
            _ = try? await worker.close()
            throw error
        }
    }
}
#endif
