import AppKit
import EPUBContracts
import EPUBCLI
import EPUBInspection

@MainActor
final class PreflightViewController: NSViewController {
    private let openButton = NSButton(title: "选择 EPUB…", target: nil, action: nil)
    private let cleanupButton = NSButton(title: "执行 CSS Cleanup…", target: nil, action: nil)
    private let statusLabel = NSTextField(labelWithString: "选择一个 EPUB，执行只读 package inspection。")
    private let detailsView = NSTextView()
    private var selectedEPUBURL: URL?
    private var inspectionStatus: InspectionStatus?
    private var isCleanupRunning = false

    override func loadView() {
        view = NSView()
        configureInterface()
    }

    private func configureInterface() {
        openButton.target = self
        openButton.action = #selector(selectEPUB)
        openButton.bezelStyle = .rounded

        cleanupButton.target = self
        cleanupButton.action = #selector(runCSSCleanup)
        cleanupButton.bezelStyle = .rounded
        updateActionAvailability()

        statusLabel.font = .preferredFont(forTextStyle: .headline)
        statusLabel.maximumNumberOfLines = 0

        detailsView.isEditable = false
        detailsView.isSelectable = true
        detailsView.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        detailsView.string = "尚未读取文件。"
        let scrollView = NSScrollView()
        scrollView.hasVerticalScroller = true
        scrollView.documentView = detailsView

        let stack = NSStackView(views: [openButton, cleanupButton, statusLabel, scrollView])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 16
        stack.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 24),
            stack.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -24),
            stack.topAnchor.constraint(equalTo: view.topAnchor, constant: 24),
            stack.bottomAnchor.constraint(equalTo: view.bottomAnchor, constant: -24),
            scrollView.widthAnchor.constraint(equalTo: stack.widthAnchor),
            scrollView.heightAnchor.constraint(greaterThanOrEqualToConstant: 220),
        ])
    }

    @objc private func selectEPUB() {
        guard !isCleanupRunning else {
            return
        }
        let panel = NSOpenPanel()
        panel.allowsMultipleSelection = false
        panel.canChooseDirectories = false
        panel.allowedContentTypes = [.epub]
        guard panel.runModal() == .OK, let url = panel.url else {
            return
        }

        selectedEPUBURL = nil
        inspectionStatus = nil
        updateActionAvailability()
        statusLabel.stringValue = "正在读取 \(url.lastPathComponent)…"
        detailsView.string = ""
        Task.detached {
            let granted = url.startAccessingSecurityScopedResource()
            defer {
                if granted {
                    url.stopAccessingSecurityScopedResource()
                }
            }
            let report = PackageInspector.inspect(.init(uri: url, kind: .epub))
            if report.status == .pass {
                let detail = PreflightPresentation.summary(for: report)
                await self.completeInspection(report: report, selectedURL: url, status: "读取完成：\(url.lastPathComponent)", detail: detail)
            } else {
                let messages = report.findings.map(\.message).joined(separator: "\n")
                await self.completeInspection(report: report, selectedURL: nil, status: "读取失败：\(url.lastPathComponent)", detail: messages)
            }
        }
    }

    @objc private func runCSSCleanup() {
        guard let input = selectedEPUBURL,
              let inspectionStatus,
              CSSCleanupAvailability.isEnabled(reportStatus: inspectionStatus, hasInput: true),
              !isCleanupRunning
        else {
            return
        }

        let confirmation = NSAlert()
        confirmation.messageText = "执行原生 CSS Cleanup？"
        confirmation.informativeText = "将创建新的 EPUB，不会修改输入文件。输出前必须通过 preflight、CSS cleanup、文字/锚点和 package redline 四个原生 gate。"
        confirmation.addButton(withTitle: "继续")
        confirmation.addButton(withTitle: "取消")
        guard confirmation.runModal() == .alertFirstButtonReturn else {
            return
        }

        let panel = NSSavePanel()
        panel.allowedContentTypes = [.epub]
        panel.canCreateDirectories = true
        panel.isExtensionHidden = false
        panel.nameFieldStringValue = CSSCleanupRunPresentation.suggestedOutputName(for: input)
        guard panel.runModal() == .OK, let output = panel.url else {
            return
        }

        isCleanupRunning = true
        updateActionAvailability()
        statusLabel.stringValue = "正在清理 CSS：\(input.lastPathComponent)…"
        detailsView.string = ""
        let complete: @MainActor @Sendable (String, String) -> Void = { [weak self] status, detail in
            self?.completeCleanup(status: status, detail: detail)
        }
        Task.detached {
            let inputGranted = input.startAccessingSecurityScopedResource()
            let outputGranted = output.startAccessingSecurityScopedResource()
            defer {
                if inputGranted {
                    input.stopAccessingSecurityScopedResource()
                }
                if outputGranted {
                    output.stopAccessingSecurityScopedResource()
                }
            }
            guard inputGranted, outputGranted else {
                await complete(
                    "CSS Cleanup 无法访问所选文件。",
                    "input access: \(inputGranted); output access: \(outputGranted)"
                )
                return
            }
            do {
                let workspace = try Self.makeCleanupWorkspace()
                let report = await SwiftCLIService.normalizeCSS(
                    input: input,
                    output: output,
                    workspaceRoot: workspace
                )
                await complete(
                    report.status == .complete ? "CSS Cleanup 已完成：\(output.lastPathComponent)" : "CSS Cleanup 未完成：\(input.lastPathComponent)",
                    CSSCleanupRunPresentation.detail(for: report)
                )
            } catch {
                await complete("CSS Cleanup 无法创建 transaction workspace。", String(describing: error))
            }
        }
    }

    private func completeInspection(
        report: InspectionReport,
        selectedURL: URL?,
        status: String,
        detail: String
    ) {
        inspectionStatus = report.status
        selectedEPUBURL = selectedURL
        statusLabel.stringValue = status
        detailsView.string = detail
        updateActionAvailability()
    }

    private func completeCleanup(status: String, detail: String) {
        isCleanupRunning = false
        statusLabel.stringValue = status
        detailsView.string = detail
        updateActionAvailability()
    }

    private func updateActionAvailability() {
        openButton.isEnabled = !isCleanupRunning
        cleanupButton.isEnabled = !isCleanupRunning
            && inspectionStatus.map { CSSCleanupAvailability.isEnabled(reportStatus: $0, hasInput: selectedEPUBURL != nil) } == true
    }

    private nonisolated static func makeCleanupWorkspace() throws -> URL {
        guard let applicationSupport = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first else {
            throw CocoaError(.fileNoSuchFile)
        }
        let workspace = applicationSupport
            .appending(path: "CSSCleanup", directoryHint: .isDirectory)
            .appending(path: UUID().uuidString, directoryHint: .isDirectory)
        try FileManager.default.createDirectory(at: workspace, withIntermediateDirectories: true)
        return workspace
    }
}
