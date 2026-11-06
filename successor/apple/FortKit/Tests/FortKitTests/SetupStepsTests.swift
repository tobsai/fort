import Testing
@testable import FortKit

// The first run asks for an account, then a first Agent, and never for a device. Same vectors as web/transcript.test.mjs.
@Test func firstRunAsksForAnAccountThenAFirstAgent() {
    let fresh = SetupSteps.make(account:false,agents:0)
    #expect(fresh.steps.map(\.id) == ["account","agent"])
    #expect(fresh.steps.map(\.current) == [true,false])
    #expect(!fresh.done)
    #expect(SetupSteps.make(account:true,agents:0).steps.map(\.current) == [false,true])
    #expect(SetupSteps.make(account:false,agents:1).steps.map(\.current) == [true,false])
    #expect(SetupSteps.make(account:true,agents:2).done)
    #expect(fresh.steps.map(\.tab) == [.accounts,.crew])
}

@Test func macConnectStepsFollowTheHelperThenTheConnection() {
    let off = MacConnectSteps.make(DevicesBoard.Facts(hostSetup:.needsEnable))
    #expect(off.steps.map(\.action) == [.enableHost,nil])
    #expect(off.steps.map(\.current) == [true,false])
    let approval = MacConnectSteps.make(DevicesBoard.Facts(hostSetup:.needsApproval))
    #expect(approval.steps.first?.action == .checkAgain)
    let started = MacConnectSteps.make(DevicesBoard.Facts(hostSetup:.enabled))
    #expect(started.steps.map(\.done) == [true,false])
    #expect(started.steps.last?.action == .connect)
    let connected = MacConnectSteps.make(DevicesBoard.Facts(hostSetup:.enabled,connected:true))
    #expect(connected.steps.allSatisfy(\.done))
    #expect(connected.steps.allSatisfy { $0.action == nil })
}
