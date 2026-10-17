import SwiftUI

public enum AgentAvatar: String, Codable, CaseIterable, Sendable, Identifiable {
    case orbit, ember, sprout
    public var id: String { rawValue }
    public var name: String { rawValue.capitalized }
}
enum AvatarMotion {
    static func phase(time: TimeInterval, reducedMotion: Bool) -> Double { reducedMotion ? 0 : sin(time * 1.7) }
}
struct AgentAvatarView: View {
    let avatar: AgentAvatar
    var size: CGFloat = 40
    @Environment(\.accessibilityReduceMotion) private var reducedMotion
    @Environment(\.scenePhase) private var scenePhase
    private var color: Color { switch avatar { case .orbit: .indigo; case .ember: .orange; case .sprout: .green } }
    var body: some View {
        TimelineView(.animation(minimumInterval:1.0/20,paused:reducedMotion || scenePhase != .active)) { context in
            let phase = AvatarMotion.phase(time:context.date.timeIntervalSinceReferenceDate,reducedMotion:reducedMotion)
            ZStack {
                RoundedRectangle(cornerRadius:size*0.32).fill(color.opacity(0.12))
                if avatar == .orbit {
                    Circle().stroke(color.opacity(0.4),lineWidth:1.5).padding(size*0.2)
                    Circle().fill(color).frame(width:size*0.32,height:size*0.32).scaleEffect(1+phase*0.07)
                    Circle().fill(color).frame(width:size*0.12,height:size*0.12).offset(x:size*0.25,y:phase*size*0.18)
                } else {
                    Image(systemName:avatar == .ember ? "flame.fill" : "leaf.fill").font(.system(size:size*0.52)).foregroundStyle(color)
                        .rotationEffect(.degrees(phase*5)).scaleEffect(1+phase*0.04)
                }
            }.frame(width:size,height:size)
        }.accessibilityHidden(true)
    }
}
