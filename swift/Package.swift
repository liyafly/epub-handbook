// swift-tools-version: 6.3

import PackageDescription

let package = Package(
    name: "EPUBHandbook",
    platforms: [.macOS(.v15), .iOS(.v18)],
    products: [
        .library(name: "EPUBContracts", targets: ["EPUBContracts"]),
        .library(name: "EPUBRuntime", targets: ["EPUBRuntime"]),
        .library(name: "EPUBArchive", targets: ["EPUBArchive"]),
        .library(name: "EPUBPackage", targets: ["EPUBPackage"]),
        .library(name: "EPUBInspection", targets: ["EPUBInspection"]),
        .library(name: "EPUBStructuredTransforms", targets: ["EPUBStructuredTransforms"]),
    ],
    dependencies: [
        .package(url: "https://github.com/weichsel/ZIPFoundation.git", from: "0.9.20"),
        .package(url: "https://github.com/scinfu/SwiftSoup.git", from: "2.13.5"),
    ],
    targets: [
        .target(name: "EPUBContracts"),
        .testTarget(name: "EPUBContractsTests", dependencies: ["EPUBContracts"]),
        .target(name: "EPUBRuntime", dependencies: ["EPUBContracts"]),
        .testTarget(name: "EPUBRuntimeTests", dependencies: ["EPUBRuntime"]),
        .target(
            name: "EPUBArchive",
            dependencies: [
                "EPUBContracts",
                .product(name: "ZIPFoundation", package: "ZIPFoundation"),
            ]
        ),
        .testTarget(
            name: "EPUBArchiveTests",
            dependencies: [
                "EPUBArchive",
                .product(name: "ZIPFoundation", package: "ZIPFoundation"),
            ]
        ),
        .target(name: "EPUBPackage", dependencies: ["EPUBContracts", "EPUBArchive"]),
        .testTarget(
            name: "EPUBPackageTests",
            dependencies: [
                "EPUBPackage",
                .product(name: "ZIPFoundation", package: "ZIPFoundation"),
            ]
        ),
        .target(name: "EPUBInspection", dependencies: ["EPUBContracts", "EPUBPackage"]),
        .testTarget(
            name: "EPUBInspectionTests",
            dependencies: [
                "EPUBInspection",
                .product(name: "ZIPFoundation", package: "ZIPFoundation"),
            ]
        ),
        .target(
            name: "EPUBStructuredTransforms",
            dependencies: [
                "EPUBContracts",
                .product(name: "SwiftSoup", package: "SwiftSoup"),
            ]
        ),
        .testTarget(name: "EPUBStructuredTransformsTests", dependencies: ["EPUBStructuredTransforms"]),
    ],
    swiftLanguageModes: [.v6]
)
