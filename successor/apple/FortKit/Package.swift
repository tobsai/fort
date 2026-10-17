// swift-tools-version: 6.0
import PackageDescription
let package = Package(
    name: "FortKit",
    platforms: [.macOS(.v14), .iOS(.v17)],
    products: [.library(name: "FortKit", targets: ["FortKit"])],
    targets: [.target(name: "FortKit", resources:[.process("Resources/InteractiveQA")]), .executableTarget(name: "FortNativeCheck", dependencies: ["FortKit"]), .testTarget(name: "FortKitTests", dependencies: ["FortKit"])]
)
