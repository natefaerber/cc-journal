// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "CCJournal",
    platforms: [
        .macOS(.v14)
    ],
    targets: [
        .executableTarget(
            name: "CCJournal",
            path: "Sources/CCJournal"
        )
    ]
)
