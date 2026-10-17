import AppKit

final class Delegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        print("READY")
        fflush(stdout)
    }
}

let application = NSApplication.shared
application.setActivationPolicy(.accessory)
let delegate = Delegate()
application.delegate = delegate
Timer.scheduledTimer(withTimeInterval: 15, repeats: false) { _ in application.terminate(nil) }
application.run()
