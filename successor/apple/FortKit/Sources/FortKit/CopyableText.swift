import SwiftUI
#if os(iOS)
import UIKit
#elseif os(macOS)
import AppKit
#endif

/// An error or status line that can be selected and copied, so nobody has to retype it.
struct CopyableText: View {
    let text: String
    var id: String?
    init(_ text: String,id: String? = nil) { self.text = text; self.id = id }
    var body: some View {
        HStack(alignment:.firstTextBaseline,spacing:6) {
            if let id { Text(text).textSelection(.enabled).accessibilityIdentifier(id) } else { Text(text).textSelection(.enabled) }
            if !text.isEmpty {
                Button { Clipboard.copy(text) } label: { Image(systemName:"doc.on.doc") }
                    .buttonStyle(.borderless).help("Copy").accessibilityLabel("Copy message")
            }
        }
    }
}

enum Clipboard {
    static func copy(_ text: String) {
        #if os(iOS)
        UIPasteboard.general.string = text
        #elseif os(macOS)
        NSPasteboard.general.clearContents(); NSPasteboard.general.setString(text,forType:.string)
        #endif
    }
}
