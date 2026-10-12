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
                    var directory = try FileManager.default.url(for:.applicationSupportDirectory,in:.userDomainMask,appropriateFor:nil,create:true).appendingPathComponent("FortSpec058",isDirectory:true)
                    #if DEBUG
                    let environment = ProcessInfo.processInfo.environment
                    let isUITest = ProcessInfo.processInfo.arguments.contains("--fixture-ui-test") && UUID(uuidString:environment["FORT_UI_TEST_SESSION"] ?? "") != nil
                    if isUITest { directory = directory.appendingPathComponent("UITests/" + environment["FORT_UI_TEST_SESSION"]!,isDirectory:true) }
                    #endif
                    let created = try MessengerModel(directory:directory)
                    #if DEBUG
                    if isUITest {
                        created.endpoint = environment["FORT_FIXTURE_URL"] ?? created.endpoint
                        created.expectedInstallation = environment["FORT_FIXTURE_INSTALLATION"] ?? ""
                        created.enrollmentCode = environment["FORT_FIXTURE_CODE"] ?? ""
                    }
                    #endif
                    model = created
                } catch { startupError = "Local storage is unavailable. Your service history has not changed." }
            }
        }
    }
}
