//
//  Messaging.swift
//  FortKit
//
//  Provider-neutral wire and identity contract for Spec 053 Hermes Messaging
//  Channels. A Messaging Channel is not a Fort execution provider or Agent.
//

import CryptoKit
import Foundation

public enum MessagingPeerState: String, Codable, Sendable, Hashable {
    case connected
    case offline
}

public enum MessagingChannelSourceError: Error, Sendable {
    case machineIsNotPinned
    case invalidAccountIdentity
}

extension MessagingChannelSourceError: LocalizedError {
    public var errorDescription: String? {
        switch self {
        case .machineIsNotPinned:
            return "Fort will only load messaging channels from an already trusted machine."
        case .invalidAccountIdentity:
            return "Fort could not identify the signed-in gateway account for Messaging Channels."
        }
    }
}

/// One exact machine transport used to discover and route Messaging Channels.
/// Production construction requires the gateway's current key to match an
/// existing account pin; discovery never expands trust.
public struct MessagingChannelSource: Identifiable, @unchecked Sendable {
    public let machineID: String
    public let machineName: String
    package let client: FortClient
    package let isReachable: Bool
    package let accountScope: String
    package let transportRevision: String

    public var id: String { machineID }

    public init(account: GatewayAccount, machine: GatewayMachine) throws {
        guard account.pinnedPublicKeys[machine.machineID] == machine.publicKey else {
            throw MessagingChannelSourceError.machineIsNotPinned
        }
        let client = FortClient.gatewayOnly()
        try client.useGateway(account: account, machine: machine)
        let identity = try Self.makeAccountIdentity(account)
        self.machineID = machine.machineID
        self.machineName = machine.name
        self.client = client
        self.isReachable = machine.online
        self.accountScope = identity.scope
        self.transportRevision = identity.transportRevision
    }

    package init(
        machineID: String,
        machineName: String,
        client: FortClient,
        isReachable: Bool = true,
        accountScope: String = "contract-account",
        transportRevision: String = "contract-transport"
    ) {
        self.machineID = machineID
        self.machineName = machineName
        self.client = client
        self.isReachable = isReachable
        self.accountScope = accountScope
        self.transportRevision = transportRevision
    }

    private static func makeAccountIdentity(
        _ account: GatewayAccount
    ) throws -> (scope: String, transportRevision: String) {
        // The gateway authenticates this JWT; FortKit decodes its stable email
        // only to isolate local presentation state. It is never authorization.
        guard let gatewayURL = account.gatewayURL,
              let token = account.bearerToken,
              !token.isEmpty,
              let email = nativeTokenEmail(token)
        else { throw MessagingChannelSourceError.invalidAccountIdentity }
        let normalizedGateway: URL
        do {
            normalizedGateway = try GatewayAddress.normalize(gatewayURL)
        } catch {
            throw MessagingChannelSourceError.invalidAccountIdentity
        }
        let normalizedEmail = email.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        guard !normalizedEmail.isEmpty else {
            throw MessagingChannelSourceError.invalidAccountIdentity
        }
        let gateway = normalizedGateway.absoluteString
        let material = "\(gateway.utf8.count):\(gateway)\(normalizedEmail.utf8.count):\(normalizedEmail)"
        let scope = SHA256.hash(data: Data(material.utf8))
            .map { String(format: "%02x", $0) }
            .joined()
        let transportRevision = SHA256.hash(data: Data(token.utf8))
            .map { String(format: "%02x", $0) }
            .joined()
        return (scope, transportRevision)
    }

    private static func nativeTokenEmail(_ token: String) -> String? {
        let segments = token.split(separator: ".", omittingEmptySubsequences: false)
        guard segments.count == 3,
              segments.allSatisfy({ !$0.isEmpty })
        else { return nil }
        var payload = String(segments[1])
            .replacingOccurrences(of: "-", with: "+")
            .replacingOccurrences(of: "_", with: "/")
        let remainder = payload.count % 4
        if remainder != 0 {
            payload.append(String(repeating: "=", count: 4 - remainder))
        }
        guard let data = Data(base64Encoded: payload),
              let claims = try? JSONDecoder().decode(NativeTokenClaims.self, from: data)
        else { return nil }
        return claims.email
    }

    private struct NativeTokenClaims: Decodable {
        let email: String
    }
}

/// Fail-closed production projection of gateway machines into exact pinned
/// Messaging Channel transports. Rejected machines are surfaced but never
/// queried, trusted, or enrolled as a side effect of directory loading.
public struct MessagingChannelSourceResolution: Sendable {
    public let sources: [MessagingChannelSource]
    public let warning: String?

    public init(account: GatewayAccount, machines: [GatewayMachine]) {
        var accepted: [MessagingChannelSource] = []
        var rejectedCount = 0
        for machine in machines {
            do {
                accepted.append(try MessagingChannelSource(account: account, machine: machine))
            } catch {
                rejectedCount += 1
            }
        }
        sources = accepted
        if rejectedCount == 0 {
            warning = nil
        } else {
            let noun = rejectedCount == 1 ? "source" : "sources"
            warning = "Fort skipped \(rejectedCount) Hermes Messaging Channel \(noun) because trust or account identity could not be verified."
        }
    }
}

/// Client identity is composite because two Fort machines may expose the same
/// server-local channel identifier without licensing a merge or fallback.
public struct MessagingChannelIdentity: Codable, Sendable, Hashable, Identifiable {
    public let machineID: String
    public let channelID: String

    public var id: MessagingChannelIdentity { self }

    public init(machineID: String, channelID: String) {
        self.machineID = machineID
        self.channelID = channelID
    }
}

public struct MessagingChannel: Codable, Sendable, Hashable, Identifiable {
    public let identity: MessagingChannelIdentity
    public let sourceID: String
    public let canonicalProfileID: String
    public let displayName: String
    public let machineName: String
    public let conversationID: String
    public let state: MessagingPeerState
    public let reason: String?

    public var id: MessagingChannelIdentity { identity }

    public var subtitle: String {
        "Hermes · \(machineName) · \(state == .connected ? "Connected" : "Offline")"
    }

    package init(source: MessagingChannelSource, peer: MessagingPeer) {
        identity = MessagingChannelIdentity(machineID: source.machineID, channelID: peer.id)
        sourceID = peer.sourceID
        canonicalProfileID = peer.canonicalProfileID
        displayName = peer.displayName
        machineName = source.machineName
        conversationID = peer.conversationID
        state = peer.state
        reason = peer.reason
    }

    package func projectedOffline(
        reason: String,
        trustedMachineName: String? = nil
    ) -> MessagingChannel {
        MessagingChannel(
            identity: identity,
            sourceID: sourceID,
            canonicalProfileID: canonicalProfileID,
            displayName: displayName,
            machineName: trustedMachineName ?? machineName,
            conversationID: conversationID,
            state: .offline,
            reason: reason
        )
    }

    private init(
        identity: MessagingChannelIdentity,
        sourceID: String,
        canonicalProfileID: String,
        displayName: String,
        machineName: String,
        conversationID: String,
        state: MessagingPeerState,
        reason: String?
    ) {
        self.identity = identity
        self.sourceID = sourceID
        self.canonicalProfileID = canonicalProfileID
        self.displayName = displayName
        self.machineName = machineName
        self.conversationID = conversationID
        self.state = state
        self.reason = reason
    }
}

public struct MessagingPeer: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let sourceID: String
    public let canonicalProfileID: String
    public let displayName: String
    public let machineName: String
    public let conversationID: String
    public let state: MessagingPeerState
    public let reason: String?

    package var headerSubtitle: String {
        "\(machineName) · \(state == .connected ? "Connected" : "Offline")"
    }

    enum CodingKeys: String, CodingKey {
        case id, state, reason
        case sourceID = "source_id"
        case canonicalProfileID = "canonical_profile_id"
        case displayName = "display_name"
        case machineName = "machine_name"
        case conversationID = "conversation_id"
    }
}

public enum MessagingAuthorKind: String, Codable, Sendable, Hashable {
    case human
    case peer
}

public struct MessagingMessage: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let conversationID: String
    public let authorKind: MessagingAuthorKind
    public let authorID: String
    public let body: String
    public let inReplyToMessageID: String?
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, body
        case conversationID = "conversation_id"
        case authorKind = "author_kind"
        case authorID = "author_id"
        case inReplyToMessageID = "in_reply_to_message_id"
        case createdAt = "created_at"
    }
}

public struct MessagingEvent: Codable, Sendable, Hashable, Identifiable {
    public var id: Int64 { sequence }
    public let sequence: Int64
    public let message: MessagingMessage
}

public struct MessagingEventsPage: Codable, Sendable, Hashable {
    public let conversationID: String
    public let events: [MessagingEvent]
    public let nextAfter: Int64

    enum CodingKeys: String, CodingKey {
        case events
        case conversationID = "conversation_id"
        case nextAfter = "next_after"
    }
}

public struct MessagingMessageReceipt: Codable, Sendable, Hashable {
    public let message: MessagingMessage
    public let acceptedSequence: Int64
    public let deliveryState: MessagingDeliveryState
    public let deliveryCode: String?

    enum CodingKeys: String, CodingKey {
        case message
        case acceptedSequence = "accepted_sequence"
        case deliveryState = "delivery_state"
        case deliveryCode = "delivery_code"
    }
}

public enum MessagingDeliveryState: String, Codable, Sendable, Hashable {
    /// Fort accepted the message and wrote it to the selected live adapter,
    /// but does not claim downstream Hermes processing.
    case pending

    /// Fort accepted the message, but the exact adapter write had an ambiguous
    /// outcome. The same client identity is never dispatched again.
    case unknown
}

public struct MessagingDeliveryNoticeIdentity: Codable, Sendable, Hashable {
    public let channelID: MessagingChannelIdentity
    public let messageID: String

    public init(channelID: MessagingChannelIdentity, messageID: String) {
        self.channelID = channelID
        self.messageID = messageID
    }
}

/// Device-local evidence that Fort accepted an exact message but will not
/// automatically resend because the downstream Hermes write was ambiguous.
public struct MessagingDeliveryNotice: Codable, Sendable, Hashable, Identifiable {
    public let channelID: MessagingChannelIdentity
    public let messageID: String
    public let acceptedSequence: Int64
    public let deliveryCode: String

    public var id: MessagingDeliveryNoticeIdentity {
        MessagingDeliveryNoticeIdentity(channelID: channelID, messageID: messageID)
    }

    public var marker: String { "Delivery unknown · Fort will not resend" }

    private static let markerAuthorID = "fort:messaging-delivery-outcome"

    package init(
        channelID: MessagingChannelIdentity,
        messageID: String,
        acceptedSequence: Int64,
        deliveryCode: String
    ) {
        self.channelID = channelID
        self.messageID = messageID
        self.acceptedSequence = acceptedSequence
        self.deliveryCode = deliveryCode
    }

    package func markerMessage(conversationID: String) -> MessagingMessage {
        MessagingMessage(
            id: messageID,
            conversationID: conversationID,
            authorKind: .peer,
            authorID: Self.markerAuthorID,
            body: marker,
            inReplyToMessageID: nil,
            createdAt: ""
        )
    }

    package func isMarkerMessage(_ message: MessagingMessage) -> Bool {
        message.id == messageID && message.authorID == Self.markerAuthorID
    }
}

private struct MessagingMessageRequest: Encodable {
    let clientMessageID: String
    let text: String

    enum CodingKeys: String, CodingKey {
        case clientMessageID = "client_message_id"
        case text
    }
}

public extension FortClient {
    /// Returns every Messaging Channel currently projected by the exact Fort
    /// machine selected by this client.
    func messagingChannels() async throws -> [MessagingPeer] {
        try await messagingRequest(
            path: "/api/messaging/channels",
            method: "GET",
            body: nil
        )
    }

    /// Returns the exact messaging peers configured by Fort. The client cannot
    /// select provider, model, profile, machine, or transport fields.
    func messagingPeers() async throws -> [MessagingPeer] {
        try await messagingRequest(
            path: "/api/messaging/peers",
            method: "GET",
            body: nil
        )
    }

    /// Reads the ordered messages accepted after one server-owned sequence.
    func messagingEvents(
        conversationID: String,
        after: Int64
    ) async throws -> MessagingEventsPage {
        let conversation = Self.messagingPathComponent(conversationID)
        return try await messagingRequest(
            path: "/api/messaging/conversations/\(conversation)/events?after=\(after)",
            method: "GET",
            body: nil
        )
    }

    /// Submits one text message to the exact peer bound to the Conversation.
    func postMessagingMessage(
        conversationID: String,
        clientMessageID: String,
        text: String
    ) async throws -> MessagingMessageReceipt {
        let conversation = Self.messagingPathComponent(conversationID)
        let body = try JSONEncoder().encode(MessagingMessageRequest(
            clientMessageID: clientMessageID,
            text: text
        ))
        return try await messagingRequest(
            path: "/api/messaging/conversations/\(conversation)/messages",
            method: "POST",
            body: body
        )
    }

    private func messagingRequest<Response: Decodable>(
        path: String,
        method: String,
        body: Data?
    ) async throws -> Response {
        let response = try await messagingTransportRequest(
            path: path,
            method: method,
            body: body
        )
        guard (200...299).contains(response.status) else {
            throw FortClientError.httpStatus(
                status: response.status,
                body: String(data: response.data, encoding: .utf8) ?? "",
                requestID: response.requestID
            )
        }
        return try JSONDecoder().decode(Response.self, from: response.data)
    }

    private static func messagingPathComponent(_ value: String) -> String {
        var allowed = CharacterSet.urlPathAllowed
        allowed.remove(charactersIn: "/?#")
        return value.addingPercentEncoding(withAllowedCharacters: allowed) ?? value
    }
}
