#if os(macOS) && DEBUG
import AppKit
import SwiftUI
import Testing
@testable import FortKit

@Test @MainActor func composerReturnSendsOnceAndShiftReturnKeepsAMultilineDraft() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("FortComposerQA-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let model = try await InteractiveFixture.make(directory:root)
    model.draft = "First line"
    let coordinator = MessageComposer(text:SwiftUI.Binding(get:{model.draft},set:{model.draft = $0}),editable:true,canSend:true,send:{}).makeCoordinator()
    let input = MessageTextView(frame:NSRect(x:0,y:0,width:400,height:100))
    input.delegate = coordinator; input.isRichText = false; input.isEditable = true
    input.string = model.draft; input.setSelectedRange(NSRange(location:input.string.utf16.count,length:0))
    input.canSend = true
    var tasks: [Task<Void,Never>] = []
    input.send = { tasks.append(Task { await model.send() }) }
    let window = NSWindow(contentRect:input.frame,styleMask:[.titled],backing:.buffered,defer:false)
    window.contentView = input; window.makeFirstResponder(input)
    defer { window.contentView = nil }
    func enter(_ flags: NSEvent.ModifierFlags = []) throws {
        input.keyDown(with:try #require(NSEvent.keyEvent(with:.keyDown,location:.zero,modifierFlags:flags,timestamp:0,windowNumber:window.windowNumber,context:nil,characters:"\r",charactersIgnoringModifiers:"\r",isARepeat:false,keyCode:36)))
    }
    try enter(.shift)
    #expect(model.draft == "First line\n")
    #expect(tasks.isEmpty)
    input.insertText("Second line",replacementRange:NSRange(location:NSNotFound,length:0))
    #expect(model.draft == "First line\nSecond line")
    try enter(); try enter()
    for task in tasks { await task.value }
    #expect(model.draft.isEmpty)
    #expect(model.snapshot?.messages.filter { $0.role == "user" && $0.text == "First line\nSecond line" }.count == 1)
    #expect(model.pending.isEmpty)
    let sent = model.snapshot
    input.string = ""; input.canSend = false; tasks = []
    try enter(); #expect(tasks.isEmpty && input.string.isEmpty)
    input.string = "Disabled draft"; input.canSend = true; input.isEditable = false
    try enter(); #expect(tasks.isEmpty && input.string == "Disabled draft")
    #expect(model.snapshot == sent)
}

@Test @MainActor func composerReturnCommitsMarkedTextWithoutSending() throws {
    let input = MessageTextView(frame:NSRect(x:0,y:0,width:400,height:100))
    let window = NSWindow(contentRect:input.frame,styleMask:[.titled],backing:.buffered,defer:false)
    window.contentView = input; window.makeFirstResponder(input)
    defer { window.contentView = nil }
    input.isRichText = false; input.canSend = true
    var sends = 0; input.send = { sends += 1 }
    input.setMarkedText("日本",selectedRange:NSRange(location:2,length:0),replacementRange:NSRange(location:NSNotFound,length:0))
    #expect(input.hasMarkedText())
    let event = try #require(NSEvent.keyEvent(with:.keyDown,location:.zero,modifierFlags:[],timestamp:0,windowNumber:window.windowNumber,context:nil,characters:"\r",charactersIgnoringModifiers:"\r",isARepeat:false,keyCode:36))
    input.keyDown(with:event)
    #expect(sends == 0)
    input.unmarkText(); input.string = "日本"; input.keyDown(with:event)
    #expect(sends == 1)
}
@Test @MainActor func composerHostedDocumentRemainsVisibleWhileDraftGrows() async throws {
    var draft = "Visible first line"
    let content = MessageComposer(text:SwiftUI.Binding(get:{draft},set:{draft = $0}),editable:true,canSend:true,send:{})
    let host = NSHostingView(rootView:content)
    let window = NSWindow(contentRect:NSRect(x:0,y:0,width:400,height:100),styleMask:[.titled],backing:.buffered,defer:false)
    window.contentView = host
    defer { window.contentView = nil }
    host.layoutSubtreeIfNeeded()
    func find(_ view: NSView) -> MessageTextView? {
        if let input = view as? MessageTextView { return input }
        return view.subviews.lazy.compactMap { find($0) }.first
    }
    let input = try #require(find(host))
    #expect(input.frame.height >= 24 && input.frame.width > 0)
    #expect(input.visibleRect.height > 0)
    #expect(input.string == draft)
    window.makeFirstResponder(input)
    input.setSelectedRange(NSRange(location:input.string.utf16.count,length:0))
    input.insertText("\nVisible second line",replacementRange:NSRange(location:NSNotFound,length:0))
    host.layoutSubtreeIfNeeded()
    #expect(draft == "Visible first line\nVisible second line")
    #expect(input.frame.height >= 24 && input.visibleRect.height > 0)
    #expect(window.firstResponder === input)
}
#endif
