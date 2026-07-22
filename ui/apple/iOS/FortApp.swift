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
    @StateObject private var gateway = GatewayCoordinator()

    var body: some Scene {
        WindowGroup {
            RootTabView()
                .environmentObject(client)
                .environmentObject(gateway)
                .tint(FortPalette.brass)
                .preferredColorScheme(.dark)
                .task { await gateway.restore(client: client) }
        }
    }
}

/// The Command Deck owns the canonical five-item mobile navigation. Keeping the
/// root as one NavigationStack prevents a second, competing system tab bar.
struct RootTabView: View {
    @EnvironmentObject private var client: FortClient
    @EnvironmentObject private var gateway: GatewayCoordinator

    @State private var showConnectionSettings = false

    var body: some View {
        Group {
            if !gateway.restoreComplete {
                ProgressView("Connecting to Fort…")
            } else if gateway.connectedMachineID != nil || gateway.directHostEnabled {
                NavigationStack { BoardView() }
            } else {
                VStack(spacing: 16) {
                    Image(systemName: "lock.shield").font(.system(size: 42)).foregroundStyle(FortPalette.brass)
                    Text("Connect to Fort").font(.title2.bold())
                    Text("Sign in to your gateway and choose a machine. Fort will verify and pin its encrypted relay identity.")
                        .multilineTextAlignment(.center)
                        .foregroundStyle(.secondary)
                        .padding(.horizontal, 32)
                    Button("Open connection settings") { showConnectionSettings = true }
                        .buttonStyle(.borderedProminent)
                }
                .sheet(isPresented: $showConnectionSettings) { SettingsView() }
            }
        }
    }
}

/// Minimal host editor. FortClient.baseURL is @Published, so committing a valid
/// URL here re-points every tab's next poll and SSE reconnect.
struct SettingsView: View {
    @EnvironmentObject private var client: FortClient
    @EnvironmentObject private var gateway: GatewayCoordinator
    @Environment(\.dismiss) private var dismiss
    @State private var urlText: String = ""
    @State private var gatewayText: String = ""
    @State private var pendingTrust: GatewayMachine?

    var body: some View {
        NavigationStack {
            Form {
                Section("Remote gateway") {
                    TextField("https://your-fort-gateway.example", text: $gatewayText)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.URL)
                    Button(gateway.account.bearerToken == nil ? "Sign in with Google" : "Sign in again") {
                        guard let url = URL(string: gatewayText.trimmingCharacters(in: .whitespacesAndNewlines)) else { return }
                        gateway.signIn(gatewayURL: url, client: client)
                    }
                    .disabled(URL(string: gatewayText.trimmingCharacters(in: .whitespacesAndNewlines)) == nil)
                    if gateway.isLoading { ProgressView("Loading machines…") }
                    ForEach(gateway.machines) { machine in
                        Button {
                            if gateway.account.pinnedPublicKeys[machine.machineID] == nil {
                                pendingTrust = machine
                            } else {
                                gateway.connect(machine, client: client, trustIfNeeded: false)
                            }
                        } label: {
                            VStack(alignment: .leading, spacing: 4) {
                                HStack {
                                    Text(machine.name)
                                    Spacer()
                                    if gateway.connectedMachineID == machine.machineID { Image(systemName: "checkmark.circle.fill") }
                                    Circle().fill(machine.online ? Color.green : Color.secondary).frame(width: 8, height: 8)
                                }
                                Text(machine.fingerprint).font(.caption.monospaced()).foregroundStyle(.secondary)
                            }
                        }
                    }
                    if gateway.account.bearerToken != nil {
                        Button("Disconnect gateway", role: .destructive) { gateway.disconnect(client: client) }
                    }
                    if let error = gateway.errorMessage { Text(error).foregroundStyle(.red).font(.footnote) }
                }
                Section("Control-plane host") {
                    TextField("http://127.0.0.1:4087", text: $urlText)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.URL)
                        .submitLabel(.done)
                        .onSubmit(commit)
                    Button("Use direct host") { commit() }
                        .disabled(URL(string: urlText) == nil)
                }
                Section {
                    Text("Direct-host mode is for the simulator or a reachable LAN host. A physical iPhone cannot reach your Mac at 127.0.0.1.")
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                }
            }
            .navigationTitle("Settings")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                }
            }
            .onAppear {
                urlText = client.baseURL.absoluteString
                gatewayText = gateway.account.gatewayURL?.absoluteString ?? ""
            }
            .confirmationDialog(
                "Trust this Fort?",
                isPresented: Binding(get: { pendingTrust != nil }, set: { if !$0 { pendingTrust = nil } }),
                presenting: pendingTrust
            ) { machine in
                Button("Trust and connect") {
                    gateway.connect(machine, client: client, trustIfNeeded: true)
                    pendingTrust = nil
                }
                Button("Cancel", role: .cancel) { pendingTrust = nil }
            } message: { machine in
                Text("Verify this fingerprint on the Fort host before trusting it:\n\(machine.fingerprint)")
            }
        }
    }

    private func commit() {
        if let url = URL(string: urlText.trimmingCharacters(in: .whitespacesAndNewlines)) {
            gateway.useDirectHost(url, client: client)
        }
        dismiss()
    }
}
