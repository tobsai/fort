#if os(macOS)
import Foundation
import Darwin
import Security

/// Mac-side inputs for the phone listener: the Tailscale address, the imported Tailscale
/// HTTPS certificate and key, and fresh invitation values. Nothing here starts a listener.
public struct MacPhoneAccess: Sendable {
    public static let port = 8443
    public static let prefixLength = 10
    public struct Identity: Equatable, Sendable {
        public let certificate: Data
        public let privateKey: Data
        public let serverName: String
    }
    private let directory: URL
    public init(directory: URL) { self.directory = directory.appendingPathComponent("phone-access",isDirectory:true) }

    /// Picks a 100.64.0.0/10 address only on a tunnel interface, so a carrier-grade NAT
    /// address on Wi-Fi or Ethernet (same range, other subscribers) is never chosen.
    public static func tailscaleAddress(in interfaces: [(name: String, address: String)]) -> String? {
        interfaces.first { entry in
            let parts = entry.address.split(separator:".").compactMap { UInt8($0) }
            return entry.name.hasPrefix("utun") && parts.count == 4 && parts[0] == 100 && (64...127).contains(parts[1])
        }?.address
    }
    public static func currentTailscaleAddress() -> String? {
        var list: UnsafeMutablePointer<ifaddrs>?
        guard getifaddrs(&list) == 0, let first = list else { return nil }
        defer { freeifaddrs(list) }
        var interfaces: [(name: String, address: String)] = []
        for entry in sequence(first:first,next:{ $0.pointee.ifa_next }) {
            guard let address = entry.pointee.ifa_addr, address.pointee.sa_family == UInt8(AF_INET) else { continue }
            var host = [CChar](repeating:0,count:Int(NI_MAXHOST))
            guard getnameinfo(address,socklen_t(address.pointee.sa_len),&host,socklen_t(host.count),nil,0,NI_NUMERICHOST) == 0 else { continue }
            interfaces.append((String(cString:entry.pointee.ifa_name),String(decoding:host.prefix { $0 != 0 }.map { UInt8(bitPattern:$0) },as:UTF8.self)))
        }
        return tailscaleAddress(in:interfaces)
    }

    public static func serverName(certificatePEM: Data) throws -> String {
        guard let der = firstPEMBlock(certificatePEM,label:"CERTIFICATE"), SecCertificateCreateWithData(nil,der as CFData) != nil,
              let name = dnsNames(der:[UInt8](der)).first,
              name.range(of:"^[a-z0-9][a-z0-9.-]{0,252}$",options:.regularExpression) != nil, !name.contains("..") else { throw ClientError.invalidResponse }
        return name
    }

    public func identity() throws -> Identity? {
        let certificateURL = directory.appendingPathComponent("certificate.pem"), keyURL = directory.appendingPathComponent("private-key.pem")
        guard FileManager.default.fileExists(atPath:certificateURL.path) else { return nil }
        let certificate = try Data(contentsOf:certificateURL), key = try Data(contentsOf:keyURL)
        return Identity(certificate:certificate,privateKey:key,serverName:try Self.serverName(certificatePEM:certificate))
    }
    public func save(certificate: Data, privateKey: Data) throws {
        guard certificate.count <= 65536, privateKey.count <= 65536, Self.firstPEMBlock(privateKey,label:"PRIVATE KEY") != nil
                || Self.firstPEMBlock(privateKey,label:"EC PRIVATE KEY") != nil || Self.firstPEMBlock(privateKey,label:"RSA PRIVATE KEY") != nil else { throw ClientError.invalidResponse }
        _ = try Self.serverName(certificatePEM:certificate)
        try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:true,attributes:[.posixPermissions:0o700])
        for (name,data) in [("certificate.pem",certificate),("private-key.pem",privateKey)] {
            let url = directory.appendingPathComponent(name)
            try data.write(to:url,options:[.atomic])
            try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:url.path)
        }
    }

    public static func newInvitation() -> (id: String, code: String) {
        func random(_ count: Int) -> String {
            var bytes = [UInt8](repeating:0,count:count)
            precondition(SecRandomCopyBytes(kSecRandomDefault,count,&bytes) == errSecSuccess)
            return Data(bytes).base64EncodedString().replacingOccurrences(of:"+",with:"-").replacingOccurrences(of:"/",with:"_").replacingOccurrences(of:"=",with:"")
        }
        return ("phone-" + random(12),random(32))
    }

    static func firstPEMBlock(_ pem: Data, label: String) -> Data? {
        guard let text = String(data:pem,encoding:.utf8),
              let begin = text.range(of:"-----BEGIN \(label)-----"),
              let end = text.range(of:"-----END \(label)-----",range:begin.upperBound..<text.endIndex) else { return nil }
        return Data(base64Encoded:String(text[begin.upperBound..<end.lowerBound]).filter { !$0.isWhitespace })
    }
    /// Reads dNSName entries from the subjectAltName extension (OID 2.5.29.17) of a DER certificate.
    static func dnsNames(der: [UInt8]) -> [String] {
        func length(_ index: inout Int) -> Int? {
            guard index < der.count else { return nil }
            let first = Int(der[index]); index += 1
            if first < 0x80 { return first }
            let count = first & 0x7f
            guard (1...3).contains(count), index + count <= der.count else { return nil }
            var value = 0
            for _ in 0..<count { value = value << 8 | Int(der[index]); index += 1 }
            return value
        }
        let oid: [UInt8] = [0x06,0x03,0x55,0x1d,0x11]
        guard let start = (0...max(0,der.count - oid.count)).first(where:{ Array(der[$0..<$0 + oid.count]) == oid }) else { return [] }
        var index = start + oid.count
        if index < der.count, der[index] == 0x01 { index += 3 }
        guard index < der.count, der[index] == 0x04 else { return [] }
        index += 1; guard length(&index) != nil, index < der.count, der[index] == 0x30 else { return [] }
        index += 1; guard let sequence = length(&index) else { return [] }
        let end = min(der.count,index + sequence)
        var names: [String] = []
        while index < end {
            let tag = der[index]; index += 1
            guard let size = length(&index), index + size <= end else { break }
            if tag == 0x82, let name = String(bytes:der[index..<index + size],encoding:.ascii) { names.append(name.lowercased()) }
            index += size
        }
        return names
    }
}
#endif
