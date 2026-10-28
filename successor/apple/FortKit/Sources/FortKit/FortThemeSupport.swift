import SwiftUI
#if os(iOS)
import UIKit
#else
import AppKit
#endif

// What draws with FortTheme: colours from hex, the crew colour for an Agent, the display face and the
// comic panel (ink outline plus a hard offset shadow, with the holographic glow when it is live).
extension Color {
    init(hex: UInt32) { self.init(.sRGB,red:Double((hex >> 16) & 0xFF)/255,green:Double((hex >> 8) & 0xFF)/255,blue:Double(hex & 0xFF)/255,opacity:1) }
    init(light: UInt32, dark: UInt32) {
        #if os(iOS)
        self.init(uiColor:UIColor { traits in UIColor(fortHex:traits.userInterfaceStyle == .dark ? dark : light) })
        #else
        self.init(nsColor:NSColor(name:nil) { appearance in NSColor(fortHex:appearance.bestMatch(from:[.darkAqua,.aqua]) == .darkAqua ? dark : light) })
        #endif
    }
}
#if os(iOS)
private extension UIColor {
    convenience init(fortHex hex: UInt32) { self.init(red:CGFloat((hex >> 16) & 0xFF)/255,green:CGFloat((hex >> 8) & 0xFF)/255,blue:CGFloat(hex & 0xFF)/255,alpha:1) }
}
#else
private extension NSColor {
    convenience init(fortHex hex: UInt32) { self.init(srgbRed:CGFloat((hex >> 16) & 0xFF)/255,green:CGFloat((hex >> 8) & 0xFF)/255,blue:CGFloat(hex & 0xFF)/255,alpha:1) }
}
#endif

extension FortTheme {
    /// The crew slot for an Agent: FNV-1a over its id's UTF-8 bytes, the same as the web client, so an Agent keeps its colour everywhere.
    public static func crewSlot(for agentID: String, count: Int = FortTheme.crew.count) -> Int {
        var hash: UInt32 = 0x811c9dc5
        for byte in agentID.utf8 { hash = (hash ^ UInt32(byte)) &* 0x01000193 }
        return Int(hash % UInt32(count))
    }
    public static func crewColor(for agentID: String) -> Color { crew[crewSlot(for:agentID)] }
    /// The comic display face, scaled with Dynamic Type.
    public static func display(_ size: CGFloat, relativeTo style: Font.TextStyle = .title) -> Font { .custom(displayFont,size:size,relativeTo:style) }
}

struct ComicPanel: ViewModifier {
    var fill: Color = FortTheme.panel
    var live = false
    func body(content: Content) -> some View {
        let shape = RoundedRectangle(cornerRadius:FortTheme.radius)
        content.background(shape.fill(fill))
            .overlay(shape.stroke(FortTheme.line,lineWidth:FortTheme.stroke))
            .background(shape.fill(FortTheme.line).offset(x:FortTheme.shadow,y:FortTheme.shadow))
            .overlay(shape.stroke(FortTheme.glow,lineWidth:live ? 3 : 0).padding(-3).shadow(color:FortTheme.glow.opacity(live ? 0.7 : 0),radius:8))
    }
}
extension View {
    /// An ink-outlined comic panel; `live` adds the cyan holographic glow for the thing in play.
    func comicPanel(fill: Color = FortTheme.panel, live: Bool = false) -> some View { modifier(ComicPanel(fill:fill,live:live)) }
}

/// A comic caption box: the label for a section or a status line.
struct CaptionBox: View {
    let text: String
    var fill: Color = FortTheme.caption
    var body: some View {
        Text(text.uppercased()).font(.system(size:11,weight:.black,design:.rounded)).tracking(0.6)
            .foregroundStyle(fill == FortTheme.failed ? Color.white : FortTheme.captionInk)
            .padding(.horizontal,8).padding(.vertical,2)
            .background(fill).overlay(Rectangle().stroke(FortTheme.line,lineWidth:2))
    }
}
