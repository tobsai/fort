import XCTest

final class NativeJourneyTests: XCTestCase {
    @MainActor func testEnrollSendAndReplayAfterAppRestart() async throws {
        continueAfterFailure = false
        let environment = ProcessInfo.processInfo.environment
        guard let endpoint = environment["FORT_FIXTURE_URL"], let installation = environment["FORT_FIXTURE_INSTALLATION"], let code = environment["FORT_FIXTURE_CODE"] else {
            throw XCTSkip("Run through the explicit local fixture UI harness")
        }
        let app = XCUIApplication()
        app.launchArguments = ["--fixture-ui-test"]
        app.launchEnvironment = ["FORT_UI_TEST_SESSION":UUID().uuidString,"FORT_FIXTURE_URL":endpoint,"FORT_FIXTURE_INSTALLATION":installation,"FORT_FIXTURE_CODE":code]
        app.launch()
        defer { app.terminate() }
        let enroll = app.buttons["Enroll this device"]
        XCTAssertTrue(enroll.waitForExistence(timeout:15))
        XCTAssertTrue(enroll.isEnabled)
        activate(enroll)
        if !app.descendants(matching:.any)["message-composer"].firstMatch.waitForExistence(timeout:3) {
            print("Fixture accessibility after enrollment: " + app.windows.firstMatch.debugDescription)
            let capture = XCTAttachment(screenshot:capture(app)); capture.name = "Enrollment result"; capture.lifetime = .keepAlways; add(capture)
            let agent = app.staticTexts["Fixture Agent"].firstMatch
            XCTAssertTrue(agent.waitForExistence(timeout:10))
            activate(agent)
        }
        let hierarchy = XCTAttachment(string:app.windows.firstMatch.debugDescription); hierarchy.name = "Enrolled native accessibility hierarchy"; hierarchy.lifetime = .keepAlways; add(hierarchy)
        let composer = app.descendants(matching:.any)["message-composer"].firstMatch
        XCTAssertTrue(composer.waitForExistence(timeout:15))
        activate(composer)
        composer.typeText("Native UI fixture message")
        let sendButton = app.buttons["send-message"]
        XCTAssertTrue(sendButton.isEnabled)
        activate(sendButton)
        XCTAssertTrue(app.staticTexts["Fixture reply saved"].waitForExistence(timeout:15))
        if !containsText("Native UI fixture message",in:app) {
            let detail = XCTAttachment(string:app.windows.firstMatch.debugDescription); detail.name = "Transcript text accessibility failure"; detail.lifetime = .keepAlways; add(detail)
            let screen = XCTAttachment(screenshot:capture(app)); screen.name = "Transcript text visual failure"; screen.lifetime = .keepAlways; add(screen)
        }
        XCTAssertTrue(containsText("Native UI fixture message",in:app))
        let screenshot = XCTAttachment(screenshot:capture(app))
        screenshot.name = "Native conversation with durable fixture reply"
        screenshot.lifetime = .keepAlways
        add(screenshot)
        app.terminate()
        app.launch()
        XCTAssertTrue(app.staticTexts["Fixture reply saved"].waitForExistence(timeout:15))
        XCTAssertTrue(containsText("Native UI fixture message",in:app))
        let restart = XCTAttachment(screenshot:capture(app))
        restart.name = "Native restart recovered transcript"
        restart.lifetime = .keepAlways
        add(restart)
        try await control("offline",environment)
        app.terminate(); app.launch()
        XCTAssertTrue(app.staticTexts["Fixture reply saved"].waitForExistence(timeout:15))
        try send("Saved while offline",in:app)
        XCTAssertTrue(app.staticTexts["Saved on this device · acceptance not confirmed"].waitForExistence(timeout:10))
        app.terminate(); app.launch()
        XCTAssertTrue(app.staticTexts["Saved on this device · acceptance not confirmed"].waitForExistence(timeout:15))
        let offline = XCTAttachment(screenshot:capture(app)); offline.name = "Cold offline restart preserves pending and history"; offline.lifetime = .keepAlways; add(offline)
        try await control("online",environment)
        awaitCount(app.staticTexts.matching(identifier:"Fixture reply saved"),2)
        try await control("drop-acceptance",environment)
        try send("Lost reply",in:app)
        XCTAssertTrue(app.staticTexts["Saved on this device · acceptance not confirmed"].waitForExistence(timeout:10))
        try await control("restart",environment)
        app.terminate(); app.launch()
        XCTAssertTrue(app.staticTexts["Saved on this device · acceptance not confirmed"].waitForExistence(timeout:15))
        try await control("online",environment)
        awaitCount(app.staticTexts.matching(identifier:"Fixture reply saved"),3)
        XCTAssertFalse(app.staticTexts["Saved on this device · acceptance not confirmed"].exists)
        let recovered = XCTAttachment(screenshot:capture(app)); recovered.name = "Lost acknowledgement reconciled after service and app restart"; recovered.lifetime = .keepAlways; add(recovered)
        app.terminate()
    }
    @MainActor func testApproveAndCancelFixtureRuns() throws {
        continueAfterFailure = false
        let environment = ProcessInfo.processInfo.environment
        guard let endpoint = environment["FORT_FIXTURE_URL"], let installation = environment["FORT_FIXTURE_INSTALLATION"], let code = environment["FORT_FIXTURE_CODE"] else { throw XCTSkip("Use explicit fixture UI harness") }
        let app = XCUIApplication(); app.launchArguments = ["--fixture-ui-test"]
        app.launchEnvironment = ["FORT_UI_TEST_SESSION":UUID().uuidString,"FORT_FIXTURE_URL":endpoint,"FORT_FIXTURE_INSTALLATION":installation,"FORT_FIXTURE_CODE":code]
        app.launch(); defer { app.terminate() }
        let enroll = app.buttons["Enroll this device"]; XCTAssertTrue(enroll.waitForExistence(timeout:15)); activate(enroll)
        try send("Native approval fixture request",in:app)
        let approve = app.buttons["Approve fixture reply"]
        XCTAssertTrue(approve.waitForExistence(timeout:15))
        XCTAssertFalse(app.staticTexts["Fixture reply saved"].exists)
        reveal(approve,in:app)
        activate(approve)
        XCTAssertTrue(app.staticTexts["Fixture reply saved"].waitForExistence(timeout:15))
        try send("Native cancellation fixture request",in:app)
        XCTAssertTrue(app.staticTexts["Waiting for your approval"].waitForExistence(timeout:15))
        let cancel = app.buttons["Cancel run"]
        reveal(cancel,in:app)
        activate(cancel)
        XCTAssertTrue(app.staticTexts["Canceled"].waitForExistence(timeout:15))
        XCTAssertEqual(app.staticTexts.matching(identifier:"Fixture reply saved").count,1)
        let evidence = XCTAttachment(screenshot:capture(app)); evidence.name = "Native approved reply and canceled fixture run"; evidence.lifetime = .keepAlways; add(evidence)
    }
    @MainActor func testCloseUnknownWithoutRetry() async throws {
        continueAfterFailure = false
        let environment = ProcessInfo.processInfo.environment
        guard let endpoint = environment["FORT_FIXTURE_URL"], let installation = environment["FORT_FIXTURE_INSTALLATION"], let code = environment["FORT_FIXTURE_CODE"] else { throw XCTSkip("Use explicit fixture UI harness") }
        let app = XCUIApplication(); app.launchArguments = ["--fixture-ui-test"]
        app.launchEnvironment = ["FORT_UI_TEST_SESSION":UUID().uuidString,"FORT_FIXTURE_URL":endpoint,"FORT_FIXTURE_INSTALLATION":installation,"FORT_FIXTURE_CODE":code]
        app.launch(); defer { app.terminate() }
        let enroll = app.buttons["Enroll this device"]; XCTAssertTrue(enroll.waitForExistence(timeout:15)); activate(enroll)
        try send("Native uncertain fixture request",in:app)
        let unknown = app.staticTexts["Outcome unknown · review required"]
        XCTAssertTrue(unknown.waitForExistence(timeout:15))
        try send("Queued behind uncertain work",in:app)
        XCTAssertTrue(app.staticTexts["Accepted · waiting for fixture host"].waitForExistence(timeout:15))
        XCTAssertFalse(app.staticTexts["Fixture reply saved"].exists)
        try await control("restart",environment)
        app.terminate(); app.launch()
        XCTAssertTrue(unknown.waitForExistence(timeout:15))
        XCTAssertTrue(app.staticTexts["Accepted · waiting for fixture host"].exists)
        let close = app.buttons["Close without retry"]
        reveal(close,in:app); activate(close)
        let explanation = "The original outcome remains unknown. Closing preserves its record and allows future messages. Fort will not retry this run."
        XCTAssertTrue(app.staticTexts[explanation].waitForExistence(timeout:5))
        let cancel = app.buttons["Cancel"]
        if cancel.exists { activate(cancel) }
        else {
            let dismiss = app.otherElements["PopoverDismissRegion"]
            XCTAssertTrue(dismiss.exists)
            dismiss.coordinate(withNormalizedOffset:CGVector(dx:0.95,dy:0.1)).tap()
        }
        XCTAssertTrue(unknown.exists)
        XCTAssertFalse(app.staticTexts["Fixture reply saved"].exists)
        reveal(close,in:app); activate(close)
        let confirm = app.sheets.buttons["Close without retry"]
        XCTAssertTrue(confirm.waitForExistence(timeout:5))
        try await control("hold-refresh",environment)
        try await control("await-held-refresh",environment)
        activate(confirm)
        try await control("release-refresh",environment)
        let closed = app.staticTexts["Closed without retry · original outcome remains unknown"]
        XCTAssertTrue(closed.waitForExistence(timeout:15))
        XCTAssertTrue(app.staticTexts["Fixture reply saved"].waitForExistence(timeout:15))
        app.terminate(); app.launch()
        XCTAssertTrue(closed.waitForExistence(timeout:15))
        XCTAssertEqual(app.staticTexts.matching(identifier:"Fixture reply saved").count,1)
        XCTAssertFalse(unknown.exists)
        let proof = XCTAttachment(screenshot:capture(app)); proof.name = "Closed unknown outcome and later completed turn"; proof.lifetime = .keepAlways; add(proof)
    }
    @MainActor func testRaceApprovalAcrossDevices() async throws {
        continueAfterFailure = false
        let environment = ProcessInfo.processInfo.environment
        guard let endpoint = environment["FORT_FIXTURE_URL"], let installation = environment["FORT_FIXTURE_INSTALLATION"], let code = environment["FORT_FIXTURE_CODE"], let role = environment["FORT_FIXTURE_ROLE"], ["A","B"].contains(role) else { throw XCTSkip("Use explicit two-device fixture harness") }
        let app = XCUIApplication(); app.launchArguments = ["--fixture-ui-test"]
        app.launchEnvironment = ["FORT_UI_TEST_SESSION":UUID().uuidString,"FORT_FIXTURE_URL":endpoint,"FORT_FIXTURE_INSTALLATION":installation,"FORT_FIXTURE_CODE":code]
        app.launch(); defer { app.terminate() }
        let enroll = app.buttons["Enroll this device"]; XCTAssertTrue(enroll.waitForExistence(timeout:15)); activate(enroll)
        if role == "A" { try send("Native two-device race",in:app) }
        let action = app.buttons[role == "A" ? "Approve fixture reply" : "Reject"]
        XCTAssertTrue(action.waitForExistence(timeout:100)); reveal(action,in:app)
        try await control("ready/" + role,environment)
        activate(action)
        let approved = app.staticTexts["Approval: approved"]
        let rejected = app.staticTexts["Approval: rejected"]
        let settled = XCTNSPredicateExpectation(predicate:NSPredicate { _,_ in approved.exists || rejected.exists },object:nil)
        await fulfillment(of:[settled],timeout:30)
        let decision = approved.exists ? "approved" : "rejected"
        let state = decision == "approved" ? "Fixture reply saved" : "Canceled"
        XCTAssertTrue(app.staticTexts[state].waitForExistence(timeout:15))
        XCTAssertFalse(action.exists)
        app.terminate(); app.launch()
        XCTAssertTrue(app.staticTexts["Approval: " + decision].waitForExistence(timeout:15))
        XCTAssertTrue(app.staticTexts[state].exists)
        try await control("observed/" + role + "/" + decision,environment)
        let proof = XCTAttachment(screenshot:capture(app)); proof.name = "Device " + role + " sees one durable race winner"; proof.lifetime = .keepAlways; add(proof)
    }
    @MainActor private func reveal(_ element: XCUIElement, in app: XCUIApplication) {
        let history = app.scrollViews["conversation-history"]
        for _ in 0..<4 {
            if element.isHittable { return }
            #if os(macOS)
            history.scroll(byDeltaX:0,deltaY:-160)
            #else
            history.swipeUp()
            #endif
        }
        XCTAssertTrue(element.isHittable)
    }
    @MainActor private func capture(_ app: XCUIApplication) -> XCUIScreenshot {
        #if os(macOS)
        app.windows.firstMatch.screenshot()
        #else
        app.screenshot()
        #endif
    }
    @MainActor private func containsText(_ text: String, in app: XCUIApplication) -> Bool {
        app.descendants(matching:.any).matching(NSPredicate(format:"label CONTAINS %@ OR value CONTAINS %@",text,text)).firstMatch.exists
    }
    @MainActor private func send(_ text: String, in app: XCUIApplication) throws {
        let composer = app.descendants(matching:.any)["message-composer"].firstMatch
        XCTAssertTrue(composer.waitForExistence(timeout:10)); activate(composer); composer.typeText(text)
        XCTAssertEqual(composer.value as? String,text)
        let button = app.buttons["send-message"]
        let ready = XCTNSPredicateExpectation(predicate:NSPredicate(format:"enabled == true"),object:button)
        XCTAssertEqual(XCTWaiter.wait(for:[ready],timeout:10),.completed)
        activate(button)
    }
    @MainActor private func awaitCount(_ query: XCUIElementQuery, _ count: Int) {
        let expected = XCTNSPredicateExpectation(predicate:NSPredicate(format:"count == %d",count),object:query)
        XCTAssertEqual(XCTWaiter.wait(for:[expected],timeout:15),.completed)
    }
    @MainActor private func control(_ action: String, _ environment: [String:String]) async throws {
        guard let base = environment["FORT_FIXTURE_CONTROL_URL"], let token = environment["FORT_FIXTURE_CONTROL_TOKEN"] else { throw XCTSkip("Missing explicit fixture control harness") }
        var request = URLRequest(url:URL(string:base+"/"+action)!); request.timeoutInterval = 130; request.httpMethod = "POST"; request.setValue("Bearer \(token)",forHTTPHeaderField:"Authorization")
        let (_,response) = try await URLSession.shared.data(for:request)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode,200)
    }
    @MainActor private func activate(_ element: XCUIElement) {
        #if os(macOS)
        element.click()
        #else
        element.tap()
        #endif
    }
}
