//go:build appstore && darwin

package desktop

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>

// zen_bookmark_create writes a security-scoped bookmark for path into *out
// (malloc'd, *n bytes). It returns NULL or a malloc'd error message.
static char *zen_bookmark_create(const char *path, void **out, int *n) {
	@autoreleasepool {
		NSURL *u = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path] isDirectory:YES];
		NSError *err = nil;
		NSData *d = [u bookmarkDataWithOptions:NSURLBookmarkCreationWithSecurityScope
		        includingResourceValuesForKeys:nil relativeToURL:nil error:&err];
		if (d == nil) return strdup([[err localizedDescription] UTF8String]);
		*n = (int)[d length];
		*out = malloc(*n);
		memcpy(*out, [d bytes], *n);
		return NULL;
	}
}

// zen_bookmark_resolve starts access from data. *path is a malloc'd path, *stale
// is 1 for stale data, *token an opaque retained URL for zen_bookmark_stop.
static char *zen_bookmark_resolve(const void *data, int n, char **path, int *stale, void **token) {
	@autoreleasepool {
		NSData *d = [NSData dataWithBytes:data length:n];
		BOOL isStale = NO;
		NSError *err = nil;
		NSURL *u = [NSURL URLByResolvingBookmarkData:d
		        options:NSURLBookmarkResolutionWithSecurityScope|NSURLBookmarkResolutionWithoutUI
		        relativeToURL:nil bookmarkDataIsStale:&isStale error:&err];
		if (u == nil) return strdup([[err localizedDescription] UTF8String]);
		if (![u startAccessingSecurityScopedResource]) return strdup("access to the folder was refused");
		*path = strdup([[u path] UTF8String]);
		*stale = isStale ? 1 : 0;
		*token = (void *)CFBridgingRetain(u);
		return NULL;
	}
}

static void zen_bookmark_stop(void *token) {
	NSURL *u = (NSURL *)CFBridgingRelease(token);
	[u stopAccessingSecurityScopedResource];
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// cocoaBookmarks is the bookmarker of the App Store build: NSURL
// security-scoped bookmarks.
type cocoaBookmarks struct{}

func (cocoaBookmarks) Create(path string) ([]byte, error) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var out unsafe.Pointer
	var n C.int
	if e := C.zen_bookmark_create(cp, &out, &n); e != nil {
		defer C.free(unsafe.Pointer(e))
		return nil, errors.New(C.GoString(e))
	}
	defer C.free(out)
	return C.GoBytes(out, n), nil
}

func (cocoaBookmarks) Resolve(data []byte) (string, bool, func(), error) {
	if len(data) == 0 {
		return "", false, nil, ErrNoBookmark
	}
	var path *C.char
	var stale C.int
	var token unsafe.Pointer
	if e := C.zen_bookmark_resolve(unsafe.Pointer(&data[0]), C.int(len(data)), &path, &stale, &token); e != nil {
		defer C.free(unsafe.Pointer(e))
		// A bookmark that cannot be resolved (moved to the Trash, volume gone)
		// is treated like a stale one: the user is asked again.
		return "", true, noop, errors.Join(ErrStale, errors.New(C.GoString(e)))
	}
	defer C.free(unsafe.Pointer(path))
	return C.GoString(path), stale != 0, func() { C.zen_bookmark_stop(token) }, nil
}
