import Foundation
import Testing
@testable import FortKit

private let runningMac = DevicesBoard.Facts(isCompanion:false,hostSetup:.enabled,connected:true,runtimeState:"ready",claudeServing:true,managesPhone:true,phoneAddress:"mac.tail.ts.net",listenerActive:true,phone:.paired(deviceID:"p1",deviceName:"Toby's iPhone",expiresAt:nil,active:true))

@Test func devicesBoardShowsARunningMacWithClaudeAndAConnectedPhoneAsAllGreen() {
    let board = DevicesBoard.make(runningMac)
    #expect(board.rows.map(\.id) == ["mac","claude","phone"])
    #expect(board.rows.allSatisfy { $0.lamp == .ready && $0.action == nil })
    #expect(board.rows[2].line == "Toby's iPhone is connected.")
}

@Test func devicesBoardWalksAFreshMacThroughEnableThenConnect() {
    var facts = DevicesBoard.Facts(hostSetup:.needsEnable)
    #expect(DevicesBoard.make(facts).rows.map(\.action) == [.enableHost])
    facts.hostSetup = .needsApproval
    #expect(DevicesBoard.make(facts).rows[0].action == .openWorkshop)
    facts.hostSetup = .enabled
    #expect(DevicesBoard.make(facts).rows[0].action == .connect)
    #expect(DevicesBoard.make(facts).rows.count == 1)
    facts.hostSetup = .unknown
    #expect(DevicesBoard.make(facts).rows[0].lamp == .off)
}

@Test func devicesBoardOffersResumeOrRestartForAStoppedCrew() {
    var facts = runningMac
    facts.runtimeState = "stopped"
    #expect(DevicesBoard.make(facts).rows[0].action == .resume && DevicesBoard.make(facts).rows[0].lamp == .attention)
    facts.runtimeState = "restartRequired"
    #expect(DevicesBoard.make(facts).rows[0].action == .restart)
    facts.runtimeState = "ready"; facts.claudeServing = false
    #expect(DevicesBoard.make(facts).rows[1].action == .openWorkshop && DevicesBoard.make(facts).rows[1].lamp == .off)
}

@Test func devicesBoardGivesThePhoneOneNextStep() {
    var facts = runningMac
    facts.phone = .available; facts.phoneAddress = nil; facts.listenerActive = false
    #expect(DevicesBoard.make(facts).rows[2].action == .openWorkshop)
    facts.phoneAddress = "mac.tail.ts.net"
    #expect(DevicesBoard.make(facts).rows[2].action == .startListener)
    facts.listenerActive = true
    #expect(DevicesBoard.make(facts).rows[2].action == .invite)
    facts.phone = .invited(invitationID:"i1",expiresAt:"2026-10-25T00:00:00Z")
    #expect(DevicesBoard.make(facts).rows[2].lamp == .waiting && DevicesBoard.make(facts).rows[2].action == .invite)
    facts.phone = .paired(deviceID:"p1",deviceName:"Toby's iPhone",expiresAt:"2026-10-01",active:false)
    #expect(DevicesBoard.make(facts).rows[2].action == .invite && DevicesBoard.make(facts).rows[2].lamp == .attention)
    facts.managesPhone = false
    #expect(!DevicesBoard.make(facts).rows.contains { $0.id == "phone" })
}

@Test func devicesBoardOnAnIPhoneOnlyTalksAboutThisPhone() {
    #expect(DevicesBoard.make(DevicesBoard.Facts(isCompanion:true,connected:true)).rows.map(\.lamp) == [.ready])
    let unpaired = DevicesBoard.make(DevicesBoard.Facts(isCompanion:true,connected:false))
    #expect(unpaired.rows.count == 1 && unpaired.rows[0].action == .openWorkshop)
}
