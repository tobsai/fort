import SwiftUI
import FortKit

@main struct FortApp: App {
    @State private var model: MessengerModel?
    @State private var startupError: String?
    private var windowTitle: String {
        #if FORT_INTERACTIVE_QA
        return "Fort QA — First-wave interactive fixture"
        #else
        return "Fort"
        #endif
    }
    var body: some Scene {
        WindowGroup(windowTitle) {
            Group {
                if let model { MessengerView(model:model) }
                else { ContentUnavailableView("Fort could not open local storage",systemImage:"externaldrive.badge.exclamationmark",description:Text(startupError ?? "Opening Fort…")) }
            }
            .task {
                guard model == nil else { return }
                do {
                    #if FORT_INTERACTIVE_QA
                    #if DEBUG
                    let arguments = ProcessInfo.processInfo.arguments
                    let session: String
                    if let position = arguments.firstIndex(of:"--qa-session") {
                        guard arguments.indices.contains(position + 1), let id = UUID(uuidString:arguments[position + 1]) else { throw ClientError.invalidResponse }
                        session = id.uuidString.lowercased()
                    } else { session = UUID().uuidString.lowercased() }
                    model = try await InteractiveFixture.make(directory:FileManager.default.temporaryDirectory.appendingPathComponent("FortInteractiveQA-" + session,isDirectory:true))
                    return
                    #else
                    throw ClientError.invalidResponse
                    #endif
                    #else
                    #if DEBUG
                    if ProcessInfo.processInfo.arguments.contains("--offline-preview") {
                        model = try OfflinePreview.make(directory:FileManager.default.temporaryDirectory.appendingPathComponent("FortOfflinePreview-" + UUID().uuidString,isDirectory:true))
                        return
                    }
                    #endif
                    var directory = try FileManager.default.url(for:.applicationSupportDirectory,in:.userDomainMask,appropriateFor:nil,create:true).appendingPathComponent("FortCandidateB",isDirectory:true)
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
                    #endif
                } catch { startupError = "Local storage is unavailable. Your service history has not changed." }
            }
        }
        #if os(macOS)
        .defaultSize(width:900,height:650)
        #endif
    }
}
