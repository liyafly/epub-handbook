import ProjectDescription

let project = Project(
    name: "EPUBHandbook",
    organizationName: "epub-handbook",
    packages: [
        .local(path: "../swift"),
    ],
    targets: [
        .target(
            name: "HandbookMac",
            destinations: .macOS,
            product: .app,
            bundleId: "local.epub-handbook.mac",
            deploymentTargets: .macOS("15.0"),
            infoPlist: .extendingDefault(with: [
                "CFBundleDisplayName": "EPUB Handbook",
                "NSPrincipalClass": "NSApplication",
                "NSMainStoryboardFile": "",
            ]),
            sources: ["Targets/HandbookMac/Sources/**"],
            resources: [],
            entitlements: .file(path: "Targets/HandbookMac/HandbookMac.entitlements"),
            dependencies: [
                .package(product: "EPUBContracts"),
                .package(product: "EPUBInspection"),
                .package(product: "EPUBRuntime"),
            ],
            settings: .settings(base: ["SWIFT_VERSION": "6.0"])
        ),
        .target(
            name: "HandbookMacTests",
            destinations: .macOS,
            product: .unitTests,
            bundleId: "local.epub-handbook.mac.tests",
            deploymentTargets: .macOS("15.0"),
            infoPlist: .default,
            sources: ["Targets/HandbookMacTests/Sources/**"],
            dependencies: [
                .target(name: "HandbookMac"),
                .package(product: "EPUBContracts"),
            ],
            settings: .settings(base: ["SWIFT_VERSION": "6.0"])
        )
    ]
)
