//
//  FortApp.swift
//  Fort (iOS)
//
//  The iOS client for Fort's private Primary Channels. Connection setup stays
//  at the app root so physical/TestFlight builds reach the shared Phase 1
//  surface only through a pinned encrypted gateway relay. An isolated direct
//  host exists solely in DEBUG iOS Simulator builds.
//
//  A single FortClient is held at the app root as a @StateObject and injected
//  into the environment; every tab talks to Fort only through it.
//

import SwiftUI
import FortKit

@main
struct FortApp: App {
    /// Physical/TestFlight builds start fail-closed and acquire a usable
    /// transport only after an authenticated, fingerprint-pinned gateway relay
    /// is selected. DEBUG Simulator builds retain the isolated QA host seam.
    @StateObject private var client = FortApp.makeClient()
    @StateObject private var gateway = GatewayCoordinator()

    var body: some Scene {
        WindowGroup {
            RootTabView()
                .environmentObject(client)
                .environmentObject(gateway)
                .task { await gateway.restore(client: client) }
        }
    }

    private static func makeClient() -> FortClient {
        #if DEBUG && targetEnvironment(simulator)
        return FortClient()
        #endif
        return FortClient.gatewayOnly()
    }
}

/// Connection state is deliberately outside the shared Channels hierarchy.
/// Once connected, macOS and iPhone render the same Primary Channels view.
struct RootTabView: View {
    @EnvironmentObject private var client: FortClient
    @EnvironmentObject private var gateway: GatewayCoordinator

    @State private var showConnectionSettings = false

    var body: some View {
        Group {
            if !gateway.restoreComplete {
                ProgressView("Connecting to Fort…")
            } else if gateway.hasPrimaryTransport {
                PrimaryChannelsView()
                    .primaryConnectionSettings { showConnectionSettings = true }
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
            }
        }
        .sheet(isPresented: $showConnectionSettings) { SettingsView() }
    }
}

/// Gateway connection editor. The DEBUG iOS Simulator adds an isolated fixture
/// host section; physical/TestFlight builds compile that entire section out.
struct SettingsView: View {
    @EnvironmentObject private var client: FortClient
    @EnvironmentObject private var gateway: GatewayCoordinator
    @Environment(\.dismiss) private var dismiss
    @State private var gatewayText: String = ""
    @State private var pendingTrust: GatewayMachine?

    #if DEBUG && targetEnvironment(simulator)
    @State private var urlText: String = ""
    #endif

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
                #if DEBUG && targetEnvironment(simulator)
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
                    Text("Direct-host mode exists only in DEBUG iOS Simulator builds for isolated QA fixtures.")
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                }
                #endif
            }
            .navigationTitle("Settings")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                }
            }
            .onAppear {
                gatewayText = gateway.account.gatewayURL?.absoluteString ?? ""
                #if DEBUG && targetEnvironment(simulator)
                urlText = client.baseURL.absoluteString
                #endif
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

    #if DEBUG && targetEnvironment(simulator)
    private func commit() {
        if let url = URL(string: urlText.trimmingCharacters(in: .whitespacesAndNewlines)) {
            gateway.useDirectHost(url, client: client)
        }
        dismiss()
    }
    #endif
}
