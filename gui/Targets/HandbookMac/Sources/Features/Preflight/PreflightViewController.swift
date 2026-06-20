import AppKit
import EPUBContracts
import EPUBInspection

@MainActor
final class PreflightViewController: NSViewController {
    private let openButton = NSButton(title: "选择 EPUB…", target: nil, action: nil)
    private let statusLabel = NSTextField(labelWithString: "选择一个 EPUB，执行只读 package inspection。")
    private let detailsView = NSTextView()

    override func loadView() {
        view = NSView()
        configureInterface()
    }

    private func configureInterface() {
        openButton.target = self
        openButton.action = #selector(selectEPUB)
        openButton.bezelStyle = .rounded

        statusLabel.font = .preferredFont(forTextStyle: .headline)
        statusLabel.maximumNumberOfLines = 0

        detailsView.isEditable = false
        detailsView.isSelectable = true
        detailsView.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        detailsView.string = "尚未读取文件。"
        let scrollView = NSScrollView()
        scrollView.hasVerticalScroller = true
        scrollView.documentView = detailsView

        let stack = NSStackView(views: [openButton, statusLabel, scrollView])
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
        let panel = NSOpenPanel()
        panel.allowsMultipleSelection = false
        panel.canChooseDirectories = false
        panel.allowedContentTypes = [.epub]
        guard panel.runModal() == .OK, let url = panel.url else {
            return
        }

        statusLabel.stringValue = "正在读取 \(url.lastPathComponent)…"
        detailsView.string = ""
        let render: @MainActor @Sendable (String, String) -> Void = { [weak self] status, detail in
            self?.statusLabel.stringValue = status
            self?.detailsView.string = detail
        }
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
                await render("读取完成：\(url.lastPathComponent)", detail)
            } else {
                let messages = report.findings.map(\.message).joined(separator: "\n")
                await render("读取失败：\(url.lastPathComponent)", messages)
            }
        }
    }
}
