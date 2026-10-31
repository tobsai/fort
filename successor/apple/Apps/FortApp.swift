import SwiftUI
import FortKit

@main struct FortApp: App {
    @State private var model: MessengerModel?
    @State private var startupError: String?
    private var windowTitle: String {
        #if FORT_INTERACTIVE_QA
        if ProcessInfo.processInfo.arguments.contains("--qa-phone") { return "Fort QA — Phone controls (synthetic)" }
        return ProcessInfo.processInfo.arguments.contains("--qa-account") ? "Fort QA — Account controls (synthetic)" : "Fort QA — First-wave interactive fixture"
        #else
        return "Fort"
        #endif
    }
    var body: some Scene {
        WindowGroup(windowTitle) {
            Group {
                if let model {
                    #if FORT_INTERACTIVE_QA && DEBUG && os(macOS)
                    if ProcessInfo.processInfo.arguments.contains("--qa-phone") { CompanionSetupFixtureView(model:model) }
                    else if ProcessInfo.processInfo.arguments.contains("--qa-account") { AccountSignInFixtureView(model:model) }
                    else { MessengerView(model:model) }
                    #elseif os(macOS)
                    MacWorkspaceView(model:model)
                    #else
                    MessengerView(model:model)
                    #endif
                }
                // Opening is not a failure: only show the storage error once there is one.
                else if let startupError { ContentUnavailableView("Fort could not open local storage",systemImage:"externaldrive.badge.exclamationmark",description:Text(startupError)) }
                else { ProgressView("Opening Fort…") }
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
                    #if os(macOS)
                    if arguments.contains("--qa-phone") {
                        model = try await CompanionSetupFixture.make(directory:FileManager.default.temporaryDirectory.appendingPathComponent("FortPhoneQA-" + session,isDirectory:true),expired:arguments.contains("--qa-phone-expired"))
                        return
                    }
                    if arguments.contains("--qa-account") {
                        let scenario: AccountFixtureScenario
                        if let index = arguments.firstIndex(of:"--qa-account-scenario"), arguments.indices.contains(index + 1), let selected = AccountFixtureScenario(rawValue:arguments[index + 1]) { scenario = selected }
                        else { scenario = .challenge }
                        model = try await AccountSignInFixture.make(directory:FileManager.default.temporaryDirectory.appendingPathComponent("FortAccountQA-" + session,isDirectory:true),scenario:scenario)
                        return
                    }
                    #endif
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
                    let created: MessengerModel
                    #if os(macOS)
                    #if DEBUG
                    if isUITest { created = try MessengerModel(fixtureDirectory:directory) }
                    else { created = try MessengerModel(directory:directory,localHost:MacHostControl()) }
                    #else
                    created = try MessengerModel(directory:directory,localHost:MacHostControl())
                    #endif
                    #else
                    #if DEBUG
                    if isUITest { created = try MessengerModel(fixtureDirectory:directory) }
                    else { created = try MessengerModel(directory:directory) }
                    #else
                    created = try MessengerModel(directory:directory)
                    #endif
                    #endif
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
