import SwiftUI
import AuthenticationServices

/// Shown when the cloud hub wants this device's Google sign-in before it answers (Spec 066).
struct HubSignInBanner: View {
    let model: MessengerModel
    @Environment(\.webAuthenticationSession) private var webAuthenticationSession
    var body: some View {
        if model.googleSignInNeeded {
            HStack(spacing:10) {
                LampView(lamp:.attention)
                VStack(alignment:.leading,spacing:2) {
                    Text("GOOGLE SIGN-IN").font(.caption.weight(.heavy))
                    Text(model.googleSignInMessage ?? "Your hub wants you to sign in with Google first.").font(.caption).foregroundStyle(.secondary).lineLimit(3)
                }
                Spacer(minLength:8)
                Button(model.googleSignInBusy ? "Signing in…" : "Sign in with Google") {
                    Task { await model.signInWithGoogle { url, scheme in try await webAuthenticationSession.authenticate(using:url,callbackURLScheme:scheme) } }
                }
                .modifier(GoldButton(on:!model.googleSignInBusy)).controlSize(.small).disabled(model.googleSignInBusy)
            }
            .padding(8).hudFrame().padding(.horizontal).padding(.vertical,8)
            .accessibilityElement(children:.contain).accessibilityIdentifier("hub-google-sign-in")
        }
    }
}
