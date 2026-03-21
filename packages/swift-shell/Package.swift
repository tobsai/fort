// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "FortShell",
    platforms: [
        .macOS(.v13)
    ],
    targets: [
        .executableTarget(
            name: "FortShell",
            path: "Sources/FortShell"
        )
    ]
)
