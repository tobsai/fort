#if os(macOS)
import Foundation
import Testing
@testable import FortKit

private let sanCertificate = Data("""
-----BEGIN CERTIFICATE-----
MIIBhjCCASygAwIBAgIUda42p1/YCVrLzzXCp7GYE/+UocEwCgYIKoZIzj0EAwIw
ADAeFw0yNjEwMjMwMDE1MjJaFw0zNjEwMjAwMDE1MjJaMAAwWTATBgcqhkjOPQIB
BggqhkjOPQMBBwNCAASkttr/f3M6Wu9DCclpbpcgFQ6h9WeMoDAw7aOLoGOLuOcy
0uM3SgCw42bq1uv2yXr5y3fZBqpmMN7oMMqU8nMzo4GDMIGAMB0GA1UdDgQWBBT/
hEx+Lv6ty1xrltEshzUXDqI7uTAfBgNVHSMEGDAWgBT/hEx+Lv6ty1xrltEshzUX
DqI7uTAPBgNVHRMBAf8EBTADAQH/MC0GA1UdEQQmMCSCE21hYy50YWlsMDAwMC50
cy5uZXSCDW90aGVyLmV4YW1wbGUwCgYIKoZIzj0EAwIDSAAwRQIhAMGEu1maSuec
fLm6v6ef9xkYaIAT/FBFCjOWtJTUVnEsAiAoR2uPHQWyn8ixQpErH+urJYwkLzh5
8IZg46PXs/AGRQ==
-----END CERTIFICATE-----

""".utf8)

@Test func phoneAccessSelectsOnlyATailscaleSharedAddress() {
    #expect(MacPhoneAccess.tailscaleAddress(in:[("lo0","127.0.0.1"),("en0","192.168.1.20"),("utun4","100.111.16.2")]) == "100.111.16.2")
    #expect(MacPhoneAccess.tailscaleAddress(in:[("utun4","100.63.255.1"),("utun5","100.128.0.1"),("en0","10.0.0.2")]) == nil)
    #expect(MacPhoneAccess.tailscaleAddress(in:[("en0","100.72.4.9"),("pdp_ip0","100.80.1.2")]) == nil)
}

@Test func phoneAccessReadsTheCertificateDNSNameEvenWithoutASubject() throws {
    #expect(try MacPhoneAccess.serverName(certificatePEM:sanCertificate) == "mac.tail0000.ts.net")
    #expect(throws:(any Error).self) { try MacPhoneAccess.serverName(certificatePEM:Data("not a certificate".utf8)) }
}

@Test func phoneAccessStoresTheImportedIdentityPrivately() throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString,isDirectory:true)
    defer { try? FileManager.default.removeItem(at:directory) }
    try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:true)
    let access = MacPhoneAccess(directory:directory)
    #expect(try access.identity() == nil)
    try access.save(certificate:sanCertificate,privateKey:Data("-----BEGIN PRIVATE KEY-----\nAA==\n-----END PRIVATE KEY-----\n".utf8))
    let saved = try #require(try access.identity())
    #expect(saved.serverName == "mac.tail0000.ts.net" && saved.certificate == sanCertificate)
    let mode = try FileManager.default.attributesOfItem(atPath:directory.appendingPathComponent("phone-access/private-key.pem").path)[.posixPermissions] as? Int
    #expect(mode == 0o600)
    #expect(throws:(any Error).self) { try access.save(certificate:sanCertificate,privateKey:Data("plain".utf8)) }
}

@Test func newInvitationsUseExactIdentifiersAndCodes() {
    let invitation = MacPhoneAccess.newInvitation()
    #expect(validCompanionIdentifier(invitation.id) && validCompanionToken(invitation.code))
    #expect(MacPhoneAccess.newInvitation().code != invitation.code)
}
#endif
