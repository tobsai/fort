import Foundation

#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

#if !MESSAGING_CONTRACT_STANDALONE
import FortKit
#endif

enum MessagingContractTests {
    static func run() async throws {
        try await modelRejectsGappedEventPageWithoutAdvancingCursor()
        try await modelRejectsEventPageMessageIDCollisionsTransactionally()
        try await modelRejectsReceiptMessageIDCollisionTransactionally()
        try await modelKeepsReplayCursorBeforeAnUnseenPeerEvent()
        try await modelRejectsReplayThatConflictsWithRetainedAcceptance()
        try await modelAggregatesDynamicChannelsAcrossExactMachineSources()
        try await modelRoutesSelectionAndSendThroughTheExactMachineChannel()
        try await conversationProjectionNeverRendersAnotherSelectedChannel()
        try await modelSurfacesUnknownDeliveryWithoutResendingAcceptance()
        try await lateAcceptedSendRetainsItsExactChannelOutcomeAfterSelectionChanges()
        try await lateInvalidSendReceiptFailsClosedWithoutOverwritingCurrentChannelError()
        try await modelPersistsHideAndProjectsOnlyPreviouslySeenUnavailableChannelsOffline()
        try await modelRetriesAnExactPinnedSourceAfterAnOfflineMachineSnapshot()
        try await modelFailsClosedWhenAConnectedMachineReturnsInvalidChannelIdentity()
        try await modelScopesCacheAndTreatsEmptyProcessRosterAsOffline()
        try await modelResetsProcessLocalTranscriptWhenExactSourceRosterBecomesEmpty()
        try await staleAccountRefreshCannotOverwriteTheCurrentAccountDirectory()
        try messagingSourceScopesStableEmailAcrossRenewalAndRejectsMalformedIdentity()
        try messagingSourceResolutionSurfacesRejectedMachinesWithoutTrustExpansion()
        try wireModelsDecodeTheMessagingProofContract()
        try await legacyModelRejectsCursorAheadPageWithoutAdvancingCursor()
        try await legacyModelRejectsEventPageMessageIDCollisionsTransactionally()
        try await legacyModelRejectsReceiptMessageIDCollisionTransactionally()
        try await legacyModelKeepsReplayCursorBeforeAnUnseenPeerEvent()
        try await clientUsesOnlyTheMessagingProofEndpoints()
        try await modelPresentsAndSendsThroughTheExactPeerConversation()
        try await modelClearsTranscriptWhenServingMachineChanges()
        try await modelReconcilesAReceiptAlreadyObservedByPolling()
        try await modelKeepsAcceptedReceiptWhenAnOlderPollCompletesLater()
    }

    private static func messagingSourceScopesStableEmailAcrossRenewalAndRejectsMalformedIdentity() throws {
        let publicKey = "86nFNj7PKVZC81MQ2j3/1YOsYiryw9jUK1csCZWnB3c="
        let fingerprint = RelayFingerprint.of(publicKey: Data(base64Encoded: publicKey)!)
        let machineJSON = #"{"machine_id":"machine:one","name":"Trusted Mac","fingerprint":"\#(fingerprint)","online":true,"public_key":"\#(publicKey)"}"#
        let machine = try JSONDecoder().decode(GatewayMachine.self, from: Data(machineJSON.utf8))

        func account(url: String, token: String) -> GatewayAccount {
            GatewayAccount(
                gatewayURL: URL(string: url)!,
                bearerToken: token,
                pinnedPublicKeys: [machine.machineID: publicKey]
            )
        }
        let first = try MessagingChannelSource(
            account: account(
                url: "https://fort-gateway.test",
                token: jwt(email: " Person@Example.com ", nonce: "first")
            ),
            machine: machine
        )
        let renewed = try MessagingChannelSource(
            account: account(
                url: "https://fort-gateway.test/native",
                token: jwt(email: "person@example.com", nonce: "renewed")
            ),
            machine: machine
        )
        expect(first.accountScope == renewed.accountScope, "token renewal changed the stable presentation cache scope")
        expect(first.transportRevision != renewed.transportRevision, "token renewal did not replace the retained gateway transport")

        let anotherAccount = try MessagingChannelSource(
            account: account(
                url: "https://fort-gateway.test",
                token: jwt(email: "another@example.com", nonce: "other")
            ),
            machine: machine
        )
        expect(first.accountScope != anotherAccount.accountScope, "different gateway emails shared a presentation cache scope")

        let anotherGateway = try MessagingChannelSource(
            account: account(
                url: "https://other-fort-gateway.test",
                token: jwt(email: "person@example.com", nonce: "other-gateway")
            ),
            machine: machine
        )
        expect(first.accountScope != anotherGateway.accountScope, "different gateways shared a presentation cache scope")

        do {
            _ = try MessagingChannelSource(
                account: account(url: "https://fort-gateway.test", token: "not-a-jwt"),
                machine: machine
            )
            fatalError("malformed native token created a Messaging Channel source")
        } catch MessagingChannelSourceError.invalidAccountIdentity {
            // Expected: presentation cache identity fails closed.
        }
    }

    private static func messagingSourceResolutionSurfacesRejectedMachinesWithoutTrustExpansion() throws {
        let trustedKey = Data(repeating: 1, count: 32).base64EncodedString()
        let untrustedKey = Data(repeating: 2, count: 32).base64EncodedString()
        let trustedFingerprint = RelayFingerprint.of(publicKey: Data(base64Encoded: trustedKey)!)
        let untrustedFingerprint = RelayFingerprint.of(publicKey: Data(base64Encoded: untrustedKey)!)
        let machinesJSON = #"""
        [{
          "machine_id":"machine:trusted",
          "name":"Trusted Mac",
          "fingerprint":"\#(trustedFingerprint)",
          "online":true,
          "public_key":"\#(trustedKey)"
        },{
          "machine_id":"machine:untrusted",
          "name":"Untrusted Mac",
          "fingerprint":"\#(untrustedFingerprint)",
          "online":true,
          "public_key":"\#(untrustedKey)"
        }]
        """#
        let machines = try JSONDecoder().decode([GatewayMachine].self, from: Data(machinesJSON.utf8))
        let account = GatewayAccount(
            gatewayURL: URL(string: "https://fort-gateway.test")!,
            bearerToken: jwt(email: "person@example.com", nonce: "source-resolution"),
            pinnedPublicKeys: ["machine:trusted": trustedKey]
        )

        let resolution = MessagingChannelSourceResolution(account: account, machines: machines)

        expect(resolution.sources.map(\.machineID) == ["machine:trusted"], "source resolution queried or trusted a rejected machine")
        expect(resolution.warning?.contains("1") == true, "rejected Messaging Channel source was silently discarded")

        let malformed = MessagingChannelSourceResolution(
            account: GatewayAccount(
                gatewayURL: URL(string: "https://fort-gateway.test")!,
                bearerToken: "not-a-jwt",
                pinnedPublicKeys: ["machine:trusted": trustedKey]
            ),
            machines: [machines[0]]
        )
        expect(malformed.sources.isEmpty, "malformed account identity produced a Messaging Channel source")
        expect(malformed.warning != nil, "malformed account identity was silently discarded")
    }

    private static func jwt(email: String, nonce: String) -> String {
        func base64URL(_ data: Data) -> String {
            data.base64EncodedString()
                .replacingOccurrences(of: "+", with: "-")
                .replacingOccurrences(of: "/", with: "_")
                .replacingOccurrences(of: "=", with: "")
        }
        let header = base64URL(Data(#"{"alg":"HS256","typ":"JWT"}"#.utf8))
        let payload = base64URL(Data(#"{"aud":"fort-native","email":"\#(email)","jti":"\#(nonce)"}"#.utf8))
        let signature = base64URL(Data("signature-\(nonce)".utf8))
        return "\(header).\(payload).\(signature)"
    }

    private static func wait(
        for semaphore: DispatchSemaphore,
        timeout: DispatchTime
    ) -> Bool {
        semaphore.wait(timeout: timeout) == .success
    }

    @MainActor
    private static func modelRejectsGappedEventPageWithoutAdvancingCursor() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var eventCall = 0
        var requestedAfters: [Int64] = []
        MessagingStubURLProtocol.handler = { request in
            let body: String
            switch request.url?.path {
            case "/api/messaging/channels":
                body = Fixtures.macBookChannels
            case "/api/messaging/conversations/conversation:aria:home/events":
                let components = URLComponents(
                    url: request.url!,
                    resolvingAgainstBaseURL: false
                )
                let value = components?.queryItems?.first(where: { $0.name == "after" })?.value
                requestedAfters.append(Int64(value ?? "") ?? -1)
                eventCall += 1
                switch eventCall {
                case 1: body = Fixtures.ariaSequenceOne
                case 2: body = Fixtures.ariaGappedSequenceThree
                default: body = Fixtures.ariaSequenceTwo
                }
            default:
                fatalError("unexpected gapped-page request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-gap.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        let source = MessagingChannelSource(
            machineID: "machine:macbook",
            machineName: "Tobias's MacBook Pro",
            client: FortClient(
                baseURL: URL(string: "https://macbook.test")!,
                session: URLSession(configuration: configuration)
            )
        )
        let model = MessagingChannelsModel(defaults: defaults)

        await model.refresh(sources: [source])
        guard let aria = model.visibleChannels.first else {
            fatalError("Aria was not projected for the gapped-page contract")
        }
        await model.select(channelID: aria.id)
        await model.pollSelected()

        expect(model.errorMessage != nil, "gapped event page was accepted")
        expect(model.messages.map(\.body) == ["First"], "gapped event page changed the transcript")

        await model.pollSelected()

        expect(requestedAfters == [0, 1, 1], "gapped event page advanced the server cursor")
        expect(model.messages.map(\.body) == ["First", "Second"], "valid replay after the gap was skipped")
    }

    @MainActor
    private static func modelRejectsEventPageMessageIDCollisionsTransactionally() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var eventCall = 0
        var requestedAfters: [Int64] = []
        MessagingStubURLProtocol.handler = { request in
            let body: String
            switch request.url?.path {
            case "/api/messaging/channels":
                body = Fixtures.macBookChannels
            case "/api/messaging/conversations/conversation:aria:home/events":
                let components = URLComponents(
                    url: request.url!,
                    resolvingAgainstBaseURL: false
                )
                let value = components?.queryItems?.first(where: { $0.name == "after" })?.value
                requestedAfters.append(Int64(value ?? "") ?? -1)
                eventCall += 1
                switch eventCall {
                case 1: body = Fixtures.ariaSequenceOne
                case 2: body = Fixtures.ariaDuplicateMessageIDsInPage
                case 3: body = Fixtures.ariaExistingMessageIDAtNewSequence
                default: body = Fixtures.ariaSequenceTwo
                }
            default:
                fatalError("unexpected dynamic message-id collision request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-id-collision.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        let source = MessagingChannelSource(
            machineID: "machine:macbook",
            machineName: "Tobias's MacBook Pro",
            client: FortClient(
                baseURL: URL(string: "https://macbook.test")!,
                session: URLSession(configuration: configuration)
            )
        )
        let model = MessagingChannelsModel(defaults: defaults)

        await model.refresh(sources: [source])
        guard let aria = model.visibleChannels.first else {
            fatalError("Aria was not projected for the message-id collision contract")
        }
        await model.select(channelID: aria.id)

        await model.pollSelected()
        expect(model.errorMessage != nil, "duplicate message IDs within one event page were accepted")
        expect(model.messages.map(\.body) == ["First"], "duplicate message IDs partially changed the transcript")

        await model.pollSelected()
        expect(model.errorMessage != nil, "an existing message ID was accepted at a new sequence")
        expect(model.messages.map(\.body) == ["First"], "existing message ID collision changed the transcript")

        await model.pollSelected()
        expect(requestedAfters == [0, 1, 1, 1], "message-ID collision advanced the dynamic cursor")
        expect(model.messages.map(\.body) == ["First", "Second"], "valid event after message-ID collisions was skipped")
    }

    @MainActor
    private static func modelRejectsReceiptMessageIDCollisionTransactionally() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var eventCall = 0
        var requestedAfters: [Int64] = []
        MessagingStubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let body: String
            switch (method, request.url?.path ?? "") {
            case ("GET", "/api/messaging/channels"):
                body = Fixtures.macBookChannels
            case ("GET", "/api/messaging/conversations/conversation:aria:home/events"):
                let components = URLComponents(
                    url: request.url!,
                    resolvingAgainstBaseURL: false
                )
                let value = components?.queryItems?.first(where: { $0.name == "after" })?.value
                requestedAfters.append(Int64(value ?? "") ?? -1)
                eventCall += 1
                body = eventCall == 1 ? Fixtures.ariaSequenceOne : Fixtures.ariaSequenceTwo
            case ("POST", "/api/messaging/conversations/conversation:aria:home/messages"):
                body = Fixtures.ariaReceiptReusesExistingMessageID
            default:
                fatalError("unexpected dynamic receipt message-id collision request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: method == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-receipt-id-collision.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        let source = MessagingChannelSource(
            machineID: "machine:macbook",
            machineName: "Tobias's MacBook Pro",
            client: FortClient(
                baseURL: URL(string: "https://macbook.test")!,
                session: URLSession(configuration: configuration)
            )
        )
        let model = MessagingChannelsModel(defaults: defaults)

        await model.refresh(sources: [source])
        guard let aria = model.visibleChannels.first else {
            fatalError("Aria was not projected for the receipt message-id collision contract")
        }
        await model.select(channelID: aria.id)

        let accepted = await model.send(text: "Collision")

        expect(!accepted, "receipt reused an existing immutable message ID at a new sequence")
        expect(model.errorMessage != nil, "receipt message-ID collision did not surface an error")
        expect(model.messages.map(\.body) == ["First"], "receipt message-ID collision partially changed the transcript")

        await model.pollSelected()
        expect(requestedAfters == [0, 1], "receipt message-ID collision advanced the dynamic cursor")
        expect(model.messages.map(\.body) == ["First", "Second"], "valid event after receipt collision was skipped")
    }

    @MainActor
    private static func modelKeepsReplayCursorBeforeAnUnseenPeerEvent() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var eventCall = 0
        var requestedAfters: [Int64] = []
        MessagingStubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let body: String
            switch (method, request.url?.path ?? "") {
            case ("GET", "/api/messaging/channels"):
                body = Fixtures.macBookChannels
            case ("GET", "/api/messaging/conversations/conversation:aria:home/events"):
                let components = URLComponents(
                    url: request.url!,
                    resolvingAgainstBaseURL: false
                )
                let value = components?.queryItems?.first(where: { $0.name == "after" })?.value
                requestedAfters.append(Int64(value ?? "") ?? -1)
                eventCall += 1
                body = eventCall == 1
                    ? Fixtures.ariaSequenceOne
                    : Fixtures.ariaEventsTwoAndThree
            case ("POST", "/api/messaging/conversations/conversation:aria:home/messages"):
                body = Fixtures.ariaReceiptSequenceThree
            default:
                fatalError("unexpected unseen-peer request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: method == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-unseen-peer.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        let source = MessagingChannelSource(
            machineID: "machine:macbook",
            machineName: "Tobias's MacBook Pro",
            client: FortClient(
                baseURL: URL(string: "https://macbook.test")!,
                session: URLSession(configuration: configuration)
            )
        )
        let model = MessagingChannelsModel(defaults: defaults)

        await model.refresh(sources: [source])
        guard let aria = model.visibleChannels.first else {
            fatalError("Aria was not projected for the unseen-peer contract")
        }
        await model.select(channelID: aria.id)
        let accepted = await model.send(text: "Human after peer")
        expect(accepted, "sequence-three Fort acceptance was rejected")

        await model.pollSelected()

        expect(requestedAfters == [0, 1], "send receipt skipped an unseen peer event")
        expect(
            model.messages.map(\.body) == ["First", "Unseen peer", "Human after peer"],
            "unseen peer event was not reconciled in order"
        )
    }

    @MainActor
    private static func modelRejectsReplayThatConflictsWithRetainedAcceptance() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var eventCall = 0
        MessagingStubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let body: String
            switch (method, request.url?.path ?? "") {
            case ("GET", "/api/messaging/channels"):
                body = Fixtures.macBookChannels
            case ("GET", "/api/messaging/conversations/conversation:aria:home/events"):
                eventCall += 1
                body = eventCall == 1
                    ? Fixtures.ariaSequenceOne
                    : Fixtures.ariaConflictingEventsTwoAndThree
            case ("POST", "/api/messaging/conversations/conversation:aria:home/messages"):
                body = Fixtures.ariaReceiptSequenceThree
            default:
                fatalError("unexpected conflicting-replay request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: method == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-conflicting-replay.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        let source = MessagingChannelSource(
            machineID: "machine:macbook",
            machineName: "Tobias's MacBook Pro",
            client: FortClient(
                baseURL: URL(string: "https://macbook.test")!,
                session: URLSession(configuration: configuration)
            )
        )
        let model = MessagingChannelsModel(defaults: defaults)

        await model.refresh(sources: [source])
        guard let aria = model.visibleChannels.first else {
            fatalError("Aria was not projected for the conflicting-replay contract")
        }
        await model.select(channelID: aria.id)
        let accepted = await model.send(text: "Human after peer")
        expect(accepted, "exact acceptance was rejected before conflicting replay")

        await model.pollSelected()

        expect(model.errorMessage != nil, "replay conflicting with exact acceptance was accepted")
        expect(
            model.messages.map(\.body) == ["First", "Human after peer"],
            "conflicting replay changed or duplicated retained acceptance"
        )
    }

    @MainActor
    private static func legacyModelRejectsCursorAheadPageWithoutAdvancingCursor() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var eventCall = 0
        var requestedAfters: [Int64] = []
        MessagingStubURLProtocol.handler = { request in
            let body: String
            switch request.url?.path {
            case "/api/messaging/peers":
                body = Fixtures.peers
            case "/api/messaging/conversations/conversation:lewis:home/events":
                let components = URLComponents(
                    url: request.url!,
                    resolvingAgainstBaseURL: false
                )
                let value = components?.queryItems?.first(where: { $0.name == "after" })?.value
                requestedAfters.append(Int64(value ?? "") ?? -1)
                eventCall += 1
                body = eventCall == 1
                    ? Fixtures.cursorAheadEmptyEvents
                    : Fixtures.lewisSequenceOne
            default:
                fatalError("unexpected cursor-ahead request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )
        let model = MessagingModel()

        await model.load(using: client)

        expect(model.errorMessage != nil, "cursor-ahead empty page was accepted")
        expect(model.messages.isEmpty, "cursor-ahead empty page changed the transcript")

        await model.poll(using: client)

        expect(requestedAfters == [0, 0], "cursor-ahead empty page advanced the legacy cursor")
        expect(model.messages.map(\.body) == ["Recovered"], "valid replay after cursor-ahead page was skipped")
    }

    @MainActor
    private static func legacyModelRejectsEventPageMessageIDCollisionsTransactionally() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var eventCall = 0
        var requestedAfters: [Int64] = []
        MessagingStubURLProtocol.handler = { request in
            let body: String
            switch request.url?.path {
            case "/api/messaging/peers":
                body = Fixtures.peers
            case "/api/messaging/conversations/conversation:lewis:home/events":
                let components = URLComponents(
                    url: request.url!,
                    resolvingAgainstBaseURL: false
                )
                let value = components?.queryItems?.first(where: { $0.name == "after" })?.value
                requestedAfters.append(Int64(value ?? "") ?? -1)
                eventCall += 1
                switch eventCall {
                case 1: body = Fixtures.lewisSequenceOne
                case 2: body = Fixtures.lewisDuplicateMessageIDsInPage
                case 3: body = Fixtures.lewisExistingMessageIDAtNewSequence
                default: body = Fixtures.lewisSequenceTwo
                }
            default:
                fatalError("unexpected legacy message-id collision request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )
        let model = MessagingModel()

        await model.load(using: client)

        await model.poll(using: client)
        expect(model.errorMessage != nil, "legacy page accepted duplicate message IDs")
        expect(model.messages.map(\.body) == ["Recovered"], "legacy duplicate IDs partially changed the transcript")

        await model.poll(using: client)
        expect(model.errorMessage != nil, "legacy page accepted an existing message ID at a new sequence")
        expect(model.messages.map(\.body) == ["Recovered"], "legacy existing-ID collision changed the transcript")

        await model.poll(using: client)
        expect(requestedAfters == [0, 1, 1, 1], "message-ID collision advanced the legacy cursor")
        expect(model.messages.map(\.body) == ["Recovered", "Second Lewis"], "legacy model skipped valid event after collisions")
    }

    @MainActor
    private static func legacyModelRejectsReceiptMessageIDCollisionTransactionally() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var eventCall = 0
        var requestedAfters: [Int64] = []
        MessagingStubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let body: String
            switch (method, request.url?.path ?? "") {
            case ("GET", "/api/messaging/peers"):
                body = Fixtures.peers
            case ("GET", "/api/messaging/conversations/conversation:lewis:home/events"):
                let components = URLComponents(
                    url: request.url!,
                    resolvingAgainstBaseURL: false
                )
                let value = components?.queryItems?.first(where: { $0.name == "after" })?.value
                requestedAfters.append(Int64(value ?? "") ?? -1)
                eventCall += 1
                body = eventCall == 1 ? Fixtures.lewisSequenceOne : Fixtures.lewisSequenceTwo
            case ("POST", "/api/messaging/conversations/conversation:lewis:home/messages"):
                body = Fixtures.lewisReceiptReusesExistingMessageID
            default:
                fatalError("unexpected legacy receipt message-id collision request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: method == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )
        let model = MessagingModel()

        await model.load(using: client)

        let accepted = await model.send(text: "Legacy collision", using: client)

        expect(!accepted, "legacy receipt reused an existing immutable message ID at a new sequence")
        expect(model.errorMessage != nil, "legacy receipt message-ID collision did not surface an error")
        expect(model.messages.map(\.body) == ["Recovered"], "legacy receipt collision partially changed the transcript")

        await model.poll(using: client)
        expect(requestedAfters == [0, 1], "receipt message-ID collision advanced the legacy cursor")
        expect(model.messages.map(\.body) == ["Recovered", "Second Lewis"], "legacy model skipped valid event after receipt collision")
    }

    @MainActor
    private static func legacyModelKeepsReplayCursorBeforeAnUnseenPeerEvent() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var eventCall = 0
        var requestedAfters: [Int64] = []
        MessagingStubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let body: String
            switch (method, request.url?.path ?? "") {
            case ("GET", "/api/messaging/peers"):
                body = Fixtures.peers
            case ("GET", "/api/messaging/conversations/conversation:lewis:home/events"):
                let components = URLComponents(
                    url: request.url!,
                    resolvingAgainstBaseURL: false
                )
                let value = components?.queryItems?.first(where: { $0.name == "after" })?.value
                requestedAfters.append(Int64(value ?? "") ?? -1)
                eventCall += 1
                body = eventCall == 1
                    ? Fixtures.lewisSequenceOne
                    : Fixtures.lewisEventsTwoAndThree
            case ("POST", "/api/messaging/conversations/conversation:lewis:home/messages"):
                body = Fixtures.lewisReceiptSequenceThree
            default:
                fatalError("unexpected legacy unseen-peer request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: method == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )
        let model = MessagingModel()

        await model.load(using: client)
        let accepted = await model.send(text: "Human after Lewis", using: client)
        expect(accepted, "legacy sequence-three Fort acceptance was rejected")

        await model.poll(using: client)

        expect(requestedAfters == [0, 1], "legacy send receipt skipped an unseen peer event")
        expect(
            model.messages.map(\.body) == ["Recovered", "Unseen Lewis", "Human after Lewis"],
            "legacy unseen peer event was not reconciled in order"
        )
    }

    @MainActor
    private static func staleAccountRefreshCannotOverwriteTheCurrentAccountDirectory() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        let accountAStarted = DispatchSemaphore(value: 0)
        var completeAccountA: ((HTTPURLResponse, Data) -> Void)?
        MessagingStubURLProtocol.deferredHandler = { request, complete in
            guard request.url?.host == "account-a.test",
                  request.url?.path == "/api/messaging/channels"
            else { return false }
            completeAccountA = complete
            accountAStarted.signal()
            return true
        }
        defer { MessagingStubURLProtocol.deferredHandler = nil }
        MessagingStubURLProtocol.handler = { request in
            guard request.url?.host == "account-b.test" else {
                fatalError("unexpected account refresh")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data("[]".utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-account-race.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        func source(host: String, scope: String) -> MessagingChannelSource {
            MessagingChannelSource(
                machineID: "machine:macbook",
                machineName: "Tobias's MacBook Pro",
                client: FortClient(
                    baseURL: URL(string: "https://\(host)")!,
                    session: URLSession(configuration: configuration)
                ),
                accountScope: scope
            )
        }
        let model = MessagingChannelsModel(defaults: defaults)

        let staleRefresh = Task {
            await model.refresh(sources: [source(host: "account-a.test", scope: "account:a")])
        }
        let requestWasHeld = await Task.detached {
            wait(for: accountAStarted, timeout: .now() + 5)
        }.value
        expect(requestWasHeld, "account A refresh did not reach the deterministic hold point")

        await model.refresh(sources: [source(host: "account-b.test", scope: "account:b")])
        guard let completeAccountA else { fatalError("account A response was not held") }
        let staleResponse = HTTPURLResponse(
            url: URL(string: "https://account-a.test/api/messaging/channels")!,
            statusCode: 200,
            httpVersion: "HTTP/1.1",
            headerFields: ["Content-Type": "application/json"]
        )!
        completeAccountA(staleResponse, Data(Fixtures.macBookChannels.utf8))
        await staleRefresh.value

        expect(model.visibleChannels.isEmpty, "late account A response overwrote account B's authoritative directory")
        expect(model.hiddenChannels.isEmpty, "late account A response leaked hidden metadata into account B")
    }

    @MainActor
    private static func modelResetsProcessLocalTranscriptWhenExactSourceRosterBecomesEmpty() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var directory = Fixtures.macBookChannels
        var eventPage = Fixtures.ariaEventsBeforeProcessRestart
        var requestedAfters: [Int64] = []
        MessagingStubURLProtocol.handler = { request in
            let body: String
            switch request.url?.path {
            case "/api/messaging/channels":
                body = directory
            case "/api/messaging/conversations/conversation:aria:home/events":
                let components = URLComponents(
                    url: request.url!,
                    resolvingAgainstBaseURL: false
                )
                let value = components?.queryItems?.first(where: { $0.name == "after" })?.value
                requestedAfters.append(Int64(value ?? "") ?? -1)
                body = eventPage
            default:
                fatalError("unexpected process-restart request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-process-restart.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        let source = MessagingChannelSource(
            machineID: "machine:macbook",
            machineName: "Tobias's MacBook Pro",
            client: FortClient(
                baseURL: URL(string: "https://macbook.test")!,
                session: URLSession(configuration: configuration)
            )
        )
        let model = MessagingChannelsModel(defaults: defaults)

        await model.refresh(sources: [source])
        guard let aria = model.visibleChannels.first else {
            fatalError("Aria was not projected before the process restart")
        }
        await model.select(channelID: aria.id)
        expect(model.messages.map(\.body) == ["Before process restart"], "initial process transcript did not load")

        directory = "[]"
        await model.refresh(sources: [source])
        expect(model.visibleChannels.first?.state == .offline, "empty process roster did not retain the cached channel Offline")
        expect(model.messages.isEmpty, "empty process roster retained a stale process-local transcript")

        directory = Fixtures.macBookChannels
        eventPage = Fixtures.ariaEventsAfterProcessRestart
        await model.refresh(sources: [source])
        await model.pollSelected()

        expect(requestedAfters == [0, 0], "reconnected process reused a cursor from the previous process")
        expect(model.messages.map(\.body) == ["After process restart"], "reconnected process skipped its restarted event sequence")
    }

    @MainActor
    private static func modelScopesCacheAndTreatsEmptyProcessRosterAsOffline() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var response: Result<String, Error> = .success(Fixtures.macBookChannels)
        MessagingStubURLProtocol.handler = { request in
            let body = try response.get()
            let http = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (http, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-account-scope.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        func source(accountScope: String) -> MessagingChannelSource {
            MessagingChannelSource(
                machineID: "machine:macbook",
                machineName: "Tobias's MacBook Pro",
                client: FortClient(
                    baseURL: URL(string: "https://macbook.test")!,
                    session: URLSession(configuration: configuration)
                ),
                accountScope: accountScope
            )
        }

        let firstModel = MessagingChannelsModel(defaults: defaults)
        await firstModel.refresh(sources: [source(accountScope: "account:a")])
        expect(firstModel.visibleChannels.map(\.displayName) == ["Aria"], "account A did not cache its channel")

        response = .failure(URLError(.cannotConnectToHost))
        let reopenedModel = MessagingChannelsModel(defaults: defaults)
        await reopenedModel.refresh(sources: [source(accountScope: "account:a")])
        expect(reopenedModel.visibleChannels.map(\.displayName) == ["Aria"], "fetch failure discarded account A's last-known channel")
        expect(reopenedModel.visibleChannels[0].state == .offline, "fetch failure did not mark the last-known channel Offline")

        response = .success("[]")
        await reopenedModel.refresh(sources: [source(accountScope: "account:a")])
        expect(reopenedModel.visibleChannels.map(\.displayName) == ["Aria"], "empty process-local roster erased the previously seen channel")
        expect(reopenedModel.visibleChannels[0].state == .offline, "empty process-local roster did not retain the channel Offline")

        response = .success(Fixtures.macBookChannels)
        await reopenedModel.refresh(sources: [source(accountScope: "account:a")])
        guard let accountAChannel = reopenedModel.visibleChannels.first else {
            fatalError("account A channel was not restored before the account switch")
        }
        reopenedModel.setHidden(channelID: accountAChannel.id, hidden: true)
        expect(reopenedModel.hiddenChannels.map(\.displayName) == ["Aria"], "account A Hide preference was not recorded")
        response = .failure(URLError(.cannotConnectToHost))
        await reopenedModel.refresh(sources: [source(accountScope: "account:b")])
        expect(reopenedModel.visibleChannels.isEmpty, "account B projected account A's cached Bot metadata")
        expect(reopenedModel.hiddenChannels.isEmpty, "account B inherited account A's hidden channel metadata")
    }

    @MainActor
    private static func modelFailsClosedWhenAConnectedMachineReturnsInvalidChannelIdentity() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var responseBody = Fixtures.macBookChannels
        MessagingStubURLProtocol.handler = { request in
            guard request.url?.path == "/api/messaging/channels" else {
                fatalError("unexpected invalid-projection request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(responseBody.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-invalid.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        let source = MessagingChannelSource(
            machineID: "machine:macbook",
            machineName: "Tobias's MacBook Pro",
            client: FortClient(
                baseURL: URL(string: "https://macbook.test")!,
                session: URLSession(configuration: configuration)
            )
        )
        let model = MessagingChannelsModel(defaults: defaults)
        await model.refresh(sources: [source])
        expect(model.visibleChannels.map(\.displayName) == ["Aria"], "valid initial channel did not load")

        responseBody = Fixtures.invalidMacBookChannels
        await model.refresh(sources: [source])

        expect(model.visibleChannels.map(\.displayName) == ["Aria"], "invalid projection was treated as explicit deregistration")
        expect(model.visibleChannels[0].state == .offline, "invalid projection left the stale channel Connected")

        responseBody = Fixtures.macBookChannels
        await model.refresh(sources: [source])
        responseBody = Fixtures.mismatchedMachineChannels
        await model.refresh(sources: [source])
        expect(model.visibleChannels.map(\.displayName) == ["Aria"], "machine-label mismatch removed the last-known exact channel")
        expect(model.visibleChannels[0].machineName == "Tobias's MacBook Pro", "daemon row relabelled the already-pinned owning machine")
        expect(model.visibleChannels[0].state == .offline, "machine-label mismatch did not fail closed")
    }

    @MainActor
    private static func modelPersistsHideAndProjectsOnlyPreviouslySeenUnavailableChannelsOffline() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var machinesAreReachable = true
        MessagingStubURLProtocol.handler = { request in
            guard request.httpMethod == "GET",
                  request.url?.path == "/api/messaging/channels"
            else { fatalError("Hide attempted a server mutation") }
            guard machinesAreReachable else {
                throw URLError(.cannotConnectToHost)
            }
            guard request.url?.host == "macbook.test" else {
                fatalError("unexpected first-seen channel source")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(Fixtures.macBookChannels.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-hide.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        let knownSource = MessagingChannelSource(
            machineID: "machine:macbook",
            machineName: "Tobias's MacBook Pro",
            client: FortClient(
                baseURL: URL(string: "https://macbook.test")!,
                session: URLSession(configuration: configuration)
            )
        )

        let firstModel = MessagingChannelsModel(defaults: defaults)
        await firstModel.refresh(sources: [knownSource])
        guard let aria = firstModel.visibleChannels.first else {
            fatalError("connected Aria channel was not cached")
        }
        firstModel.setHidden(channelID: aria.id, hidden: true)
        expect(firstModel.visibleChannels.isEmpty, "Hide left Aria in the visible directory")
        expect(firstModel.hiddenChannels.map(\.displayName) == ["Aria"], "Hide removed the channel instead of preserving it")

        machinesAreReachable = false
        let neverSeenSource = MessagingChannelSource(
            machineID: "machine:never-seen",
            machineName: "Never Seen Mac",
            client: FortClient(
                baseURL: URL(string: "https://never-seen.test")!,
                session: URLSession(configuration: configuration)
            )
        )
        let reopenedModel = MessagingChannelsModel(defaults: defaults)
        await reopenedModel.refresh(sources: [knownSource, neverSeenSource])

        expect(reopenedModel.visibleChannels.isEmpty, "reopening forgot the local Hide preference")
        expect(reopenedModel.hiddenChannels.count == 1, "unreachable discovery invented a never-seen channel")
        expect(reopenedModel.hiddenChannels[0].displayName == "Aria", "offline cache changed the exact Hermes Bot name")
        expect(reopenedModel.hiddenChannels[0].state == .offline, "previously seen unreachable channel was not marked Offline")
        reopenedModel.setHidden(channelID: aria.id, hidden: false)
        expect(reopenedModel.visibleChannels.map(\.displayName) == ["Aria"], "Unhide did not restore the cached channel")
        expect(MessagingStubURLProtocol.requests.allSatisfy { $0.httpMethod == "GET" }, "Hide or Unhide mutated Hermes/Fort server state")
    }

    @MainActor
    private static func modelRetriesAnExactPinnedSourceAfterAnOfflineMachineSnapshot() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        MessagingStubURLProtocol.handler = { request in
            guard request.httpMethod == "GET",
                  request.url?.host == "mac-mini.test",
                  request.url?.path == "/api/messaging/channels"
            else { fatalError("unexpected offline-snapshot recovery request") }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(Fixtures.macMiniChannels.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-offline-snapshot.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        let source = MessagingChannelSource(
            machineID: "machine:mac-mini",
            machineName: "Talos Mac mini",
            client: FortClient(
                baseURL: URL(string: "https://mac-mini.test")!,
                session: URLSession(configuration: configuration)
            ),
            isReachable: false
        )
        let model = MessagingChannelsModel(defaults: defaults)

        await model.refresh(sources: [source])

        expect(MessagingStubURLProtocol.requests.count == 1, "offline GatewayMachine snapshot permanently skipped the exact pinned source")
        expect(model.visibleChannels.map(\.displayName) == ["Pascal"], "recovered exact source did not project its Hermes channel")
        expect(model.visibleChannels[0].state == .connected, "successful recovery remained stuck Offline")
    }

    @MainActor
    private static func conversationProjectionNeverRendersAnotherSelectedChannel() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        MessagingStubURLProtocol.handler = { request in
            let body: String
            switch (request.url?.host, request.url?.path) {
            case ("macbook.test", "/api/messaging/channels"):
                body = Fixtures.macBookChannels
            case ("mac-mini.test", "/api/messaging/channels"):
                body = Fixtures.macMiniChannels
            case ("macbook.test", "/api/messaging/conversations/conversation:aria:home/events"):
                body = Fixtures.ariaEvents
            case ("mac-mini.test", "/api/messaging/conversations/conversation:pascal:home/events"):
                body = Fixtures.pascalEvents
            default:
                fatalError("unexpected channel-presentation request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-channel-presentation.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        func source(machineID: String, machineName: String, host: String) -> MessagingChannelSource {
            MessagingChannelSource(
                machineID: machineID,
                machineName: machineName,
                client: FortClient(
                    baseURL: URL(string: "https://\(host)")!,
                    session: URLSession(configuration: configuration)
                )
            )
        }
        let model = MessagingChannelsModel(defaults: defaults)
        await model.refresh(sources: [
            source(machineID: "machine:macbook", machineName: "Tobias's MacBook Pro", host: "macbook.test"),
            source(machineID: "machine:mac-mini", machineName: "Talos Mac mini", host: "mac-mini.test"),
        ])
        guard let aria = model.visibleChannels.first(where: { $0.displayName == "Aria" }),
              let pascal = model.visibleChannels.first(where: { $0.displayName == "Pascal" })
        else { fatalError("channel-presentation fixtures were not projected") }

        await model.select(channelID: pascal.id)
        model.errorMessage = "Pascal-only warning"

        expect(model.conversationMessages(channelID: aria.id).isEmpty, "Aria rendered Pascal's transcript before Aria selection completed")
        expect(model.conversationError(channelID: aria.id) == nil, "Aria rendered Pascal's error before Aria selection completed")

        await model.select(channelID: aria.id)

        expect(model.conversationMessages(channelID: aria.id).isEmpty, "Aria inherited Pascal's transcript after selection")
        expect(model.conversationError(channelID: aria.id) == nil, "Aria inherited Pascal's error after selection")
        expect(model.conversationMessages(channelID: pascal.id).isEmpty, "Pascal transcript remained renderable while Aria was selected")
    }

    @MainActor
    private static func modelRoutesSelectionAndSendThroughTheExactMachineChannel() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        MessagingStubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let path = request.url?.path ?? ""
            let body: String
            switch (request.url?.host, method, path) {
            case ("macbook.test", "GET", "/api/messaging/channels"):
                body = Fixtures.macBookChannels
            case ("mac-mini.test", "GET", "/api/messaging/channels"):
                body = Fixtures.macMiniChannels
            case ("mac-mini.test", "GET", "/api/messaging/conversations/conversation:pascal:home/events"):
                body = Fixtures.pascalEvents
            case ("mac-mini.test", "POST", "/api/messaging/conversations/conversation:pascal:home/messages"):
                body = Fixtures.pascalReceipt
            default:
                fatalError("unexpected exact-route request: \(request.url?.absoluteString ?? "")")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: method == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-route.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        let model = MessagingChannelsModel(defaults: defaults)
        let sources = [
            MessagingChannelSource(
                machineID: "machine:macbook",
                machineName: "Tobias's MacBook Pro",
                client: FortClient(
                    baseURL: URL(string: "https://macbook.test")!,
                    session: URLSession(configuration: configuration)
                )
            ),
            MessagingChannelSource(
                machineID: "machine:mac-mini",
                machineName: "Talos Mac mini",
                client: FortClient(
                    baseURL: URL(string: "https://mac-mini.test")!,
                    session: URLSession(configuration: configuration)
                )
            ),
        ]
        await model.refresh(sources: sources)
        guard let pascal = model.visibleChannels.first(where: { $0.displayName == "Pascal" }) else {
            fatalError("Pascal channel was not projected")
        }

        await model.select(channelID: pascal.id)
        expect(model.selectedChannel?.id == pascal.id, "selection substituted a different channel")
        expect(model.messages.map(\.body) == ["Ready from Pascal"], "selection loaded a different transcript")

        let sent = await model.send(text: "  Hello Pascal  ")
        expect(sent, "exact Pascal send was not accepted")
        let posts = MessagingStubURLProtocol.requests.filter { $0.httpMethod == "POST" }
        expect(posts.count == 1, "send dispatched more than once")
        expect(posts[0].url?.host == "mac-mini.test", "send fell back to the wrong machine")
        expect(posts[0].url?.path == "/api/messaging/conversations/conversation:pascal:home/messages", "send changed the exact Conversation")
    }

    @MainActor
    private static func modelSurfacesUnknownDeliveryWithoutResendingAcceptance() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        MessagingStubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let path = request.url?.path ?? ""
            let body: String
            switch (method, path) {
            case ("GET", "/api/messaging/channels"):
                body = Fixtures.macMiniChannels
            case ("GET", "/api/messaging/conversations/conversation:pascal:home/events"):
                body = Fixtures.pascalEvents
            case ("POST", "/api/messaging/conversations/conversation:pascal:home/messages"):
                body = Fixtures.pascalUnknownReceipt
            default:
                fatalError("unexpected unknown-delivery request: \(method) \(path)")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: method == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-unknown.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        let model = MessagingChannelsModel(defaults: defaults)
        let source = MessagingChannelSource(
            machineID: "machine:mac-mini",
            machineName: "Talos Mac mini",
            client: FortClient(
                baseURL: URL(string: "https://mac-mini.test")!,
                session: URLSession(configuration: configuration)
            )
        )

        await model.refresh(sources: [source])
        guard let pascal = model.visibleChannels.first else {
            fatalError("Pascal channel was not projected")
        }
        await model.select(channelID: pascal.id)

        let accepted = await model.send(text: "One ambiguous message")

        expect(accepted, "an accepted message was treated as safe to resubmit")
        expect(model.messages.last?.id == "message:human:pascal:unknown", "unknown delivery hid the Fort acceptance")
        guard let notice = model.deliveryNotices.first else {
            fatalError("unknown Hermes delivery did not create a transcript notice")
        }
        expect(notice.channelID == pascal.id, "unknown delivery notice lost the exact channel")
        expect(notice.messageID == "message:human:pascal:unknown", "unknown delivery notice lost the accepted message identity")
        expect(notice.marker == "Delivery unknown · Fort will not resend", "unknown delivery marker did not state the no-resend guarantee")
        let persistedValues = defaults.dictionaryRepresentation().values.compactMap { $0 as? Data }
        expect(
            persistedValues.allSatisfy { !String(decoding: $0, as: UTF8.self).contains("One ambiguous message") },
            "unknown delivery persisted the private submitted message body"
        )

        await model.refresh(sources: [source])
        await model.pollSelected()
        expect(model.deliveryNotices == [notice], "unrelated refresh or poll cleared the unknown delivery notice")

        let reopened = MessagingChannelsModel(defaults: defaults)
        await reopened.refresh(sources: [source])
        await reopened.select(channelID: pascal.id)
        expect(reopened.deliveryNotices == [notice], "account-scoped unknown delivery did not survive reopening")
        expect(reopened.messages.contains(where: { $0.id == notice.messageID }), "reopened transcript lost the accepted unknown-delivery outcome")
        expect(reopened.messages.contains(where: { $0.body == notice.marker }), "reopened transcript did not render the persisted no-resend outcome marker")
        expect(!reopened.messages.contains(where: { $0.body == "One ambiguous message" }), "reopened transcript reconstructed the private submitted message body")
        let posts = MessagingStubURLProtocol.requests.filter { $0.httpMethod == "POST" }
        expect(posts.count == 1, "unknown delivery caused an automatic redispatch")
    }

    @MainActor
    private static func lateAcceptedSendRetainsItsExactChannelOutcomeAfterSelectionChanges() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        let postStarted = DispatchSemaphore(value: 0)
        var completePost: ((HTTPURLResponse, Data) -> Void)?
        MessagingStubURLProtocol.deferredHandler = { request, complete in
            guard request.httpMethod == "POST",
                  request.url?.host == "mac-mini.test",
                  request.url?.path == "/api/messaging/conversations/conversation:pascal:home/messages"
            else { return false }
            completePost = complete
            postStarted.signal()
            return true
        }
        defer { MessagingStubURLProtocol.deferredHandler = nil }
        MessagingStubURLProtocol.handler = { request in
            let body: String
            switch (request.url?.host, request.url?.path) {
            case ("macbook.test", "/api/messaging/channels"):
                body = Fixtures.macBookChannels
            case ("mac-mini.test", "/api/messaging/channels"):
                body = Fixtures.macMiniChannels
            case ("macbook.test", "/api/messaging/conversations/conversation:aria:home/events"):
                body = Fixtures.ariaEvents
            case ("mac-mini.test", "/api/messaging/conversations/conversation:pascal:home/events"):
                body = Fixtures.pascalEvents
            default:
                fatalError("unexpected late-acceptance request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-late-acceptance.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        func source(machineID: String, machineName: String, host: String) -> MessagingChannelSource {
            MessagingChannelSource(
                machineID: machineID,
                machineName: machineName,
                client: FortClient(
                    baseURL: URL(string: "https://\(host)")!,
                    session: URLSession(configuration: configuration)
                )
            )
        }
        let sources = [
            source(machineID: "machine:macbook", machineName: "Tobias's MacBook Pro", host: "macbook.test"),
            source(machineID: "machine:mac-mini", machineName: "Talos Mac mini", host: "mac-mini.test"),
        ]
        let model = MessagingChannelsModel(defaults: defaults)
        await model.refresh(sources: sources)
        guard let aria = model.visibleChannels.first(where: { $0.displayName == "Aria" }),
              let pascal = model.visibleChannels.first(where: { $0.displayName == "Pascal" })
        else { fatalError("late-acceptance channels were not projected") }

        await model.select(channelID: pascal.id)
        let lateSend = Task { await model.send(text: "One ambiguous message") }
        let requestWasHeld = await Task.detached {
            wait(for: postStarted, timeout: .now() + 5)
        }.value
        expect(requestWasHeld, "Pascal send did not reach the deterministic hold point")

        await model.select(channelID: aria.id)
        model.errorMessage = "Current Aria warning"
        guard let completeUnknown = completePost else { fatalError("Pascal post response was not retained") }
        let response = HTTPURLResponse(
            url: URL(string: "https://mac-mini.test/api/messaging/conversations/conversation:pascal:home/messages")!,
            statusCode: 202,
            httpVersion: "HTTP/1.1",
            headerFields: ["Content-Type": "application/json"]
        )!
        completeUnknown(response, Data(Fixtures.pascalUnknownReceipt.utf8))

        let accepted = await lateSend.value
        expect(accepted, "late exact acceptance was reported as safe to retry")
        expect(model.selectedChannel?.id == aria.id, "late Pascal acceptance changed the selected channel")
        expect(model.errorMessage == "Current Aria warning", "late Pascal acceptance overwrote Aria's current error")
        expect(
            model.deliveryNotices.contains(where: {
                $0.channelID == pascal.id && $0.messageID == "message:human:pascal:unknown"
            }),
            "late acceptance did not retain its unknown-delivery outcome for Pascal"
        )
        await model.select(channelID: pascal.id)
        expect(
            model.messages.contains(where: { $0.id == "message:human:pascal:unknown" }),
            "returning to Pascal did not restore the late accepted message"
        )

        let latePending = Task { await model.send(text: "Late pending message") }
        let pendingRequestWasHeld = await Task.detached {
            wait(for: postStarted, timeout: .now() + 5)
        }.value
        expect(pendingRequestWasHeld, "pending Pascal send did not reach the deterministic hold point")
        await model.select(channelID: aria.id)
        guard let completePending = completePost else {
            fatalError("pending Pascal post response was not retained")
        }
        completePending(response, Data(Fixtures.pascalLatePendingReceipt.utf8))
        let pendingAccepted = await latePending.value
        expect(pendingAccepted, "late pending acceptance was reported as safe to retry")
        await model.select(channelID: pascal.id)
        expect(
            model.messages.contains(where: { $0.id == "message:human:pascal:late-pending" }),
            "late pending acceptance was not retained in Pascal's exact transcript"
        )
    }

    @MainActor
    private static func lateInvalidSendReceiptFailsClosedWithoutOverwritingCurrentChannelError() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        let postStarted = DispatchSemaphore(value: 0)
        var completePost: ((HTTPURLResponse, Data) -> Void)?
        MessagingStubURLProtocol.deferredHandler = { request, complete in
            guard request.httpMethod == "POST",
                  request.url?.host == "mac-mini.test",
                  request.url?.path == "/api/messaging/conversations/conversation:pascal:home/messages"
            else { return false }
            completePost = complete
            postStarted.signal()
            return true
        }
        defer { MessagingStubURLProtocol.deferredHandler = nil }
        MessagingStubURLProtocol.handler = { request in
            let body: String
            switch (request.url?.host, request.url?.path) {
            case ("macbook.test", "/api/messaging/channels"):
                body = Fixtures.macBookChannels
            case ("mac-mini.test", "/api/messaging/channels"):
                body = Fixtures.macMiniChannels
            case ("macbook.test", "/api/messaging/conversations/conversation:aria:home/events"):
                body = Fixtures.ariaEvents
            case ("mac-mini.test", "/api/messaging/conversations/conversation:pascal:home/events"):
                body = Fixtures.pascalEvents
            default:
                fatalError("unexpected late-invalid-receipt request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-late-invalid.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        func source(machineID: String, machineName: String, host: String) -> MessagingChannelSource {
            MessagingChannelSource(
                machineID: machineID,
                machineName: machineName,
                client: FortClient(
                    baseURL: URL(string: "https://\(host)")!,
                    session: URLSession(configuration: configuration)
                )
            )
        }
        let sources = [
            source(machineID: "machine:macbook", machineName: "Tobias's MacBook Pro", host: "macbook.test"),
            source(machineID: "machine:mac-mini", machineName: "Talos Mac mini", host: "mac-mini.test"),
        ]
        let model = MessagingChannelsModel(defaults: defaults)
        await model.refresh(sources: sources)
        guard let aria = model.visibleChannels.first(where: { $0.displayName == "Aria" }),
              let pascal = model.visibleChannels.first(where: { $0.displayName == "Pascal" })
        else { fatalError("late-invalid channels were not projected") }

        await model.select(channelID: pascal.id)
        let lateSend = Task { await model.send(text: "Invalid ambiguous response") }
        let requestWasHeld = await Task.detached {
            wait(for: postStarted, timeout: .now() + 5)
        }.value
        expect(requestWasHeld, "invalid Pascal receipt did not reach the deterministic hold point")

        await model.select(channelID: aria.id)
        model.errorMessage = "Current Aria warning"
        guard let completePost else { fatalError("invalid Pascal post response was not retained") }
        let response = HTTPURLResponse(
            url: URL(string: "https://mac-mini.test/api/messaging/conversations/conversation:pascal:home/messages")!,
            statusCode: 202,
            httpVersion: "HTTP/1.1",
            headerFields: ["Content-Type": "application/json"]
        )!
        completePost(response, Data(Fixtures.pascalInvalidUnknownReceipt.utf8))

        let accepted = await lateSend.value
        expect(!accepted, "late invalid receipt bypassed delivery-state validation")
        expect(model.deliveryNotices.isEmpty, "late invalid receipt created unknown-delivery evidence")
        expect(model.errorMessage == "Current Aria warning", "late Pascal receipt error overwrote Aria's current error")
    }

    @MainActor
    private static func modelAggregatesDynamicChannelsAcrossExactMachineSources() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        MessagingStubURLProtocol.handler = { request in
            guard request.httpMethod == "GET",
                  request.url?.path == "/api/messaging/channels"
            else {
                fatalError("unexpected dynamic-channel request")
            }
            let body: String
            switch request.url?.host {
            case "macbook.test":
                body = Fixtures.macBookChannels
            case "mac-mini.test":
                body = Fixtures.macMiniChannels
            default:
                fatalError("unexpected dynamic-channel machine")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let defaultsName = "FortKitContractChecks.messaging-directory.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: defaultsName)!
        defer { defaults.removePersistentDomain(forName: defaultsName) }
        let model = MessagingChannelsModel(defaults: defaults)
        let sources = [
            MessagingChannelSource(
                machineID: "machine:macbook",
                machineName: "Tobias's MacBook Pro",
                client: FortClient(
                    baseURL: URL(string: "https://macbook.test")!,
                    session: URLSession(configuration: configuration)
                )
            ),
            MessagingChannelSource(
                machineID: "machine:mac-mini",
                machineName: "Talos Mac mini",
                client: FortClient(
                    baseURL: URL(string: "https://mac-mini.test")!,
                    session: URLSession(configuration: configuration)
                )
            ),
        ]

        await model.refresh(sources: sources)

        expect(model.visibleChannels.map(\.displayName) == ["Aria", "Pascal"], "directory did not use exact Hermes Bot names")
        expect(model.visibleChannels.map(\.machineName) == ["Tobias's MacBook Pro", "Talos Mac mini"], "directory did not retain each owning machine")
        expect(Set(model.visibleChannels.map(\.id)).count == 2, "directory merged channels from different machines")
        expect(model.visibleChannels.allSatisfy { $0.subtitle.hasPrefix("Hermes · ") }, "directory did not present Hermes as channel kind")
        let ariaID = model.visibleChannels[0].id
        expect(model.channel(channelID: ariaID)?.displayName == "Aria", "exact destination lookup substituted another selected channel")
    }

    @MainActor
    private static func modelClearsTranscriptWhenServingMachineChanges() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        var peers = Fixtures.peers
        var events = Fixtures.events
        MessagingStubURLProtocol.handler = { request in
            let body: String
            switch request.url?.path ?? "" {
            case "/api/messaging/peers":
                body = peers
            case "/api/messaging/conversations/conversation:lewis:home/events":
                body = events
            default:
                fatalError("unexpected machine-switch request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )
        let model = MessagingModel()

        await model.load(using: client)
        expect(model.messages.count == 2, "first serving machine did not load its transcript")

        peers = Fixtures.peersOnMacBook
        events = Fixtures.emptyEvents
        await model.load(using: client)

        expect(model.peer?.machineName == "tobiass.macbook.pro.lan", "model did not adopt the new serving machine")
        expect(model.messages.isEmpty, "new serving machine inherited the previous machine transcript")
    }

    private static func wireModelsDecodeTheMessagingProofContract() throws {
        let peers = try JSONDecoder().decode(
            [MessagingPeer].self,
            from: Data(Fixtures.peers.utf8)
        )
        expect(peers.count == 1, "messaging peers did not decode as a top-level list")
        expect(peers[0].id == "peer:hermes:lewis", "peer identity changed")
        expect(peers[0].displayName == "Lewis", "peer title is not the exact bot name")
        expect(peers[0].machineName == "taloss.mac.mini.lan", "peer lost its exact serving machine")
        expect(peers[0].headerSubtitle == "taloss.mac.mini.lan · Connected", "peer header did not present machine and state")
        expect(peers[0].conversationID == "conversation:lewis:home", "peer lost its Home Conversation")
        expect(peers[0].state == .connected, "connected peer decoded as unavailable")
        expect(peers[0].reason == nil, "connected peer invented an unavailable reason")

        let page = try JSONDecoder().decode(
            MessagingEventsPage.self,
            from: Data(Fixtures.events.utf8)
        )
        expect(page.conversationID == peers[0].conversationID, "event page changed Conversation identity")
        expect(page.events.map(\.sequence) == [1, 2], "events lost their server order")
        expect(page.nextAfter == 2, "event cursor did not remain server-owned")
        expect(page.events[0].message.authorKind == .human, "human message attribution changed")
        expect(page.events[1].message.authorKind == .peer, "Hermes message attribution changed")
        expect(page.events[1].message.body == "Hello from Lewis", "Hermes message body changed")
        expect(page.events[1].message.inReplyToMessageID == "message:human:1", "Hermes reply attribution changed")

        let receipt = try JSONDecoder().decode(
            MessagingMessageReceipt.self,
            from: Data(Fixtures.receipt.utf8)
        )
        expect(receipt.acceptedSequence == 3, "acceptance receipt lost its event sequence")
        expect(receipt.message.id == "message:human:2", "acceptance receipt lost its message")
        expect(receipt.deliveryState == .pending, "acceptance receipt overclaimed downstream delivery")
        expect(receipt.deliveryCode == nil, "pending acceptance invented a delivery failure")
    }

    private static func clientUsesOnlyTheMessagingProofEndpoints() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        MessagingStubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let path = request.url?.path ?? ""
            let body: String
            switch (method, path) {
            case ("GET", "/api/messaging/peers"):
                body = Fixtures.peers
            case ("GET", "/api/messaging/conversations/conversation:lewis:home/events"):
                body = Fixtures.events
            case ("POST", "/api/messaging/conversations/conversation:lewis:home/messages"):
                body = Fixtures.receipt
            default:
                fatalError("unexpected messaging request: \(method) \(path)")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: method == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )

        let peers = try await client.messagingPeers()
        _ = try await client.messagingEvents(
            conversationID: peers[0].conversationID,
            after: 5
        )
        _ = try await client.postMessagingMessage(
            conversationID: peers[0].conversationID,
            clientMessageID: "client-message:one",
            text: "Hello Lewis"
        )

        let signatures = MessagingStubURLProtocol.requests.map { request in
            let query = request.url?.query.map { "?\($0)" } ?? ""
            return "\(request.httpMethod ?? "") \(request.url?.path ?? "")\(query)"
        }
        expect(signatures == [
            "GET /api/messaging/peers",
            "GET /api/messaging/conversations/conversation:lewis:home/events?after=5",
            "POST /api/messaging/conversations/conversation:lewis:home/messages",
        ], "messaging endpoint surface drifted: \(signatures)")
        expect(MessagingStubURLProtocol.bodies[0].isEmpty, "peer discovery encoded a body")
        expect(MessagingStubURLProtocol.bodies[1].isEmpty, "event polling encoded a body")

        let submitted = try JSONSerialization.jsonObject(
            with: MessagingStubURLProtocol.bodies[2]
        ) as? [String: String]
        expect(submitted == [
            "client_message_id": "client-message:one",
            "text": "Hello Lewis",
        ], "message submission body changed identity or text: \(String(describing: submitted))")
    }

    @MainActor
    private static func modelPresentsAndSendsThroughTheExactPeerConversation() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        MessagingStubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let body: String
            switch (method, request.url?.path ?? "") {
            case ("GET", "/api/messaging/peers"):
                body = Fixtures.peers
            case ("GET", "/api/messaging/conversations/conversation:lewis:home/events"):
                body = Fixtures.events
            case ("POST", "/api/messaging/conversations/conversation:lewis:home/messages"):
                body = Fixtures.receipt
            default:
                fatalError("unexpected messaging model request: \(method) \(request.url?.path ?? "")")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: method == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )
        let model = MessagingModel()

        await model.load(using: client)
        expect(model.peer?.displayName == "Lewis", "model changed the exact bot title")
        expect(model.peer?.machineName == "taloss.mac.mini.lan", "model changed the exact serving machine")
        expect(model.peer?.headerSubtitle == "taloss.mac.mini.lan · Connected", "model changed the machine subtitle")
        expect(model.messages.map(\.body) == ["Hello Lewis", "Hello from Lewis"], "model changed event order")

        let sent = await model.send(text: "  Hello Lewis  ", using: client)
        expect(sent, "model did not accept the proof message")
        expect(model.messages.last?.id == "message:human:2", "accepted message did not enter the transcript")

        let post = MessagingStubURLProtocol.requests.last
        expect(post?.httpMethod == "POST", "model did not submit through the message command")
        let submitted = try JSONSerialization.jsonObject(
            with: MessagingStubURLProtocol.bodies.last ?? Data()
        ) as? [String: String]
        expect(submitted?["text"] == "Hello Lewis", "model did not normalize the submitted text")
        expect(UUID(uuidString: submitted?["client_message_id"] ?? "") != nil, "model omitted its client message UUID")
    }

    @MainActor
    private static func modelReconcilesAReceiptAlreadyObservedByPolling() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []
        MessagingStubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let body: String
            switch (method, request.url?.path ?? "") {
            case ("GET", "/api/messaging/peers"):
                body = Fixtures.peers
            case ("GET", "/api/messaging/conversations/conversation:lewis:home/events"):
                body = Fixtures.eventsContainingAcceptedMessage
            case ("POST", "/api/messaging/conversations/conversation:lewis:home/messages"):
                body = Fixtures.reconciledReceipt
            default:
                fatalError("unexpected messaging reconciliation request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: method == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )
        let model = MessagingModel()

        await model.load(using: client)
        expect(model.messages.map(\.id) == ["message:human:2"], "polling did not observe the accepted message")

        let sent = await model.send(text: "Hello Lewis", using: client)
        expect(sent, "an already-observed acceptance receipt became a false send failure")
        expect(model.messages.map(\.id) == ["message:human:2"], "receipt reconciliation duplicated the accepted message")
    }

    @MainActor
    private static func modelKeepsAcceptedReceiptWhenAnOlderPollCompletesLater() async throws {
        MessagingStubURLProtocol.requests = []
        MessagingStubURLProtocol.bodies = []

        let stalePollStarted = DispatchSemaphore(value: 0)
        var completeStalePoll: ((HTTPURLResponse, Data) -> Void)?
        MessagingStubURLProtocol.deferredHandler = { request, complete in
            guard request.httpMethod == "GET",
                  request.url?.path == "/api/messaging/conversations/conversation:lewis:home/events",
                  request.url?.query == "after=1"
            else { return false }
            completeStalePoll = complete
            stalePollStarted.signal()
            return true
        }
        defer { MessagingStubURLProtocol.deferredHandler = nil }

        MessagingStubURLProtocol.handler = { request in
            let method = request.httpMethod ?? "GET"
            let body: String
            switch (method, request.url?.path ?? "", request.url?.query) {
            case ("GET", "/api/messaging/peers", _):
                body = Fixtures.peers
            case ("GET", "/api/messaging/conversations/conversation:lewis:home/events", "after=0"):
                body = Fixtures.raceInitialEvents
            case ("POST", "/api/messaging/conversations/conversation:lewis:home/messages", _):
                body = Fixtures.raceReceipt
            default:
                fatalError("unexpected stale-poll reconciliation request")
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: method == "POST" ? 202 : 200,
                httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"]
            )!
            return (response, Data(body.utf8))
        }

        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MessagingStubURLProtocol.self]
        let client = FortClient(
            baseURL: URL(string: "https://fort.test")!,
            session: URLSession(configuration: configuration)
        )
        let model = MessagingModel()

        await model.load(using: client)
        expect(model.messages.map(\.id) == ["message:peer:observed"], "model did not start after the observed sequence 1 event")

        let stalePoll = Task { await model.poll(using: client) }
        let pollWasHeld = await Task.detached {
            wait(for: stalePollStarted, timeout: .now() + 5)
        }.value
        expect(pollWasHeld, "poll(after=1) did not reach the deterministic hold point")

        let sent = await model.send(text: "Race-safe message", using: client)
        expect(sent, "successful sequence 2 receipt became a send failure")
        expect(
            model.messages.map(\.id) == ["message:peer:observed", "message:human:accepted"],
            "successful receipt did not append the exact accepted message once"
        )

        guard let completeStalePoll else {
            fatalError("held poll did not retain its response completion")
        }
        let staleResponse = HTTPURLResponse(
            url: URL(string: "https://fort.test/api/messaging/conversations/conversation:lewis:home/events?after=1")!,
            statusCode: 200,
            httpVersion: "HTTP/1.1",
            headerFields: ["Content-Type": "application/json"]
        )!
        completeStalePoll(staleResponse, Data(Fixtures.raceStaleEmptyEvents.utf8))
        await stalePoll.value

        expect(model.errorMessage == nil, "stale empty poll became a false messaging error: \(String(describing: model.errorMessage))")
        expect(
            model.messages.map(\.id) == ["message:peer:observed", "message:human:accepted"],
            "stale empty poll removed or duplicated the accepted message"
        )
    }

    private static func expect(_ condition: @autoclosure () -> Bool, _ message: String) {
        guard condition() else { fatalError(message) }
    }

    private enum Fixtures {
        static let macBookChannels = #"""
        [{
          "id":"messaging-channel:hermes:aria",
          "source_id":"messaging-source:macbook",
          "canonical_profile_id":"default",
          "display_name":"Aria",
          "machine_name":"Tobias's MacBook Pro",
          "conversation_id":"conversation:aria:home",
          "state":"connected"
        }]
        """#

        static let invalidMacBookChannels = #"""
        [{
          "id":"messaging-channel:hermes:aria",
          "source_id":"messaging-source:macbook",
          "canonical_profile_id":"default",
          "display_name":"",
          "machine_name":"Tobias's MacBook Pro",
          "conversation_id":"conversation:aria:home",
          "state":"connected"
        }]
        """#

        static let mismatchedMachineChannels = #"""
        [{
          "id":"messaging-channel:hermes:aria",
          "source_id":"messaging-source:macbook",
          "canonical_profile_id":"default",
          "display_name":"Aria",
          "machine_name":"Imposter Mac",
          "conversation_id":"conversation:aria:home",
          "state":"connected"
        }]
        """#

        static let macMiniChannels = #"""
        [{
          "id":"messaging-channel:hermes:pascal",
          "source_id":"messaging-source:mac-mini",
          "canonical_profile_id":"writer",
          "display_name":"Pascal",
          "machine_name":"Talos Mac mini",
          "conversation_id":"conversation:pascal:home",
          "state":"connected"
        }]
        """#

        static let pascalEvents = #"""
        {
          "conversation_id":"conversation:pascal:home",
          "events":[{
            "sequence":1,
            "message":{
              "id":"message:pascal:ready",
              "conversation_id":"conversation:pascal:home",
              "author_kind":"peer",
              "author_id":"messaging-channel:hermes:pascal",
              "body":"Ready from Pascal",
              "created_at":"2026-08-24T10:00:00Z"
            }
          }],
          "next_after":1
        }
        """#

        static let ariaEvents = #"""
        {
          "conversation_id":"conversation:aria:home",
          "events":[],
          "next_after":0
        }
        """#

        static let ariaSequenceOne = #"""
        {
          "conversation_id":"conversation:aria:home",
          "events":[{
            "sequence":1,
            "message":{
              "id":"message:aria:one",
              "conversation_id":"conversation:aria:home",
              "author_kind":"peer",
              "author_id":"messaging-channel:hermes:aria",
              "body":"First",
              "created_at":"2026-08-24T10:00:00Z"
            }
          }],
          "next_after":1
        }
        """#

        static let ariaGappedSequenceThree = #"""
        {
          "conversation_id":"conversation:aria:home",
          "events":[{
            "sequence":3,
            "message":{
              "id":"message:aria:three",
              "conversation_id":"conversation:aria:home",
              "author_kind":"peer",
              "author_id":"messaging-channel:hermes:aria",
              "body":"Third",
              "created_at":"2026-08-24T10:00:02Z"
            }
          }],
          "next_after":3
        }
        """#

        static let ariaSequenceTwo = #"""
        {
          "conversation_id":"conversation:aria:home",
          "events":[{
            "sequence":2,
            "message":{
              "id":"message:aria:two",
              "conversation_id":"conversation:aria:home",
              "author_kind":"peer",
              "author_id":"messaging-channel:hermes:aria",
              "body":"Second",
              "created_at":"2026-08-24T10:00:01Z"
            }
          }],
          "next_after":2
        }
        """#

        static let ariaDuplicateMessageIDsInPage = #"""
        {
          "conversation_id":"conversation:aria:home",
          "events":[{
            "sequence":2,
            "message":{
              "id":"message:aria:duplicate",
              "conversation_id":"conversation:aria:home",
              "author_kind":"peer",
              "author_id":"messaging-channel:hermes:aria",
              "body":"Duplicate first",
              "created_at":"2026-08-24T10:00:01Z"
            }
          },{
            "sequence":3,
            "message":{
              "id":"message:aria:duplicate",
              "conversation_id":"conversation:aria:home",
              "author_kind":"peer",
              "author_id":"messaging-channel:hermes:aria",
              "body":"Duplicate second",
              "created_at":"2026-08-24T10:00:02Z"
            }
          }],
          "next_after":3
        }
        """#

        static let ariaExistingMessageIDAtNewSequence = #"""
        {
          "conversation_id":"conversation:aria:home",
          "events":[{
            "sequence":2,
            "message":{
              "id":"message:aria:one",
              "conversation_id":"conversation:aria:home",
              "author_kind":"peer",
              "author_id":"messaging-channel:hermes:aria",
              "body":"Existing ID at sequence two",
              "created_at":"2026-08-24T10:00:01Z"
            }
          }],
          "next_after":2
        }
        """#

        static let ariaReceiptReusesExistingMessageID = #"""
        {
          "message":{
            "id":"message:aria:one",
            "conversation_id":"conversation:aria:home",
            "author_kind":"human",
            "author_id":"human:toby",
            "body":"Collision",
            "created_at":"2026-08-24T10:00:01Z"
          },
          "accepted_sequence":2,
          "delivery_state":"pending"
        }
        """#

        static let ariaReceiptSequenceThree = #"""
        {
          "message":{
            "id":"message:aria:human-three",
            "conversation_id":"conversation:aria:home",
            "author_kind":"human",
            "author_id":"human:toby",
            "body":"Human after peer",
            "created_at":"2026-08-24T10:00:02Z"
          },
          "accepted_sequence":3,
          "delivery_state":"pending"
        }
        """#

        static let ariaEventsTwoAndThree = #"""
        {
          "conversation_id":"conversation:aria:home",
          "events":[{
            "sequence":2,
            "message":{
              "id":"message:aria:peer-two",
              "conversation_id":"conversation:aria:home",
              "author_kind":"peer",
              "author_id":"messaging-channel:hermes:aria",
              "body":"Unseen peer",
              "created_at":"2026-08-24T10:00:01Z"
            }
          },{
            "sequence":3,
            "message":{
              "id":"message:aria:human-three",
              "conversation_id":"conversation:aria:home",
              "author_kind":"human",
              "author_id":"human:toby",
              "body":"Human after peer",
              "created_at":"2026-08-24T10:00:02Z"
            }
          }],
          "next_after":3
        }
        """#

        static let ariaConflictingEventsTwoAndThree = #"""
        {
          "conversation_id":"conversation:aria:home",
          "events":[{
            "sequence":2,
            "message":{
              "id":"message:aria:peer-two",
              "conversation_id":"conversation:aria:home",
              "author_kind":"peer",
              "author_id":"messaging-channel:hermes:aria",
              "body":"Unseen peer",
              "created_at":"2026-08-24T10:00:01Z"
            }
          },{
            "sequence":3,
            "message":{
              "id":"message:aria:conflict-three",
              "conversation_id":"conversation:aria:home",
              "author_kind":"human",
              "author_id":"human:toby",
              "body":"Conflicting replay",
              "created_at":"2026-08-24T10:00:02Z"
            }
          }],
          "next_after":3
        }
        """#

        static let ariaEventsBeforeProcessRestart = #"""
        {
          "conversation_id":"conversation:aria:home",
          "events":[{
            "sequence":1,
            "message":{
              "id":"message:aria:before-restart",
              "conversation_id":"conversation:aria:home",
              "author_kind":"peer",
              "author_id":"messaging-channel:hermes:aria",
              "body":"Before process restart",
              "created_at":"2026-08-24T09:59:00Z"
            }
          }],
          "next_after":1
        }
        """#

        static let ariaEventsAfterProcessRestart = #"""
        {
          "conversation_id":"conversation:aria:home",
          "events":[{
            "sequence":1,
            "message":{
              "id":"message:aria:after-restart",
              "conversation_id":"conversation:aria:home",
              "author_kind":"peer",
              "author_id":"messaging-channel:hermes:aria",
              "body":"After process restart",
              "created_at":"2026-08-24T10:00:00Z"
            }
          }],
          "next_after":1
        }
        """#

        static let pascalReceipt = #"""
        {
          "message":{
            "id":"message:human:pascal:1",
            "conversation_id":"conversation:pascal:home",
            "author_kind":"human",
            "author_id":"human:toby",
            "body":"Hello Pascal",
            "created_at":"2026-08-24T10:00:01Z"
          },
          "accepted_sequence":2,
          "delivery_state":"pending"
        }
        """#

        static let pascalUnknownReceipt = #"""
        {
          "message":{
            "id":"message:human:pascal:unknown",
            "conversation_id":"conversation:pascal:home",
            "author_kind":"human",
            "author_id":"human:toby",
            "body":"One ambiguous message",
            "created_at":"2026-08-24T10:00:02Z"
          },
          "accepted_sequence":2,
          "delivery_state":"unknown",
          "delivery_code":"hermes_relay_delivery_failed"
        }
        """#

        static let pascalInvalidUnknownReceipt = #"""
        {
          "message":{
            "id":"message:human:pascal:invalid",
            "conversation_id":"conversation:pascal:home",
            "author_kind":"human",
            "author_id":"human:toby",
            "body":"Invalid ambiguous response",
            "created_at":"2026-08-24T10:00:03Z"
          },
          "accepted_sequence":2,
          "delivery_state":"unknown",
          "delivery_code":"unexpected_delivery_code"
        }
        """#

        static let pascalLatePendingReceipt = #"""
        {
          "message":{
            "id":"message:human:pascal:late-pending",
            "conversation_id":"conversation:pascal:home",
            "author_kind":"human",
            "author_id":"human:toby",
            "body":"Late pending message",
            "created_at":"2026-08-24T10:00:04Z"
          },
          "accepted_sequence":3,
          "delivery_state":"pending"
        }
        """#

        static let peers = #"""
        [{
          "id":"peer:hermes:lewis",
          "source_id":"messaging-source:mac-mini",
          "canonical_profile_id":"default",
          "display_name":"Lewis",
          "machine_name":"taloss.mac.mini.lan",
          "conversation_id":"conversation:lewis:home",
          "state":"connected"
        }]
        """#

        static let peersOnMacBook = #"""
        [{
          "id":"peer:hermes:lewis",
          "source_id":"messaging-source:macbook",
          "canonical_profile_id":"default",
          "display_name":"Lewis",
          "machine_name":"tobiass.macbook.pro.lan",
          "conversation_id":"conversation:lewis:home",
          "state":"connected"
        }]
        """#

        static let emptyEvents = #"""
        {
          "conversation_id":"conversation:lewis:home",
          "events":[],
          "next_after":0
        }
        """#

        static let cursorAheadEmptyEvents = #"""
        {
          "conversation_id":"conversation:lewis:home",
          "events":[],
          "next_after":1
        }
        """#

        static let lewisSequenceOne = #"""
        {
          "conversation_id":"conversation:lewis:home",
          "events":[{
            "sequence":1,
            "message":{
              "id":"message:lewis:one",
              "conversation_id":"conversation:lewis:home",
              "author_kind":"peer",
              "author_id":"peer:hermes:lewis",
              "body":"Recovered",
              "created_at":"2026-08-24T10:00:00Z"
            }
          }],
          "next_after":1
        }
        """#

        static let lewisDuplicateMessageIDsInPage = #"""
        {
          "conversation_id":"conversation:lewis:home",
          "events":[{
            "sequence":2,
            "message":{
              "id":"message:lewis:duplicate",
              "conversation_id":"conversation:lewis:home",
              "author_kind":"peer",
              "author_id":"peer:hermes:lewis",
              "body":"Duplicate Lewis first",
              "created_at":"2026-08-24T10:00:01Z"
            }
          },{
            "sequence":3,
            "message":{
              "id":"message:lewis:duplicate",
              "conversation_id":"conversation:lewis:home",
              "author_kind":"peer",
              "author_id":"peer:hermes:lewis",
              "body":"Duplicate Lewis second",
              "created_at":"2026-08-24T10:00:02Z"
            }
          }],
          "next_after":3
        }
        """#

        static let lewisExistingMessageIDAtNewSequence = #"""
        {
          "conversation_id":"conversation:lewis:home",
          "events":[{
            "sequence":2,
            "message":{
              "id":"message:lewis:one",
              "conversation_id":"conversation:lewis:home",
              "author_kind":"peer",
              "author_id":"peer:hermes:lewis",
              "body":"Existing Lewis ID at sequence two",
              "created_at":"2026-08-24T10:00:01Z"
            }
          }],
          "next_after":2
        }
        """#

        static let lewisSequenceTwo = #"""
        {
          "conversation_id":"conversation:lewis:home",
          "events":[{
            "sequence":2,
            "message":{
              "id":"message:lewis:two",
              "conversation_id":"conversation:lewis:home",
              "author_kind":"peer",
              "author_id":"peer:hermes:lewis",
              "body":"Second Lewis",
              "created_at":"2026-08-24T10:00:01Z"
            }
          }],
          "next_after":2
        }
        """#

        static let lewisReceiptReusesExistingMessageID = #"""
        {
          "message":{
            "id":"message:lewis:one",
            "conversation_id":"conversation:lewis:home",
            "author_kind":"human",
            "author_id":"human:toby",
            "body":"Legacy collision",
            "created_at":"2026-08-24T10:00:01Z"
          },
          "accepted_sequence":2,
          "delivery_state":"pending"
        }
        """#

        static let lewisReceiptSequenceThree = #"""
        {
          "message":{
            "id":"message:lewis:human-three",
            "conversation_id":"conversation:lewis:home",
            "author_kind":"human",
            "author_id":"human:toby",
            "body":"Human after Lewis",
            "created_at":"2026-08-24T10:00:02Z"
          },
          "accepted_sequence":3,
          "delivery_state":"pending"
        }
        """#

        static let lewisEventsTwoAndThree = #"""
        {
          "conversation_id":"conversation:lewis:home",
          "events":[{
            "sequence":2,
            "message":{
              "id":"message:lewis:peer-two",
              "conversation_id":"conversation:lewis:home",
              "author_kind":"peer",
              "author_id":"peer:hermes:lewis",
              "body":"Unseen Lewis",
              "created_at":"2026-08-24T10:00:01Z"
            }
          },{
            "sequence":3,
            "message":{
              "id":"message:lewis:human-three",
              "conversation_id":"conversation:lewis:home",
              "author_kind":"human",
              "author_id":"human:toby",
              "body":"Human after Lewis",
              "created_at":"2026-08-24T10:00:02Z"
            }
          }],
          "next_after":3
        }
        """#

        static let events = #"""
        {
          "conversation_id":"conversation:lewis:home",
          "events":[
            {
              "sequence":1,
              "message":{
                "id":"message:human:1",
                "conversation_id":"conversation:lewis:home",
                "author_kind":"human",
                "author_id":"human:toby",
                "body":"Hello Lewis",
                "created_at":"2026-08-23T20:00:00Z"
              }
            },
            {
              "sequence":2,
              "message":{
                "id":"message:hermes:1",
                "conversation_id":"conversation:lewis:home",
                "author_kind":"peer",
                "author_id":"peer:hermes:lewis",
                "body":"Hello from Lewis",
                "in_reply_to_message_id":"message:human:1",
                "created_at":"2026-08-23T20:00:01Z"
              }
            }
          ],
          "next_after":2
        }
        """#

        static let receipt = #"""
        {
          "message":{
            "id":"message:human:2",
            "conversation_id":"conversation:lewis:home",
            "author_kind":"human",
            "author_id":"human:toby",
            "body":"Hello Lewis",
            "created_at":"2026-08-23T20:00:02Z"
          },
          "accepted_sequence":3,
          "delivery_state":"pending"
        }
        """#

        static let eventsContainingAcceptedMessage = #"""
        {
          "conversation_id":"conversation:lewis:home",
          "events":[{
            "sequence":1,
            "message":{
              "id":"message:human:2",
              "conversation_id":"conversation:lewis:home",
              "author_kind":"human",
              "author_id":"human:toby",
              "body":"Hello Lewis",
              "created_at":"2026-08-23T20:00:02Z"
            }
          }],
          "next_after":1
        }
        """#

        static let reconciledReceipt = #"""
        {
          "message":{
            "id":"message:human:2",
            "conversation_id":"conversation:lewis:home",
            "author_kind":"human",
            "author_id":"human:toby",
            "body":"Hello Lewis",
            "created_at":"2026-08-23T20:00:02Z"
          },
          "accepted_sequence":1,
          "delivery_state":"pending"
        }
        """#

        static let raceInitialEvents = #"""
        {
          "conversation_id":"conversation:lewis:home",
          "events":[{
            "sequence":1,
            "message":{
              "id":"message:peer:observed",
              "conversation_id":"conversation:lewis:home",
              "author_kind":"peer",
              "author_id":"peer:hermes:lewis",
              "body":"Ready",
              "created_at":"2026-08-23T20:00:00Z"
            }
          }],
          "next_after":1
        }
        """#

        static let raceReceipt = #"""
        {
          "message":{
            "id":"message:human:accepted",
            "conversation_id":"conversation:lewis:home",
            "author_kind":"human",
            "author_id":"human:toby",
            "body":"Race-safe message",
            "created_at":"2026-08-23T20:00:02Z"
          },
          "accepted_sequence":2,
          "delivery_state":"pending"
        }
        """#

        static let raceStaleEmptyEvents = #"""
        {
          "conversation_id":"conversation:lewis:home",
          "events":[],
          "next_after":1
        }
        """#
    }
}

private final class MessagingStubURLProtocol: URLProtocol {
    static var requests: [URLRequest] = []
    static var bodies: [Data] = []
    static var handler: ((URLRequest) throws -> (HTTPURLResponse, Data))?
    static var deferredHandler: ((URLRequest, @escaping (HTTPURLResponse, Data) -> Void) -> Bool)?

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        Self.requests.append(request)
        Self.bodies.append(request.httpBody ?? request.httpBodyStream.flatMap(Self.read) ?? Data())
        if Self.deferredHandler?(request, { [weak self] response, data in
            self?.complete(response: response, data: data)
        }) == true {
            return
        }
        do {
            guard let handler = Self.handler else { fatalError("missing messaging stub handler") }
            let (response, data) = try handler(request)
            complete(response: response, data: data)
        } catch {
            client?.urlProtocol(self, didFailWithError: error)
        }
    }

    override func stopLoading() {}

    private func complete(response: HTTPURLResponse, data: Data) {
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: data)
        client?.urlProtocolDidFinishLoading(self)
    }

    private static func read(_ stream: InputStream) -> Data? {
        stream.open()
        defer { stream.close() }
        var data = Data()
        var buffer = [UInt8](repeating: 0, count: 4096)
        while stream.hasBytesAvailable {
            let count = stream.read(&buffer, maxLength: buffer.count)
            if count <= 0 { break }
            data.append(buffer, count: count)
        }
        return data
    }
}

#if MESSAGING_CONTRACT_STANDALONE
@main
struct MessagingContractRunner {
    static func main() async throws {
        try await MessagingContractTests.run()
        print("FortKit messaging contract checks passed")
    }
}
#endif
