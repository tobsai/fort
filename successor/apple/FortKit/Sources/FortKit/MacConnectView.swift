#if os(macOS)
import SwiftUI

/// The Mac's setup sheet (revision 2026-11-06): two guided steps with one button each, and every other control
/// (startup registration, Claude on this Mac, cancelling or recovering a setup) under More options, unchanged.
/// When the Mac connects the sheet closes by itself and the first run takes over in the main window.
struct MacConnectView: View {
    @Bindable var model: MessengerModel
    /// Screenshots pass the facts; the app reads them from the model.
    var facts: DevicesBoard.Facts?
    @State private var showMore = false
    private var recovering: Bool { model.pendingOwnerSetup?.phase == .canceling || model.pendingOwnerSetup?.phase == .canceled || model.hasCompletedHostRemoval }

    var body: some View {
        let steps = MacConnectSteps.make(facts ?? model.devicesFacts).steps
        Form {
            Section {
                VStack(alignment:.leading,spacing:6) {
                    Text("Set up this Mac").font(FortTheme.display(24)).tracking(2.6).textCase(.uppercase).accessibilityAddTraits(.isHeader)
                    Text("Two steps. Accounts, agents and devices come after, from Setup.").font(.callout).foregroundStyle(FortTheme.muted)
                }.padding(.vertical,4)
                ForEach(Array(steps.enumerated()),id:\.element.id) { index, step in
                    HStack(alignment:.top,spacing:12) {
                        ZStack {
                            Circle().fill(step.done ? FortTheme.done : Color.clear)
                            Circle().strokeBorder(step.done ? FortTheme.done : step.current ? FortTheme.accent : FortTheme.line,lineWidth:FortTheme.stroke)
                            Text(step.done ? "✓" : "\(index + 1)").font(.system(size:12,weight:.bold)).foregroundStyle(step.done ? FortTheme.captionInk : step.current ? FortTheme.accent : FortTheme.muted)
                        }.frame(width:26,height:26).accessibilityHidden(true)
                        VStack(alignment:.leading,spacing:2) {
                            Text(step.title).font(.body.weight(.semibold))
                            Text(step.line).font(.callout).foregroundStyle(FortTheme.muted).fixedSize(horizontal:false,vertical:true)
                        }
                        Spacer(minLength:8)
                        if let action = step.action { button(action) }
                    }
                    .padding(.vertical,4).opacity(step.done ? 0.7 : 1)
                    .accessibilityElement(children:.contain).accessibilityIdentifier("mac-connect-\(step.id)")
                }
                if model.ownerSetupBusy || model.localHost?.isBusy == true { ProgressView().controlSize(.small) }
                if !showMore && !recovering && !model.status.isEmpty { CopyableText(model.status,id:"mac-connect-status").font(.caption).foregroundStyle(FortTheme.muted) }
            }
            Section {
                Button { showMore.toggle() } label: {
                    Label("More options",systemImage:showMore || recovering ? "chevron.down" : "chevron.right").font(.callout.weight(.semibold)).foregroundStyle(FortTheme.muted)
                }.buttonStyle(.plain).disabled(recovering).accessibilityValue(showMore || recovering ? "Expanded" : "Collapsed").accessibilityIdentifier("mac-connect-more")
            }
            if showMore || recovering { ConnectionSections(model:model) }
        }
        .formStyle(.grouped)
        .tint(FortTheme.accent)
    }

    @ViewBuilder private func button(_ action: MacConnectSteps.Action) -> some View {
        switch action {
        case .enableHost:
            Button("Turn on") { Task { await model.enableLocalHost() } }.buttonStyle(ComicButtonStyle(fill:FortTheme.accent))
                .disabled(facts == nil && !model.canEnableLocalHost).accessibilityIdentifier("mac-connect-enable")
        case .checkAgain:
            Button("Check again") { Task { await model.inspectLocalHost() } }.buttonStyle(ComicButtonStyle())
                .disabled(facts == nil && !model.canInspectLocalHost).accessibilityIdentifier("mac-connect-check")
        case .connect:
            Button("Connect") { Task { await model.completeOwnerSetup() } }.buttonStyle(ComicButtonStyle(fill:FortTheme.accent))
                .disabled(facts == nil && !model.canCompleteOwnerSetup).accessibilityIdentifier("mac-connect-connect")
        }
    }
}
#endif
