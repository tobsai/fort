import Foundation
import SwiftUI

public struct FortReleaseIdentity: Equatable, Sendable {
    public let marketingVersion: String?
    public let build: String?

    public init(marketingVersion: String?, build: String?) {
        self.marketingVersion = Self.nonEmpty(marketingVersion)
        self.build = Self.nonEmpty(build)
    }

    public init(bundle: Bundle) {
        self.init(
            marketingVersion: bundle.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String,
            build: bundle.object(forInfoDictionaryKey: "CFBundleVersion") as? String
        )
    }

    public static var current: FortReleaseIdentity {
        FortReleaseIdentity(bundle: .main)
    }

    public var displayText: String {
        guard let marketingVersion, let build else {
            return "Version unavailable"
        }
        return "Version \(marketingVersion) (\(build))"
    }

    private static func nonEmpty(_ value: String?) -> String? {
        let trimmed = value?.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed?.isEmpty == false ? trimmed : nil
    }
}

public struct FortReleaseIdentityView: View {
    private let identity: FortReleaseIdentity

    public init(identity: FortReleaseIdentity = FortReleaseIdentity.current) {
        self.identity = identity
    }

    public var body: some View {
        Text(identity.displayText)
            .font(.caption)
            .foregroundStyle(.secondary)
            .accessibilityIdentifier("fort.release.identity")
    }
}
