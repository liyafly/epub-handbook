import AppKit

final class MainWindowController: NSWindowController {
    init() {
        let contentViewController = PreflightViewController()
        let window = NSWindow(contentViewController: contentViewController)
        window.title = "EPUB Handbook"
        window.setContentSize(NSSize(width: 760, height: 480))
        window.minSize = NSSize(width: 620, height: 360)
        window.styleMask.insert(.fullSizeContentView)
        super.init(window: window)
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) {
        nil
    }
}
