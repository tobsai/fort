import Foundation
import CryptoKit

public enum RelaySecurityError: Error, Sendable {
    case invalidKey
    case invalidMessage
    case unexpectedHandshakeState
    case handshakeIncomplete
    case authenticationFailed
}

public struct RelayKeypair: Sendable {
    public let privateKey: Data
    public let publicKey: Data

    public init(privateKey: Data) throws {
        guard privateKey.count == 32 else { throw RelaySecurityError.invalidKey }
        let key = try Curve25519.KeyAgreement.PrivateKey(rawRepresentation: privateKey)
        self.privateKey = privateKey
        self.publicKey = key.publicKey.rawRepresentation
    }

    public init() {
        let key = Curve25519.KeyAgreement.PrivateKey()
        self.privateKey = key.rawRepresentation
        self.publicKey = key.publicKey.rawRepresentation
    }
}

private enum RelayBlake2s {
    private static let iv: [UInt32] = [
        0x6A09E667, 0xBB67AE85, 0x3C6EF372, 0xA54FF53A,
        0x510E527F, 0x9B05688C, 0x1F83D9AB, 0x5BE0CD19,
    ]
    private static let sigma: [[Int]] = [
        [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15],
        [14, 10, 4, 8, 9, 15, 13, 6, 1, 12, 0, 2, 11, 7, 5, 3],
        [11, 8, 12, 0, 5, 2, 15, 13, 10, 14, 3, 6, 7, 1, 9, 4],
        [7, 9, 3, 1, 13, 12, 11, 14, 2, 6, 5, 10, 4, 0, 15, 8],
        [9, 0, 5, 7, 2, 4, 10, 15, 14, 1, 11, 12, 6, 8, 3, 13],
        [2, 12, 6, 10, 0, 11, 8, 3, 4, 13, 7, 5, 15, 14, 1, 9],
        [12, 5, 1, 15, 14, 13, 4, 10, 0, 7, 6, 3, 9, 2, 8, 11],
        [13, 11, 7, 14, 12, 1, 3, 9, 5, 0, 15, 4, 8, 6, 2, 10],
        [6, 15, 14, 9, 11, 3, 0, 8, 12, 2, 13, 7, 1, 4, 10, 5],
        [10, 2, 8, 4, 7, 6, 1, 5, 15, 11, 9, 14, 3, 12, 13, 0],
    ]

    static func hash(_ data: Data) -> Data {
        var h = iv
        h[0] ^= 0x01010020
        var offset = 0
        var count: UInt64 = 0
        repeat {
            let remaining = max(0, data.count - offset)
            let length = min(64, remaining)
            var block = Data(repeating: 0, count: 64)
            if length > 0 {
                block.replaceSubrange(0..<length, with: data[offset..<(offset + length)])
            }
            count += UInt64(length)
            offset += length
            compress(&h, block: block, count: count, last: offset >= data.count)
        } while offset < data.count

        var output = Data()
        for word in h {
            output.append(contentsOf: littleEndianBytes(word))
        }
        return output
    }

    static func hmac(key: Data, data: Data) -> Data {
        var normalized = key.count > 64 ? hash(key) : key
        if normalized.count < 64 { normalized.append(Data(repeating: 0, count: 64 - normalized.count)) }
        let outer = Data(normalized.map { $0 ^ 0x5c })
        let inner = Data(normalized.map { $0 ^ 0x36 })
        return hash(outer + hash(inner + data))
    }

    private static func compress(_ h: inout [UInt32], block: Data, count: UInt64, last: Bool) {
        var m = [UInt32](repeating: 0, count: 16)
        let bytes = [UInt8](block)
        for i in 0..<16 {
            let p = i * 4
            m[i] = UInt32(bytes[p]) | UInt32(bytes[p + 1]) << 8 | UInt32(bytes[p + 2]) << 16 | UInt32(bytes[p + 3]) << 24
        }
        var v = h + iv
        v[12] ^= UInt32(truncatingIfNeeded: count)
        v[13] ^= UInt32(truncatingIfNeeded: count >> 32)
        if last { v[14] = ~v[14] }
        for round in 0..<10 {
            let s = sigma[round]
            g(&v, 0, 4, 8, 12, m[s[0]], m[s[1]])
            g(&v, 1, 5, 9, 13, m[s[2]], m[s[3]])
            g(&v, 2, 6, 10, 14, m[s[4]], m[s[5]])
            g(&v, 3, 7, 11, 15, m[s[6]], m[s[7]])
            g(&v, 0, 5, 10, 15, m[s[8]], m[s[9]])
            g(&v, 1, 6, 11, 12, m[s[10]], m[s[11]])
            g(&v, 2, 7, 8, 13, m[s[12]], m[s[13]])
            g(&v, 3, 4, 9, 14, m[s[14]], m[s[15]])
        }
        for i in 0..<8 { h[i] ^= v[i] ^ v[i + 8] }
    }

    private static func g(_ v: inout [UInt32], _ a: Int, _ b: Int, _ c: Int, _ d: Int, _ x: UInt32, _ y: UInt32) {
        v[a] = v[a] &+ v[b] &+ x
        v[d] = rotateRight(v[d] ^ v[a], 16)
        v[c] = v[c] &+ v[d]
        v[b] = rotateRight(v[b] ^ v[c], 12)
        v[a] = v[a] &+ v[b] &+ y
        v[d] = rotateRight(v[d] ^ v[a], 8)
        v[c] = v[c] &+ v[d]
        v[b] = rotateRight(v[b] ^ v[c], 7)
    }

    private static func rotateRight(_ value: UInt32, _ amount: UInt32) -> UInt32 {
        (value >> amount) | (value << (32 - amount))
    }

    private static func littleEndianBytes(_ value: UInt32) -> [UInt8] {
        [UInt8(truncatingIfNeeded: value), UInt8(truncatingIfNeeded: value >> 8), UInt8(truncatingIfNeeded: value >> 16), UInt8(truncatingIfNeeded: value >> 24)]
    }
}

private final class RelayCipherState {
    private let key: SymmetricKey
    private var nonce: UInt64 = 0

    init(key: Data) { self.key = SymmetricKey(data: key) }

    func encrypt(_ plaintext: Data, associatedData: Data = Data()) throws -> Data {
        let sealed = try ChaChaPoly.seal(plaintext, using: key, nonce: try makeNonce(), authenticating: associatedData)
        nonce &+= 1
        return sealed.ciphertext + sealed.tag
    }

    func decrypt(_ ciphertextAndTag: Data, associatedData: Data = Data()) throws -> Data {
        guard ciphertextAndTag.count >= 16 else { throw RelaySecurityError.authenticationFailed }
        let split = ciphertextAndTag.count - 16
        let box = try ChaChaPoly.SealedBox(
            nonce: makeNonce(),
            ciphertext: ciphertextAndTag.prefix(split),
            tag: ciphertextAndTag.suffix(16)
        )
        do {
            let plaintext = try ChaChaPoly.open(box, using: key, authenticating: associatedData)
            nonce &+= 1
            return plaintext
        } catch {
            throw RelaySecurityError.authenticationFailed
        }
    }

    private func makeNonce() throws -> ChaChaPoly.Nonce {
        var bytes = Data(repeating: 0, count: 12)
        for i in 0..<8 { bytes[4 + i] = UInt8(truncatingIfNeeded: nonce >> UInt64(8 * i)) }
        return try ChaChaPoly.Nonce(data: bytes)
    }
}

private final class RelaySymmetricState {
    private(set) var chainingKey = Data()
    private(set) var handshakeHash = Data()
    private var cipher: RelayCipherState?

    init(protocolName: String, responderPublicKey: Data) {
        let name = Data(protocolName.utf8)
        handshakeHash = name.count <= 32 ? name + Data(repeating: 0, count: 32 - name.count) : RelayBlake2s.hash(name)
        chainingKey = handshakeHash
        mixHash(Data())
        mixHash(responderPublicKey)
    }

    func mixKey(_ input: Data) {
        let outputs = hkdf(chainingKey, input, 2)
        chainingKey = outputs[0]
        cipher = RelayCipherState(key: outputs[1])
    }

    func mixHash(_ data: Data) { handshakeHash = RelayBlake2s.hash(handshakeHash + data) }

    func encryptAndHash(_ plaintext: Data) throws -> Data {
        let result = try cipher?.encrypt(plaintext, associatedData: handshakeHash) ?? plaintext
        mixHash(result)
        return result
    }

    func decryptAndHash(_ ciphertext: Data) throws -> Data {
        let result = try cipher?.decrypt(ciphertext, associatedData: handshakeHash) ?? ciphertext
        mixHash(ciphertext)
        return result
    }

    func split() -> (RelayCipherState, RelayCipherState) {
        let outputs = hkdf(chainingKey, Data(), 2)
        return (RelayCipherState(key: outputs[0]), RelayCipherState(key: outputs[1]))
    }

    private func hkdf(_ key: Data, _ input: Data, _ count: Int) -> [Data] {
        let temporary = RelayBlake2s.hmac(key: key, data: input)
        let first = RelayBlake2s.hmac(key: temporary, data: Data([1]))
        let second = RelayBlake2s.hmac(key: temporary, data: first + Data([2]))
        if count == 2 { return [first, second] }
        return [first, second, RelayBlake2s.hmac(key: temporary, data: second + Data([3]))]
    }
}

public final class RelayNoiseSession: @unchecked Sendable {
    private let encryptor: RelayCipherState
    private let decryptor: RelayCipherState

    fileprivate init(encryptor: RelayCipherState, decryptor: RelayCipherState) {
        self.encryptor = encryptor
        self.decryptor = decryptor
    }

    public func seal(_ plaintext: Data) throws -> Data { try encryptor.encrypt(plaintext) }
    public func open(_ ciphertext: Data) throws -> Data { try decryptor.decrypt(ciphertext) }
}

public final class RelayNoiseInitiator: @unchecked Sendable {
    public static let protocolName = "Noise_IK_25519_ChaChaPoly_BLAKE2s"

    private let staticKeypair: RelayKeypair
    private let responderPublicKey: Data
    private let ephemeralKeypair: RelayKeypair
    private let symmetric: RelaySymmetricState
    private var messageIndex = 0
    private var transport: RelayNoiseSession?

    public init(
        staticKeypair: RelayKeypair = RelayKeypair(),
        responderPublicKey: Data,
        ephemeralKeypair: RelayKeypair? = nil
    ) throws {
        guard responderPublicKey.count == 32 else { throw RelaySecurityError.invalidKey }
        self.staticKeypair = staticKeypair
        self.responderPublicKey = responderPublicKey
        self.ephemeralKeypair = ephemeralKeypair ?? RelayKeypair()
        self.symmetric = RelaySymmetricState(protocolName: Self.protocolName, responderPublicKey: responderPublicKey)
    }

    public func writeMessage(_ payload: Data = Data()) throws -> Data {
        guard messageIndex == 0 else { throw RelaySecurityError.unexpectedHandshakeState }
        var output = Data()
        output.append(ephemeralKeypair.publicKey)
        symmetric.mixHash(ephemeralKeypair.publicKey)
        symmetric.mixKey(try dh(privateKey: ephemeralKeypair.privateKey, publicKey: responderPublicKey))
        output.append(try symmetric.encryptAndHash(staticKeypair.publicKey))
        symmetric.mixKey(try dh(privateKey: staticKeypair.privateKey, publicKey: responderPublicKey))
        output.append(try symmetric.encryptAndHash(payload))
        messageIndex = 1
        return output
    }

    public func readMessage(_ message: Data) throws -> Data {
        guard messageIndex == 1, message.count >= 48 else { throw RelaySecurityError.invalidMessage }
        let remoteEphemeral = Data(message.prefix(32))
        symmetric.mixHash(remoteEphemeral)
        symmetric.mixKey(try dh(privateKey: ephemeralKeypair.privateKey, publicKey: remoteEphemeral))
        symmetric.mixKey(try dh(privateKey: staticKeypair.privateKey, publicKey: remoteEphemeral))
        let payload = try symmetric.decryptAndHash(Data(message.dropFirst(32)))
        let (enc, dec) = symmetric.split()
        transport = RelayNoiseSession(encryptor: enc, decryptor: dec)
        messageIndex = 2
        return payload
    }

    public func session() throws -> RelayNoiseSession {
        guard let transport else { throw RelaySecurityError.handshakeIncomplete }
        return transport
    }

    private func dh(privateKey: Data, publicKey: Data) throws -> Data {
        let local = try Curve25519.KeyAgreement.PrivateKey(rawRepresentation: privateKey)
        let remote = try Curve25519.KeyAgreement.PublicKey(rawRepresentation: publicKey)
        let shared = try local.sharedSecretFromKeyAgreement(with: remote)
        return shared.withUnsafeBytes { Data($0) }
    }
}

public enum RelayFingerprint {
    public static func of(publicKey: Data) -> String {
        let digest = Data(SHA256.hash(data: publicKey)).prefix(16)
        let alphabet = Array("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567")
        var bits = 0
        var value = 0
        var encoded = ""
        for byte in digest {
            value = (value << 8) | Int(byte)
            bits += 8
            while bits >= 5 {
                bits -= 5
                encoded.append(alphabet[(value >> bits) & 31])
                value &= (1 << bits) - 1
            }
        }
        if bits > 0 { encoded.append(alphabet[(value << (5 - bits)) & 31]) }
        return stride(from: 0, to: encoded.count, by: 4).map { offset in
            let start = encoded.index(encoded.startIndex, offsetBy: offset)
            let end = encoded.index(start, offsetBy: min(4, encoded.distance(from: start, to: encoded.endIndex)))
            return String(encoded[start..<end])
        }.joined(separator: "-")
    }
}
