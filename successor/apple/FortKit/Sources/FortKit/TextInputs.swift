import SwiftUI

/// Every place Toby types looks the same: a recessed box with a visible edge that turns gold while focused.
/// Text Fort or an Agent wrote is never boxed, so the two can't be confused.
enum FortInput {
    static let fill = Color.primary.opacity(0.07)
    static let edge = Color.primary.opacity(0.32)
    static let cornerRadius: CGFloat = 7
}
struct FortInputBox: ViewModifier {
    let focused: Bool
    func body(content: Content) -> some View {
        content
            .background(FortInput.fill,in:RoundedRectangle(cornerRadius:FortInput.cornerRadius))
            .overlay(RoundedRectangle(cornerRadius:FortInput.cornerRadius).strokeBorder(focused ? HUDStyle.gold : FortInput.edge,lineWidth:focused ? 2 : 1))
            .animation(.easeOut(duration:0.12),value:focused)
    }
}
/// A single-line field in the shared input box. Single-line fields stay plain text.
struct FortInputField: ViewModifier {
    @FocusState private var focused: Bool
    func body(content: Content) -> some View {
        content.textFieldStyle(.plain).focused($focused)
            .padding(.horizontal,10).padding(.vertical,7)
            .modifier(FortInputBox(focused:focused))
    }
}
extension View {
    func fortInput() -> some View { modifier(FortInputField()) }
    func fortInputBox(focused: Bool) -> some View { modifier(FortInputBox(focused:focused)) }
}

/// A multi-line input that takes Markdown and formats each line once the caret leaves it.
struct MarkdownEditor: View {
    @SwiftUI.Binding var text: String
    let placeholder: String
    var minHeight: CGFloat = 120
    var accessibilityLabel: String
    var showsHint = true
    @State private var focused = false
    static let inset: CGFloat = 8
    var body: some View {
        VStack(alignment:.leading,spacing:4) {
            MarkdownTextView(text:$text,focused:$focused,label:accessibilityLabel)
                .frame(minHeight:minHeight)
                .overlay(alignment:.topLeading) {
                    if text.isEmpty {
                        Text(verbatim:placeholder).foregroundStyle(.tertiary).padding(Self.inset).allowsHitTesting(false).accessibilityHidden(true)
                    }
                }
                .fortInputBox(focused:focused)
            if showsHint { Text(verbatim:MarkdownPreview.hint).font(.caption2).foregroundStyle(.secondary) }
        }
    }
}

#if os(macOS)
import AppKit

struct MarkdownTextView: NSViewRepresentable {
    @SwiftUI.Binding var text: String
    @SwiftUI.Binding var focused: Bool
    let label: String
    @MainActor static let font = NSFont.systemFont(ofSize:NSFont.systemFontSize + 1)
    func makeCoordinator() -> Coordinator { Coordinator(self) }
    func makeNSView(context: Context) -> NSScrollView {
        let scroll = MarkdownScrollView()
        scroll.drawsBackground = false; scroll.hasVerticalScroller = true; scroll.autohidesScrollers = true; scroll.borderType = .noBorder
        let input = MarkdownNSTextView(frame:NSRect(x:0,y:0,width:300,height:120))
        input.isRichText = false; input.allowsUndo = true; input.drawsBackground = false
        input.font = Self.font; input.textColor = .labelColor; input.insertionPointColor = .labelColor
        input.isAutomaticQuoteSubstitutionEnabled = false; input.isAutomaticDashSubstitutionEnabled = false
        input.minSize = NSSize(width:0,height:0); input.maxSize = NSSize(width:CGFloat.greatestFiniteMagnitude,height:CGFloat.greatestFiniteMagnitude)
        input.isVerticallyResizable = true; input.isHorizontallyResizable = false
        input.textContainerInset = NSSize(width:MarkdownEditor.inset,height:MarkdownEditor.inset); input.textContainer?.lineFragmentPadding = 0
        input.textContainer?.widthTracksTextView = true; input.autoresizingMask = [.width]
        input.setAccessibilityLabel(label)
        input.delegate = context.coordinator
        input.string = text
        scroll.documentView = input
        context.coordinator.restyle(input)
        return scroll
    }
    func updateNSView(_ scroll: NSScrollView, context: Context) {
        context.coordinator.parent = self
        guard let input = scroll.documentView as? MarkdownNSTextView, input.string != text else { return }
        input.string = text
        context.coordinator.restyle(input)
    }

    @MainActor final class Coordinator: NSObject, NSTextViewDelegate {
        var parent: MarkdownTextView
        init(_ parent: MarkdownTextView) { self.parent = parent }
        func restyle(_ input: NSTextView, focused: Bool? = nil) {
            guard !input.hasMarkedText(), let storage = input.textStorage else { return }
            let editing = (focused ?? (input.window?.firstResponder === input)) ? input.selectedRange() : nil
            MarkdownPreview.apply(to:storage,editing:editing,font:MarkdownTextView.font,color:.labelColor)
            input.typingAttributes = [.font:MarkdownTextView.font,.foregroundColor:NSColor.labelColor]
        }
        func textDidChange(_ notification: Notification) {
            guard let input = notification.object as? NSTextView else { return }
            parent.text = input.string; restyle(input)
        }
        func textViewDidChangeSelection(_ notification: Notification) {
            guard let input = notification.object as? NSTextView else { return }
            restyle(input)
        }
        func focusChanged(_ input: NSTextView, _ focused: Bool) {
            if parent.focused != focused { parent.focused = focused }
            restyle(input,focused:focused)
        }
    }
}
private final class MarkdownScrollView: NSScrollView {
    override func layout() {
        super.layout()
        guard let input = documentView else { return }
        if input.frame.width != contentSize.width { input.setFrameSize(NSSize(width:contentSize.width,height:max(input.frame.height,contentSize.height))) }
    }
}
final class MarkdownNSTextView: NSTextView {
    // The caret line goes back to raw Markdown as soon as the view takes focus, not only after the first keystroke.
    override func becomeFirstResponder() -> Bool {
        let accepted = super.becomeFirstResponder()
        if accepted { (delegate as? MarkdownTextView.Coordinator)?.focusChanged(self,true) }
        return accepted
    }
    override func resignFirstResponder() -> Bool {
        let accepted = super.resignFirstResponder()
        if accepted { (delegate as? MarkdownTextView.Coordinator)?.focusChanged(self,false) }
        return accepted
    }
}
#elseif os(iOS)
import UIKit

struct MarkdownTextView: UIViewRepresentable {
    @SwiftUI.Binding var text: String
    @SwiftUI.Binding var focused: Bool
    let label: String
    @MainActor static let font = UIFont.preferredFont(forTextStyle:.body)
    func makeCoordinator() -> Coordinator { Coordinator(self) }
    func makeUIView(context: Context) -> UITextView {
        let input = UITextView()
        input.backgroundColor = .clear; input.font = Self.font; input.textColor = .label
        input.textContainerInset = UIEdgeInsets(top:MarkdownEditor.inset,left:MarkdownEditor.inset,bottom:MarkdownEditor.inset,right:MarkdownEditor.inset)
        input.textContainer.lineFragmentPadding = 0
        input.smartQuotesType = .no; input.smartDashesType = .no
        input.accessibilityLabel = label
        input.delegate = context.coordinator
        input.text = text
        context.coordinator.restyle(input)
        return input
    }
    func updateUIView(_ input: UITextView, context: Context) {
        context.coordinator.parent = self
        guard input.text != text else { return }
        input.text = text
        context.coordinator.restyle(input)
    }

    @MainActor final class Coordinator: NSObject, UITextViewDelegate {
        var parent: MarkdownTextView
        init(_ parent: MarkdownTextView) { self.parent = parent }
        func restyle(_ input: UITextView) {
            guard input.markedTextRange == nil else { return }
            let selection = input.selectedRange
            MarkdownPreview.apply(to:input.textStorage,editing:input.isFirstResponder ? selection : nil,font:MarkdownTextView.font,color:.label)
            input.typingAttributes = [.font:MarkdownTextView.font,.foregroundColor:UIColor.label]
            if input.selectedRange != selection { input.selectedRange = selection }
        }
        func textViewDidChange(_ input: UITextView) { parent.text = input.text; restyle(input) }
        func textViewDidChangeSelection(_ input: UITextView) { restyle(input) }
        func textViewDidBeginEditing(_ input: UITextView) { parent.focused = true; restyle(input) }
        func textViewDidEndEditing(_ input: UITextView) { parent.focused = false; restyle(input) }
    }
}
#endif
