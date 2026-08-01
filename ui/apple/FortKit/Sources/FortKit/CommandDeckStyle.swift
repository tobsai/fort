import SwiftUI

public enum FortOrbMotion {
    public static func shouldPulse(state: FortProjectState) -> Bool {
        state == .working
    }

    public static func allowsSpatialMotion(state: FortProjectState, reduceMotion: Bool) -> Bool {
        state == .working && !reduceMotion
    }
}

public enum FortPalette {
    public static let canvas = Color(red: 2 / 255, green: 9 / 255, blue: 20 / 255)
    public static let page = Color(red: 3 / 255, green: 11 / 255, blue: 20 / 255)
    public static let panel = Color(red: 7 / 255, green: 19 / 255, blue: 32 / 255)
    public static let line = Color(red: 16 / 255, green: 34 / 255, blue: 53 / 255)
    public static let raised = Color(red: 36 / 255, green: 84 / 255, blue: 127 / 255)
    public static let outline = Color(red: 41 / 255, green: 68 / 255, blue: 95 / 255)
    public static let primary = Color(red: 241 / 255, green: 246 / 255, blue: 252 / 255)
    public static let body = Color(red: 185 / 255, green: 199 / 255, blue: 215 / 255)
    public static let muted = Color(red: 133 / 255, green: 150 / 255, blue: 170 / 255)
    public static let faint = Color(red: 102 / 255, green: 125 / 255, blue: 150 / 255)
    public static let brass = Color(red: 22 / 255, green: 140 / 255, blue: 255 / 255)
    public static let brassBright = Color(red: 96 / 255, green: 184 / 255, blue: 255 / 255)
    public static let working = Color(red: 37 / 255, green: 164 / 255, blue: 255 / 255)
    public static let needsYou = Color(red: 214 / 255, green: 159 / 255, blue: 53 / 255)
    public static let accepted = Color(red: 102 / 255, green: 199 / 255, blue: 145 / 255)
    public static let failed = Color(red: 221 / 255, green: 112 / 255, blue: 123 / 255)
    public static let queued = Color(red: 21 / 255, green: 42 / 255, blue: 66 / 255)
}

public extension FortProjectState {
    var color: Color {
        switch self {
        case .needsYou: return FortPalette.needsYou
        case .working: return FortPalette.working
        case .delivered: return FortPalette.accepted
        case .failed: return FortPalette.failed
        case .idle: return FortPalette.faint
        }
    }
}

/// The supplied Fort intelligence-core raster with motion tied only to
/// evidence-backed Working state. The asset is resolved from each Apple app's
/// main asset catalog so FortKit can share the truthful motion treatment.
public struct FortAgentOrbView: View {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    private let name: String
    private let state: FortProjectState
    private let size: CGFloat

    public init(name: String, state: FortProjectState, size: CGFloat = 38) {
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

/// The generated project identity with a separate status ring.
public struct FortSigilView: View {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    private let name: String
    private let state: FortProjectState
    private let size: CGFloat

    public init(name: String, state: FortProjectState, size: CGFloat = 38) {
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
                Circle()
                    .stroke(state.color, lineWidth: 2)

                Canvas { context, canvasSize in
                    let inset = canvasSize.width * 0.13
                    let unit = (canvasSize.width - inset * 2) / 5
                    for cell in FortSigil.cells(for: name) {
                        let rect = CGRect(
                            x: inset + CGFloat(cell.x) * unit + unit * 0.06,
                            y: inset + CGFloat(cell.y) * unit + unit * 0.06,
                            width: unit * 0.88,
                            height: unit * 0.88
                        )
                        context.fill(
                            Path(roundedRect: rect, cornerRadius: unit * 0.2),
                            with: .color(state.color)
                        )
                    }
                }
                .rotationEffect(.degrees(drift))
                .scaleEffect(spatialMotion ? 0.99 + energy * 0.045 : 1)

                if state == .working {
                    Circle()
                        .trim(from: 0, to: 0.12)
                        .stroke(Color.white, style: StrokeStyle(lineWidth: 2.2, lineCap: .round))
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

public struct FortStatusPill: View {
    private let state: FortProjectState

    public init(_ state: FortProjectState) {
        self.state = state
    }

    public var body: some View {
        Text(state.label.lowercased())
            .font(.caption2.weight(.semibold))
            .padding(.horizontal, 9)
            .padding(.vertical, 4)
            .foregroundStyle(state.color)
            .background(state.color.opacity(0.13), in: Capsule())
    }
}

/// Segmented progress made only from accepted/waiting checkpoints.
public struct FortCheckpointBar: View {
    private let checkpoints: CheckpointSummary?

    public init(_ checkpoints: CheckpointSummary?) {
        self.checkpoints = checkpoints
    }

    public var body: some View {
        let total = max(checkpoints?.total ?? 0, 1)
        HStack(spacing: 4) {
            ForEach(0..<total, id: \.self) { index in
                segment(index)
                    .frame(maxWidth: .infinity)
                    .frame(height: 7)
            }
        }
        .accessibilityLabel(checkpoints?.deckCaption ?? "No checkpoints yet")
    }

    @ViewBuilder
    private func segment(_ index: Int) -> some View {
        let accepted = checkpoints?.accepted ?? 0
        let waiting = checkpoints?.waiting ?? 0
        let isCurrent = (checkpoints?.done ?? 0) > accepted && index == accepted + waiting
        if index < accepted {
            RoundedRectangle(cornerRadius: 3).fill(FortPalette.accepted)
        } else if index < accepted + waiting {
            RoundedRectangle(cornerRadius: 3).fill(FortPalette.needsYou)
        } else if isCurrent {
            RoundedRectangle(cornerRadius: 3).fill(FortPalette.working)
        } else {
            RoundedRectangle(cornerRadius: 3)
                .fill(FortPalette.panel)
                .overlay(RoundedRectangle(cornerRadius: 3).stroke(FortPalette.outline))
        }
    }
}

public struct FortDeckCard<Content: View>: View {
    private let accent: Color?
    private let content: Content

    public init(accent: Color? = nil, @ViewBuilder content: () -> Content) {
        self.accent = accent
        self.content = content()
    }

    public var body: some View {
        content
            .padding(16)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 12))
            .overlay {
                RoundedRectangle(cornerRadius: 12)
                    .stroke(accent?.opacity(0.75) ?? FortPalette.line, lineWidth: 1)
            }
            .overlay(alignment: .leading) {
                if let accent {
                    Rectangle().fill(accent).frame(width: 3).clipShape(Capsule())
                }
            }
    }
}

/// The immutable route card shared by the native handoff composers.
public struct FortRoutePreviewCard: View {
    private let preview: RoutePreview
    private let onChange: (() -> Void)?

    public init(_ preview: RoutePreview, onChange: (() -> Void)? = nil) {
        self.preview = preview
        self.onChange = onChange
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 8) {
                Text("ROUTE")
                    .font(.caption2.weight(.semibold))
                    .tracking(1.2)
                    .foregroundStyle(FortPalette.working)
                Text(preview.playbookName).font(.callout.weight(.semibold))
                Spacer()
                if let onChange {
                    Button("Change…", action: onChange)
                        .buttonStyle(.plain)
                        .font(.callout.weight(.medium))
                        .foregroundStyle(FortPalette.brassBright)
                }
            }

            if preview.delivery == "answer", let stage = preview.stages.first {
                HStack(spacing: 7) {
                    Image(systemName: "bolt.fill").foregroundStyle(FortPalette.brassBright)
                    Text("Quick answer").font(.callout.weight(.semibold))
                    Text("· \(stage.agent)").foregroundStyle(FortPalette.body)
                    if let model = stage.model, !model.isEmpty {
                        Text("· \(model)").font(.caption.monospaced()).foregroundStyle(FortPalette.body)
                    }
                }
            } else {
                VStack(alignment: .leading, spacing: 7) {
                    ForEach(preview.stages) { stage in
                        HStack(alignment: .firstTextBaseline, spacing: 9) {
                            Text("\(stage.order)")
                                .font(.caption2.monospaced().weight(.semibold))
                                .foregroundStyle(FortPalette.faint)
                                .frame(width: 12, alignment: .leading)
                            Text(stage.agent).font(.callout.weight(.semibold))
                            Text(stage.name.lowercased()).font(.callout).foregroundStyle(FortPalette.body)
                            if let model = stage.model, !model.isEmpty {
                                Text("· \(model)").font(.caption.monospaced()).foregroundStyle(FortPalette.body)
                            }
                            if stage.memory == true {
                                Image(systemName: "memorychip.fill").font(.caption2).foregroundStyle(FortPalette.working)
                            }
                        }
                    }
                }
            }

            Text(routeNote)
                .font(.caption)
                .foregroundStyle(FortPalette.muted)
        }
        .padding(15)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(FortPalette.working.opacity(0.07), in: RoundedRectangle(cornerRadius: 12))
        .overlay(RoundedRectangle(cornerRadius: 12).stroke(FortPalette.raised))
    }

    private var routeNote: String {
        if preview.delivery == "answer" {
            return "Answering inline — no assignment, checkpoints, or schedule entry."
        }
        return preview.planGate
            ? "Plan first — you sign off before build starts."
            : "Starts when you hand it off."
    }
}

/// A stacked/horizontal playbook card that keeps agents in sans and models in
/// mono, matching the canonical Playbooks pane on both Apple form factors.
public struct FortPlaybookCard: View {
    private let playbook: Playbook
    private let onDuplicate: (() -> Void)?
    private let onPlanGateChange: ((Bool) -> Void)?

    public init(
        _ playbook: Playbook,
        onDuplicate: (() -> Void)? = nil,
        onPlanGateChange: ((Bool) -> Void)? = nil
    ) {
        self.playbook = playbook
        self.onDuplicate = onDuplicate
        self.onPlanGateChange = onPlanGateChange
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 11) {
            HStack(spacing: 9) {
                if playbook.delivery == "answer" {
                    Image(systemName: "bolt.fill").foregroundStyle(FortPalette.brassBright)
                }
                Text(playbook.name).font(.headline)
                if playbook.isDefault == true {
                    Text("default")
                        .font(.caption2.weight(.semibold))
                        .foregroundStyle(FortPalette.brassBright)
                        .padding(.horizontal, 8).padding(.vertical, 3)
                        .background(FortPalette.brass.opacity(0.14), in: Capsule())
                }
                Spacer()
                if let onPlanGateChange, playbook.delivery != "answer" {
                    Text("plan gate \(playbook.planGate == true ? "on" : "off")")
                        .font(.caption).foregroundStyle(FortPalette.faint)
                    Toggle("", isOn: Binding(
                        get: { playbook.planGate == true },
                        set: onPlanGateChange
                    ))
                    .labelsHidden().toggleStyle(.switch).tint(FortPalette.brass)
                } else if playbook.delivery != "answer" {
                    Text("plan gate \(playbook.planGate == true ? "on" : "off")")
                        .font(.caption).foregroundStyle(FortPalette.faint)
                }
                if let onDuplicate {
                    Button(action: onDuplicate) { Image(systemName: "plus.square.on.square") }
                        .buttonStyle(.plain).foregroundStyle(FortPalette.brassBright)
                        .help("Duplicate playbook")
                }
            }

            ViewThatFits(in: .horizontal) {
                HStack(spacing: 8) { pipeline(horizontal: true) }
                VStack(alignment: .leading, spacing: 8) { pipeline(horizontal: false) }
            }

            Text(playbook.delivery == "answer"
                 ? "Replies inline · no checkpoints · nothing scheduled"
                 : "\(playbook.stages.count) stage\(playbook.stages.count == 1 ? "" : "s") · trigger: \(playbook.trigger.kind.replacingOccurrences(of: "_", with: " "))")
                .font(.caption).foregroundStyle(FortPalette.muted)
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 12))
        .overlay(RoundedRectangle(cornerRadius: 12).stroke(playbook.isDefault == true ? FortPalette.raised : FortPalette.line))
    }

    @ViewBuilder
    private func pipeline(horizontal: Bool) -> some View {
        ForEach(Array(playbook.stages.enumerated()), id: \.element.id) { index, stage in
            stageChip(stage)
            if horizontal && index < playbook.stages.count - 1 {
                Image(systemName: "arrow.right").font(.caption).foregroundStyle(FortPalette.faint)
            }
        }
    }

    private func stageChip(_ stage: PlaybookStage) -> some View {
        let primary = stage.assignments.first(where: { $0.taskType == nil }) ?? stage.assignments.first
        let branches = stage.assignments.filter { $0.taskType != nil }
        return HStack(spacing: 5) {
            Text(primary?.agent ?? "Unassigned").font(.caption.weight(.semibold))
            if let model = primary?.model, !model.isEmpty {
                Text("· \(model)").font(.caption2.monospaced()).foregroundStyle(FortPalette.body)
            }
            if stage.memory == true {
                Text("· memory").font(.caption2).foregroundStyle(FortPalette.working)
                Circle().fill(FortPalette.working).frame(width: 5, height: 5)
            }
            if let branch = branches.first {
                Text("/ \(branch.agent) on \((branch.taskType ?? "typed").replacingOccurrences(of: "_", with: " ")) tasks")
                    .font(.caption2).foregroundStyle(FortPalette.faint)
            }
        }
        .padding(.horizontal, 11).padding(.vertical, 6)
        .background(FortPalette.line, in: Capsule())
    }
}

public enum FortTime {
    public static func relative(_ value: String?, now: Date = Date()) -> String {
        guard let value, let date = ISO8601DateFormatter().date(from: value) else { return "just now" }
        let seconds = max(0, Int(now.timeIntervalSince(date)))
        if seconds < 60 { return "just now" }
        if seconds < 3_600 { return "\(seconds / 60)m ago" }
        if seconds < 86_400 { return "\(seconds / 3_600)h ago" }
        return "\(seconds / 86_400)d ago"
    }

    public static func elapsed(_ value: String?, now: Date = Date()) -> String {
        guard let value, let date = ISO8601DateFormatter().date(from: value) else { return "now" }
        let seconds = max(0, Int(now.timeIntervalSince(date)))
        if seconds < 3_600 { return "\(max(1, seconds / 60))m in" }
        return "\(seconds / 3_600)h in"
    }
}
