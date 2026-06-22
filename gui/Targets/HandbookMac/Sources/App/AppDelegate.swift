import AppKit

@main
@MainActor
enum HandbookMacApplication {
    private static let applicationDelegate = AppDelegate()

    static func main() {
        let application = NSApplication.shared
        application.setActivationPolicy(.regular)
        application.delegate = applicationDelegate
        application.run()
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    private var mainWindowController: MainWindowController?

    func applicationDidFinishLaunching(_ notification: Notification) {
        let controller = MainWindowController()
        mainWindowController = controller
        controller.showWindow(self)
        controller.window?.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        true
    }
}
