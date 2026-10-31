import AppKit
import ApplicationServices
import Foundation

struct CheckFailure: Error { let message: String }
func require(_ value: Bool, _ message: String) throws {
    if !value { throw CheckFailure(message:message) }
}
func emit(_ value: [String:Any]) {
    if let data = try? JSONSerialization.data(withJSONObject:value,options:.sortedKeys) {
        FileHandle.standardOutput.write(data + Data([10]))
    }
}
func attribute(_ element: AXUIElement, _ key: String) -> CFTypeRef? {
    var result: CFTypeRef?
    return AXUIElementCopyAttributeValue(element,key as CFString,&result) == .success ? result : nil
}
func text(_ element: AXUIElement, _ key: String) -> String { attribute(element,key) as? String ?? "" }
func children(_ element: AXUIElement) -> [AXUIElement] { attribute(element,kAXChildrenAttribute) as? [AXUIElement] ?? [] }
func descendants(_ root: AXUIElement) -> [AXUIElement] {
    var result: [AXUIElement] = []
    func visit(_ element: AXUIElement, _ depth: Int) {
        guard depth < 30, result.count < 1500 else { return }
        result.append(element)
        guard !["AXTextField","AXTextArea"].contains(text(element,kAXRoleAttribute)) else { return }
        for child in children(element) { visit(child,depth+1) }
    }
    visit(root,0); return result
}
func wait(_ message: String, seconds: TimeInterval = 5, until condition: () -> Bool) throws {
    let deadline = Date().addingTimeInterval(seconds)
    while !condition() && Date() < deadline { RunLoop.current.run(until:Date().addingTimeInterval(0.05)) }
    try require(condition(),message)
}
func press(_ element: AXUIElement) throws {
    try require((attribute(element,kAXEnabledAttribute) as? NSNumber)?.boolValue != false,"Control is disabled")
    try require(AXUIElementPerformAction(element,kAXPressAction as CFString) == .success,"Control action failed")
}
func control(_ root: AXUIElement, identifier: String, label: String) throws -> AXUIElement {
    let matches = descendants(root).filter {
        text($0,kAXRoleAttribute) == "AXButton" &&
        (text($0,kAXIdentifierAttribute) == identifier || text($0,kAXTitleAttribute) == label || text($0,kAXDescriptionAttribute) == label)
    }
    try require(matches.count == 1,"Expected one control: " + label)
    return matches[0]
}

@MainActor func run() throws {
    try require(CommandLine.arguments.count == 3 && AXIsProcessTrusted(),"Provide the exact installed app path and cycle count; existing accessibility permission is required")
    let selected = URL(fileURLWithPath:CommandLine.arguments[1]).resolvingSymlinksInPath()
    let cycles = Int(CommandLine.arguments[2]) ?? 0
    try require((1...5).contains(cycles),"Cycle count must be 1 through 5")
    try require(Bundle(url:selected)?.bundleIdentifier == "io.mtree.fort.mac","Expected the exact Fort Mac bundle identifier")
    func running() -> [NSRunningApplication] {
        NSRunningApplication.runningApplications(withBundleIdentifier:"io.mtree.fort.mac").filter { $0.bundleURL?.resolvingSymlinksInPath() == selected }
    }
    try require(running().isEmpty,"Selected client already running; preserve it and stop explicitly before testing")
    let verifier = Process(); verifier.executableURL = URL(fileURLWithPath:"/usr/bin/codesign")
    verifier.arguments = ["--verify","--strict","--deep",selected.path]
    try verifier.run(); verifier.waitUntilExit()
    try require(verifier.terminationStatus == 0,"Installed signature verification failed")
    func open(newInstance: Bool) throws -> NSRunningApplication {
        var completed = false, result: NSRunningApplication?, failed = false
        let configuration = NSWorkspace.OpenConfiguration(); configuration.activates = true; configuration.createsNewApplicationInstance = newInstance
        NSWorkspace.shared.openApplication(at:selected,configuration:configuration) { app,error in
            result = app; failed = error != nil; completed = true
        }
        try wait("App launch did not complete",seconds:30) { completed }
        try require(!failed && result != nil,"App launch failed")
        let app = result!
        try require(app.bundleURL?.resolvingSymlinksInPath() == selected,"Launched app path differs")
        try wait("Expected exactly one selected client") { running().count == 1 }
        return app
    }
    func root(_ app: NSRunningApplication) -> AXUIElement { AXUIElementCreateApplication(app.processIdentifier) }
    func windows(_ app: NSRunningApplication) -> [AXUIElement] { attribute(root(app),kAXWindowsAttribute) as? [AXUIElement] ?? [] }
    func sheet(_ app: NSRunningApplication) -> Bool { descendants(root(app)).contains { text($0,kAXRoleAttribute) == "AXSheet" } }
    func setup(_ app: NSRunningApplication) throws {
        try wait("Setup sheet did not appear") { sheet(app) }
        let enable = try control(root(app),identifier:"local-host-enable",label:"Enable local host")
        let connect = try control(root(app),identifier:"owner-setup-connect",label:"Connect this Mac")
        try require((attribute(enable,kAXEnabledAttribute) as? NSNumber)?.boolValue == true,"Enable must be available")
        try require((attribute(connect,kAXEnabledAttribute) as? NSNumber)?.boolValue == false,"Connect must remain disabled")
    }
    // After first-run setup closes, connection controls live under Devices in the pause menu.
    func openConnection(_ app: NSRunningApplication) throws {
        try press(control(root(app),identifier:"pause-menu-open",label:"Setup"))
        try wait("Pause menu did not open") { sheet(app) }
        try press(control(root(app),identifier:"pause-tab-devices",label:"Devices"))
    }
    // Devices opens on its summary rows; the setup controls checked by setup() sit in its Workshop.
    func devices(_ app: NSRunningApplication) throws {
        try wait("Devices did not show this Mac") { descendants(root(app)).contains { text($0,kAXIdentifierAttribute) == "devices-row-mac" } }
    }
    func closeConnection(_ app: NSRunningApplication) throws {
        if let resume = try? control(root(app),identifier:"pause-resume",label:"Done") { try press(resume) }
        else { try press(control(root(app),identifier:"setup-close",label:"Close setup")) }
    }
    func quit(_ app: NSRunningApplication, cycle: Int, phase: String) throws {
        guard let raw = attribute(root(app),kAXMenuBarAttribute) else { throw CheckFailure(message:"App menu unavailable") }
        let items = descendants(raw as! AXUIElement).filter { text($0,kAXRoleAttribute) == "AXMenuItem" && text($0,kAXTitleAttribute) == "Quit Fort" }
        try require(items.count == 1,"Expected exact Quit Fort menu item")
        let started = Date(); try press(items[0])
        try wait("Normal Quit left the selected client running") { app.isTerminated && running().isEmpty }
        emit(["cycle":cycle,"phase":phase,"pid":app.processIdentifier,"exitSeconds":Date().timeIntervalSince(started),"passed":true])
    }
    for cycle in 1...cycles {
        let cold = try open(newInstance:true); try setup(cold)
        try quit(cold,cycle:cycle,phase:"quit-with-setup-open")
        let client = try open(newInstance:true); try setup(client)
        for repetition in 1...3 {
            try closeConnection(client)
            try wait("Setup did not close") { !sheet(client) }
            try openConnection(client)
            try devices(client)
            emit(["cycle":cycle,"phase":"setup-close-reopen","repetition":repetition,"passed":true])
        }
        try closeConnection(client)
        try wait("Setup did not close before window close") { !sheet(client) }
        let openedWindows = windows(client)
        try require(openedWindows.count == 1,"Expected one client window")
        guard let close = attribute(openedWindows[0],kAXCloseButtonAttribute) else { throw CheckFailure(message:"Window close unavailable") }
        try press(close as! AXUIElement)
        try wait("Client window did not close") { windows(client).isEmpty }
        try require(!client.isTerminated,"Closing the window unexpectedly terminated the app")
        let reopened = try open(newInstance:false)
        try require(reopened.processIdentifier == client.processIdentifier,"Window reopen created another process")
        try wait("Client window did not reopen") { !windows(client).isEmpty }
        try quit(client,cycle:cycle,phase:"quit-after-window-reopen")
    }
    emit(["passed":true,"cycles":cycles,"clientAbsent":running().isEmpty,"activationActions":0])
}
@main struct InstalledSetupCheck {
    @MainActor static func main() {
        do { try run() }
        catch let error as CheckFailure { emit(["passed":false,"failure":error.message]); exit(1) }
        catch { emit(["passed":false,"failure":"Installed setup check failed"]); exit(1) }
    }
}
