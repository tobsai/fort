import SwiftUI

/// The only runtime states that may change the Phase 1 agent orb. Queued,
/// failed, and canceled state remain textual; motion is reserved for durable
/// Working evidence.
public enum PrimaryOrbState: String, Sendable, CaseIterable {
    case idle
    case working

    public var label: String {
        switch self {
        case .idle: return "Idle"
        case .working: return "Working"
        }
    }
}

public enum FortOrbMotion {
    public static func shouldPulse(state: PrimaryOrbState) -> Bool {
        state == .working
    }

    public static func allowsSpatialMotion(state: PrimaryOrbState, reduceMotion: Bool) -> Bool {
        state == .working && !reduceMotion
    }
}

/// Small shared palette for native connection and menu-bar chrome outside the
/// device-local Phase 1 theme environment.
public enum FortPalette {
    public static let faint = Color(red: 102 / 255, green: 125 / 255, blue: 150 / 255)
    public static let brass = Color(red: 22 / 255, green: 140 / 255, blue: 255 / 255)
    public static let brassBright = Color(red: 96 / 255, green: 184 / 255, blue: 255 / 255)
    public static let working = Color(red: 37 / 255, green: 164 / 255, blue: 255 / 255)
    public static let needsYou = Color(red: 214 / 255, green: 159 / 255, blue: 53 / 255)
}

private extension PrimaryOrbState {
    var color: Color {
        switch self {
        case .idle: return FortPalette.faint
        case .working: return FortPalette.working
        }
    }
}

/// The approved Fort intelligence-core raster with motion tied only to
/// evidence-backed Working state.
public struct FortAgentOrbView: View {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    private let name: String
    private let state: PrimaryOrbState
    private let size: CGFloat

    public init(name: String, state: PrimaryOrbState, size: CGFloat = 38) {
        self.name = name
        self.state = state
        self.size = size
    }

    public var body: some View {
        let pulsing = FortOrbMotion.shouldPulse(state: state)
        let spatialMotion = FortOrbMotion.allowsSpatialMotion(state: state, reduceMotion: reduceMotion)
        TimelineView(.animation(minimumInterval: 1 / 30, paused: !pulsing)) { timeline in
            let phase = timeline.date.timeIntervalSinceReferenceDate
            let energy = pulsing ? (sin(phase * 1.1) + 1) / 2 : 0
            let drift = spatialMotion ? sin(phase * 1.25) * 3.5 : 0
            ZStack {
                Image("FortAgentOrb")
                    .resizable()
                    .scaledToFill()
                    .clipShape(Circle())
                    .rotationEffect(.degrees(drift))
                    .scaleEffect(spatialMotion ? 0.99 + energy * 0.045 : 1)

                Circle()
                    .stroke(state.color, lineWidth: size < 24 ? 1.2 : 1.8)

                if state == .working {
                    Circle()
                        .trim(from: 0, to: 0.14)
                        .stroke(Color.white, style: StrokeStyle(lineWidth: 2, lineCap: .round))
                        .rotationEffect(.degrees(spatialMotion ? phase * 78 : 18))
                        .opacity(pulsing ? 0.72 + energy * 0.28 : 0.72)
                }
            }
            .shadow(
                color: state.color.opacity(pulsing ? 0.2 + energy * 0.22 : 0.18),
                radius: size * (pulsing ? 0.12 + energy * 0.1 : 0.12)
            )
        }
        .frame(width: size, height: size)
        .accessibilityLabel("\(name), \(state.label)")
    }
}
