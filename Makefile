BINARY     := jsonlv
APP        := jsonlv.app
INSTALL_DIR := /usr/local/bin

.PHONY: build app install clean

build:
	go build -o $(BINARY) .

app: build
	rm -rf $(APP)
	mkdir -p $(APP)/Contents/MacOS
	cp $(BINARY) $(APP)/Contents/MacOS/$(BINARY)
	@printf '<?xml version="1.0" encoding="UTF-8"?>\n\
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">\n\
<plist version="1.0">\n\
<dict>\n\
  <key>CFBundleName</key>             <string>Log Viewer</string>\n\
  <key>CFBundleDisplayName</key>      <string>Log Viewer</string>\n\
  <key>CFBundleIdentifier</key>       <string>de.kohnen.jsonlv</string>\n\
  <key>CFBundleVersion</key>          <string>1.0</string>\n\
  <key>CFBundlePackageType</key>      <string>APPL</string>\n\
  <key>CFBundleExecutable</key>       <string>$(BINARY)</string>\n\
  <key>NSHighResolutionCapable</key>  <true/>\n\
  <key>NSSupportsAutomaticGraphicsSwitching</key> <true/>\n\
</dict>\n\
</plist>\n' > $(APP)/Contents/Info.plist

install: app
	rm -rf /Applications/$(APP)
	cp -r $(APP) /Applications/$(APP)

clean:
	rm -rf $(BINARY) $(APP)
