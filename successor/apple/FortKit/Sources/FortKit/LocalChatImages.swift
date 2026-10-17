import Foundation
import CryptoKit
import CoreGraphics
import ImageIO
import UniformTypeIdentifiers
import SwiftUI
import Darwin

public struct LocalChatImage: Codable, Equatable, Sendable, Identifiable {
    public let id: String
    public let installationID: String
    public let conversationID: String
    public let afterSequence: Int
    public let createdAt: Date
    public let name: String
    public let width: Int
    public let height: Int
    public let png: Data
    var rendered: CGImage? { CGImageSourceCreateWithData(png as CFData,[kCGImageSourceShouldCache:false] as CFDictionary).flatMap { CGImageSourceCreateImageAtIndex($0,0,nil) } }
}
enum LocalImageError: Error, LocalizedError, Equatable {
    case unsupported, tooLarge, unavailable, capacity
    var errorDescription: String? {
        switch self {
        case .unsupported: "Choose a still PNG or JPEG image. Remote addresses and linked files are not loaded."
        case .tooLarge: "Choose an image up to 5 MB and 16 megapixels, with neither side above 8,192 pixels."
        case .unavailable: "This local image could not be read or saved. Your conversation has not changed."
        case .capacity: "This conversation already has 20 local images. Remove a preview before adding another."
        }
    }
}
struct LocalChatImageStore: Sendable {
    static let maximumBytes = 5 * 1024 * 1024
    let directory: URL
    private func scope(_ installation: String, _ conversation: String) -> URL {
        let digest = SHA256.hash(data:Data((installation+"\0"+conversation).utf8)).map { String(format:"%02x",$0) }.joined()
        return directory.appendingPathComponent(digest,isDirectory:true)
    }
    private func plainDirectory(_ url: URL) throws {
        let values = try url.resourceValues(forKeys:[.isDirectoryKey,.isSymbolicLinkKey])
        guard values.isDirectory == true, values.isSymbolicLink != true else { throw LocalImageError.unavailable }
    }
    private func read(_ url: URL, maximum: Int) throws -> Data {
        guard url.isFileURL else { throw LocalImageError.unsupported }
        let fd = url.withUnsafeFileSystemRepresentation { path in path.map { Darwin.open($0,O_RDONLY|O_NONBLOCK|O_NOFOLLOW|O_CLOEXEC) } ?? -1 }
        guard fd >= 0 else { throw LocalImageError.unsupported }
        let file = FileHandle(fileDescriptor:fd,closeOnDealloc:true)
        defer { try? file.close() }
        var status = stat()
        guard fstat(fd,&status) == 0, status.st_mode & mode_t(S_IFMT) == mode_t(S_IFREG) else { throw LocalImageError.unsupported }
        guard status.st_size > 0, status.st_size <= maximum else { throw LocalImageError.tooLarge }
        let data = try file.read(upToCount:maximum+1) ?? Data()
        guard !data.isEmpty, data.count <= maximum else { throw LocalImageError.tooLarge }
        return data
    }
    private func normalize(_ bytes: Data) throws -> (Data,Int,Int) {
        guard bytes.count <= Self.maximumBytes else { throw LocalImageError.tooLarge }
        guard let source = CGImageSourceCreateWithData(bytes as CFData,[kCGImageSourceShouldCache:false] as CFDictionary), let type = CGImageSourceGetType(source), [UTType.png.identifier,UTType.jpeg.identifier].contains(type as String), CGImageSourceGetCount(source) == 1 else { throw LocalImageError.unsupported }
        guard let properties = CGImageSourceCopyPropertiesAtIndex(source,0,nil) as? [CFString:Any], let width = properties[kCGImagePropertyPixelWidth] as? Int, let height = properties[kCGImagePropertyPixelHeight] as? Int, width > 0, height > 0, width <= 8192, height <= 8192, width * height <= 16_000_000 else { throw LocalImageError.tooLarge }
        let options: [CFString:Any] = [kCGImageSourceCreateThumbnailFromImageAlways:true,kCGImageSourceCreateThumbnailWithTransform:true,kCGImageSourceThumbnailMaxPixelSize:1600,kCGImageSourceShouldCacheImmediately:true]
        guard let image = CGImageSourceCreateThumbnailAtIndex(source,0,options as CFDictionary) else { throw LocalImageError.unsupported }
        let output = NSMutableData()
        guard let destination = CGImageDestinationCreateWithData(output,UTType.png.identifier as CFString,1,nil) else { throw LocalImageError.unavailable }
        CGImageDestinationAddImage(destination,image,nil)
        guard CGImageDestinationFinalize(destination), output.length <= Self.maximumBytes else { throw LocalImageError.tooLarge }
        return (output as Data,image.width,image.height)
    }
    func importImage(from url: URL, installationID: String, conversationID: String, afterSequence: Int) throws -> LocalChatImage {
        guard !installationID.isEmpty, !conversationID.isEmpty, afterSequence >= 0 else { throw LocalImageError.unavailable }
        guard try images(installationID:installationID,conversationID:conversationID).count < 20 else { throw LocalImageError.capacity }
        let (png,width,height) = try normalize(read(url,maximum:Self.maximumBytes))
        let record = LocalChatImage(id:UUID().uuidString,installationID:installationID,conversationID:conversationID,afterSequence:afterSequence,createdAt:Date(),name:String(url.lastPathComponent.prefix(128)),width:width,height:height,png:png)
        let folder = scope(installationID,conversationID)
        try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:true,attributes:[.posixPermissions:0o700]);try plainDirectory(directory)
        try FileManager.default.createDirectory(at:folder,withIntermediateDirectories:true,attributes:[.posixPermissions:0o700]);try plainDirectory(folder)
        let target = folder.appendingPathComponent(record.id+".json")
        guard !FileManager.default.fileExists(atPath:target.path) else { throw LocalImageError.unavailable }
        try JSONEncoder().encode(record).write(to:target,options:.atomic)
        try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:target.path)
        return record
    }
    func images(installationID: String, conversationID: String) throws -> [LocalChatImage] {
        let folder = scope(installationID,conversationID)
        guard FileManager.default.fileExists(atPath:folder.path) else { return [] }
        try plainDirectory(directory);try plainDirectory(folder)
        let files = try FileManager.default.contentsOfDirectory(at:folder,includingPropertiesForKeys:nil)
        guard files.count <= 20 else { throw LocalImageError.capacity }
        return try files.map { url in
            let image = try JSONDecoder().decode(LocalChatImage.self,from:read(url,maximum:Self.maximumBytes*2))
            guard UUID(uuidString:image.id) != nil, url.lastPathComponent == image.id+".json", image.installationID == installationID, image.conversationID == conversationID, image.afterSequence >= 0, image.createdAt.timeIntervalSince1970.isFinite, image.name.count <= 128, image.png.count <= Self.maximumBytes else { throw LocalImageError.unavailable }
            let (_,width,height) = try normalize(image.png)
            guard width == image.width, height == image.height else { throw LocalImageError.unavailable }
            return image
        }.sorted { $0.createdAt == $1.createdAt ? $0.id < $1.id : $0.createdAt < $1.createdAt }
    }
    func remove(_ image: LocalChatImage) throws {
        guard UUID(uuidString:image.id) != nil, try images(installationID:image.installationID,conversationID:image.conversationID).contains(where:{$0.id == image.id}) else { throw LocalImageError.unavailable }
        try FileManager.default.removeItem(at:scope(image.installationID,image.conversationID).appendingPathComponent(image.id+".json"))
    }
}
struct LocalImageCard: View {
    let image: LocalChatImage
    let remove: () -> Void
    var body: some View {
        VStack(alignment:.leading,spacing:8) {
            if let bitmap = image.rendered {
                Image(bitmap,scale:1,label:Text("Local image: " + image.name)).resizable().scaledToFit().frame(maxWidth:480,maxHeight:360).clipShape(RoundedRectangle(cornerRadius:10)).accessibilityLabel("Local image: " + image.name)
            } else { Label("Image unavailable",systemImage:"photo") }
            HStack {
                VStack(alignment:.leading) { Text(image.name).font(.caption); Text("Only on this device · not sent to the agent").font(.caption2).foregroundStyle(.secondary) }
                Spacer()
                Button("Remove preview",role:.destructive,action:remove).font(.caption)
            }
        }.padding(12).background(.quaternary,in:RoundedRectangle(cornerRadius:12))
    }
}
