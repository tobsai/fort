import SwiftUI

/// The first run, in place of the Quest log until there is an Agent: two steps, then "You're set" and a clear way in.
/// The same steps and words as welcome() on the web. Devices are optional, so they are mentioned and never asked for.
struct GetStartedView: View {
    @Bindable var model: MessengerModel
    /// The home chat of the Agent that finished setup, to show "You're set" before Fort opens on it.
    let readyChat: String?
    let openSetup: (PauseTab) -> Void
    let finish: (String?) -> Void

    var body: some View {
        let plan = SetupSteps.make(account:model.setupAccountReady,agents:model.agents.count)
        ScrollView {
            VStack(alignment:.leading,spacing:14) {
                if readyChat != nil && plan.done { ready } else { steps(plan) }
            }
            .frame(maxWidth:560,alignment:.leading).padding(.horizontal,22).padding(.top,56).padding(.bottom,32)
            .frame(maxWidth:.infinity)
        }
        .accessibilityIdentifier("get-started")
    }

    private func steps(_ plan: SetupSteps) -> some View {
        VStack(alignment:.leading,spacing:14) {
            Text("Set up Fort").font(FortTheme.display(30)).tracking(3.6).textCase(.uppercase).accessibilityAddTraits(.isHeader)
            Text("Two steps and you’re in. Fort runs your agents in the cloud, so there’s nothing else to install.").font(.title3).foregroundStyle(FortTheme.muted)
            VStack(spacing:10) {
                ForEach(Array(plan.steps.enumerated()),id:\.element.id) { index, step in
                    HStack(spacing:14) {
                        number(index,step)
                        VStack(alignment:.leading,spacing:2) {
                            Text(step.title).font(.body.weight(.semibold))
                            Text(step.done ? "Done." : step.line).font(.callout).foregroundStyle(FortTheme.muted)
                        }
                        Spacer(minLength:8)
                        if !step.done {
                            Button(step.action) { openSetup(step.tab) }.buttonStyle(ComicButtonStyle(fill:step.current ? FortTheme.accent : FortTheme.panel))
                                .accessibilityIdentifier("get-started-\(step.id)")
                        }
                    }
                    .padding(14).frame(maxWidth:.infinity,alignment:.leading)
                    .comicPanel(live:step.current)
                    .opacity(step.done ? 0.7 : 1)
                }
            }.padding(.top,8)
            Text("Optional, any time: connect another Mac or pair the iPhone app from Setup, Devices.").font(.callout).foregroundStyle(FortTheme.muted)
            if !model.agents.isEmpty {
                Button("Skip to the Quest log") { finish(nil) }.buttonStyle(.plain).font(.callout.weight(.semibold)).foregroundStyle(FortTheme.link)
            }
        }
    }

    private func number(_ index: Int, _ step: SetupSteps.Step) -> some View {
        ZStack {
            Circle().fill(step.done ? FortTheme.done : Color.clear)
            Circle().strokeBorder(step.done ? FortTheme.done : step.current ? FortTheme.accent : FortTheme.line,lineWidth:FortTheme.stroke)
            Text(step.done ? "✓" : "\(index + 1)").font(.system(size:13,weight:.bold)).foregroundStyle(step.done ? FortTheme.captionInk : step.current ? FortTheme.accent : FortTheme.muted)
        }.frame(width:28,height:28).accessibilityHidden(true)
    }

    private var ready: some View {
        VStack(alignment:.leading,spacing:14) {
            CaptionBox(text:"Setup done",fill:FortTheme.done)
            Text("You’re set").font(FortTheme.display(30)).tracking(3.6).textCase(.uppercase).accessibilityAddTraits(.isHeader)
            Text("From here on Fort opens on two things: the Quest log, for anything that needs you, and your Chats.").font(.title3).foregroundStyle(FortTheme.muted)
            HStack(spacing:12) {
                Button("Start chatting") { finish(readyChat) }.buttonStyle(ComicButtonStyle(fill:FortTheme.accent)).accessibilityIdentifier("get-started-chat")
                Button("Go to the Quest log") { finish(nil) }.buttonStyle(.plain).font(.callout.weight(.semibold)).foregroundStyle(FortTheme.link)
            }.padding(.top,8)
        }
    }
}

extension MessengerModel {
    /// An account is ready once this Mac's provider sign-in is selected, or, where this window can't manage sign-ins
    /// (the hub's workspace, an iPhone), once the runtime it talks to is ready.
    var setupAccountReady: Bool {
        if canManageProviderAccount { return accountSignInStatus?.accountSelected == true }
        return runtimeReadiness?.state == "ready"
    }
}
