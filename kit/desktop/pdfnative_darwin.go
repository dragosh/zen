//go:build darwin && cgo

package desktop

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework ApplicationServices
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <ApplicationServices/ApplicationServices.h>

// zenFindWebView finds the first web view below v.
WKWebView *zenFindWebView(NSView *v) {
	if ([v isKindOfClass:[WKWebView class]]) return (WKWebView *)v;
	for (NSView *s in v.subviews) {
		WKWebView *w = zenFindWebView(s);
		if (w) return w;
	}
	return nil;
}

// zenNewPrintInfo is the paper setup of Print and Export PDF: portrait, the
// system's default paper, half-inch margins, fitted to the page width.
// window.print() does nothing in WKWebView, and Wails' own Print() forces landscape.
NSPrintInfo *zenNewPrintInfo(void) {
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
	return info;
}

@interface ZenPDFJob : NSObject
@property (atomic) int state; // 0 running, 1 written, 2 failed
@property (strong) NSWindow *window;
@property BOOL wasHidden;
@property CGFloat alpha;
@property BOOL ignores;
@end
@implementation ZenPDFJob
- (void)printOperationDidRun:(NSPrintOperation *)op success:(BOOL)ok contextInfo:(void *)ci {
	self.state = ok ? 1 : 2;
}
- (void)restore {
	if (self.wasHidden) {
		[self.window orderOut:nil];
		self.window.alphaValue = self.alpha;
		self.window.ignoresMouseEvents = self.ignores;
	}
}
@end
static ZenPDFJob *zenJob;

// zenSavePDF runs a save job (no panels) for the window's web view into path.
// 0: written; 1: no web view; 2: the operation failed.
static int zenSavePDF(void *window, const char *path, const char *title) {
	NSWindow *w = (__bridge NSWindow *)window;
	WKWebView *wv = zenFindWebView(w.contentView);
	if (wv == nil) return 1;
	NSPrintInfo *info = zenNewPrintInfo();
	info.jobDisposition = NSPrintSaveJob;
	NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
	[info.dictionary setObject:url forKey:NSPrintJobSavingURL];
	NSPrintOperation *po = [wv printOperationWithPrintInfo:info];
	po.showsPrintPanel = NO;
	po.showsProgressPanel = NO;
	po.jobTitle = [NSString stringWithUTF8String:title];
	// Without a frame the operation throws; the content is laid out for paper anyway.
	po.view.frame = wv.bounds;
	// A web view in a window that is not on screen is not laid out for printing (the job never
	// ends, with blank pages). A hidden window is shown fully transparent for the job.
	zenJob = [ZenPDFJob new];
	zenJob.window = w;
	zenJob.wasHidden = !w.visible;
	zenJob.alpha = w.alphaValue;
	zenJob.ignores = w.ignoresMouseEvents;
	if (zenJob.wasHidden) {
		w.alphaValue = 0;
		w.ignoresMouseEvents = YES;
		[w orderFrontRegardless];
	}
	[po runOperationModalForWindow:w delegate:zenJob didRunSelector:@selector(printOperationDidRun:success:contextInfo:) contextInfo:nil];
	return 0;
}

static void zenPDFRestore(void) { [zenJob restore]; }

static int zenPDFState(void) { return zenJob ? zenJob.state : 2; }

static int zenPDFPages(const char *path) {
	NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
	CGPDFDocumentRef doc = CGPDFDocumentCreateWithURL((__bridge CFURLRef)url);
	if (doc == NULL) return 0;
	int n = (int)CGPDFDocumentGetNumberOfPages(doc);
	CGPDFDocumentRelease(doc);
	return n;
}

static int zenHasWindowServer(void) {
	CFDictionaryRef d = CGSessionCopyCurrentDictionary();
	if (d == NULL) return 0;
	CFRelease(d);
	return 1;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// pdfJobTimeout bounds one save job.
const pdfJobTimeout = 60 * time.Second

func hasWindowServer() bool { return C.zenHasWindowServer() != 0 }

// nativeSavePDF runs the save job on the main thread, into a temporary file next to path.
func nativeSavePDF(w application.Window, path, title string) (int, error) {
	if w == nil {
		return 0, errors.New("no window to export")
	}
	ptr := w.NativeWindow()
	if ptr == nil {
		return 0, errors.New("the window is not ready")
	}
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".zenmd-export.pdf")
	_ = os.Remove(tmp)
	defer os.Remove(tmp)
	cp, ct := C.CString(tmp), C.CString(title)
	defer C.free(unsafe.Pointer(cp))
	defer C.free(unsafe.Pointer(ct))
	var code C.int
	application.InvokeSync(func() { code = C.zenSavePDF(ptr, cp, ct) })
	if code == 1 {
		return 0, errors.New("this window has nothing to export")
	}
	// The job runs on the main thread's run loop; wait for its delegate to say it ended.
	deadline := time.Now().Add(pdfJobTimeout)
	for {
		st := C.zenPDFState()
		if st == 1 {
			application.InvokeSync(func() { C.zenPDFRestore() })
			break
		}
		if st != 0 {
			return 0, errors.New("macOS could not write the PDF")
		}
		if time.Now().After(deadline) {
			return 0, errors.New("macOS did not finish writing the PDF")
		}
		time.Sleep(20 * time.Millisecond)
	}
	data, err := os.ReadFile(tmp)
	if err != nil || len(data) == 0 {
		return 0, errors.New("macOS wrote no PDF")
	}
	pages := int(C.zenPDFPages(cp))
	if pages == 0 {
		pages = countPDFPages(data)
	}
	if pages == 0 {
		return 0, errors.New("macOS wrote an empty PDF")
	}
	if err := os.Rename(tmp, path); err != nil {
		return 0, fmt.Errorf("cannot write %s: %v", filepath.Base(path), err)
	}
	return pages, nil
}
