import SwiftUI
import ImageIO

/// Spec 065: an Agent's drawn portrait when it has one, otherwise its stock icon.
struct AgentPortraitView: View {
    let model: MessengerModel
    let agent: Agent
    var size: CGFloat = 40
    var body: some View {
        Group {
            if let image = model.portraitImage(for:agent) {
                Image(decorative:image,scale:1).resizable().interpolation(.high).scaledToFill()
                    .frame(width:size,height:size).clipShape(RoundedRectangle(cornerRadius:size * 0.32))
                    .accessibilityHidden(true)
            } else {
                AgentAvatarView(avatar:agent.avatar ?? .orbit,size:size)
            }
        }
        .task(id:agent.portrait?.revision) { await model.loadPortrait(agent) }
    }
}

/// Spec 067: an image Fort drew in a conversation, loaded once per session.
struct ConversationImageView: View {
    let model: MessengerModel
    let image: MessageImage
    var body: some View {
        Group {
            if let bitmap = model.conversationImage(image) {
                Image(decorative:bitmap,scale:1).resizable().interpolation(.high).scaledToFit()
                    .frame(maxWidth:480,maxHeight:480).clipShape(RoundedRectangle(cornerRadius:10))
                    .accessibilityLabel("Image the Agent drew")
            } else {
                RoundedRectangle(cornerRadius:10).fill(.quaternary).aspectRatio(CGFloat(image.width) / CGFloat(max(image.height,1)),contentMode:.fit)
                    .frame(maxWidth:480,maxHeight:480).overlay(ProgressView())
            }
        }
        .task(id:image.imageId) { await model.loadConversationImage(image) }
    }
}

enum PortraitDecoder {
    /// Decodes one PNG into a CGImage, or nil for anything else.
    static func image(_ data: Data) -> CGImage? {
        guard let source = CGImageSourceCreateWithData(data as CFData,[kCGImageSourceShouldCache:false] as CFDictionary),
              CGImageSourceGetType(source) as String? == "public.png", CGImageSourceGetCount(source) == 1 else { return nil }
        return CGImageSourceCreateImageAtIndex(source,0,nil)
    }
}
