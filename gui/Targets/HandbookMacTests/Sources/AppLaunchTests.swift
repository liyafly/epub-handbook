import CoreGraphics
import XCTest

@MainActor
final class AppLaunchTests: XCTestCase {
    func testTestHostStartsWithMainWindow() async {
        let hasVisibleWindow = await waitsForVisibleWindow(
            ownedBy: pid_t(ProcessInfo.processInfo.processIdentifier)
        )
        XCTAssertTrue(hasVisibleWindow)
    }

    private func waitsForVisibleWindow(ownedBy processIdentifier: pid_t) async -> Bool {
        let deadline = Date().addingTimeInterval(3)
        while Date() < deadline {
            let windows = CGWindowListCopyWindowInfo(
                [.optionOnScreenOnly, .excludeDesktopElements],
                kCGNullWindowID
            ) as? [[String: Any]] ?? []
            if windows.contains(where: { window in
                window[kCGWindowOwnerPID as String] as? pid_t == processIdentifier
            }) {
                return true
            }
            try? await Task.sleep(for: .milliseconds(100))
        }
        return false
    }
}
