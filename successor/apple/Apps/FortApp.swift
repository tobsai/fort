import SwiftUI
import FortKit

@main struct FortApp: App {
    @State private var model: MessengerModel?
    @State private var startupError: String?
    var body: some Scene {
        WindowGroup("Fort Fixture") {
            Group {
                if let model { MessengerView(model:model) }
                else { ContentUnavailableView("Fort could not open local storage",systemImage:"externaldrive.badge.exclamationmark",description:Text(startupError ?? "Opening Fort…")) }
            }
            .task {
                guard model == nil else { return }
                do {
                    let directory = try FileManager.default.url(for:.applicationSupportDirectory,in:.userDomainMask,appropriateFor:nil,create:true).appendingPathComponent("FortSpec058",isDirectory:true)
                    model = try MessengerModel(directory:directory)
                } catch { startupError = "Local storage is unavailable. Your service history has not changed." }
            }
        }
    }
}
