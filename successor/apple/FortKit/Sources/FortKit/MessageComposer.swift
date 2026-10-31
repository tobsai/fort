#if os(macOS)
import AppKit
import SwiftUI

struct MessageComposer: NSViewRepresentable {
    @SwiftUI.Binding var text: String
    let editable: Bool
    let canSend: Bool
    let send: () -> Void
    func makeCoordinator() -> Coordinator { Coordinator(self) }
    func makeNSView(context: Context) -> NSScrollView {
        let scroll = MessageScrollView()
        scroll.drawsBackground = false; scroll.hasVerticalScroller = true; scroll.autohidesScrollers = true
        let input = MessageTextView(frame:NSRect(x:0,y:0,width:300,height:24))
        input.isRichText = false; input.drawsBackground = false; input.font = .systemFont(ofSize:NSFont.systemFontSize)
        input.isAutomaticQuoteSubstitutionEnabled = false; input.isAutomaticDashSubstitutionEnabled = false
        input.isAutomaticSpellingCorrectionEnabled = false
        input.minSize = NSSize(width:0,height:24); input.maxSize = NSSize(width:CGFloat.greatestFiniteMagnitude,height:CGFloat.greatestFiniteMagnitude)
        input.isVerticallyResizable = true; input.isHorizontallyResizable = false
        input.textContainerInset = NSSize(width:0,height:3); input.textContainer?.lineFragmentPadding = 0
        input.textContainer?.widthTracksTextView = true
        input.autoresizingMask = [.width]; input.delegate = context.coordinator
        input.setAccessibilityLabel("Message"); input.setAccessibilityIdentifier("message-composer")
        scroll.documentView = input
        return scroll
    }
    func updateNSView(_ scroll: NSScrollView, context: Context) {
        context.coordinator.parent = self
        guard let input = scroll.documentView as? MessageTextView else { return }
        if input.string != text { input.string = text }
        input.isEditable = editable; input.canSend = canSend; input.send = send; input.needsDisplay = true; scroll.needsLayout = true
    }
    func sizeThatFits(_ proposal: ProposedViewSize, nsView scroll: NSScrollView, context: Context) -> CGSize? {
        guard let input = scroll.documentView as? MessageTextView else { return nil }
        let width = proposal.width ?? 300
        return CGSize(width:width,height:min(132,input.contentHeight(width:width,text:text)))
    }

    @MainActor final class Coordinator: NSObject, NSTextViewDelegate {
        var parent: MessageComposer
        init(_ parent: MessageComposer) { self.parent = parent }
        func textDidChange(_ notification: Notification) {
            guard let input = notification.object as? MessageTextView else { return }
            parent.text = input.string; input.needsDisplay = true
        }
    }
}
private final class MessageScrollView: NSScrollView {
    override func layout() {
        super.layout()
        guard let input = documentView as? MessageTextView else { return }
        let width = contentSize.width
        input.setFrameSize(NSSize(width:width,height:input.contentHeight(width:width)))
    }
}
final class MessageTextView: NSTextView {
    func contentHeight(width: CGFloat, text: String? = nil) -> CGFloat {
        let text = ((text ?? string) + " ") as NSString
        let size = text.boundingRect(with:NSSize(width:max(1,width),height:CGFloat.greatestFiniteMagnitude),options:[.usesLineFragmentOrigin,.usesFontLeading],attributes:[.font:font ?? NSFont.systemFont(ofSize:NSFont.systemFontSize)]).size
        return max(24,ceil(size.height) + 6)
    }
    var canSend = false
    var send: () -> Void = {}
    override func keyDown(with event: NSEvent) {
        if [36,76].contains(event.keyCode), !hasMarkedText() {
            guard isEditable else { return }
            if event.modifierFlags.contains(.shift) { insertNewline(nil) }
            else if canSend && !string.isEmpty && !event.isARepeat { send() }
            return
        }
        super.keyDown(with:event)
    }
    override func draw(_ dirtyRect: NSRect) {
        super.draw(dirtyRect)
        if string.isEmpty {
            ("Message" as NSString).draw(at:NSPoint(x:0,y:3),withAttributes:[.font:font ?? NSFont.systemFont(ofSize:NSFont.systemFontSize),.foregroundColor:NSColor.placeholderTextColor])
        }
    }
}
#endif
