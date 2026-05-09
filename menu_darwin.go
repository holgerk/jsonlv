package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#import <Cocoa/Cocoa.h>

static void dot(NSColor *c, CGFloat x, CGFloat y, CGFloat r) {
    [c setFill];
    [[NSBezierPath bezierPathWithOvalInRect:NSMakeRect(x, y, r, r)] fill];
}
static void bar(CGFloat x, CGFloat y, CGFloat w) {
    [[NSColor colorWithWhite:1.0 alpha:0.82] setFill];
    [[NSBezierPath bezierPathWithRoundedRect:NSMakeRect(x, y, w, 22) xRadius:11 yRadius:11] fill];
}

void setupAppIcon() {
    NSSize sz = NSMakeSize(512, 512);
    NSImage *icon = [[NSImage alloc] initWithSize:sz];
    [icon lockFocus];

    // background
    NSBezierPath *bg = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(0,0,512,512) xRadius:100 yRadius:100];
    [[NSColor colorWithRed:0.06 green:0.09 blue:0.14 alpha:1.0] setFill];
    [bg fill];

    // inner card
    NSBezierPath *card = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(60,100,392,312) xRadius:18 yRadius:18];
    [[NSColor colorWithRed:0.11 green:0.15 blue:0.21 alpha:1.0] setFill];
    [card fill];

    CGFloat dx = 96, dr = 32, barX = 148;

    // INFO row
    dot([NSColor colorWithRed:0.24 green:0.73 blue:0.31 alpha:1.0], dx, 348, dr);
    bar(barX, 350, 240);

    // WARN row
    dot([NSColor colorWithRed:0.89 green:0.70 blue:0.25 alpha:1.0], dx, 284, dr);
    bar(barX, 286, 180);

    // ERROR row
    dot([NSColor colorWithRed:0.97 green:0.32 blue:0.29 alpha:1.0], dx, 220, dr);
    bar(barX, 222, 210);

    // DEBUG row
    dot([NSColor colorWithWhite:0.55 alpha:1.0], dx, 156, dr);
    bar(barX, 158, 150);

    [icon unlockFocus];
    [[NSApplication sharedApplication] setApplicationIconImage:icon];
}

// Declarations of Go-exported callbacks (defined in filecb_darwin.go).
extern void cMenuOpenFiles(void);
extern void cOpenFile(const char *path);
extern void cClearRecent(void);
extern void cRestartApp(void);
extern void cSaveWindowFrame(CGFloat x, CGFloat y, CGFloat w, CGFloat h);

// ── File menu handler ─────────────────────────────────────────────────────────

@interface JSONLVMenuHandler : NSObject
@property (nonatomic, copy) NSString *filePath;
@end
@implementation JSONLVMenuHandler
- (void)doOpen:(id)sender     { cMenuOpenFiles(); }
- (void)doOpenFile:(id)sender { cOpenFile([self.filePath UTF8String]); }
- (void)doClear:(id)sender    { cClearRecent(); }
- (void)doRestart:(id)sender  { cRestartApp(); }
@end

static JSONLVMenuHandler *gMenuHandler = nil;
static NSMenu            *gRecentMenu  = nil;

void rebuildRecentMenuC(const char *recentNL) {
    [gRecentMenu removeAllItems];
    NSString *joined = recentNL ? [NSString stringWithUTF8String:recentNL] : @"";
    NSArray<NSString*> *paths = joined.length > 0
        ? [joined componentsSeparatedByString:@"\n"] : @[];
    for (NSString *p in paths) {
        if (!p.length) continue;
        JSONLVMenuHandler *h = [JSONLVMenuHandler new];
        h.filePath = p;
        NSMenuItem *item = [[NSMenuItem alloc]
            initWithTitle:p.lastPathComponent
                   action:@selector(doOpenFile:)
            keyEquivalent:@""];
        item.target  = h;
        item.toolTip = p;
        [gRecentMenu addItem:item];
    }
    if (paths.count > 0) [gRecentMenu addItem:[NSMenuItem separatorItem]];
    NSMenuItem *clear = [[NSMenuItem alloc]
        initWithTitle:@"Liste leeren" action:@selector(doClear:) keyEquivalent:@""];
    clear.target = gMenuHandler;
    [gRecentMenu addItem:clear];
}

void setupFileMenu(const char *recentNL) {
    gMenuHandler = [JSONLVMenuHandler new];

    NSMenuItem *fileItem = [NSMenuItem new];
    [[NSApp mainMenu] insertItem:fileItem atIndex:1];
    NSMenu *fileMenu = [[NSMenu alloc] initWithTitle:@"Datei"];
    [fileItem setSubmenu:fileMenu];

    NSMenuItem *openItem = [[NSMenuItem alloc]
        initWithTitle:@"Öffnen…" action:@selector(doOpen:) keyEquivalent:@"o"];
    openItem.target = gMenuHandler;
    [fileMenu addItem:openItem];

    [fileMenu addItem:[NSMenuItem separatorItem]];

    NSMenuItem *recentItem = [[NSMenuItem alloc]
        initWithTitle:@"Zuletzt geöffnet" action:nil keyEquivalent:@""];
    [fileMenu addItem:recentItem];
    gRecentMenu = [[NSMenu alloc] initWithTitle:@"Zuletzt geöffnet"];
    [recentItem setSubmenu:gRecentMenu];

    rebuildRecentMenuC(recentNL);
}

// ── App delegate (saves window frame on quit) ─────────────────────────────────

@interface JSONLVAppDelegate : NSObject <NSApplicationDelegate>
@property (nonatomic, assign) NSWindow *window;
@end
@implementation JSONLVAppDelegate
- (void)applicationWillTerminate:(NSNotification *)n {
    NSRect f = self.window.frame;
    cSaveWindowFrame(f.origin.x, f.origin.y, f.size.width, f.size.height);
}
@end
static JSONLVAppDelegate *gAppDelegate = nil;

void installAppDelegate(void *winPtr) {
    gAppDelegate = [JSONLVAppDelegate new];
    gAppDelegate.window = (__bridge NSWindow*)winPtr;
    [NSApp setDelegate:gAppDelegate];
}

// ── Window geometry ───────────────────────────────────────────────────────────

void getWindowFrame(void *winPtr, CGFloat *x, CGFloat *y, CGFloat *w, CGFloat *h) {
    NSWindow *win = (__bridge NSWindow*)winPtr;
    NSRect f = win.frame;
    *x = f.origin.x;
    *y = f.origin.y;
    *w = f.size.width;
    *h = f.size.height;
}

void setWindowFrame(void *winPtr, CGFloat x, CGFloat y, CGFloat w, CGFloat h) {
    NSWindow *win = (__bridge NSWindow*)winPtr;
    [win setFrame:NSMakeRect(x, y, w, h) display:YES animate:NO];
}

// ── Reopen alert ──────────────────────────────────────────────────────────────

int showReopenAlert(int count) {
    NSAlert *alert = [NSAlert new];
    alert.messageText = @"Letzte Sitzung wiederherstellen?";
    alert.informativeText = [NSString stringWithFormat:
        @"%d zuletzt geöffnete Datei(en) erneut laden?", count];
    [alert addButtonWithTitle:@"Öffnen"];
    [alert addButtonWithTitle:@"Überspringen"];
    return ([alert runModal] == NSAlertFirstButtonReturn) ? 1 : 0;
}

// ── File pickers ──────────────────────────────────────────────────────────────

char* openMultipleFilesPicker(void) {
    NSOpenPanel *panel = [NSOpenPanel openPanel];
    panel.canChooseFiles          = YES;
    panel.canChooseDirectories    = NO;
    panel.allowsMultipleSelection = YES;
    panel.title = @"Log-Dateien öffnen";
    if ([panel runModal] == NSModalResponseOK) {
        NSMutableArray<NSString*> *paths = [NSMutableArray array];
        for (NSURL *url in panel.URLs) [paths addObject:url.path];
        return strdup([[paths componentsJoinedByString:@"\n"] UTF8String]);
    }
    return NULL;
}

char* openFilePicker(void) {
    NSOpenPanel *panel = [NSOpenPanel openPanel];
    panel.canChooseFiles        = YES;
    panel.canChooseDirectories  = NO;
    panel.allowsMultipleSelection = NO;
    panel.title = @"Lokale Datei auswählen";
    if ([panel runModal] == NSModalResponseOK) {
        NSString *path = [[[panel URLs] firstObject] path];
        return strdup([path UTF8String]);
    }
    return NULL;
}

void setupAppMenu() {
    NSMenu *menubar = [NSMenu new];
    [NSApp setMainMenu:menubar];

    // ── App menu ──────────────────────────────────────────────────────────────
    NSMenuItem *appItem = [NSMenuItem new];
    [menubar addItem:appItem];
    NSMenu *appMenu = [NSMenu new];
    [appItem setSubmenu:appMenu];
    NSString *name = [[NSProcessInfo processInfo] processName];

    JSONLVMenuHandler *appHandler = [JSONLVMenuHandler new];
    NSMenuItem *restartItem = [[NSMenuItem alloc]
        initWithTitle:@"Neu starten" action:@selector(doRestart:) keyEquivalent:@"r"];
    restartItem.target = appHandler;
    [appMenu addItem:restartItem];
    [appMenu addItem:[NSMenuItem separatorItem]];

    [appMenu addItemWithTitle:[@"Quit " stringByAppendingString:name]
                       action:@selector(terminate:)
                keyEquivalent:@"q"];

    // ── Edit menu ─────────────────────────────────────────────────────────────
    NSMenuItem *editItem = [NSMenuItem new];
    [menubar addItem:editItem];
    NSMenu *editMenu = [[NSMenu alloc] initWithTitle:@"Edit"];
    [editItem setSubmenu:editMenu];
    [editMenu addItemWithTitle:@"Undo"       action:@selector(undo:)      keyEquivalent:@"z"];
    [editMenu addItemWithTitle:@"Redo"       action:@selector(redo:)      keyEquivalent:@"Z"];
    [editMenu addItem:[NSMenuItem separatorItem]];
    [editMenu addItemWithTitle:@"Cut"        action:@selector(cut:)       keyEquivalent:@"x"];
    [editMenu addItemWithTitle:@"Copy"       action:@selector(copy:)      keyEquivalent:@"c"];
    [editMenu addItemWithTitle:@"Paste"      action:@selector(paste:)     keyEquivalent:@"v"];
    [editMenu addItemWithTitle:@"Select All" action:@selector(selectAll:) keyEquivalent:@"a"];
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

func setupAppMenu() { C.setupAppMenu() }
func setupAppIcon()  { C.setupAppIcon() }

func PickLocalFile() string {
	p := C.openFilePicker()
	if p == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(p))
	return C.GoString(p)
}

func PickMultipleFiles() []string {
	p := C.openMultipleFilesPicker()
	if p == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(p))
	s := C.GoString(p)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func SetupFileMenu(recent []string) {
	cs := C.CString(strings.Join(recent, "\n"))
	defer C.free(unsafe.Pointer(cs))
	C.setupFileMenu(cs)
}

func RebuildRecentMenu(recent []string) {
	cs := C.CString(strings.Join(recent, "\n"))
	defer C.free(unsafe.Pointer(cs))
	C.rebuildRecentMenuC(cs)
}

func InstallAppDelegate(winPtr unsafe.Pointer) {
	C.installAppDelegate(winPtr)
}

func GetWindowFrame(winPtr unsafe.Pointer) (x, y, w, h float64) {
	var cx, cy, cw, ch C.CGFloat
	C.getWindowFrame(winPtr, &cx, &cy, &cw, &ch)
	return float64(cx), float64(cy), float64(cw), float64(ch)
}

func SetWindowFrame(winPtr unsafe.Pointer, x, y, w, h float64) {
	C.setWindowFrame(winPtr, C.CGFloat(x), C.CGFloat(y), C.CGFloat(w), C.CGFloat(h))
}

func ShowReopenAlert(count int) bool {
	return C.showReopenAlert(C.int(count)) == 1
}
