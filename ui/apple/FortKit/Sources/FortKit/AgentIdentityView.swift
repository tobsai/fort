import SwiftUI

/// Deterministic local fallback styling for an agent whose accepted identity
/// does not supply artwork. It never borrows Fort's product mark.
public enum AgentIdentityFallback {
    public static func initials(for name: String) -> String {
        let words = name.split { !$0.isLetter && !$0.isNumber }
        let initials = words.prefix(2).compactMap(\.first).map { String($0).uppercased() }.joined()
        return initials.isEmpty ? "A" : initials
    }

    /// FNV-1a is stable across launches, unlike Swift's intentionally seeded
    /// `Hasher`, so one agent keeps the same fallback treatment on every device.
    public static func paletteIndex(for identity: String, paletteCount: Int) -> Int {
        guard paletteCount > 0 else { return 0 }
        var hash: UInt64 = 14_695_981_039_346_656_037
        for byte in identity.utf8 {
            hash ^= UInt64(byte)
            hash = hash &* 1_099_511_628_211
        }
        return Int(hash % UInt64(paletteCount))
    }
}

public struct AgentIdentityView: View {
    private static let palette: [Color] = [
        Color(red: 0.18, green: 0.48, blue: 0.76),
        Color(red: 0.38, green: 0.32, blue: 0.74),
        Color(red: 0.58, green: 0.28, blue: 0.66),
        Color(red: 0.10, green: 0.55, blue: 0.53),
        Color(red: 0.22, green: 0.58, blue: 0.34),
        Color(red: 0.72, green: 0.42, blue: 0.16),
        Color(red: 0.70, green: 0.28, blue: 0.32),
        Color(red: 0.34, green: 0.43, blue: 0.54),
    ]

    private let name: String
    private let identityKey: String
    private let suppliedImageName: String?
    private let size: CGFloat
    private let decorative: Bool

    public init(
        name: String,
        identityKey: String,
        suppliedImageName: String? = nil,
        size: CGFloat = 38,
        decorative: Bool = true
    ) {
        self.name = name
        self.identityKey = identityKey
        self.suppliedImageName = suppliedImageName
        self.size = size
        self.decorative = decorative
    }

    public var body: some View {
        Group {
            if let suppliedImageName, !suppliedImageName.isEmpty {
                Image(suppliedImageName)
                    .resizable()
                    .scaledToFill()
            } else {
                Circle()
                    .fill(Self.palette[AgentIdentityFallback.paletteIndex(
                        for: identityKey,
                        paletteCount: Self.palette.count
                    )])
                    .overlay {
                        Text(AgentIdentityFallback.initials(for: name))
                            .font(.system(size: size * 0.34, weight: .bold, design: .rounded))
                            .foregroundStyle(.white)
                    }
            }
        }
        .frame(width: size, height: size)
        .clipShape(Circle())
        .accessibilityElement(children: .ignore)
        .accessibilityHidden(decorative)
        .accessibilityLabel(name)
    }
}
