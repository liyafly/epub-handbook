import Foundation

public enum OPFParseError: Error, Equatable, Sendable {
    case malformedXML
    case missingPackageElement
}

public struct OPFManifestItem: Hashable, Codable, Sendable {
    public let id: String
    public let href: String
    public let mediaType: String
    public let properties: Set<String>

    public init(id: String, href: String, mediaType: String, properties: Set<String> = []) {
        self.id = id
        self.href = href
        self.mediaType = mediaType
        self.properties = properties
    }
}

public struct OPFPackageSnapshot: Hashable, Codable, Sendable {
    public let uniqueIdentifier: String?
    public let coreMetadata: [String: [String]]
    public let manifest: [OPFManifestItem]
    public let spineItemIDs: [String]

    public init(
        uniqueIdentifier: String?,
        coreMetadata: [String: [String]] = [:],
        manifest: [OPFManifestItem],
        spineItemIDs: [String]
    ) {
        self.uniqueIdentifier = uniqueIdentifier
        self.coreMetadata = coreMetadata
        self.manifest = manifest
        self.spineItemIDs = spineItemIDs
    }

    public var navigationItem: OPFManifestItem? {
        manifest.first { $0.properties.contains("nav") }
    }
}

public enum OPFDocument {
    public static func parse(_ data: Data) throws -> OPFPackageSnapshot {
        let delegate = OPFDelegate()
        let parser = XMLParser(data: data)
        parser.delegate = delegate
        parser.shouldProcessNamespaces = false
        parser.shouldResolveExternalEntities = false
        guard parser.parse() else {
            throw OPFParseError.malformedXML
        }
        guard delegate.foundPackage else {
            throw OPFParseError.missingPackageElement
        }
        return OPFPackageSnapshot(
            uniqueIdentifier: delegate.uniqueIdentifier,
            coreMetadata: delegate.coreMetadata,
            manifest: delegate.manifest,
            spineItemIDs: delegate.spineItemIDs
        )
    }
}

private final class OPFDelegate: NSObject, XMLParserDelegate {
    private(set) var foundPackage = false
    private(set) var uniqueIdentifier: String?
    private(set) var coreMetadata = Dictionary(uniqueKeysWithValues: ["title", "creator", "identifier", "language"].map { ($0, [String]()) })
    private(set) var manifest: [OPFManifestItem] = []
    private(set) var spineItemIDs: [String] = []
    private var currentMetadataName: String?
    private var currentMetadataText = ""

    func parser(
        _ parser: XMLParser,
        didStartElement elementName: String,
        namespaceURI: String?,
        qualifiedName qName: String?,
        attributes attributeDict: [String: String] = [:]
    ) {
        switch elementName {
        case "package":
            foundPackage = true
            uniqueIdentifier = attributeDict["unique-identifier"]
        case "item":
            guard let id = attributeDict["id"],
                  let href = attributeDict["href"],
                  let mediaType = attributeDict["media-type"]
            else {
                return
            }
            let properties = Set((attributeDict["properties"] ?? "").split(whereSeparator: \.isWhitespace).map(String.init))
            manifest.append(.init(id: id, href: href, mediaType: mediaType, properties: properties))
        case "itemref":
            if let idref = attributeDict["idref"] {
                spineItemIDs.append(idref)
            }
        default:
            break
        }
        let localName = elementName.split(separator: ":").last.map(String.init) ?? elementName
        if coreMetadata[localName] != nil {
            currentMetadataName = localName
            currentMetadataText = ""
        }
    }

    func parser(_ parser: XMLParser, foundCharacters string: String) {
        currentMetadataText.append(string)
    }

    func parser(
        _ parser: XMLParser,
        didEndElement elementName: String,
        namespaceURI: String?,
        qualifiedName qName: String?
    ) {
        let localName = elementName.split(separator: ":").last.map(String.init) ?? elementName
        guard localName == currentMetadataName else {
            return
        }
        let normalized = currentMetadataText
            .components(separatedBy: .whitespacesAndNewlines)
            .filter { !$0.isEmpty }
            .joined(separator: " ")
            .precomposedStringWithCanonicalMapping
        if !normalized.isEmpty {
            coreMetadata[localName, default: []].append(normalized)
        }
        currentMetadataName = nil
        currentMetadataText = ""
    }
}
