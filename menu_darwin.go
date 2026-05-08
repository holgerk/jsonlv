package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

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

func setupAppMenu() {
	C.setupAppMenu()
}
