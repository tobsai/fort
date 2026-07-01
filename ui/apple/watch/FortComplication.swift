//
//  FortComplication.swift
//  FortWatch
//
//  A WidgetKit complication for the watch face showing the pending-gate count
//  from Fort's control plane. Supports the circular and inline accessory
//  families. Data comes from FortKit's FortClient (GET /api/summary).
//
//  The complication reads the count of waiting gates so the wearer can see at a
//  glance whether a human decision is pending. Tapping it launches the app,
//  where the first gate can be approved with a single tap.
//

import WidgetKit
import SwiftUI
import FortKit

/// One timeline point: the number of gates waiting for a decision, plus whether
/// an execution plane is attached (control-only faces still show the count).
struct GateEntry: TimelineEntry {
    let date: Date
    let pendingGates: Int
    let hasExecution: Bool

    static let placeholder = GateEntry(date: .now, pendingGates: 0, hasExecution: true)
}

/// Fetches the current gate count from the control plane on each refresh.
struct GateProvider: TimelineProvider {
    /// How long until WidgetKit asks for a fresh timeline.
    private static let refreshInterval: TimeInterval = 15 * 60

    func placeholder(in context: Context) -> GateEntry {
        .placeholder
    }

    func getSnapshot(in context: Context, completion: @escaping (GateEntry) -> Void) {
        if context.isPreview {
            completion(GateEntry(date: .now, pendingGates: 2, hasExecution: true))
            return
        }
        Task {
            completion(await Self.fetchEntry())
        }
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<GateEntry>) -> Void) {
        Task {
            let entry = await Self.fetchEntry()
            let next = Date(timeIntervalSinceNow: Self.refreshInterval)
            completion(Timeline(entries: [entry], policy: .after(next)))
        }
    }

    /// Reads `GET /api/summary`; on any error falls back to a zero-count entry so
    /// the complication degrades quietly rather than showing stale/garbage data.
    private static func fetchEntry() async -> GateEntry {
        let client = FortClient()
        do {
            let summary = try await client.summary()
            return GateEntry(
                date: .now,
                pendingGates: summary.gates.count,
                hasExecution: summary.execution
            )
        } catch {
            return GateEntry(date: .now, pendingGates: 0, hasExecution: true)
        }
    }
}

/// The complication's rendered view, adapting to the accessory family.
struct FortComplicationView: View {
    @Environment(\.widgetFamily) private var family
    let entry: GateEntry

    var body: some View {
        switch family {
        case .accessoryInline:
            Label("\(entry.pendingGates) gate\(entry.pendingGates == 1 ? "" : "s")",
                  systemImage: "checkmark.shield")
        case .accessoryCircular:
            circular
        default:
            circular
        }
    }

    private var circular: some View {
        ZStack {
            AccessoryWidgetBackground()
            VStack(spacing: 0) {
                Image(systemName: "checkmark.shield")
                    .font(.caption2)
                Text("\(entry.pendingGates)")
                    .font(.system(.title3, design: .rounded).weight(.semibold))
                    .minimumScaleFactor(0.6)
            }
        }
        .widgetLabel {
            Text(entry.hasExecution ? "gates" : "control-only")
        }
    }
}

/// The complication widget. Add its type to the app's `@main WidgetBundle` (or
/// mark this `@main` in the widget-extension target).
struct FortComplication: Widget {
    private let kind = "FortGateComplication"

    var body: some WidgetConfiguration {
        StaticConfiguration(kind: kind, provider: GateProvider()) { entry in
            FortComplicationView(entry: entry)
        }
        .configurationDisplayName("Fort Gates")
        .description("Shows how many gates are waiting for a decision.")
        .supportedFamilies([.accessoryCircular, .accessoryInline])
    }
}

#Preview(as: .accessoryCircular) {
    FortComplication()
} timeline: {
    GateEntry(date: .now, pendingGates: 0, hasExecution: true)
    GateEntry(date: .now, pendingGates: 3, hasExecution: true)
    GateEntry(date: .now, pendingGates: 1, hasExecution: false)
}
