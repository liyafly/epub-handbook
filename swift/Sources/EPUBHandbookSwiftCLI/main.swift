import Darwin
import Foundation
import EPUBCLI

private struct CLIError: Encodable {
    let schemaVersion = "1"
    let status = "failed"
    let error: Detail

    struct Detail: Codable {
        let code: String
        let message: String
    }
}

private struct CLIHelp: Encodable {
    let schemaVersion = "1"
    let commands: [String]
}

@main
struct EPUBHandbookSwiftCLI {
    static func main() async {
        let exitCode = await run(Array(CommandLine.arguments.dropFirst()))
        exit(Int32(exitCode))
    }

    private static func run(_ arguments: [String]) async -> Int {
        guard let command = arguments.first else {
            return emitError(code: "usage", message: usage)
        }
        if command == "--help" || command == "-h" {
            emit(CLIHelp(commands: [usage]))
            return 0
        }
        let parsed: [String: String]
        do {
            parsed = try parseOptions(Array(arguments.dropFirst()))
        } catch {
            return emitError(code: "usage", message: String(describing: error))
        }
        guard parsed["format", default: "json"] == "json" else {
            return emitError(code: "format", message: "Only --format json is supported.")
        }

        switch command {
        case "inspect":
            guard let input = fileURL(parsed["input"]) else {
                return emitError(code: "input", message: "inspect requires --input <absolute path or file URI>.")
            }
            let report = SwiftCLIService.inspect(input: input)
            emit(report)
            return report.status == .fail ? 1 : 0

        case "validate-redlines":
            guard let before = fileURL(parsed["before"]), let after = fileURL(parsed["after"]) else {
                return emitError(code: "input", message: "validate-redlines requires --before and --after.")
            }
            do {
                let pathMap = try parsed["path-map"].map { value -> [String: String] in
                    guard let url = fileURL(value) else { throw OptionError.invalidPath("path-map") }
                    return try SwiftCLIService.loadPathMap(from: url)
                } ?? [:]
                let report = try SwiftCLIService.validateRedlines(
                    before: before,
                    after: after,
                    pathMap: pathMap,
                    allowStandardFontObfuscation: parsed["allow-standard-font-obfuscation"] == "true"
                )
                emit(report)
                return report.status == .complete ? 0 : 1
            } catch {
                return emitError(code: "validation", message: String(describing: error))
            }

        case "normalize-popup":
            return await normalizePopup(options: parsed)

        case "run":
            guard let capability = parsed["_positional"] else {
                return emitError(code: "capability", message: "run requires a capability id.")
            }
            guard capability == "epub.notes.popup.normalize" else {
                return emitError(code: "capability", message: "Swift CLI has not completed this capability: \(capability).")
            }
            return await normalizePopup(options: parsed)

        default:
            return emitError(code: "usage", message: usage)
        }
    }

    private static func normalizePopup(options: [String: String]) async -> Int {
        guard let input = fileURL(options["input"]),
              let output = fileURL(options["output"]),
              let workspace = fileURL(options["workspace"])
        else {
            return emitError(code: "input", message: "normalize-popup requires --input, --output, and --workspace.")
        }
        let report = await SwiftCLIService.normalizePopup(input: input, output: output, workspaceRoot: workspace)
        emit(report)
        return report.status == .complete ? 0 : 1
    }

    private static func parseOptions(_ arguments: [String]) throws -> [String: String] {
        var options: [String: String] = [:]
        var index = 0
        while index < arguments.count {
            let argument = arguments[index]
            if argument.hasPrefix("--") {
                let name = String(argument.dropFirst(2))
                if name == "allow-standard-font-obfuscation" {
                    options[name] = "true"
                    index += 1
                    continue
                }
                guard index + 1 < arguments.count, !arguments[index + 1].hasPrefix("--") else {
                    throw OptionError.missingValue(name)
                }
                options[name] = arguments[index + 1]
                index += 2
            } else if options["_positional"] == nil {
                options["_positional"] = argument
                index += 1
            } else {
                throw OptionError.unexpectedArgument(argument)
            }
        }
        return options
    }

    private static func fileURL(_ value: String?) -> URL? {
        guard let value, !value.isEmpty else { return nil }
        if value.hasPrefix("file://") { return URL(string: value) }
        guard value.hasPrefix("/") else { return nil }
        return URL(filePath: value)
    }

    private static func emit<T: Encodable>(_ value: T) {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        guard let data = try? encoder.encode(value) else { return }
        print(String(decoding: data, as: UTF8.self))
    }

    private static func emitError(code: String, message: String) -> Int {
        emit(CLIError(error: .init(code: code, message: message)))
        return 2
    }

    private enum OptionError: Error, CustomStringConvertible {
        case missingValue(String)
        case unexpectedArgument(String)
        case invalidPath(String)

        var description: String {
            switch self {
            case let .missingValue(option): "Missing value for --\(option)."
            case let .unexpectedArgument(argument): "Unexpected argument: \(argument)."
            case let .invalidPath(option): "--\(option) must be an absolute path or file URI."
            }
        }
    }

    private static let usage = "Usage: epub-handbook-swift inspect --input <path> --format json | validate-redlines --before <path> --after <path> [--path-map <json>] [--allow-standard-font-obfuscation] --format json | normalize-popup --input <path> --output <path> --workspace <directory> --format json | run epub.notes.popup.normalize --input <path> --output <path> --workspace <directory> --format json"
}
