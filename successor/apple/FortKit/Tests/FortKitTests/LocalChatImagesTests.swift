import Foundation
import CoreGraphics
import ImageIO
import UniformTypeIdentifiers
import Testing
@testable import FortKit
private func samplePNG() throws -> Data {
    let context = try #require(CGContext(data:nil,width:12,height:8,bitsPerComponent:8,bytesPerRow:0,space:CGColorSpaceCreateDeviceRGB(),bitmapInfo:CGImageAlphaInfo.premultipliedLast.rawValue))
    context.setFillColor(CGColor(red:0.2,green:0.7,blue:0.4,alpha:1));context.fill(CGRect(x:0,y:0,width:12,height:8))
    let image = try #require(context.makeImage()),data = NSMutableData()
    let destination = try #require(CGImageDestinationCreateWithData(data,UTType.png.identifier as CFString,1,nil))
    CGImageDestinationAddImage(destination,image,nil);#expect(CGImageDestinationFinalize(destination));return data as Data
}
@Test func localImageImportSurvivesRelaunchAndStaysInItsConversation() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    try FileManager.default.createDirectory(at:root,withIntermediateDirectories:true)
    let input = root.appendingPathComponent("selected.png");try samplePNG().write(to:input)
    let store = LocalChatImageStore(directory:root.appendingPathComponent("cache"))
    let saved = try store.importImage(from:input,installationID:"one",conversationID:"home",afterSequence:8)
    #expect(saved.width == 12 && saved.height == 8 && saved.afterSequence == 8)
    let restored = try LocalChatImageStore(directory:store.directory).images(installationID:"one",conversationID:"home")
    #expect(restored == [saved]);#expect(try store.images(installationID:"two",conversationID:"home").isEmpty)
    #expect(try store.images(installationID:"one",conversationID:"other").isEmpty)
    try store.remove(saved);#expect(try store.images(installationID:"one",conversationID:"home").isEmpty)
    #expect(FileManager.default.fileExists(atPath:input.path))
}
@Test func localImageDisplayRejectsRemoteLocationsSymlinksDirectoriesAndDisguisedFiles() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    try FileManager.default.createDirectory(at:root,withIntermediateDirectories:true)
    let store = LocalChatImageStore(directory:root.appendingPathComponent("cache")),input = root.appendingPathComponent("fake.png"),link = root.appendingPathComponent("link.png")
    try Data("<svg onload='untrusted'>".utf8).write(to:input)
    try FileManager.default.createSymbolicLink(at:link,withDestinationURL:input)
    for url in [URL(string:"https://example.invalid/picture.png")!,input,link,root] {
        #expect(throws:(any Error).self) { try store.importImage(from:url,installationID:"one",conversationID:"home",afterSequence:0) }
    }
    try Data(repeating:0,count:LocalChatImageStore.maximumBytes+1).write(to:input)
    #expect(throws:(any Error).self) { try store.importImage(from:input,installationID:"one",conversationID:"home",afterSequence:0) }
    #expect(try store.images(installationID:"one",conversationID:"home").isEmpty)
}
