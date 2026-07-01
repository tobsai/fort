//
//  FortApp.swift
//  Fort (iOS)
//
//  The iOS control-plane client for Fort. A three-tab surface over FortKit:
//    • Board  — runs with status badges + a chat field (client.chat)
//    • Gates  — the gate inbox with Approve/Reject (client.decideGate)
//    • Feed   — the SSE live feed (client.events)
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
        }
    }
}

/// The app's tab bar. Board / Gates / Feed, plus a lightweight Settings sheet
/// for pointing the client at a different host.
struct RootTabView: View {
    @EnvironmentObject private var client: FortClient
    @State private var showSettings = false

    var body: some View {
        TabView {
            NavigationStack {
                BoardView()
                    .toolbar { settingsButton }
            }
            .tabItem { Label("Board", systemImage: "square.stack.3d.up") }

            NavigationStack {
                GatesView()
                    .toolbar { settingsButton }
            }
            .tabItem { Label("Gates", systemImage: "hand.raised") }

            NavigationStack {
                FeedView()
                    .toolbar { settingsButton }
            }
            .tabItem { Label("Feed", systemImage: "dot.radiowaves.left.and.right") }
        }
        .sheet(isPresented: $showSettings) {
            SettingsView()
                .environmentObject(client)
        }
    }

    @ToolbarContentBuilder
    private var settingsButton: some ToolbarContent {
        ToolbarItem(placement: .navigationBarTrailing) {
            Button {
                showSettings = true
            } label: {
                Image(systemName: "gearshape")
            }
            .accessibilityLabel("Settings")
        }
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
