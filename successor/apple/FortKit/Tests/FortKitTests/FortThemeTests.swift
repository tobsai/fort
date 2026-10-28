import Testing
@testable import FortKit

struct FortThemeTests {
    @Test func anAgentKeepsItsCrewColourOnEverySurface() {
        // The same vectors are in successor/design/tokens.test.mjs for the web client.
        let vectors: [(String,Int)] = [("",1),("agent-1",4),("Lewis",3),("b6f1c2d0-5a4e-4c1b-9f3e-2d7a8c9e0f11",2),("émile",2)]
        for (id,slot) in vectors { #expect(FortTheme.crewSlot(for:id) == slot,"\(id)") }
        #expect(FortTheme.crew.count == 6)
    }
}
