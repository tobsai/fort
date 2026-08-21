import Combine
import SwiftUI

private struct FortMarkWindowVisibleKey: EnvironmentKey {
    static let defaultValue = true
}

private extension EnvironmentValues {
    var fortMarkWindowVisible: Bool {
        get { self[FortMarkWindowVisibleKey.self] }
        set { self[FortMarkWindowVisibleKey.self] = newValue }
    }
}

private struct FortMarkSurfaceFrame: Equatable {
    let phase: TimeInterval
    let environment: FortMarkMotionEnvironment
}

private struct FortMarkSurfaceFrameKey: EnvironmentKey {
    static let defaultValue: FortMarkSurfaceFrame? = nil
}

private extension EnvironmentValues {
    var fortMarkSurfaceFrame: FortMarkSurfaceFrame? {
        get { self[FortMarkSurfaceFrameKey.self] }
        set { self[FortMarkSurfaceFrameKey.self] = newValue }
    }
}

public extension View {
    /// macOS window hosts supply false while minimized or fully occluded.
    func fortMarkWindowVisible(_ visible: Bool) -> some View {
        environment(\.fortMarkWindowVisible, visible)
    }
}

private struct FortMarkPowerSnapshot: Equatable {
    let isLowPower: Bool
    let thermalState: FortMarkThermalState

    static var current: FortMarkPowerSnapshot {
        let process = ProcessInfo.processInfo
        let thermal: FortMarkThermalState
        switch process.thermalState {
        case .nominal: thermal = .nominal
        case .fair: thermal = .fair
        case .serious: thermal = .serious
        case .critical: thermal = .critical
        @unknown default: thermal = .serious
        }
        return FortMarkPowerSnapshot(
            isLowPower: process.isLowPowerModeEnabled,
            thermalState: thermal
        )
    }
}

@MainActor
private final class FortMarkViewClock: ObservableObject {
    private var clock = FortMarkPhaseClock()

    func sample(at date: Date, isActive: Bool) -> TimeInterval {
        clock.sample(at: date.timeIntervalSinceReferenceDate, isActive: isActive)
    }

    func pause() {
        clock.sample(at: 0, isActive: false)
    }
}

/// Owns one animation schedule and phase accumulator for every Fort mark in a
/// visible app surface. Hosts place it outside their product-mode branch so
/// Agent and rollback shells share the same lifecycle and never create one
/// timer per decorative mark.
public struct FortMarkSurface<Content: View>: View {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.scenePhase) private var scenePhase
    @Environment(\.fortMarkWindowVisible) private var windowVisible
    @StateObject private var clock = FortMarkViewClock()
    @State private var isOnscreen = false
    @State private var power = FortMarkPowerSnapshot.current

    private let content: Content

    public init(@ViewBuilder content: () -> Content) {
        self.content = content()
    }

    public var body: some View {
        let environment = motionEnvironment
        let seed = FortMarkMotion.frame(
            activity: .ambient,
            phase: 0,
            environment: environment
        )
        TimelineView(.animation(
            minimumInterval: seed.frameInterval ?? (1 / 15),
            paused: !environment.isForegroundVisible
        )) { timeline in
            let phase = clock.sample(
                at: timeline.date,
                isActive: environment.isForegroundVisible
            )
            content.environment(
                \.fortMarkSurfaceFrame,
                FortMarkSurfaceFrame(phase: phase, environment: environment)
            )
        }
        .onAppear { isOnscreen = true }
        .onDisappear {
            isOnscreen = false
            clock.pause()
        }
        .onChange(of: scenePhase) { phase in
            if phase != .active { clock.pause() }
        }
        .onChange(of: windowVisible) { visible in
            if !visible { clock.pause() }
        }
        .onReceive(NotificationCenter.default.publisher(for: ProcessInfo.thermalStateDidChangeNotification)) { _ in
            power = .current
        }
        .onReceive(NotificationCenter.default.publisher(for: Notification.Name.NSProcessInfoPowerStateDidChange)) { _ in
            power = .current
        }
    }

    private var motionEnvironment: FortMarkMotionEnvironment {
        FortMarkMotionEnvironment(
            reduceMotion: reduceMotion,
            isOnscreen: isOnscreen,
            isSceneActive: scenePhase == .active,
            isWindowVisible: windowVisible,
            isLowPower: power.isLowPower,
            thermalState: power.thermalState
        )
    }
}

/// Fort's product identity. It is intentionally separate from agent identity
/// and exposes only the semantic brand energy selected by durable Fort state.
public struct FortProductMarkView: View {
    @Environment(\.fortMarkSurfaceFrame) private var surfaceFrame
    @State private var isOnscreen = false

    private let activity: FortMarkActivity
    private let size: CGFloat
    private let decorative: Bool

    public init(
        activity: FortMarkActivity = .ambient,
        size: CGFloat = 38,
        decorative: Bool = false
    ) {
        self.activity = activity
        self.size = size
        self.decorative = decorative
    }

    public var body: some View {
        let shared = surfaceFrame ?? FortMarkSurfaceFrame(
            phase: 0,
            environment: FortMarkMotionEnvironment(isOnscreen: false)
        )
        let environment = FortMarkMotionEnvironment(
            reduceMotion: shared.environment.reduceMotion,
            isOnscreen: isOnscreen && shared.environment.isOnscreen,
            isSceneActive: shared.environment.isSceneActive,
            isWindowVisible: shared.environment.isWindowVisible,
            isLowPower: shared.environment.isLowPower,
            thermalState: shared.environment.thermalState
        )
        mark(frame: FortMarkMotion.frame(
            activity: activity,
            phase: shared.phase,
            environment: environment
        ))
        .frame(width: size, height: size)
        .accessibilityElement(children: .ignore)
        .accessibilityHidden(decorative)
        .accessibilityLabel("Fort")
        .onAppear { isOnscreen = true }
        .onDisappear {
            isOnscreen = false
        }
    }

    private var color: Color {
        activity == .working
            ? Color(red: 37 / 255, green: 164 / 255, blue: 255 / 255)
            : Color(red: 102 / 255, green: 125 / 255, blue: 150 / 255)
    }

    private func mark(frame: FortMarkMotionFrame) -> some View {
        ZStack {
            Image("FortAgentOrb")
                .resizable()
                .scaledToFill()
                .clipShape(Circle())
                .rotationEffect(.degrees(frame.rotationDegrees))
                .scaleEffect(frame.scale)

            Circle()
                .stroke(color, lineWidth: size < 24 ? 1.2 : 1.8)

            if activity == .working {
                Circle()
                    .trim(from: 0, to: 0.14)
                    .stroke(Color.white, style: StrokeStyle(lineWidth: 2, lineCap: .round))
                    .rotationEffect(.degrees(frame.orbitDegrees))
                    .opacity(0.72 + frame.glowOpacity * 0.28)
            }
        }
        .shadow(
            color: color.opacity(frame.glowOpacity),
            radius: size * (0.12 + frame.glowOpacity * 0.1)
        )
    }
}
