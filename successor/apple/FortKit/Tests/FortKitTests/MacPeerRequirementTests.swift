#if os(macOS)
import Foundation
import Security
import Testing
@testable import FortKit

@Test func peerRequirementCompilesAndAcceptsAppleStoreSigning() throws {
    let expression = try macPeerRequirement(identifier:"io.mtree.fort.host",signingTeam:"T3JB5MYZ93")
    var requirement: SecRequirement?
    #expect(SecRequirementCreateWithString(expression as CFString,SecCSFlags(),&requirement) == errSecSuccess)
    #expect(requirement != nil)
    #expect(expression.contains("identifier \"io.mtree.fort.host\""))
    #expect(expression.contains("certificate leaf[subject.OU] = \"T3JB5MYZ93\""))
    #expect(expression.contains("certificate leaf[field.1.2.840.113635.100.6.1.9] exists"))
    // TestFlight for Mac re-signs with a leaf marked 1.2.840.113635.100.6.1.25.1 and no team OU; without it
    // the app refused its own helper ("The exact client or host code signature could not be verified").
    #expect(expression.contains("certificate leaf[field.1.2.840.113635.100.6.1.25.1] exists"))
    #expect(expression.hasPrefix("anchor apple generic and identifier \"io.mtree.fort.host\" and ("))
    #expect(throws:ClientError.self) { try macPeerRequirement(identifier:"io.mtree.fort\" or anchor apple",signingTeam:"T3JB5MYZ93") }
    #expect(throws:ClientError.self) { try macPeerRequirement(identifier:"io.mtree.fort",signingTeam:"T3JB5MYZ93\" or true") }
}
#endif
