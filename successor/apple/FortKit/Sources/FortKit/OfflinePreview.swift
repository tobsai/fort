#if DEBUG
import Foundation

private struct PreviewCredentials: DeviceCredentials {
    func save(_ connection: DeviceConnection, token: String) throws { throw URLError(.notConnectedToInternet) }
    func token(for connection: DeviceConnection) throws -> String { throw URLError(.notConnectedToInternet) }
}

@MainActor public enum OfflinePreview {
    public static func make(directory: URL) throws -> MessengerModel {
        let model = MessengerModel(storage:try DeviceStorage(directory:directory,credentials:PreviewCredentials()),session:.shared,isOfflinePreview:true)
        model.installation = Installation(contractVersion:"1.0",installationId:"offline-sample",recoveryEpoch:"sample-epoch",mode:"fixture",serviceName:"Offline preview")
        let binding = Binding(bindingRevisionId:"sample-binding",provider:"offline",runtimeVersion:"sample",selectedModel:"Sample replies",hostId:"This Mac · offline",enabled:false)
        model.agents = [Agent(agentId:"sample-writer",name:"Sample conversation",homeConversationId:"sample-home",binding:binding,readiness:"offline")]
        model.selectedAgentID = "sample-writer"
        var messages: [Message] = [], runs: [Run] = []
        for turn in 1...6 {
            let request = "sample-request-\(turn)"
            messages.append(Message(messageId:"sample-user-\(turn)",conversationId:"sample-home",requestId:request,role:"user",text:"Sample packing question \(turn)",sequence:messages.count + 1,createdAt:"2026-10-17T00:00:00Z"))
            if turn != 2 {
                messages.append(Message(messageId:"sample-reply-\(turn)",conversationId:"sample-home",requestId:request,role:"assistant",text:turn == 6 ? "Latest sample reply: pack the red notebook." : "Sample reply \(turn): the red notebook is on the list.",sequence:messages.count + 1,createdAt:"2026-10-17T00:00:01Z",agentName:"Sample writer"))
            }
            runs.append(Run(runId:"sample-run-\(turn)",requestId:request,conversationId:"sample-home",bindingRevisionId:binding.bindingRevisionId,state:turn == 2 ? "canceled" : "completed",selectedModel:binding.selectedModel,effectiveModel:nil,modelEvidence:"unreported",attemptId:nil,configuration:TurnConfiguration(agentId:"sample-writer",agentName:"Sample writer",bindingRevisionId:binding.bindingRevisionId,selectedModel:binding.selectedModel,instructions:"",skills:[]),statusDetail:turn == 2 ? "Sample cancellation; no reply was produced." : nil))
        }
        model.snapshot = Snapshot(installationId:"offline-sample",recoveryEpoch:"sample-epoch",conversationId:"sample-home",cursor:messages.count,messages:messages,runs:runs,lastReadMessageSequence:0)
        model.showConnection = false
        model.status = "Read-only sample · no connection"
        return model
    }
}
#endif
