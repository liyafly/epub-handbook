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
        .library(name: "EPUBValidation", targets: ["EPUBValidation"]),
        .library(name: "EPUBStructuredTransforms", targets: ["EPUBStructuredTransforms"]),
        .library(name: "EPUBCLI", targets: ["EPUBCLI"]),
        .executable(name: "epub-handbook-swift", targets: ["EPUBHandbookSwiftCLI"]),
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
            name: "EPUBValidation",
            dependencies: [
                "EPUBArchive",
                "EPUBContracts",
                "EPUBPackage",
                .product(name: "SwiftSoup", package: "SwiftSoup"),
            ]
        ),
        .testTarget(
            name: "EPUBValidationTests",
            dependencies: [
                "EPUBValidation",
                .product(name: "ZIPFoundation", package: "ZIPFoundation"),
            ]
        ),
        .target(
            name: "EPUBStructuredTransforms",
            dependencies: [
                "EPUBContracts",
                "EPUBArchive",
                "EPUBPackage",
                .product(name: "SwiftSoup", package: "SwiftSoup"),
            ]
        ),
        .testTarget(name: "EPUBStructuredTransformsTests", dependencies: ["EPUBStructuredTransforms", "EPUBValidation"]),
        .target(
            name: "EPUBCLI",
            dependencies: [
                "EPUBContracts",
                "EPUBInspection",
                "EPUBRuntime",
                "EPUBStructuredTransforms",
                "EPUBValidation",
            ]
        ),
        .testTarget(name: "EPUBCLITests", dependencies: ["EPUBCLI", "EPUBContracts", "EPUBArchive", "ZIPFoundation"]),
        .executableTarget(name: "EPUBHandbookSwiftCLI", dependencies: ["EPUBCLI"]),
    ],
    swiftLanguageModes: [.v6]
)
