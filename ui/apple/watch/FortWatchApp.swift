//
//  FortWatchApp.swift
//  FortWatch
//
//  The watchOS app entry point. Holds the shared FortClient at the app root
//  and injects it into the environment so the glance can read summary/gates
//  and decide the first pending gate with a single tap.
//
//  The client and all wire models come from FortKit — this surface never
//  redefines them.
//

import SwiftUI
import FortKit

@main
struct FortWatchApp: App {
    /// One control-plane client for the whole app. Default base URL is
    /// http://127.0.0.1:4087; change `client.baseURL` to point elsewhere.
    @StateObject private var client = FortClient()

    var body: some Scene {
        WindowGroup {
            GlanceView()
                .environmentObject(client)
        }
    }
}
