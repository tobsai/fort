#if os(macOS)
import Foundation
import Testing
@testable import FortKit

private func keys(_ value: some Encodable) throws -> [String:Any] {
    try #require(JSONSerialization.jsonObject(with:JSONEncoder().encode(value)) as? [String:Any])
}

@Test func networkCommandsEncodeTheCoreChannelsExactKeys() throws {
    let owner = MacNetworkOwner(installationId:"one",deviceId:"mac")
    let configuration = MacNetworkConfiguration(bindAddress:"100.111.16.2",prefixLength:10,port:8443,serverName:"mac.tail0000.ts.net",certificate:Data("c".utf8),privateKey:Data("k".utf8))
    #expect(Set(try keys(MacNetworkCommand.inspect(owner:owner)).keys) == ["operation","owner"])
    #expect(Set(try keys(MacNetworkCommand.stop(owner:owner,sessionId:"s",generation:1)).keys) == ["operation","owner","sessionId","generation"])
    let start = try keys(MacNetworkCommand.start(owner:owner,sessionId:"s",generation:0,configuration:configuration))
    #expect(Set(start.keys) == ["operation","owner","sessionId","generation","configuration"])
    #expect(Set(try #require(start["configuration"] as? [String:Any]).keys) == ["bindAddress","prefixLength","port","serverName","certificate","privateKey"])
    #expect(Set(try keys(try MacNetworkRequest(command:.inspect(owner:owner))).keys) == ["command","timeout"])
}

@Test func networkRequestsAndStatusesRefuseMismatchedOwners() throws {
    #expect(throws:(any Error).self) { try MacNetworkRequest(command:.inspect(owner:MacNetworkOwner(installationId:"one/two",deviceId:"mac"))) }
    #expect(throws:(any Error).self) { try MacNetworkRequest(command:.inspect(owner:MacNetworkOwner(installationId:"one",deviceId:"mac")),timeout:31) }
    let request = try MacNetworkRequest(command:.inspect(owner:MacNetworkOwner(installationId:"one",deviceId:"mac")))
    let active = try JSONDecoder().decode(MacNetworkStatus.self,from:Data(#"{"owner":{"installationId":"one","deviceId":"mac"},"sessionId":"session","generation":1,"listener":{"state":"active","url":"https://100.111.16.2:8443"}}"#.utf8))
    try active.validate(for:request)
    #expect(active.listener == .active(url:URL(string:"https://100.111.16.2:8443")!))
    let other = try JSONDecoder().decode(MacNetworkStatus.self,from:Data(#"{"owner":{"installationId":"one","deviceId":"phone"},"sessionId":"session","generation":0,"listener":{"state":"stopped","cause":"initial"}}"#.utf8))
    #expect(throws:(any Error).self) { try other.validate(for:request) }
    #expect(throws:(any Error).self) { try JSONDecoder().decode(MacNetworkListener.self,from:Data(#"{"state":"active","url":"http://plain"}"#.utf8)) }
}
#endif
