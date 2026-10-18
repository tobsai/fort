import Foundation

@objc(FortSandboxChannel) protocol FortSandboxChannel {
    func echo(_ text: String, reply: @escaping (String, Int32) -> Void)
}
