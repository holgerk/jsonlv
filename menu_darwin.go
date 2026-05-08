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
import "unsafe"

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
