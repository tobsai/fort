import Foundation
import Testing
@testable import FortKit

private func styled(_ text: String, editing: NSRange? = nil) -> [(String,MarkdownPreview.Style)] {
    MarkdownPreview.spans(text,editing:editing).map { ((text as NSString).substring(with:$0.range),$0.style) }
}
private func visible(_ text: String, editing: NSRange? = nil) -> [(String,MarkdownPreview.Style)] { styled(text,editing:editing).filter { $0.1 != .hidden } }
private func hidden(_ text: String, editing: NSRange? = nil) -> [String] { styled(text,editing:editing).filter { $0.1 == .hidden }.map(\.0) }

@Test func markdownLinesKeepAnEmptyLastLineAfterANewline() {
    #expect(MarkdownPreview.lineRanges("") == [NSRange(location:0,length:0)])
    #expect(MarkdownPreview.lineRanges("a\nbc\n") == [NSRange(location:0,length:1),NSRange(location:2,length:2),NSRange(location:5,length:0)])
}

@Test func markdownFormatsHeadingsBoldItalicCodeAndLinksAndHidesTheirMarkup() {
    let text = "## Ship the beta\nMake it **fast**, *calm* and `typed`.\nSee [the spec](https://example.com) ~~soon~~"
    let shown = visible(text)
    #expect(shown.map(\.0) == ["Ship the beta","typed","fast","calm","the spec","soon"])
    #expect(shown.map(\.1) == [.heading(2),.code,.bold,.italic,.link,.strike])
    #expect(hidden(text) == ["## ","`","`","**","**","*","*","[","](https://example.com)","~~","~~"])
}

@Test func markdownLeavesTheLineBeingEditedRaw() {
    let text = "# Plan\n**bold** line"
    let onHeading = NSRange(location:3,length:0), onSecond = NSRange(location:(text as NSString).length,length:0)
    #expect(visible(text,editing:onHeading).map(\.0) == ["bold"])
    #expect(visible(text,editing:onSecond).map(\.0) == ["Plan"])
    // The caret at the end of a line still counts as on it; a selection over both keeps both raw.
    #expect(visible(text,editing:NSRange(location:6,length:0)).map(\.0) == ["bold"])
    #expect(styled(text,editing:NSRange(location:2,length:8)).isEmpty)
    #expect(visible(text).count == 2)
}

@Test func markdownListsQuotesAndNestedEmphasis() {
    #expect(visible("- buy milk\n1. call bank").map(\.1) == [.bullet,.bullet])
    #expect(visible("- buy milk").map(\.0) == ["-"])
    #expect(visible("> quoted *idea*").map(\.0) == ["quoted *idea*","idea"])
    #expect(visible("**all *of* it**").map(\.0) == ["all *of* it","of"])
    // Emphasis needs real words: no bullet from bold, no italic from stray stars or snake_case.
    #expect(visible("**bold** start").map(\.1) == [.bold])
    #expect(visible("2 * 3 * 4 and snake_case_name").isEmpty)
    #expect(visible("`**not bold**`").map(\.1) == [.code])
}

#if os(macOS)
import AppKit
import SwiftUI

@Test @MainActor func markdownPreviewChangesOnlyAttributesNeverTheMarkdown() throws {
    let text = "# Title\nsome **bold** words"
    let storage = NSTextStorage(string:text)
    let font = NSFont.systemFont(ofSize:13)
    MarkdownPreview.apply(to:storage,editing:nil,font:font,color:.labelColor)
    #expect(storage.string == text)
    let bold = try #require(storage.attribute(.font,at:(text as NSString).range(of:"bold").location,effectiveRange:nil) as? NSFont)
    #expect(bold.fontDescriptor.symbolicTraits.contains(.bold))
    let marker = try #require(storage.attribute(.foregroundColor,at:(text as NSString).range(of:"**").location,effectiveRange:nil) as? NSColor)
    #expect(marker.alphaComponent == 0)
    let title = try #require(storage.attribute(.font,at:2,effectiveRange:nil) as? NSFont)
    #expect(title.pointSize > font.pointSize)
    // Put the caret back on line two: it goes back to plain text, line one stays formatted.
    MarkdownPreview.apply(to:storage,editing:NSRange(location:(text as NSString).length,length:0),font:font,color:.labelColor)
    let raw = try #require(storage.attribute(.font,at:(text as NSString).range(of:"**").location,effectiveRange:nil) as? NSFont)
    #expect(raw == font)
    #expect((storage.attribute(.font,at:2,effectiveRange:nil) as? NSFont)?.pointSize ?? 0 > font.pointSize)
}

@Test @MainActor func markdownEditorFormatsALineOnceTheCaretLeavesIt() throws {
    var text = ""
    let view = MarkdownTextView(text:SwiftUI.Binding(get:{ text },set:{ text = $0 }),focused:.constant(false),label:"Notes")
    let coordinator = view.makeCoordinator()
    let input = MarkdownNSTextView(frame:NSRect(x:0,y:0,width:400,height:120))
    input.isRichText = false; input.font = MarkdownTextView.font; input.delegate = coordinator
    let window = NSWindow(contentRect:input.frame,styleMask:[.titled],backing:.buffered,defer:false)
    window.contentView = input; window.makeFirstResponder(input)
    defer { window.contentView = nil }
    input.insertText("**done**",replacementRange:NSRange(location:NSNotFound,length:0))
    #expect(text == "**done**")
    func boldAtDone() -> Bool { (input.textStorage?.attribute(.font,at:2,effectiveRange:nil) as? NSFont)?.fontDescriptor.symbolicTraits.contains(.bold) == true }
    #expect(!boldAtDone())
    input.insertNewline(nil)
    #expect(text == "**done**\n")
    #expect(boldAtDone())
    input.setSelectedRange(NSRange(location:3,length:0))
    #expect(!boldAtDone())
    window.makeFirstResponder(nil)
    #expect(boldAtDone())
    #expect(input.string == "**done**\n")
}
#endif
