//go:build darwin && cgo

package desktop

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

static WKWebView *zenFindWebView(NSView *v) {
	if ([v isKindOfClass:[WKWebView class]]) return (WKWebView *)v;
	for (NSView *s in v.subviews) {
		WKWebView *w = zenFindWebView(s);
		if (w) return w;
	}
	return nil;
}

// zenPrintWindow shows the print panel as a sheet on the window, for its web
// view: portrait, half-inch margins, fitted to the page width. window.print()
// does nothing in WKWebView, and Wails' own Print() forces landscape.
static int zenPrintWindow(void *window, const char *title) {
	NSWindow *w = (__bridge NSWindow *)window;
	WKWebView *wv = zenFindWebView(w.contentView);
	if (wv == nil) return 1;
	NSPrintInfo *info = [[NSPrintInfo sharedPrintInfo] copy];
	info.orientation = NSPaperOrientationPortrait;
	info.horizontalPagination = NSPrintingPaginationModeFit;
	info.verticalPagination = NSPrintingPaginationModeAutomatic;
	info.horizontallyCentered = NO;
	info.verticallyCentered = NO;
	info.leftMargin = 36;
	info.rightMargin = 36;
	info.topMargin = 36;
	info.bottomMargin = 36;
	NSPrintOperation *po = [wv printOperationWithPrintInfo:info];
	po.showsPrintPanel = YES;
	po.showsProgressPanel = YES;
	po.jobTitle = [NSString stringWithUTF8String:title];
	po.printPanel.options |= NSPrintPanelShowsPaperSize | NSPrintPanelShowsOrientation | NSPrintPanelShowsScaling | NSPrintPanelShowsPreview;
	// Without a frame the operation throws; the content is laid out for paper anyway.
	po.view.frame = wv.bounds;
	[po runOperationModalForWindow:w delegate:nil didRunSelector:nil contextInfo:nil];
	return 0;
}
*/
import "C"

import (
	"errors"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// nativePrint opens the macOS print panel for w's web view, on the main thread.
func nativePrint(w application.Window, title string) error {
	ptr := w.NativeWindow()
	if ptr == nil {
		return errors.New("the window is not ready")
	}
	ct := C.CString(title)
	defer C.free(unsafe.Pointer(ct))
	var code C.int
	application.InvokeSync(func() { code = C.zenPrintWindow(ptr, ct) })
	if code != 0 {
		return errors.New("this window has nothing to print")
	}
	return nil
}
