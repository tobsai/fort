import Foundation

/// Live Markdown preview, Obsidian style: the text stays plain Markdown, and every line except the
/// one being edited is shown formatted with its markup hidden. Ranges are UTF-16, ready for NSTextStorage.
enum MarkdownPreview {
    enum Style: Equatable, Sendable { case heading(Int), bold, italic, code, strike, quote, bullet, link, hidden }
    struct Span: Equatable, Sendable {
        let range: NSRange
        let style: Style
        init(_ range: NSRange, _ style: Style) { self.range = range; self.style = style }
    }
    static let hint = "Markdown works here: **bold**, *italic*, `code`, # heading, - list. A line formats when you leave it."

    /// Each line's range without its newline. Text ending in a newline has an empty last line.
    static func lineRanges(_ text: String) -> [NSRange] {
        let ns = text as NSString
        var result: [NSRange] = [], start = 0
        while true {
            let newline = ns.range(of:"\n",options:.literal,range:NSRange(location:start,length:ns.length - start))
            let end = newline.location == NSNotFound ? ns.length : newline.location
            result.append(NSRange(location:start,length:end - start))
            if newline.location == NSNotFound { return result }
            start = end + 1
        }
    }

    /// The caret or selection keeps its lines raw. Pass nil when the field isn't being edited.
    static func isEditing(_ line: NSRange, selection: NSRange?) -> Bool {
        guard let selection else { return false }
        return selection.location <= NSMaxRange(line) && NSMaxRange(selection) >= line.location
    }

    static func spans(_ text: String, editing selection: NSRange?) -> [Span] {
        let ns = text as NSString
        return lineRanges(text).filter { !isEditing($0,selection:selection) }.flatMap { lineSpans(ns,$0) }
    }

    private static func regex(_ pattern: String) -> NSRegularExpression {
        // Patterns are fixed literals below; a bad one is a programming error caught by the tests.
        try! NSRegularExpression(pattern:pattern)
    }

    private static func lineSpans(_ text: NSString, _ line: NSRange) -> [Span] {
        let string = text.substring(with:line)
        let length = (string as NSString).length
        func shifted(_ range: NSRange) -> NSRange { NSRange(location:range.location + line.location,length:range.length) }
        func first(_ pattern: String) -> NSTextCheckingResult? { regex(pattern).firstMatch(in:string,range:NSRange(location:0,length:length)) }
        var spans: [Span] = []
        var body = NSRange(location:0,length:length)
        if let match = first(#"^(#{1,6})[ \t]+"#) {
            body = NSRange(location:match.range.length,length:length - match.range.length)
            spans.append(Span(shifted(match.range),.hidden))
            spans.append(Span(shifted(body),.heading(match.range(at:1).length)))
        } else if let match = first(#"^ {0,3}>[ \t]?"#) {
            body = NSRange(location:match.range.length,length:length - match.range.length)
            spans.append(Span(shifted(match.range),.hidden))
            spans.append(Span(shifted(body),.quote))
        } else if let match = first(#"^[ \t]*([-*+]|\d{1,9}[.)])[ \t]+"#) {
            spans.append(Span(shifted(match.range(at:1)),.bullet))
            body = NSRange(location:match.range.length,length:length - match.range.length)
        }
        // Earlier patterns win: code is literal, then links, bold, strike and italic.
        var taken: [NSRange] = []
        func inline(_ pattern: String, _ style: Style) {
            for match in regex(pattern).matches(in:string,range:body) {
                let whole = match.range, inner = match.range(at:1)
                guard !taken.contains(where:{ NSIntersectionRange($0,whole).length > 0 }) else { continue }
                let open = NSRange(location:whole.location,length:inner.location - whole.location)
                let close = NSRange(location:NSMaxRange(inner),length:NSMaxRange(whole) - NSMaxRange(inner))
                spans.append(Span(shifted(open),.hidden)); spans.append(Span(shifted(inner),style)); spans.append(Span(shifted(close),.hidden))
                taken += style == .code ? [whole] : [open,close]
            }
        }
        inline(#"`([^`]+)`"#,.code)
        inline(#"\[([^\]]+)\]\([^)\s]+\)"#,.link)
        inline(#"\*\*(?![\s*])(.+?)(?<![\s*])\*\*"#,.bold)
        inline(#"__(?![\s_])(.+?)(?<![\s_])__"#,.bold)
        inline(#"~~(?!\s)(.+?)(?<!\s)~~"#,.strike)
        inline(#"(?<![*\w])\*(?![\s*])(.+?)(?<![\s*])\*(?![*\w])"#,.italic)
        inline(#"(?<![_\w])_(?![\s_])(.+?)(?<![\s_])_(?![_\w])"#,.italic)
        return spans
    }
}

#if canImport(AppKit)
import AppKit
typealias PlatformFont = NSFont
typealias PlatformColor = NSColor
#elseif canImport(UIKit)
import UIKit
typealias PlatformFont = UIFont
typealias PlatformColor = UIColor
#endif

#if canImport(AppKit) || canImport(UIKit)
extension MarkdownPreview {
    @MainActor static let accent = PlatformColor(red:0.96,green:0.78,blue:0.26,alpha:1)

    /// Restyles the whole storage. Characters never change, only attributes.
    @MainActor static func apply(to storage: NSTextStorage, editing selection: NSRange?, font: PlatformFont, color: PlatformColor) {
        let full = NSRange(location:0,length:storage.length)
        storage.beginEditing()
        storage.setAttributes([.font:font,.foregroundColor:color],range:full)
        for span in spans(storage.string,editing:selection) where span.range.length > 0 && NSMaxRange(span.range) <= storage.length {
            let range = span.range
            switch span.style {
            case .hidden: storage.addAttributes([.font:PlatformFont.systemFont(ofSize:0.01),.foregroundColor:PlatformColor.clear],range:range)
            case .heading(let level):
                let scale: CGFloat = [1.6,1.35,1.2,1.1,1.05,1][min(level,6) - 1]
                storage.addAttribute(.font,value:PlatformFont.systemFont(ofSize:font.pointSize * scale,weight:.bold),range:range)
            case .bold: addTraits(bold:true,to:storage,range:range,fallback:font)
            case .italic: addTraits(bold:false,to:storage,range:range,fallback:font)
            case .code: storage.addAttributes([.font:PlatformFont.monospacedSystemFont(ofSize:font.pointSize * 0.95,weight:.regular),.backgroundColor:color.withAlphaComponent(0.12)],range:range)
            case .strike: storage.addAttributes([.strikethroughStyle:NSUnderlineStyle.single.rawValue,.foregroundColor:color.withAlphaComponent(0.6)],range:range)
            case .quote:
                storage.addAttribute(.foregroundColor,value:color.withAlphaComponent(0.7),range:range)
                addTraits(bold:false,to:storage,range:range,fallback:font)
            case .bullet: storage.addAttributes([.foregroundColor:accent,.font:PlatformFont.systemFont(ofSize:font.pointSize,weight:.heavy)],range:range)
            case .link: storage.addAttributes([.foregroundColor:accent,.underlineStyle:NSUnderlineStyle.single.rawValue],range:range)
            }
        }
        storage.endEditing()
    }

    @MainActor private static func addTraits(bold: Bool, to storage: NSTextStorage, range: NSRange, fallback: PlatformFont) {
        storage.enumerateAttribute(.font,in:range) { value, part, _ in
            let current = value as? PlatformFont ?? fallback
            #if canImport(AppKit)
            let traits = current.fontDescriptor.symbolicTraits.union(bold ? .bold : .italic)
            let styled = NSFont(descriptor:current.fontDescriptor.withSymbolicTraits(traits),size:current.pointSize) ?? current
            #else
            let traits = current.fontDescriptor.symbolicTraits.union(bold ? .traitBold : .traitItalic)
            let styled = current.fontDescriptor.withSymbolicTraits(traits).map { UIFont(descriptor:$0,size:current.pointSize) } ?? current
            #endif
            storage.addAttribute(.font,value:styled,range:part)
        }
    }
}
#endif
