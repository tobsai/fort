import Foundation
import Darwin
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

@Test func imageFIFOIsRejectedBeforeWaitingForAWriter() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    try FileManager.default.createDirectory(at:root,withIntermediateDirectories:true)
    let fifo = root.appendingPathComponent("pipe.png")
    #expect(mkfifo(fifo.path,0o600) == 0)
    let released = DispatchSemaphore(value:0)
    // Release an old blocking reader after a bounded delay so this regression cannot hang the suite.
    DispatchQueue.global().asyncAfter(deadline:.now() + 0.5) {
        let writer = Darwin.open(fifo.path,O_WRONLY | O_NONBLOCK)
        if writer >= 0 { Darwin.close(writer) }
        released.signal()
    }
    defer { _ = released.wait(timeout:.now() + 2); try? FileManager.default.removeItem(at:root) }
    let start = ContinuousClock.now
    let store = LocalChatImageStore(directory:root.appendingPathComponent("cache"))
    #expect(throws:(any Error).self) { try store.importImage(from:fifo,installationID:"one",conversationID:"home",afterSequence:0) }
    #expect(start.duration(to:.now) < .milliseconds(400))
}
private func encodedImage(width: Int, height: Int, type: UTType = .png, count: Int = 1, orientation: Int = 1) throws -> Data {
    let context = try #require(CGContext(data:nil,width:width,height:height,bitsPerComponent:8,bytesPerRow:0,space:CGColorSpaceCreateDeviceRGB(),bitmapInfo:CGImageAlphaInfo.premultipliedLast.rawValue))
    context.setFillColor(CGColor(red:0.3,green:0.6,blue:0.9,alpha:1));context.fill(CGRect(x:0,y:0,width:width,height:height))
    let image = try #require(context.makeImage()),data = NSMutableData()
    let destination = try #require(CGImageDestinationCreateWithData(data,type.identifier as CFString,count,nil))
    for _ in 0..<count { CGImageDestinationAddImage(destination,image,[kCGImagePropertyOrientation:orientation,kCGImagePropertyPNGDictionary:[kCGImagePropertyAPNGDelayTime:0.1]] as CFDictionary) }
    try #require(CGImageDestinationFinalize(destination));return data as Data
}
@Test func imageImporterNormalizesJPEGOrientationAndRejectsAnimatedOrOversizedPixels() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    try FileManager.default.createDirectory(at:root,withIntermediateDirectories:true)
    let input = root.appendingPathComponent("selected.jpg"),store = LocalChatImageStore(directory:root.appendingPathComponent("cache"))
    try encodedImage(width:24,height:12,type:.jpeg,orientation:6).write(to:input)
    let image = try store.importImage(from:input,installationID:"one",conversationID:"home",afterSequence:0)
    #expect(image.width == 12 && image.height == 24)
    try encodedImage(width:12,height:8,count:2).write(to:input)
    #expect(throws:LocalImageError.unsupported) { try store.importImage(from:input,installationID:"one",conversationID:"home",afterSequence:0) }
    for bytes in [try encodedImage(width:8193,height:1),try encodedImage(width:4001,height:4000)] {
        try bytes.write(to:input)
        #expect(throws:LocalImageError.tooLarge) { try store.importImage(from:input,installationID:"one",conversationID:"home",afterSequence:0) }
    }
    #expect(try store.images(installationID:"one",conversationID:"home").count == 1)
}
@Test func imageCapacityAllowsTwentyAndReuseAfterRemovingOne() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    try FileManager.default.createDirectory(at:root,withIntermediateDirectories:true)
    let input = root.appendingPathComponent("selected.png"),store = LocalChatImageStore(directory:root.appendingPathComponent("cache"))
    try samplePNG().write(to:input)
    for _ in 0..<20 { _ = try store.importImage(from:input,installationID:"one",conversationID:"home",afterSequence:0) }
    #expect(throws:(any Error).self) { try store.importImage(from:input,installationID:"one",conversationID:"home",afterSequence:0) }
    try store.remove(try #require(store.images(installationID:"one",conversationID:"home").first))
    _ = try store.importImage(from:input,installationID:"one",conversationID:"home",afterSequence:0)
    #expect(try store.images(installationID:"one",conversationID:"home").count == 20)
}
