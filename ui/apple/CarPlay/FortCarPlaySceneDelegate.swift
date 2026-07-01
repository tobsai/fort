//
//  FortCarPlaySceneDelegate.swift
//  Fort CarPlay
//
//  Fort's CarPlay surface. It is built entirely from CarPlay's template API
//  (CPTemplate...) — NOT SwiftUI — because CarPlay only renders Apple's
//  system templates. The scene shows two glanceable, driving-safe lists:
//
//    • "Gates"  — each gate awaiting a human decision is one row; tapping it
//                 shows an Approve / Reject alert. Deciding calls
//                 FortClient.decideGate, which returns `false` on HTTP 409
//                 (control-only mode, no execution plane) — we surface that as
//                 a non-fatal notice rather than an error.
//
//    • "Status" — the glanceable /api/summary counts (running, queued, blocked,
//                 succeeded, failed) plus the execution-plane mode.
//
//  Everything is refreshed on connect and on a slow timer. We deliberately keep
//  taps few and text short: the driver should be able to read a screen and act
//  in one glance.
//
//  Depends on the shared FortKit package (../FortKit) for the wire models and
//  the control-plane client.
//

import CarPlay
import FortKit
import Foundation

/// The CarPlay scene delegate. iOS instantiates this for the
/// `CPTemplateApplicationScene` declared in Info.plist (see
/// `Info-CarPlay-notes.md`). It owns the interface controller, the shared
/// `FortClient`, and the refresh timer for the scene's lifetime.
final class FortCarPlaySceneDelegate: UIResponder, CPTemplateApplicationSceneDelegate {

    /// How often we re-poll the control plane while connected. CarPlay favours
    /// glanceable, low-churn UI, so this is intentionally slow.
    private static let refreshInterval: TimeInterval = 5

    /// The control-plane client. One per scene. Point it at another host by
    /// changing the base URL (e.g. the documented control-only default :4091).
    private let client = FortClient()

    /// The interface controller handed to us on connect; the root of the
    /// template stack. Non-nil only while the scene is connected.
    private var interfaceController: CPInterfaceController?

    /// The two tabs. Held so refreshes can swap their `items` in place without
    /// rebuilding the tab bar (which would reset the driver's selection).
    private let gatesTemplate = CPListTemplate(title: "Gates", sections: [])
    private let statusTemplate = CPListTemplate(title: "Status", sections: [])

    /// The periodic refresh timer, valid only while connected.
    private var refreshTimer: Timer?

    /// Guards against overlapping refreshes if one poll outlives the interval.
    private var isRefreshing = false

    // MARK: - CPTemplateApplicationSceneDelegate

    func templateApplicationScene(
        _ templateApplicationScene: CPTemplateApplicationScene,
        didConnect interfaceController: CPInterfaceController
    ) {
        self.interfaceController = interfaceController

        gatesTemplate.tabTitle = "Gates"
        gatesTemplate.tabImage = UIImage(systemName: "checkmark.seal")
        statusTemplate.tabTitle = "Status"
        statusTemplate.tabImage = UIImage(systemName: "gauge.with.dots.needle.67percent")

        // Show a "loading" placeholder immediately so the first frame is never
        // an empty screen while the initial poll is in flight.
        gatesTemplate.updateSections([Self.messageSection("Loading gates…")])
        statusTemplate.updateSections([Self.messageSection("Loading status…")])

        let tabBar = CPTabBarTemplate(templates: [gatesTemplate, statusTemplate])
        interfaceController.setRootTemplate(tabBar, animated: false, completion: nil)

        startRefreshing()
    }

    func templateApplicationScene(
        _ templateApplicationScene: CPTemplateApplicationScene,
        didDisconnectInterfaceController interfaceController: CPInterfaceController
    ) {
        stopRefreshing()
        self.interfaceController = nil
    }

    // MARK: - Refresh loop

    /// Kicks off an immediate refresh and schedules the periodic one.
    private func startRefreshing() {
        refresh()
        let timer = Timer.scheduledTimer(withTimeInterval: Self.refreshInterval, repeats: true) { [weak self] _ in
            self?.refresh()
        }
        // Keep firing while the user scrolls CarPlay lists.
        RunLoop.main.add(timer, forMode: .common)
        refreshTimer = timer
    }

    private func stopRefreshing() {
        refreshTimer?.invalidate()
        refreshTimer = nil
    }

    /// Pulls a fresh `/api/summary` and rebuilds both tabs from it. The summary
    /// alone carries everything the two glanceable tabs need — the counts and
    /// the waiting gates — so one request drives both.
    ///
    /// Called from the main run loop (connect + timer). The `isRefreshing`
    /// guard and all template mutation stay on the main thread; only the
    /// network call runs off it, so there is no shared-state race.
    private func refresh() {
        guard !isRefreshing else { return }
        isRefreshing = true

        Task { [weak self] in
            guard let self else { return }
            do {
                let summary = try await self.client.summary()
                await MainActor.run {
                    self.applyGates(summary.gates, executionAttached: summary.execution)
                    self.applyStatus(summary)
                    self.isRefreshing = false
                }
            } catch {
                await MainActor.run {
                    self.gatesTemplate.updateSections([Self.messageSection("Can't reach Fort")])
                    self.statusTemplate.updateSections([Self.messageSection("Can't reach Fort")])
                    self.isRefreshing = false
                }
            }
        }
    }

    // MARK: - Gates tab

    /// Rebuilds the Gates tab. Each gate is a single row; the detail text is the
    /// gate's input (trimmed) so the driver sees what they're approving without
    /// drilling in.
    private func applyGates(_ gates: [GateItem], executionAttached: Bool) {
        guard !gates.isEmpty else {
            let text = executionAttached ? "No gates waiting" : "No gates (control-only)"
            gatesTemplate.updateSections([Self.messageSection(text)])
            return
        }

        let items: [CPListItem] = gates.map { gate in
            let item = CPListItem(text: Self.gateTitle(gate), detailText: Self.gateDetail(gate))
            item.handler = { [weak self] _, completion in
                self?.presentGateDecision(for: gate)
                completion()
            }
            return item
        }

        gatesTemplate.updateSections([CPListSection(items: items)])
    }

    /// Presents the Approve / Reject alert for one gate. Both actions call
    /// `decideGate`; a `false` return means HTTP 409 (control-only, no execution
    /// plane) which we surface as a non-fatal notice instead of an error.
    private func presentGateDecision(for gate: GateItem) {
        guard let interfaceController else { return }

        let approve = CPAlertAction(title: "Approve", style: .default) { [weak self] _ in
            self?.decide(gate: gate, decision: "approve")
        }
        let reject = CPAlertAction(title: "Reject", style: .destructive) { [weak self] _ in
            self?.decide(gate: gate, decision: "reject")
        }
        let cancel = CPAlertAction(title: "Cancel", style: .cancel) { [weak self] _ in
            self?.interfaceController?.dismissTemplate(animated: true, completion: nil)
        }

        let alert = CPAlertTemplate(
            titleVariants: [Self.gateTitle(gate), gate.nodeID],
            actions: [approve, reject, cancel]
        )
        interfaceController.presentTemplate(alert, animated: true, completion: nil)
    }

    /// Sends the decision, dismisses the alert, and reflects the outcome:
    /// success → refresh so the gate disappears; 409 → a "no execution plane"
    /// notice; error → a generic failure notice. All notices auto-dismiss.
    private func decide(gate: GateItem, decision: String) {
        Task { [weak self] in
            guard let self else { return }
            var applied = false
            var failed = false
            do {
                applied = try await self.client.decideGate(
                    run: gate.runID,
                    node: gate.nodeID,
                    decision: decision
                )
            } catch {
                failed = true
            }

            await MainActor.run {
                self.interfaceController?.dismissTemplate(animated: true, completion: nil)
                if failed {
                    self.presentNotice(title: "Couldn't send decision", message: "Try again in a moment.")
                } else if !applied {
                    // decideGate returned false → HTTP 409, control-only mode.
                    self.presentNotice(
                        title: "No execution plane",
                        message: "Fort is control-only — gate can't be decided here."
                    )
                } else {
                    // Applied: pull fresh state so the decided gate drops off.
                    self.refresh()
                }
            }
        }
    }

    // MARK: - Status tab

    /// Rebuilds the Status tab from the summary counts. One row per meaningful
    /// count, plus a mode row that names control-only vs. full execution.
    private func applyStatus(_ summary: Summary) {
        let mode = summary.execution ? "Execution plane attached" : "Control-only (no engine)"
        let modeItem = CPListItem(text: "Mode", detailText: mode)

        let counts: [(String, Int)] = [
            ("Running", summary.running),
            ("Queued", summary.queued),
            ("Blocked", summary.blocked),
            ("Succeeded", summary.succeeded),
            ("Failed", summary.failed),
            ("Total", summary.total),
        ]

        let countItems = counts.map { label, value in
            CPListItem(text: label, detailText: String(value))
        }

        statusTemplate.updateSections([
            CPListSection(items: [modeItem]),
            CPListSection(items: countItems),
        ])
    }

    // MARK: - Notices

    /// Shows a brief, self-dismissing alert. Used for non-fatal outcomes
    /// (control-only 409, transient send failure) — never blocks the driver.
    private func presentNotice(title: String, message: String) {
        guard let interfaceController else { return }
        let ok = CPAlertAction(title: "OK", style: .default) { [weak self] _ in
            self?.interfaceController?.dismissTemplate(animated: true, completion: nil)
        }
        let alert = CPAlertTemplate(titleVariants: [title, message], actions: [ok])
        interfaceController.presentTemplate(alert, animated: true, completion: nil)
    }

    // MARK: - Formatting helpers

    private static func gateTitle(_ gate: GateItem) -> String {
        "Gate · \(gate.runID)"
    }

    private static func gateDetail(_ gate: GateItem) -> String {
        let input = gate.input?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        if input.isEmpty {
            return gate.nodeID
        }
        // Keep it short: driving-safe rows shouldn't wrap into a paragraph.
        let oneLine = input.replacingOccurrences(of: "\n", with: " ")
        return oneLine.count > 60 ? String(oneLine.prefix(60)) + "…" : oneLine
    }

    /// A single non-interactive row used for empty/loading/error placeholders.
    private static func messageSection(_ text: String) -> CPListSection {
        CPListSection(items: [CPListItem(text: text, detailText: nil)])
    }
}
