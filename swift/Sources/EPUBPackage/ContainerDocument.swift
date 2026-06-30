import Foundation
#if canImport(FoundationXML)
import FoundationXML
#endif
import EPUBArchive

public enum PackageParseError: Error, Equatable, Sendable {
    case malformedXML
    case missingOPFRootfile
    case unsafeOPFPath
}

/// Reads the EPUB container document without extracting or trusting archive paths.
public enum ContainerDocument {
    public static func opfPath(from data: Data) throws -> ArchivePath {
        let delegate = RootfileDelegate()
        let parser = XMLParser(data: data)
        parser.delegate = delegate
        parser.shouldProcessNamespaces = false
        parser.shouldResolveExternalEntities = false

        guard parser.parse() else {
            throw PackageParseError.malformedXML
        }

        guard let fullPath = delegate.opfPath else {
            throw PackageParseError.missingOPFRootfile
        }

        do {
            return try ArchivePath(fullPath)
        } catch {
            throw PackageParseError.unsafeOPFPath
        }
    }
}

private final class RootfileDelegate: NSObject, XMLParserDelegate {
    private(set) var opfPath: String?

    func parser(
        _ parser: XMLParser,
        didStartElement elementName: String,
        namespaceURI: String?,
        qualifiedName qName: String?,
        attributes attributeDict: [String: String] = [:]
    ) {
        guard elementName == "rootfile",
              attributeDict["media-type"] == "application/oebps-package+xml",
              opfPath == nil
        else {
            return
        }

        opfPath = attributeDict["full-path"]
    }
}
