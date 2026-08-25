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
        let readyAgent = app.staticTexts.matching(
            NSPredicate(format: "label CONTAINS[c] %@", "Ready")
        ).firstMatch
        XCTAssertTrue(readyAgent.waitForExistence(timeout: 2), "No ready Agent Channel appeared")
        readyAgent.click()
        XCTAssertEqual(app.alerts.count, 0, "Fort launched with a blocking UI alert")
        for rawErrorCode in ["incompatible_version", "setup_required"] {
            XCTAssertFalse(app.staticTexts[rawErrorCode].exists, "Fort exposed raw UI error \(rawErrorCode)")
        }

        let textView = app.textViews.firstMatch
        let composer = textView.waitForExistence(timeout: 2) ? textView : app.textFields.firstMatch
        XCTAssertTrue(composer.waitForExistence(timeout: 2), "Ready Channel composer did not appear")
        composer.click()
        composer.typeText("Release UI gate")
        XCTAssertTrue(app.buttons["Send"].isEnabled, "Send stayed disabled after valid input")

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
