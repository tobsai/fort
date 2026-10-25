import Foundation
import Testing
@testable import FortKit

@Test func agentGearKeepsUnavailableModelsListedWithAReason() {
    let gear = AgentSheet.gear([
        ModelOption(id:"a",name:"Alpha",availability:"available"),
        ModelOption(id:"b",name:"Beta",availability:"unavailable"),
        ModelOption(id:"c",name:"Gamma",availability:"unchecked"),
        ModelOption(id:"d",name:"Delta",availability:"unavailable",detail:"Retired by the runtime."),
    ])
    #expect(gear.map(\.selectable) == [true,false,false,false])
    #expect(gear.map(\.note) == [nil,"Unavailable right now.","Not checked yet.","Retired by the runtime."])
    let kept = AgentSheet.gear(ModelCatalog(provider:"claude",state:"ready",models:[]).options(preserving:"old-model"))
    #expect(kept == [AgentSheet.Gear(id:"old-model",name:"old-model",selectable:false,note:"Not listed by this runtime.")])
}

@Test func agentSizeNoteOnlyAppearsNearTheLimit() {
    #expect(AgentSheet.sizeNote("Short and sweet.") == nil)
    #expect(AgentSheet.sizeNote(String(repeating:"a",count:12_287)) == nil)
    #expect(AgentSheet.sizeNote(String(repeating:"a",count:12_288)) == "\(12_288.formatted()) / \(16_384.formatted()) bytes")
    #expect(AgentSheet.sizeNote(String(repeating:"a",count:16_384)) != nil)
    #expect(AgentSheet.sizeNote(String(repeating:"a",count:16_390)) == "Too long by 6 bytes.")
    #expect(AgentSheet.instructionsTitle(name:"  Scout ") == "How Scout should work")
    #expect(AgentSheet.instructionsTitle(name:"") == "How this agent should work")
}

@Test func joinPanelAsksForAPasteThenOffersJoin() {
    #expect(JoinPanel.make(address:"",code:"",hasPending:false) == .paste)
    #expect(JoinPanel.make(address:"https://fort.example:8443",code:"",hasPending:false) == .paste)
    #expect(JoinPanel.make(address:"not a url",code:"secret",hasPending:false) == .paste)
    #expect(JoinPanel.make(address:"https://fort.example:8443",code:"secret",hasPending:false) == .ready(host:"fort.example"))
    #expect(JoinPanel.make(address:"",code:"",hasPending:true) == .pending)
}

@Test func agentSheetSaysWhatSaveIsWaitingFor() {
    #expect(AgentSheet.missing(name:"Scout",instructions:"Be brief.",modelReady:true) == nil)
    #expect(AgentSheet.missing(name:" ",instructions:"Be brief.",modelReady:true) == "To save, add a name.")
    #expect(AgentSheet.missing(name:"",instructions:"",modelReady:false) == "To save, add a name, instructions and available gear.")
    #expect(AgentSheet.missing(name:String(repeating:"a",count:129),instructions:"x",modelReady:true) == "To save, add a shorter name.")
}
