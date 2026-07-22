//
//  FortApp.swift
//  Fort (iOS)
//
//  The iOS control-plane client for Fort. The Command Deck folds sign-offs,
//  projects, direction and Today into one thumb-reachable native surface;
//  Playbooks, crew, week, activity, and settings remain available through More.
//
//  A single FortClient is held at the app root as a @StateObject and injected
//  into the environment; every tab talks to Fort only through it.
//

import SwiftUI
import FortKit

@main
struct FortApp: App {
    /// One client for the whole app. Default base URL is the local control
    /// endpoint (http://127.0.0.1:4087); it's settable in Settings below.
    @StateObject private var client = FortClient()

    var body: some Scene {
        WindowGroup {
            RootTabView()
                .environmentObject(client)
                .tint(FortPalette.brass)
                .preferredColorScheme(.dark)
        }
    }
}

/// The Command Deck owns the canonical five-item mobile navigation. Keeping the
/// root as one NavigationStack prevents a second, competing system tab bar.
struct RootTabView: View {
    var body: some View {
        NavigationStack { BoardView() }
    }
}

/// Minimal host editor. FortClient.baseURL is @Published, so committing a valid
/// URL here re-points every tab's next poll and SSE reconnect.
struct SettingsView: View {
    @EnvironmentObject private var client: FortClient
    @Environment(\.dismiss) private var dismiss
    @State private var urlText: String = ""

    var body: some View {
        NavigationStack {
            Form {
                Section("Control-plane host") {
                    TextField("http://127.0.0.1:4087", text: $urlText)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.URL)
                        .submitLabel(.done)
                        .onSubmit(commit)
                }
                Section {
                    Text("Default is the local control endpoint (:4087). The docs' control-only default is :4091 — both work; the surface adapts to whatever mode the server reports.")
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                }
            }
            .navigationTitle("Settings")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { commit() }
                        .disabled(URL(string: urlText) == nil)
                }
            }
            .onAppear { urlText = client.baseURL.absoluteString }
        }
    }

    private func commit() {
        if let url = URL(string: urlText.trimmingCharacters(in: .whitespacesAndNewlines)) {
            client.baseURL = url
        }
        dismiss()
    }
}
