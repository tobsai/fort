//
//  FeedView.swift
//  Fort (iOS)
//
//  The Activity sheet — the live event stream. Consumes client.events(since:) as an
//  AsyncThrowingStream and prepends each Event to a bounded, newest-first list.
//  On transport/parse failure it reconnects with a short backoff, resuming from
//  the highest event id seen so no frames are lost.
//
//  This file also hosts the small helpers shared across the surface
//  (errorText, ContentUnavailableCompat).
//

import SwiftUI
import FortKit

struct FeedView: View {
    @EnvironmentObject private var client: FortClient

    @State private var events: [Event] = []
    @State private var connected = false
    @State private var streamError: String?

    /// Highest event id seen — used to resume the stream on reconnect.
    @State private var lastID = 0

    /// Keep the in-memory feed bounded; the full log lives server-side.
    private let maxEvents = 500

    var body: some View {
        List {
            Section {
                HStack(spacing: 8) {
                    Circle()
                        .fill(connected ? Color.green : Color.orange)
                        .frame(width: 8, height: 8)
                    Text(connected ? "Live" : (streamError == nil ? "Connecting…" : "Reconnecting…"))
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    Spacer()
                    if let streamError {
                        Text(streamError)
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                            .lineLimit(1)
                    }
                }
            }

            Section("Events") {
                if events.isEmpty {
                    ContentUnavailableCompat(
                        title: "Waiting for events",
                        message: "Live events will appear here as runs progress.",
                        systemImage: "dot.radiowaves.left.and.right"
                    )
                } else {
                    ForEach(events) { event in
                        EventRow(event: event)
                    }
                }
            }
        }
        .listStyle(.insetGrouped)
        .navigationTitle("Feed")
        // Restart the stream whenever the host changes; cancelling this .task
        // cancels the consuming Task, which closes the stream.
        .task(id: client.baseURL) { await consume() }
    }

    /// Consume the SSE stream, reconnecting with backoff on failure. Runs until
    /// the enclosing .task is cancelled (tab change, baseURL change, teardown).
    private func consume() async {
        while !Task.isCancelled {
            do {
                for try await event in client.events(since: lastID) {
                    connected = true
                    streamError = nil
                    if event.id > lastID { lastID = event.id }
                    events.insert(event, at: 0)
                    if events.count > maxEvents {
                        events.removeLast(events.count - maxEvents)
                    }
                }
                // Clean close by the server — loop and reconnect from lastID.
                connected = false
            } catch is CancellationError {
                return
            } catch {
                connected = false
                streamError = errorText(error)
            }

            if Task.isCancelled { return }
            // Backoff before reconnecting so we don't hammer a down server.
            try? await Task.sleep(nanoseconds: 2_000_000_000)
        }
    }
}

/// One row in the event log / feed. Shared by FeedView and RunDetailView.
struct EventRow: View {
    let event: Event

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            HStack(spacing: 8) {
                Text(event.type)
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(color(for: event.type))
                Spacer()
                if let code = event.code {
                    Text("code \(code)")
                        .font(.caption2.monospacedDigit())
                        .foregroundStyle(.secondary)
                }
                Text(shortTime(event.time))
                    .font(.caption2.monospacedDigit())
                    .foregroundStyle(.secondary)
            }
            if let data = event.data, !data.isEmpty {
                Text(data)
                    .font(.footnote)
                    .foregroundStyle(.primary)
                    .lineLimit(3)
            }
            Text(event.runID)
                .font(.caption2.monospaced())
                .foregroundStyle(.tertiary)
        }
        .padding(.vertical, 2)
    }

    private func color(for type: String) -> Color {
        let t = type.lowercased()
        if t.contains("fail") || t.contains("error") { return .red }
        if t.contains("gate") || t.contains("pause") { return .purple }
        if t.contains("done") || t.contains("succeed") { return .green }
        if t.contains("start") || t.contains("run") { return .blue }
        return .primary
    }

    /// Show just the clock portion of an RFC3339-ish timestamp when we can,
    /// else the raw string.
    private func shortTime(_ raw: String) -> String {
        if let tRange = raw.range(of: "T") {
            let after = raw[tRange.upperBound...]
            // Trim any timezone/fractional suffix to HH:MM:SS.
            let clock = after.prefix(8)
            if clock.count == 8 { return String(clock) }
        }
        return raw
    }
}

// MARK: - Shared helpers

/// A human-readable message for an error thrown by FortKit or URLSession.
func errorText(_ error: Error) -> String {
    switch error {
    case let FortClientError.httpStatus(status, body):
        let trimmed = body.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "HTTP \(status)" : "HTTP \(status): \(trimmed)"
    case FortClientError.nonHTTPResponse:
        return "Unexpected non-HTTP response."
    case let urlError as URLError:
        return urlError.localizedDescription
    default:
        return error.localizedDescription
    }
}

/// A small stand-in for `ContentUnavailableView` so the surface builds on the
/// iOS 16 deployment floor (that view is iOS 17+). Uses the system view when
/// available.
struct ContentUnavailableCompat: View {
    let title: String
    let message: String
    let systemImage: String

    var body: some View {
        VStack(spacing: 8) {
            Image(systemName: systemImage)
                .font(.largeTitle)
                .foregroundStyle(.secondary)
            Text(title)
                .font(.headline)
            Text(message)
                .font(.subheadline)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 24)
        .listRowBackground(Color.clear)
    }
}
