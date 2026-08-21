import SwiftUI

/// Runtime state changes the energy of the living Fort mark. Durable status
/// remains textual; ambient motion is brand presence rather than status alone.
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
        switch state {
        case .idle, .working: return true
        }
    }

    public static func allowsSpatialMotion(state: PrimaryOrbState, reduceMotion: Bool) -> Bool {
        shouldPulse(state: state) && !reduceMotion
    }

    public static func energyRate(state: PrimaryOrbState) -> Double {
        state == .working ? 1.1 : 0.42
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

/// Compatibility adapter for the rollback Primary Channels surface. New code
/// uses `FortProductMarkView` for product identity and `AgentIdentityView` for
/// agent identity.
public struct FortAgentOrbView: View {
    private let name: String
    private let state: PrimaryOrbState
    private let size: CGFloat

    public init(name: String, state: PrimaryOrbState, size: CGFloat = 38) {
        self.name = name
        self.state = state
        self.size = size
    }

    public var body: some View {
        FortProductMarkView(
            activity: state == .working ? .working : .ambient,
            size: size
        )
        .accessibilityLabel("\(name), \(state.label)")
    }
}
