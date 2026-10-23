import Foundation
import Testing
@testable import FortKit

private let linkCode = Data(repeating:7,count:32).base64EncodedString().replacingOccurrences(of:"=",with:"")
    .replacingOccurrences(of:"+",with:"-").replacingOccurrences(of:"/",with:"_")

@Test func invitationLinkRoundTripsExactPhoneSetupFields() throws {
    let link = try CompanionInvitationLink(address:URL(string:"https://mac.tail0000.ts.net:8443")!,installationID:"one",invitationID:"invite",code:linkCode)
    #expect(link.text.hasPrefix("fort-invite:"))
    let parsed = try CompanionInvitationLink(text:"  " + link.text + "\n")
    #expect(parsed == link)
    #expect(parsed.address.absoluteString == "https://mac.tail0000.ts.net:8443")
    #expect(parsed.installationID == "one" && parsed.invitationID == "invite" && parsed.code == linkCode)
}

@Test func invitationLinkRefusesUnsafeOrIncompleteValues() throws {
    let address = URL(string:"https://mac.tail0000.ts.net:8443")!
    #expect(throws:(any Error).self) { try CompanionInvitationLink(address:URL(string:"http://mac.tail0000.ts.net")!,installationID:"one",invitationID:"invite",code:linkCode) }
    #expect(throws:(any Error).self) { try CompanionInvitationLink(address:address,installationID:"one/two",invitationID:"invite",code:linkCode) }
    #expect(throws:(any Error).self) { try CompanionInvitationLink(address:address,installationID:"one",invitationID:"invite",code:"short") }
    for text in ["", "https://mac.tail0000.ts.net", "fort-invite:", "fort-invite:not-base64!", "fort-invite:" + Data(#"{"v":1}"#.utf8).base64EncodedString()] {
        #expect(throws:(any Error).self) { try CompanionInvitationLink(text:text) }
    }
}
