import Foundation
import FortKit

#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

@main
struct FortKitContractChecks {
    static func main() async throws {
        try primaryWireModelsDecodeCanonicalFixtures()
        try await clientUsesPrimaryEndpointsAndNoContentCommands()
        try await clientSurfacesTypedPrimaryErrorCodes()
        try await primaryChannelEventsDecodeStrictReplacementSnapshots()
        try await primaryChannelEventsUseGatewayRelayPath()
        primaryStatusProgressivelyDisclosesLatestAttempt()
        primaryRecoveryActionsStayClosed()
        primaryPendingTurnPreservesClientTurnID()
        try primaryPendingTurnsPersistAndReconcileAuthoritativeTurns()
        try primarySendOutcomeDistinguishesAcceptedDeterministicAndAmbiguous()
        primarySchedulesUseCanonicalLabelsAndConfiguredTimezones()
        try primaryScheduleDetailAndOccurrenceActionsStayTruthful()
        primaryModelDisclosurePreservesRequestedAndResolvedModels()
        try primarySettingsGroupsOptionsByComputer()
        try primaryTransportGenerationChangesAcrossSameOriginMachineSwitch()
        try await clientSurfacesRequestIDOnHTTPFailure()
        try gatewayAccountPersistsNativeSession()
        try gatewayAddressNormalizesProductionOrigin()
        try await gatewayRequestsCarryCanonicalCorrelationIDs()
        try await gatewayRelayRetriesHandshakeAndExplainsFailures()
        try secureRelayMatchesGoNoiseVector()
        orbMotionSeparatesEnergyFromSpatialMovement()
        try nativeOrbUsesRasterAndHonorsReduceMotion()
        try iPhoneSimulatorSupportsDeterministicVisualQAHost()
        try iPhonePhysicalReleaseUsesOnlyAuthenticatedRelay()
        print("FortKit Phase 1 contract checks passed")
    }

    private static func expect(_ condition: @autoclosure () -> Bool, _ message: String) {
        guard condition() else { fatalError(message) }
    }

    private static func clientSurfacesRequestIDOnHTTPFailure() async throws {
        ContractURLProtocol.requests = []
        ContractURLProtocol.handler = { request in
            let response = HTTPURLResponse(
                url: request.url!,
                statusCode: 503,
                httpVersion: "HTTP/1.1",
                headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(#"{"error":"not ready"}"#.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [ContractURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )

        do {
            _ = try await client.primaryAgent()
            fatalError("failed Fort response unexpectedly decoded")
        } catch let error as FortClientError {
            guard case let .httpStatus(status, _, requestID) = error else {
                fatalError("unexpected Fort client error: \(error)")
            }
            expect(status == 503, "Fort response diagnostic lost its status")
            expectCanonicalRequestID(requestID, "Fort response diagnostic")
            let sentRequestID = ContractURLProtocol.requests.first?
                .value(forHTTPHeaderField: FortRequestID.header)
            expect(requestID == sentRequestID, "Fort response diagnostic changed the request ID")
            if let requestID {
                expect(error.localizedDescription.contains(requestID), "Fort response diagnostic omitted the request ID")
            }
        }
    }

    private static func gatewayAccountPersistsNativeSession() throws {
        let suite = "FortKitContractChecks.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        let account = GatewayAccount(
            gatewayURL: URL(string: "https://fort.example")!,
            selectedMachineID: "machine-1",
            bearerToken: "native-token",
            pinnedPublicKeys: ["machine-1": "86nFNj7PKVZC81MQ2j3/1YOsYiryw9jUK1csCZWnB3c="]
        )
        account.save(to: defaults)
        expect(GatewayAccount.load(from: defaults) == account, "native gateway session did not persist")
    }

    private static func gatewayAddressNormalizesProductionOrigin() throws {
        let expected = URL(string: "https://fort-gateway.vercel.app")!
        for raw in [
            "https://fort-gateway.vercel.app",
            "https://fort-gateway.vercel.app/",
            "https://fort-gateway.vercel.app/native",
            "  https://fort-gateway.vercel.app/native?callback=stale#fragment  ",
        ] {
            let normalized = try GatewayAddress.normalize(raw)
            expect(normalized == expected, "gateway address did not normalize \(raw)")
        }
        for invalid in [
            "fort-gateway.vercel.app",
            "http://fort-gateway.vercel.app",
            "ftp://fort-gateway.vercel.app",
            "https://user:secret@fort-gateway.vercel.app",
            "https://fort-gateway.vercel.app/not-native",
            "https://fort-gateway.tobias-053.workers.dev",
        ] {
            do {
                _ = try GatewayAddress.normalize(invalid)
                fatalError("accepted invalid gateway address \(invalid)")
            } catch is GatewayAddressError {
                // Expected.
            }
        }
    }

    private static func gatewayRelayRetriesHandshakeAndExplainsFailures() async throws {
        var attempts = 0
        let result = try await GatewayRelayRetry.handshake {
            attempts += 1
            if attempts == 1 {
                throw GatewayRelayError.httpStatus(
                    502,
                    #"{"error":"worker req: 504 {\"error\":\"daemon did not respond\"}"}"#
                )
            }
            return "connected"
        }
        expect(result == "connected", "transient relay handshake did not recover")
        expect(attempts == 2, "transient relay handshake must retry exactly once")

        attempts = 0
        do {
            _ = try await GatewayRelayRetry.handshake {
                attempts += 1
                throw GatewayRelayError.httpStatus(401, #"{"error":"unauthorized"}"#)
            } as String
            fatalError("non-transient relay failure unexpectedly succeeded")
        } catch {
            expect(attempts == 1, "non-transient relay failure must not retry")
        }

        let description = GatewayRelayError.httpStatus(
            502,
            #"{"error":"worker req: 504 {\"error\":\"daemon did not respond\"}"}"#
        ).localizedDescription
        expect(description.contains("502"), "relay failure description omitted the HTTP status")
        expect(description.contains("daemon did not respond"), "relay failure description omitted gateway detail")
        expect(!description.contains("GatewayRelayError error"), "relay failure leaked an opaque Swift enum code")
    }

    private static func gatewayRequestsCarryCanonicalCorrelationIDs() async throws {
        ContractURLProtocol.requests = []
        ContractURLProtocol.handler = { request in
            let response = HTTPURLResponse(
                url: request.url!,
                statusCode: 200,
                httpVersion: "HTTP/1.1",
                headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(#"{"machines":[]}"#.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [ContractURLProtocol.self]
        let session = URLSession(configuration: configuration)
        _ = try await GatewayService.machines(
            at: URL(string: "https://fort-gateway.test")!,
            bearerToken: "machine-list-secret",
            session: session
        )
        expectCanonicalRequestID(
            ContractURLProtocol.requests.last?.value(forHTTPHeaderField: FortRequestID.header),
            "machine discovery"
        )

        ContractURLProtocol.requests = []
        ContractURLProtocol.handler = { request in
            let response = HTTPURLResponse(
                url: request.url!,
                statusCode: 502,
                httpVersion: "HTTP/1.1",
                headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(#"{"error":"relay unavailable"}"#.utf8))
        }

        let relay = try GatewayRelayTransport(
            gatewayURL: URL(string: "https://fort-gateway.test")!,
            bearerToken: "relay-secret",
            machineID: "machine-1",
            machinePublicKey: Data(repeating: 7, count: 32),
            session: session
        )
        do {
            _ = try await relay.request(path: "/api/settings/primary-agent")
            fatalError("unavailable relay unexpectedly accepted the request")
        } catch {
            let requestIDs = ContractURLProtocol.requests.compactMap {
                $0.value(forHTTPHeaderField: FortRequestID.header)
            }
            expect(requestIDs.count >= 2, "each handshake attempt must carry a request ID")
            expect(requestIDs.count == ContractURLProtocol.requests.count, "a relay frame omitted the logical request ID")
            expect(Set(requestIDs).count == 1, "handshake retry changed the logical request ID")
            expectCanonicalRequestID(requestIDs.first, "relay handshake")
            if let requestID = requestIDs.first {
                expect(error.localizedDescription.contains(requestID), "relay diagnostic omitted the request ID")
            }
            expect(!error.localizedDescription.contains("relay-secret"), "relay diagnostic exposed authorization material")
        }
    }

    private static func expectCanonicalRequestID(_ requestID: String?, _ context: String) {
        guard let requestID, let uuid = UUID(uuidString: requestID) else {
            fatalError("\(context) omitted a canonical X-Fort-Request-ID")
        }
        expect(
            uuid.uuidString.lowercased() == requestID,
            "\(context) emitted a non-canonical X-Fort-Request-ID: \(requestID)"
        )
    }

    private static func secureRelayMatchesGoNoiseVector() throws {
        let initiatorStatic = try RelayKeypair(
            privateKey: Data(base64Encoded: "lfM23kK1E/kHySaXdRbRpdh+Wf/4mbu7wJcIq34eHUE=")!
        )
        let initiatorEphemeral = try RelayKeypair(
            privateKey: Data(base64Encoded: "BlmfkXmXtH/gYxqLYk1O2yUfA6K7M2eiFx8D6agjkXs=")!
        )
        let responderPublic = Data(base64Encoded: "86nFNj7PKVZC81MQ2j3/1YOsYiryw9jUK1csCZWnB3c=")!
        let handshake = try RelayNoiseInitiator(
            staticKeypair: initiatorStatic,
            responderPublicKey: responderPublic,
            ephemeralKeypair: initiatorEphemeral
        )
        let message1 = try handshake.writeMessage(Data("fort ik handshake payload one".utf8))
        expect(
            message1.base64EncodedString() == "3Ngz4RsEAJbFaKPfC78UHmXYfopX27T1c/YzzhtFimwbN4CCW7FVgyT0AXT/WZNHA8VJMe56k5XPw81eu0OOuyK8f/0kWAet1pU2E5K2BykL5N+ecUgldzTDB3JvsUlhaPAJ7BjU9iGf5+NZF/IlKWuCNP7R8DrUKVT9rtA=",
            "Swift Noise IK message 1 drifted from Go: \(message1.base64EncodedString())"
        )
        _ = try handshake.readMessage(
            Data(base64Encoded: "akgBv7rKL76QfAvC8LhPmKQOMw0Yleh2pbyd11MoBxuzUU9gZfDWnogHx4GLQpBZSD0utIGG3OJQp87ViOrQe/Z+9+Lf/KTrmI+/nI4=")!
        )
        let session = try handshake.session()
        let sealed = try session.seal(Data("transport frame: initiator to responder".utf8))
        expect(
            sealed.base64EncodedString() == "a4n+3ewv7yRq+z+r3c0TQD2rTCE918BA03Hvb2Eue97JfHWzfpaY+rG8E9XPCPszhc/RWyGTzQ==",
            "Swift Noise transport drifted from Go"
        )
        let opened = try session.open(
            Data(base64Encoded: "89dlOS+4ArIFQZwq5otVTJJMfnR9z0CiRovKSqb9skHOZsyBhmfR0gCXqhuiALaQ9xeup+Z6dQ==")!
        )
        expect(opened == Data("transport frame: responder to initiator".utf8), "Swift Noise response transport drifted from Go")
    }

    private static func orbMotionSeparatesEnergyFromSpatialMovement() {
        expect(FortOrbMotion.shouldPulse(state: .working), "truthful Working state should pulse the Fort orb")
        expect(
            FortOrbMotion.allowsSpatialMotion(state: .working, reduceMotion: false),
            "Working activity should move the Fort orb when spatial motion is allowed"
        )
        expect(
            !FortOrbMotion.allowsSpatialMotion(state: .working, reduceMotion: true),
            "Reduce Motion must suppress Fort orb rotation, scale, and drift"
        )
        expect(!FortOrbMotion.shouldPulse(state: .idle), "idle Fort orb must not pulse")
        expect(
            !FortOrbMotion.allowsSpatialMotion(state: .idle, reduceMotion: false),
            "idle Fort orb must not move"
        )
    }

    private static func nativeOrbUsesRasterAndHonorsReduceMotion() throws {
        let appleRoot = appleRootURL()
        let styleSource = try String(
            contentsOf: appleRoot.appendingPathComponent("FortKit/Sources/FortKit/PrimaryChannelsStyle.swift"),
            encoding: .utf8
        )
        for required in [
            "@Environment(\\.accessibilityReduceMotion)",
            "FortOrbMotion.shouldPulse",
            "FortOrbMotion.allowsSpatialMotion",
            "Image(\"FortAgentOrb\")",
            ".rotationEffect",
            ".scaleEffect",
        ] {
            expect(styleSource.contains(required), "Phase 1 orb motion missing \(required)")
        }
        expect(!styleSource.contains("FortProjectState"), "Phase 1 orb still depends on legacy project state")
    }

    private static func iPhoneSimulatorSupportsDeterministicVisualQAHost() throws {
        let source = try String(
            contentsOf: appleRootURL().appendingPathComponent("iOS/GatewayCoordinator.swift"),
            encoding: .utf8
        )
        expect(
            source.contains("#if DEBUG && targetEnvironment(simulator)"),
            "iPhone direct-host QA path must stay DEBUG Simulator-only"
        )
        expect(source.contains("FORT_DIRECT_HOST_URL"), "visual QA cannot point the simulator at a deterministic Fort fixture")
    }

    private static func iPhonePhysicalReleaseUsesOnlyAuthenticatedRelay() throws {
        let appleRoot = appleRootURL()
        let app = try String(
            contentsOf: appleRoot.appendingPathComponent("iOS/FortApp.swift"),
            encoding: .utf8
        )
        let coordinator = try String(
            contentsOf: appleRoot.appendingPathComponent("iOS/GatewayCoordinator.swift"),
            encoding: .utf8
        )
        let clientSource = try String(
            contentsOf: appleRoot.appendingPathComponent("FortKit/Sources/FortKit/FortClient.swift"),
            encoding: .utf8
        )
        let physicalRelease = removingDebugSimulatorBlocks(app)
            + removingDebugSimulatorBlocks(coordinator)

        expect(app.contains("FortClient.gatewayOnly()"), "physical iPhone does not start with a fail-closed client")
        expect(coordinator.contains("client.disconnectGateway()"), "iPhone gateway sign-out does not clear transport fail-closed")
        expect(
            clientSource.contains("#if os(macOS) || (DEBUG && targetEnvironment(simulator))\n    public func useDirectHost"),
            "FortClient direct-host action is compiled into physical iPhone"
        )
        for forbidden in [
            "FORT_DIRECT_HOST_URL",
            "directHostEnabled",
            "useDirectHost(",
            "Section(\"Control-plane host\")",
            "Button(\"Use direct host\")",
            "127.0.0.1:4087",
        ] {
            expect(!physicalRelease.contains(forbidden), "physical iPhone Release exposes direct-host surface \(forbidden)")
        }

        let disconnected = FortClient.gatewayOnly()
        expect(disconnected.baseURL.scheme == "fort-gateway-required", "gateway-only client did not start inert")
        #if os(macOS)
        disconnected.useDirectHost(URL(string: "http://192.168.1.10:4087")!)
        disconnected.disconnectGateway()
        expect(disconnected.baseURL.scheme == "fort-gateway-required", "gateway disconnect fell back to LAN transport")
        #endif
    }

    private static func removingDebugSimulatorBlocks(_ source: String) -> String {
        let guardLine = "#if DEBUG && targetEnvironment(simulator)"
        var kept: [Substring] = []
        var depth = 0
        var seen = false
        for line in source.split(separator: "\n", omittingEmptySubsequences: false) {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if depth == 0 && trimmed == guardLine {
                depth = 1
                seen = true
                continue
            }
            if depth > 0 {
                if trimmed.hasPrefix("#if ") {
                    depth += 1
                } else if trimmed == "#endif" {
                    depth -= 1
                }
                continue
            }
            kept.append(line)
        }
        expect(seen && depth == 0, "iPhone DEBUG Simulator guard is missing or unbalanced")
        return kept.joined(separator: "\n")
    }

    private static func appleRootURL() -> URL {
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
    }
}

private final class ContractURLProtocol: URLProtocol {
    static var requests: [URLRequest] = []
    static var handler: ((URLRequest) -> (HTTPURLResponse, Data))?

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        Self.requests.append(request)
        guard let handler = Self.handler else { fatalError("ContractURLProtocol handler missing") }
        let (response, data) = handler(request)
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        if !data.isEmpty {
            client?.urlProtocol(self, didLoad: data)
        }
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}
}
