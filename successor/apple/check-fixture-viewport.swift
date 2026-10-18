import AppKit
import ApplicationServices
import Foundation

guard CommandLine.arguments.count == 4, AXIsProcessTrusted() else {
    print("Provide the exact fixture app path, bundle identifier and synthetic reply identifier; existing accessibility access is required.")
    exit(2)
}
let started = ContinuousClock.now
let expected = URL(fileURLWithPath:CommandLine.arguments[1]).resolvingSymlinksInPath()
let apps = NSRunningApplication.runningApplications(withBundleIdentifier:CommandLine.arguments[2])
guard apps.count == 1, apps[0].bundleURL?.resolvingSymlinksInPath() == expected,
      CommandLine.arguments[2].hasPrefix("io.fort.qa.") else { print("Exact fixture application unavailable"); exit(2) }
func attribute(_ element: AXUIElement, _ key: String) -> CFTypeRef? {
    var value: CFTypeRef?
    guard AXUIElementCopyAttributeValue(element,key as CFString,&value) == .success else { return nil }
    return value
}
func frame(_ element: AXUIElement) -> CGRect? {
    guard let rawPosition = attribute(element,kAXPositionAttribute), CFGetTypeID(rawPosition) == AXValueGetTypeID(),
          let rawSize = attribute(element,kAXSizeAttribute), CFGetTypeID(rawSize) == AXValueGetTypeID() else { return nil }
    var point = CGPoint.zero, size = CGSize.zero
    guard AXValueGetValue(rawPosition as! AXValue,.cgPoint,&point), AXValueGetValue(rawSize as! AXValue,.cgSize,&size) else { return nil }
    return CGRect(origin:point,size:size)
}
let app = AXUIElementCreateApplication(apps[0].processIdentifier)
guard let windows = attribute(app,kAXWindowsAttribute) as? [AXUIElement], windows.count == 1 else { print("Expected one fixture window"); exit(2) }
var history: [AXUIElement] = [], reply: [AXUIElement] = [], visited = 0
func inspect(_ element: AXUIElement, _ depth: Int) {
    guard depth < 30, visited < 1000, started.duration(to:.now) < .seconds(5) else { return }
    visited += 1
    let identifier = attribute(element,kAXIdentifierAttribute) as? String
    if identifier == "conversation-history" { history.append(element) }
    if identifier == CommandLine.arguments[3] { reply.append(element) }
    if let children = attribute(element,kAXChildrenAttribute) as? [AXUIElement] { for child in children { inspect(child,depth+1) } }
}
inspect(windows[0],0)
guard history.count == 1, reply.count == 1, let viewport = frame(history[0]), let message = frame(reply[0]),
      viewport.height > 0, message.height > 0, started.duration(to:.now) < .seconds(5) else { print("Exact fixture viewport evidence unavailable"); exit(2) }
let visible = viewport.intersection(message)
let passed = !visible.isNull && visible.height >= message.height * 0.9
let result: [String:Any] = ["passed":passed,"processID":apps[0].processIdentifier,"replyVisibleFraction":visible.isNull ? 0 : visible.height/message.height,
    "viewport":[viewport.origin.x,viewport.origin.y,viewport.width,viewport.height],"reply":[message.origin.x,message.origin.y,message.width,message.height],"visited":visited]
print(String(data:try JSONSerialization.data(withJSONObject:result,options:.sortedKeys),encoding:.utf8)!)
if !passed { exit(1) }
