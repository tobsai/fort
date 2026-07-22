// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "FortKit",
    platforms: [
        .iOS(.v16),
        .macOS(.v13),
        .watchOS(.v9),
    ],
    products: [
        .library(
            name: "FortKit",
            targets: ["FortKit"]
        ),
    ],
    targets: [
        .target(
            name: "FortKit"
        ),
        .executableTarget(
            name: "FortKitContractChecks",
            dependencies: ["FortKit"],
            path: "Tests/FortKitTests"
        ),
    ]
)
