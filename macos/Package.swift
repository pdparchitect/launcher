// swift-tools-version: 6.2

import PackageDescription

let package = Package(
    name: "LauncherNative",
    platforms: [.macOS(.v26)],
    products: [
        .library(
            name: "LauncherNative",
            type: .static,
            targets: ["LauncherNative"]
        )
    ],
    targets: [
        .target(
            name: "LauncherNative",
            path: "Sources/LauncherNative"
        )
    ]
)
