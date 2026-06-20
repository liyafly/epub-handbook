import AppKit

@main
final class AppDelegate: NSObject, NSApplicationDelegate {
    private var mainWindowController: MainWindowController?

    func applicationDidFinishLaunching(_ notification: Notification) {
        let controller = MainWindowController()
        mainWindowController = controller
        controller.showWindow(self)
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        true
    }
}
