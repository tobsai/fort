import Foundation
import Testing
@testable import FortKit

// Spec 067: the client asks an Agent for an image in a conversation, then fetches the PNG by image ID.
private let png = Data([0x89,0x50,0x4e,0x47,0x0d,0x0a,0x1a,0x0a,0,0,0,13])
private let userJSON = #"{"messageId":"m1","conversationId":"c","requestId":"r","role":"user","text":"/image A lighthouse","sequence":1,"createdAt":"2026-10-25T00:00:00.000Z","recipientAgentId":"a","recipientAgentName":"Pascal","drawsImage":true}"#
private actor ImageTransport: FortTransport {
    var calls: [(String,String,Data?)] = []
    func send(_ request: FortRequest) async throws -> FortResponse {
        calls.append((request.method.rawValue,request.path,request.body))
        switch (request.method,request.path) {
        case (.post,"/v1/conversations/c/images"): return FortResponse(status:202,body:Data(#"{"message":\#(userJSON)}"#.utf8))
        case (.get,"/v1/images/i1"): return FortResponse(status:200,body:Data(#"{"imageId":"i1","png":"\#(png.base64EncodedString())"}"#.utf8))
        case (.get,"/v1/images/i2"): return FortResponse(status:200,body:Data(#"{"imageId":"other","png":"\#(png.base64EncodedString())"}"#.utf8))
        default: return FortResponse(status:404,body:Data(#"{"error":{"code":"notFound","message":"No."}}"#.utf8))
        }
    }
}

@Test func imageClientAsksAndFetchesByID() async throws {
    let transport = ImageTransport()
    let client = FortClient(installationID:"one",token:"token",transport:transport)
    let message = try await client.drawImage(conversationID:"c",agentID:"a",requestID:"r",prompt:"A lighthouse")
    #expect(message.drawsImage == true)
    #expect(try await client.image(id:"i1") == png)
    await #expect(throws:(any Error).self) { _ = try await client.image(id:"i2") }
    await #expect(throws:(any Error).self) { _ = try await client.image(id:"../i1") }
    let calls = await transport.calls
    let body = try #require(calls.first?.2.flatMap { try JSONSerialization.jsonObject(with:$0) as? [String:Any] })
    #expect(body["prompt"] as? String == "A lighthouse" && body["agentId"] as? String == "a" && body["requestId"] as? String == "r")
}

@Test func imageRepliesDecodeAndOlderMessagesStillDecode() throws {
    let reply = try JSONDecoder().decode(Message.self,from:Data(#"{"messageId":"m2","conversationId":"c","requestId":"r","role":"assistant","text":"Here’s your image.","sequence":2,"createdAt":"2026-10-25T00:00:00.000Z","agentId":"a","agentName":"Pascal","image":{"imageId":"i1","width":1536,"height":1024}}"#.utf8))
    #expect(reply.image == MessageImage(imageId:"i1",width:1536,height:1024))
    let older = try JSONDecoder().decode(Message.self,from:Data(userJSON.replacingOccurrences(of:#","drawsImage":true"#,with:"").utf8))
    #expect(older.drawsImage == nil && older.image == nil)
}
