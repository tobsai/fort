import Foundation
import Testing
@testable import FortKit

// Spec 065: the client asks for a draw, fetches the PNG by revision and removes it; anything else is refused.
private let png = Data([0x89,0x50,0x4e,0x47,0x0d,0x0a,0x1a,0x0a,0,0,0,13])
private let agentJSON = #"{"agentId":"a","name":"Pascal","instructions":"Ships Fort","homeConversationId":"h","readiness":"requiresLiveReview","binding":{"bindingRevisionId":"b","provider":"codex","runtimeVersion":"0.163.0","selectedModel":"m","hostId":"x","enabled":true}"#
private actor PortraitTransport: FortTransport {
    var calls: [(String,String,Data?)] = []
    let imageRevision: String
    init(imageRevision: String = "r1") { self.imageRevision = imageRevision }
    func send(_ request: FortRequest) async throws -> FortResponse {
        calls.append((request.method.rawValue,request.path,request.body))
        let body: String
        switch (request.method,request.path) {
        case (.post,"/v1/agents/a/portrait"): body = agentJSON + #","portraitDraw":{"state":"drawing","detail":"Drawing.","updatedAt":"2026-10-25T00:00:00.000Z"}}"#
        case (.get,"/v1/agents/a/portrait"): body = #"{"agentId":"a","revision":"\#(imageRevision)","png":"\#(png.base64EncodedString())"}"#
        case (.post,"/v1/agents/a/portrait/remove"): body = agentJSON + "}"
        default: return FortResponse(status:404,body:Data(#"{"error":{"code":"notFound","message":"No."}}"#.utf8))
        }
        return FortResponse(status:request.method == .post && !request.path.hasSuffix("remove") ? 202 : 200,body:Data(body.utf8))
    }
}

@Test func portraitClientDrawsFetchesAndRemoves() async throws {
    let transport = PortraitTransport()
    let client = FortClient(installationID:"one",token:"token",transport:transport)
    let drawing = try await client.drawPortrait(agentID:"a",note:"Waistcoat")
    #expect(drawing.portraitDraw?.isDrawing == true)
    #expect(try await client.portrait(agentID:"a",revision:"r1") == png)
    let removed = try await client.removePortrait(agentID:"a")
    #expect(removed.portrait == nil)
    let calls = await transport.calls
    #expect(calls.map { "\($0.0) \($0.1)" } == ["POST /v1/agents/a/portrait","GET /v1/agents/a/portrait","POST /v1/agents/a/portrait/remove"])
    let note = try #require(calls.first?.2.flatMap { try JSONSerialization.jsonObject(with:$0) as? [String:Any] })
    #expect(note["note"] as? String == "Waistcoat")
}

@Test func portraitClientRefusesAStaleRevisionAndUnsafeIDs() async throws {
    let client = FortClient(installationID:"one",token:"token",transport:PortraitTransport(imageRevision:"old"))
    await #expect(throws:(any Error).self) { _ = try await client.portrait(agentID:"a",revision:"r1") }
    await #expect(throws:(any Error).self) { _ = try await client.drawPortrait(agentID:"../a",note:nil) }
}

@Test func onlyChatGPTAgentsOfferDrawingAndOlderAgentsStillDecode() throws {
    let older = try JSONDecoder().decode(Agent.self,from:Data((agentJSON + "}").utf8))
    #expect(older.portrait == nil && older.portraitDraw == nil)
    #expect(PortraitDrawing.canDraw(older))
    #expect(PortraitDrawing.buttonTitle(older) == "Draw portrait")
    let drawn = try JSONDecoder().decode(Agent.self,from:Data((agentJSON + #","portrait":{"revision":"r1","createdAt":"2026-10-25T00:00:00.000Z","size":1024}}"#).utf8))
    #expect(PortraitDrawing.buttonTitle(drawn) == "Redraw portrait")
    let claude = try JSONDecoder().decode(Agent.self,from:Data((agentJSON.replacingOccurrences(of:#""provider":"codex""#,with:#""provider":"claude-code""#) + "}").utf8))
    #expect(!PortraitDrawing.canDraw(claude))
}
