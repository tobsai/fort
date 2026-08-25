import FortKit

enum ReleaseIdentityContractTests {
    static func run() {
        let release = FortReleaseIdentity(
            marketingVersion: "1.0.10",
            build: "2608243"
        )

        expect(
            release.displayText == "Version 1.0.10 (2608243)",
            "release identity did not preserve the bundle version and build"
        )
        expect(
            FortReleaseIdentity(marketingVersion: nil, build: "2608243").displayText == "Version unavailable",
            "release identity guessed a missing marketing version"
        )
        expect(
            FortReleaseIdentity(marketingVersion: "1.0.10", build: "   ").displayText == "Version unavailable",
            "release identity rendered an empty build"
        )
    }

    private static func expect(_ condition: @autoclosure () -> Bool, _ message: String) {
        guard condition() else { fatalError(message) }
    }
}
