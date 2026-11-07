import Foundation
import Testing
@testable import FortKit

// Spec 068: the memory records the hub sends decode, and the words match the web's transcript.js.
struct AgentMemoryTests {
    private let notes = MemorySystem(id:"fort-notes",version:"1",name:"Fort Notes",summary:"Short notes.",storage:"On the hub.",capabilities:["recall","search","forget","export","import"])
    private let pad = MemorySystem(id:"pad",version:"1",name:"Pad",summary:"",storage:"",capabilities:["recall"])
    private let none = MemorySystem(id:"none",version:"1",name:"No memory",summary:"Starts fresh.",storage:"",capabilities:[])
    private func status(_ state: String, provider: MemorySystem?, total: Int) -> AgentMemoryStatus {
        AgentMemoryStatus(agentId:"a",agentName:"Ada",choice:MemoryChoice(agentId:"a",providerId:provider?.id ?? "gone",revisionId:"r",changedAt:""),provider:provider,state:state,detail:"Pick another memory system.",total:total)
    }

    @Test func lineNamesTheSystemAndCount() {
        #expect(AgentMemoryCopy.line(status("ready",provider:notes,total:1)) == "Fort Notes · 1 note")
        #expect(AgentMemoryCopy.line(status("ready",provider:notes,total:12)) == "Fort Notes · 12 notes")
        #expect(AgentMemoryCopy.line(status("off",provider:none,total:0)) == "No memory")
        #expect(AgentMemoryCopy.line(status("missing",provider:nil,total:0)) == "Memory system missing")
        #expect(AgentMemoryCopy.line(nil) == "Memory: checking…")
    }

    @Test func notesCarryOnlyWhenBothSidesCan() {
        let other = MemorySystem(id:"other",version:"1",name:"Other",summary:"",storage:"",capabilities:["import"])
        let providers = [none,notes,pad,other]
        #expect(AgentMemoryCopy.canCarry(from:"fort-notes",to:other,providers:providers))
        #expect(!AgentMemoryCopy.canCarry(from:"fort-notes",to:pad,providers:providers))
        #expect(!AgentMemoryCopy.canCarry(from:"none",to:notes,providers:providers))
        #expect(!AgentMemoryCopy.canCarry(from:"fort-notes",to:notes,providers:providers))
    }

    @Test func pageDecodesFromTheHubsAnswer() throws {
        let json = #"{"agentId":"a","agentName":"Ada","choice":{"agentId":"a","providerId":"fort-notes","revisionId":"r","changedAt":"2026-11-06T00:00:00.000Z"},"provider":{"id":"fort-notes","version":"1","name":"Fort Notes","summary":"s","storage":"t","capabilities":["recall"]},"state":"ready","detail":"s","total":1,"providers":[{"id":"none","version":"1","name":"No memory","summary":"","storage":"","capabilities":[]}],"entries":[{"entryId":"e","agentId":"a","kind":"fact","text":"Likes tea.","createdAt":"2026-11-06T00:00:00.000Z"}]}"#
        let page = try JSONDecoder().decode(AgentMemoryPage.self,from:Data(json.utf8))
        #expect(page.entries.map(\.text) == ["Likes tea."])
        #expect(page.status.total == 1)
        let receipt = try JSONDecoder().decode(MemorySwitchReceipt.self,from:Data(#"{"choice":{"agentId":"a","providerId":"none","revisionId":"r2","changedAt":""},"state":"off","carried":0,"agentId":"a"}"#.utf8))
        #expect(receipt.carryDetail == nil && receipt.state == "off")
    }

    @Test func aboutSaysTheOldSystemKeepsItsNotes() throws {
        let page = AgentMemoryPage(agentId:"a",agentName:"Ada",choice:MemoryChoice(agentId:"a",providerId:"fort-notes",revisionId:"r",changedAt:""),provider:notes,state:"ready",detail:"",total:2,providers:[none,notes],entries:[])
        #expect(AgentMemoryCopy.about(selected:none,page:page).hasSuffix("Fort Notes keeps its notes, so switching back finds them."))
        #expect(AgentMemoryCopy.about(selected:notes,page:page) == "Short notes.")
    }
}
