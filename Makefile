SHELL := /bin/bash

build-macos:
	fyne-cross darwin -app-id "com.aengelberg.ttdl" -icon resources/ttdl1024x1024.png -name "TTDL" -arch='*'
	find fyne-cross/dist -name "TTDL.app" -type d -execdir sh -c 'zip -r "TTDL.zip" "{}"' \;

build-windows:
	fyne-cross windows -app-id "com.aengelberg.ttdl" -icon resources/ttdl1024x1024.png -name "TTDL.exe" -arch='amd64'

release:
	scripts/release.sh
