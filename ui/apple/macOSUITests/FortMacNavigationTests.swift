import XCTest

final class FortMacNavigationTests: XCTestCase {
    func testPrimarySidebarChangesDetail() {
        let app = XCUIApplication()
        let qaHost = ProcessInfo.processInfo.environment["FORT_DIRECT_HOST_URL"]
            ?? "http://127.0.0.1:4187"
        guard let qaURL = URL(string: qaHost),
              ["http", "https"].contains(qaURL.scheme?.lowercased() ?? ""),
              qaURL.host != nil else {
            XCTFail("FORT_DIRECT_HOST_URL must be an HTTP(S) fixture URL")
            return
        }
        XCTAssertNotEqual(qaURL.port, 4087, "UI QA must not target the real launchd service")
        app.launchEnvironment["FORT_DIRECT_HOST_URL"] = qaHost
        app.launch()

        XCTAssertTrue(app.staticTexts["FORT"].waitForExistence(timeout: 5), "Primary Channels sidebar did not appear")

        let needsYou = app.staticTexts["Needs You"].firstMatch
        XCTAssertTrue(needsYou.waitForExistence(timeout: 2), "Needs You sidebar item did not appear")
        needsYou.click()
        XCTAssertTrue(
            app.staticTexts["Only current, recoverable failed Primary Channel attempts"].waitForExistence(timeout: 2),
            "Needs You selection did not change the detail pane"
        )

        app.staticTexts["Settings"].firstMatch.click()
        XCTAssertTrue(
            app.staticTexts["Appearance on this device"].waitForExistence(timeout: 2),
            "Settings selection did not change the detail pane"
        )
    }
}
