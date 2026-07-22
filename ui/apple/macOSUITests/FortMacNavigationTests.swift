import XCTest

final class FortMacNavigationTests: XCTestCase {
    func testPrimarySidebarChangesDetail() {
        let app = XCUIApplication()
        app.launch()

        let projects = app.staticTexts["Projects"]
        XCTAssertTrue(projects.waitForExistence(timeout: 5), "Projects sidebar item did not appear")
        projects.click()
        XCTAssertTrue(
            app.staticTexts["Project rooms"].waitForExistence(timeout: 2),
            "Projects selection did not change the detail pane"
        )

        app.staticTexts["Playbooks"].click()
        XCTAssertTrue(
            app.staticTexts["who does what, with which model"].waitForExistence(timeout: 2),
            "Playbooks selection did not change the detail pane"
        )
    }
}
