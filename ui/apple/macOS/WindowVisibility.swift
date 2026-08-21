//
//  WindowVisibility.swift
//  FortMac
//
//  Window-scoped AppKit visibility for pausing the shared living-mark clock.
//

import AppKit
import SwiftUI

struct FortWindowVisibilityObserver: NSViewRepresentable {
    @Binding var isVisible: Bool

    func makeCoordinator() -> Coordinator {
        Coordinator(isVisible: $isVisible)
    }

    func makeNSView(context: Context) -> WindowAttachmentView {
        let view = WindowAttachmentView()
        view.windowDidChange = { [weak coordinator = context.coordinator] window in
            coordinator?.attach(to: window)
        }
        return view
    }

    func updateNSView(_ view: WindowAttachmentView, context: Context) {
        context.coordinator.update(isVisible: $isVisible)
        context.coordinator.attach(to: view.window)
    }

    static func dismantleNSView(_ view: WindowAttachmentView, coordinator: Coordinator) {
        view.windowDidChange = nil
        coordinator.detach()
    }

    final class Coordinator {
        private weak var window: NSWindow?
        private var observations: [NSObjectProtocol] = []
        private var setVisible: (Bool) -> Void
        private var lastPublishedVisibility: Bool?

        init(isVisible: Binding<Bool>) {
            setVisible = { isVisible.wrappedValue = $0 }
        }

        func update(isVisible: Binding<Bool>) {
            setVisible = { isVisible.wrappedValue = $0 }
            publish()
        }

        func attach(to nextWindow: NSWindow?) {
            guard window !== nextWindow else {
                publish()
                return
            }
            detach()
            guard let nextWindow else { return }
            window = nextWindow
            let center = NotificationCenter.default
            for name in [
                NSWindow.didChangeOcclusionStateNotification,
                NSWindow.didMiniaturizeNotification,
                NSWindow.didDeminiaturizeNotification,
            ] {
                observations.append(center.addObserver(
                    forName: name,
                    object: nextWindow,
                    queue: .main
                ) { [weak self] _ in
                    self?.publish()
                })
            }
            observations.append(center.addObserver(
                forName: NSWindow.willCloseNotification,
                object: nextWindow,
                queue: .main
            ) { [weak self] _ in
                self?.detach()
            })
            publish()
        }

        func detach() {
            let center = NotificationCenter.default
            for observation in observations {
                center.removeObserver(observation)
            }
            observations.removeAll()
            window = nil
            publish(false)
        }

        private func publish() {
            guard let window else {
                publish(false)
                return
            }
            publish(
                window.isVisible
                    && !window.isMiniaturized
                    && window.occlusionState.contains(.visible)
            )
        }

        private func publish(_ visible: Bool) {
            guard lastPublishedVisibility != visible else { return }
            lastPublishedVisibility = visible
            setVisible(visible)
        }

        deinit {
            let center = NotificationCenter.default
            for observation in observations {
                center.removeObserver(observation)
            }
        }
    }
}

final class WindowAttachmentView: NSView {
    var windowDidChange: ((NSWindow?) -> Void)?

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        windowDidChange?(window)
    }
}
