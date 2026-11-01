import Foundation
import Testing
@testable import FortKit

// The chat and Setup read the same as the web client. The same vectors are in design/harmony.test.mjs.

@Test func approvalsUseTheWebsWordsBeforeAndAfterTheDecision() {
    let open = ApprovalCopy.make(kind:"",state:"pending")
    #expect(open == ApprovalCopy(approve:"Approve",reject:"Reject",decided:nil,label:"Needs you"))
    #expect(ApprovalCopy.make(kind:"routineGate",state:"pending").approve == "Continue routine")
    #expect(ApprovalCopy.make(kind:"",state:"approved").decided == "Approved")
    #expect(ApprovalCopy.make(kind:"routineGate",state:"approved").label == "Continued")
    #expect(ApprovalCopy.make(kind:"",state:"rejected").label == "Rejected")
    #expect(ApprovalCopy.make(kind:"",state:"expired").label == "Expired before a decision")
    #expect(ApprovalCopy.make(kind:"",state:"canceled").label == "Canceled")
    // A state Fort does not know yet still says what it is rather than nothing.
    #expect(ApprovalCopy.make(kind:"",state:"somethingNew").label == "somethingNew")
}

@Test func aRunReadsAsOneCaptionWithTheServicesDetailUnlessItWaitsOnYou() {
    let running = RunCaptionLine.make(state:"running",displayState:"Agent is responding",detail:"Reading the repo")
    #expect(running == RunCaptionLine(tone:.working,text:"Agent is responding",detail:"Reading the repo"))
    // While it waits on you the approval panel says what it wants, so the caption stays short.
    #expect(RunCaptionLine.make(state:"waitingForApproval",displayState:"Waiting for your approval",detail:"Lewis paused.").detail == nil)
    #expect(RunCaptionLine.make(state:"failed",displayState:"Could not complete this turn",detail:"").detail == nil)
    #expect(RunCaptionLine.make(state:"failed",displayState:"Could not complete this turn",detail:nil).tone == .failed)
    #expect(RunCaptionLine.make(state:"canceled",displayState:"Canceled",detail:"Canceled").detail == nil)
}

@Test func anAgentIsListedAsProviderAndModelEverywhere() {
    #expect(CrewRow.line(provider:"Claude",model:"claude-opus-5-5") == "Claude · claude-opus-5-5")
    #expect(CrewRow.line(provider:"ChatGPT",model:"") == "ChatGPT")
}

@Test func setupHasTheWebsSectionsInTheWebsOrder() {
    #expect(PauseTab.allCases.map(\.title) == ["Crew","Routines","Accounts","Devices"])
    let schedule = TroubleCard.make(.init(scheduleFault:"A schedule could not be checked."))
    #expect(schedule.first?.button == "Open Routines")
}

@Test func statusLampsUseTheComicTokens() {
    #expect(StatusLamp.ready.color == FortTheme.done)
    #expect(StatusLamp.waiting.color == FortTheme.working)
    #expect(StatusLamp.attention.color == FortTheme.needsYou)
    #expect(StatusLamp.failed.color == FortTheme.failed)
    #expect(StatusLamp.off.color == FortTheme.off)
}
