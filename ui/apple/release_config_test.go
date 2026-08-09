package apple_test

import (
	"os"
	"strings"
	"testing"
)

func TestMacReleaseEnablesHardenedRuntime(t *testing.T) {
	project, err := os.ReadFile("project.yml")
	if err != nil {
		t.Fatal(err)
	}
	macTarget := between(t, string(project), "  FortMac:\n", "  FortMacUITests:\n")
	if !strings.Contains(macTarget, "ENABLE_HARDENED_RUNTIME: YES") {
		t.Fatal("FortMac must enable the hardened runtime required by Apple notarization")
	}
}

func TestMacReleaseHardensBundledDaemon(t *testing.T) {
	makefile, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	macTarget := between(t, string(makefile), "mac-dmg:", "\nclean:")
	for _, required := range []string{
		"codesign --force --timestamp --options runtime",
		"$(MAC_EXPORT)/FortMac.app/Contents/Resources/fort",
		"codesign --verify --deep --strict",
	} {
		if !strings.Contains(macTarget, required) {
			t.Fatalf("mac-dmg must contain %q", required)
		}
	}
}

func TestMacReleaseSignsDiskImageBeforeNotarizing(t *testing.T) {
	makefile, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	macTarget := between(t, string(makefile), "mac-dmg:", "\nclean:")
	sign := strings.Index(macTarget, `codesign --force --timestamp --sign "$(DEVELOPER_ID)" $(MAC_DMG)`)
	submit := strings.Index(macTarget, "xcrun notarytool submit $(MAC_DMG)")
	if sign == -1 {
		t.Fatal("mac-dmg must Developer ID sign the disk image")
	}
	if submit == -1 || sign > submit {
		t.Fatal("mac-dmg must sign the disk image before notarization")
	}
}

func TestMacServiceControllerUsesPromotionPreservingLifecycle(t *testing.T) {
	controller, err := os.ReadFile("FortKit/Sources/FortKit/ServiceController.swift")
	if err != nil {
		t.Fatal(err)
	}
	service, err := os.ReadFile("../../cmd/fort/service.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{
		`run(["service", "install"])`,
		`run(["service", "restart"])`,
	} {
		if !strings.Contains(string(controller), command) {
			t.Fatalf("ServiceController must delegate %s through the bundled service contract", command)
		}
	}
	for _, required := range []string{
		`"FORT_PRIMARY_CHANNELS"`,
		`"FORT_ACCEPTED_SCHEDULE_INVENTORY"`,
		"prepareServiceInstall",
		"prepareServiceRestart",
	} {
		if !strings.Contains(string(service), required) {
			t.Fatalf("bundled service lifecycle must contain %q", required)
		}
	}
}

func between(t *testing.T, value, start, end string) string {
	t.Helper()
	startIndex := strings.Index(value, start)
	if startIndex == -1 {
		t.Fatalf("missing start marker %q", start)
	}
	endIndex := strings.Index(value[startIndex+len(start):], end)
	if endIndex == -1 {
		t.Fatalf("missing end marker %q", end)
	}
	return value[startIndex : startIndex+len(start)+endIndex]
}
