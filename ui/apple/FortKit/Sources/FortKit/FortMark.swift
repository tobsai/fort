import Foundation

/// Brand energy is selected from durable Fort state. Motion is never the only
/// status signal, and only evidence-backed Working may use the stronger mode.
public enum FortMarkActivity: String, Sendable, CaseIterable {
    case ambient
    case working

    public static func fromDurableTargetState(_ state: String?) -> FortMarkActivity {
        state == "working" ? .working : .ambient
    }
}

public enum FortMarkThermalState: Int, Sendable, CaseIterable {
    case nominal
    case fair
    case serious
    case critical
}

public struct FortMarkMotionEnvironment: Sendable, Equatable {
    public let reduceMotion: Bool
    public let isOnscreen: Bool
    public let isSceneActive: Bool
    public let isWindowVisible: Bool
    public let isLowPower: Bool
    public let thermalState: FortMarkThermalState

    public init(
        reduceMotion: Bool = false,
        isOnscreen: Bool = true,
        isSceneActive: Bool = true,
        isWindowVisible: Bool = true,
        isLowPower: Bool = false,
        thermalState: FortMarkThermalState = .nominal
    ) {
        self.reduceMotion = reduceMotion
        self.isOnscreen = isOnscreen
        self.isSceneActive = isSceneActive
        self.isWindowVisible = isWindowVisible
        self.isLowPower = isLowPower
        self.thermalState = thermalState
    }

    public var isForegroundVisible: Bool {
        isOnscreen && isSceneActive && isWindowVisible
    }
}

public struct FortMarkMotionFrame: Sendable, Equatable {
    public let rotationDegrees: Double
    public let scale: Double
    public let glowOpacity: Double
    public let orbitDegrees: Double
    public let isAnimating: Bool
    public let frameInterval: TimeInterval?

    public init(
        rotationDegrees: Double,
        scale: Double,
        glowOpacity: Double,
        orbitDegrees: Double,
        isAnimating: Bool,
        frameInterval: TimeInterval?
    ) {
        self.rotationDegrees = rotationDegrees
        self.scale = scale
        self.glowOpacity = glowOpacity
        self.orbitDegrees = orbitDegrees
        self.isAnimating = isAnimating
        self.frameInterval = frameInterval
    }
}

public enum FortMarkMotion {
    public static func glowPeriod(activity: FortMarkActivity) -> TimeInterval {
        activity == .working ? 4 : 8
    }

    public static func frame(
        activity: FortMarkActivity,
        phase: TimeInterval,
        environment: FortMarkMotionEnvironment
    ) -> FortMarkMotionFrame {
        let spatialPeriod: TimeInterval = activity == .working ? 4 : 12
        let spatialPhase = phase * 2 * .pi / spatialPeriod
        let glowPhase = phase * 2 * .pi / glowPeriod(activity: activity)
        let energy = (sin(glowPhase - (.pi / 2)) + 1) / 2
        let drift = activity == .working ? 3.5 : 1.8
        let scaleEnergy = activity == .working ? 0.045 : 0.022
        let spatialMotion = !environment.reduceMotion
        let isAnimating = environment.isForegroundVisible
        return FortMarkMotionFrame(
            rotationDegrees: spatialMotion ? sin(spatialPhase) * drift : 0,
            scale: spatialMotion ? 1 + energy * scaleEnergy : 1,
            glowOpacity: 0.2 + energy * (activity == .working ? 0.22 : 0.12),
            orbitDegrees: spatialMotion ? phase * (activity == .working ? 78 : 24) : 0,
            isAnimating: isAnimating,
            frameInterval: isAnimating ? frameInterval(for: environment) : nil
        )
    }

    private static func frameInterval(for environment: FortMarkMotionEnvironment) -> TimeInterval {
        var interval: TimeInterval = 1 / 30
        if environment.reduceMotion || environment.isLowPower {
            interval = max(interval, 1 / 15)
        }
        switch environment.thermalState {
        case .nominal:
            break
        case .fair:
            interval = max(interval, 1 / 20)
        case .serious:
            interval = max(interval, 1 / 12)
        case .critical:
            interval = max(interval, 1 / 8)
        }
        return interval
    }
}

/// A deterministic surface phase accumulator. Tests and views inject sample
/// times, so pausing and resuming never depends on sleeps or wall-clock catchup.
public struct FortMarkPhaseClock: Sendable, Equatable {
    public private(set) var phase: TimeInterval
    private var previousSample: TimeInterval?

    public init(phase: TimeInterval = 0) {
        self.phase = max(0, phase)
    }

    @discardableResult
    public mutating func sample(at time: TimeInterval, isActive: Bool) -> TimeInterval {
        guard isActive else {
            previousSample = nil
            return phase
        }
        guard let previousSample else {
            self.previousSample = time
            return phase
        }
        if time >= previousSample {
            phase += time - previousSample
        }
        self.previousSample = time
        return phase
    }
}
