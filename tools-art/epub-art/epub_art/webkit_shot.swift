// epub-art page --engine webkit 的截图助手（macOS）。
// 用 WKWebView（Apple Books、Reeden 同系的 WebKit）按 CSS 视口加载页面，
// 截图后绘制成精确像素尺寸的 PNG。
//
// 用法: webkit_shot <page-url> <css-width> <css-height> <pixel-width> <pixel-height> <out.png> <wait-seconds>
import Cocoa
import WebKit

let args = CommandLine.arguments
guard args.count == 8,
      let pageURL = URL(string: args[1]),
      let cssW = Double(args[2]), let cssH = Double(args[3]),
      let pixelW = Int(args[4]), let pixelH = Int(args[5]),
      let wait = Double(args[7]) else {
    FileHandle.standardError.write("usage: webkit_shot <page-url> <css-w> <css-h> <px-w> <px-h> <out.png> <wait-s>\n".data(using: .utf8)!)
    exit(2)
}
let outPath = args[6]

func fail(_ message: String) -> Never {
    FileHandle.standardError.write((message + "\n").data(using: .utf8)!)
    exit(1)
}

let app = NSApplication.shared
app.setActivationPolicy(.accessory)
let frame = NSRect(x: 0, y: 0, width: cssW, height: cssH)
let web = WKWebView(frame: frame, configuration: WKWebViewConfiguration())
let window = NSWindow(contentRect: frame, styleMask: [.borderless], backing: .buffered, defer: false)
window.contentView = web
window.orderBack(nil)

final class Nav: NSObject, WKNavigationDelegate {
    var done = false
    var error: String?
    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) { done = true }
    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError err: Error) {
        error = err.localizedDescription; done = true
    }
    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError err: Error) {
        error = err.localizedDescription; done = true
    }
}
let nav = Nav()
web.navigationDelegate = nav
// 页面里的相对路径经 <base> 指向书里的文件，所以放开整个文件系统的只读访问。
web.loadFileURL(pageURL, allowingReadAccessTo: URL(fileURLWithPath: "/"))

func snapshot() {
    let cfg = WKSnapshotConfiguration()
    cfg.rect = frame
    cfg.snapshotWidth = NSNumber(value: Double(pixelW))  // 以点为单位，保证源图不低于目标像素
    web.takeSnapshot(with: cfg) { image, err in
        guard let image = image else { fail("snapshot failed: \(String(describing: err))") }
        guard let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: pixelW, pixelsHigh: pixelH,
                                         bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
                                         colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0) else {
            fail("cannot allocate bitmap")
        }
        rep.size = NSSize(width: pixelW, height: pixelH)
        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
        NSGraphicsContext.current?.imageInterpolation = .high
        image.draw(in: NSRect(x: 0, y: 0, width: pixelW, height: pixelH))
        NSGraphicsContext.restoreGraphicsState()
        guard let png = rep.representation(using: .png, properties: [:]) else { fail("PNG encoding failed") }
        do { try png.write(to: URL(fileURLWithPath: outPath)) } catch { fail("write failed: \(error)") }
        exit(0)
    }
}

let start = Date()
Timer.scheduledTimer(withTimeInterval: 0.1, repeats: true) { timer in
    let elapsed = Date().timeIntervalSince(start)
    if let error = nav.error { fail("page failed to load: \(error)") }
    if nav.done && elapsed >= wait {
        timer.invalidate()
        snapshot()
    } else if elapsed > wait + 20 {
        fail("timed out waiting for the page to load")
    }
}
app.run()
