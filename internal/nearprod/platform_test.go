package nearprod

import (
	"context"
	"strings"
	"testing"
)

func TestPlatformCapabilitiesByHost(t *testing.T) {
	native := J{"runtime": J{"kind": "native", "context": "default", "profile": "default"}}
	linux := platformCapabilitiesFor(native, "linux", "amd64", "wsl2")
	if str(linux["displayName"]) != "WSL2" || !contains(ss(at(linux, "runtime", "supportedKinds")), "native") || truth(at(linux, "runtime", "managedVirtualMachine")) {
		t.Fatal(linux)
	}
	if truth(at(linux, "packageManagement", "managed")) || truth(at(linux, "startup", "supported")) || !truth(at(linux, "proxy", "supported")) {
		t.Fatal(linux)
	}

	colima := J{"runtime": J{"kind": "colima", "context": "colima", "profile": "default"}}
	mac := platformCapabilitiesFor(colima, "darwin", "arm64", "macos")
	if str(mac["displayName"]) != "macOS" || !contains(ss(at(mac, "runtime", "supportedKinds")), "colima") || !contains(ss(at(mac, "runtime", "supportedKinds")), "native") || !truth(at(mac, "runtime", "managedVirtualMachine")) {
		t.Fatal(mac)
	}
	if str(at(mac, "packageManagement", "provider")) != "homebrew" || !truth(at(mac, "packageManagement", "managed")) || !truth(at(mac, "startup", "supported")) {
		t.Fatal(mac)
	}
	macNative := platformCapabilitiesFor(native, "darwin", "arm64", "macos")
	if !truth(at(macNative, "runtime", "supported")) || truth(at(macNative, "runtime", "managedVirtualMachine")) || str(at(macNative, "runtime", "displayName")) != "Docker nativo" {
		t.Fatal(macNative)
	}
}

func TestClassifyHostEnvironment(t *testing.T) {
	for _, tc := range []struct {
		platform string
		release  string
		want     string
	}{
		{"darwin", "", "macos"},
		{"linux", "6.6.87.2-microsoft-standard-WSL2", "wsl2"},
		{"linux", "4.4.0-Microsoft", "wsl"},
		{"linux", "6.8.0-generic", "linux"},
	} {
		if got := classifyHostEnvironment(tc.platform, tc.release); got != tc.want {
			t.Errorf("classifyHostEnvironment(%q, %q) = %q, want %q", tc.platform, tc.release, got, tc.want)
		}
	}
}

func TestPlatformToolPathIgnoresMacSelectionOnLinux(t *testing.T) {
	state := J{"toolPaths": J{"docker": "/opt/homebrew/bin/docker", "colima": "/opt/homebrew/bin/colima"}}
	if got := platformToolPath(state, "linux", "docker"); got != "" {
		t.Fatalf("Linux selected persisted macOS tool path %q", got)
	}
	if got := platformToolPath(state, "darwin", "docker"); got != "/opt/homebrew/bin/docker" {
		t.Fatalf("macOS tool selection changed: %q", got)
	}
}

func TestNativeLinuxSkipsMacOnlyTools(t *testing.T) {
	f := newFixture(t, false)
	f.S.Runtime.Platform = "linux"
	statuses := f.S.Runtime.ToolStatus(context.Background())
	for _, raw := range statuses {
		tool := obj(raw)
		if str(tool["id"]) == "colima" && (truth(tool["supported"]) || str(tool["status"]) != "not-applicable") {
			t.Fatal(tool)
		}
		if strings.Contains(strings.ToLower(str(tool["hint"])), "brew") {
			t.Fatal("Linux tool hint references Homebrew", tool)
		}
	}
	for _, call := range f.F.History() {
		if call.Name == "colima" {
			t.Fatal("Linux diagnostics executed Colima")
		}
	}
	before := len(f.F.History())
	updates, e := f.S.Runtime.Updates(context.Background())
	must(t, e)
	if str(updates["status"]) != "unsupported" || len(f.F.History()) != before {
		t.Fatal(updates)
	}
}

func TestRuntimeKindIsRestrictedByPlatform(t *testing.T) {
	f := newFixture(t, false)
	f.S.Runtime.Platform = "linux"
	_, e := f.S.SetRuntime(J{"kind": "colima", "context": "colima", "profile": "default", "platform": "darwin"})
	expectCode(t, e, "RUNTIME_PLATFORM")
	_, e = f.S.SetRuntime(J{"kind": "native", "context": "default", "profile": "default"})
	must(t, e)

	mac := newFixture(t, false)
	mac.S.Runtime.Platform = "darwin"
	_, e = mac.S.SetRuntime(J{"kind": "colima", "context": "colima", "profile": "default"})
	must(t, e)
	_, e = mac.S.SetRuntime(J{"kind": "native", "context": "default", "profile": "default"})
	must(t, e)
}

func TestUnsupportedSavedRuntimeDoesNotExecuteColima(t *testing.T) {
	f := newFixture(t, false)
	f.S.Runtime.Platform = "linux"
	must(t, f.S.Store.Update(func(v J) error {
		v["runtime"] = J{"kind": "colima", "context": "colima", "profile": "default"}
		return nil
	}))
	status := f.S.Runtime.Status(context.Background())
	if str(status["state"]) != "unsupported" {
		t.Fatal(status)
	}
	_, e := f.S.Runtime.Start(context.Background(), nil)
	expectCode(t, e, "RUNTIME_PLATFORM")
	_, e = f.S.Runtime.Preview(context.Background(), J{"memory": 2, "cpus": 2})
	expectCode(t, e, "RUNTIME_PLATFORM")
	_, e = f.S.Runtime.Configure(context.Background(), J{"confirm": true}, nil)
	expectCode(t, e, "RUNTIME_PLATFORM")
	_ = f.S.Runtime.Metrics(context.Background())
	for _, call := range f.F.History() {
		if call.Name == "colima" {
			t.Fatal("unsupported Linux runtime executed Colima")
		}
	}
}

func TestLinuxManagementActionsDoNotRunMacTools(t *testing.T) {
	f := newFixture(t, false)
	f.S.Runtime.Platform = "linux"
	before := len(f.F.History())
	_, e := f.S.Tools.Preview(context.Background(), J{"tool": "docker", "action": "install", "formula": "docker"})
	expectCode(t, e, "PACKAGE_PLATFORM")
	_, e = f.S.Tools.Apply(context.Background(), J{"tool": "docker", "action": "install", "formula": "docker", "confirm": true}, nil)
	expectCode(t, e, "PACKAGE_PLATFORM")
	startup := NewStartup(f.Home)
	startup.Platform = "linux"
	startup.Runner = f.F
	_, e = startup.Disable(context.Background())
	expectCode(t, e, "STARTUP_UNSUPPORTED")
	if len(f.F.History()) != before {
		t.Fatal("unsupported Linux management action executed a command")
	}
}

func TestCatalogExposesHostCapabilities(t *testing.T) {
	f := newFixture(t, false)
	f.S.Runtime.Platform = "linux"
	host := obj(f.S.Catalog()["host"])
	if str(host["os"]) != "linux" || !contains(ss(at(host, "runtime", "supportedKinds")), "native") || truth(at(host, "packageManagement", "canInstall")) {
		t.Fatal(host)
	}
}

func TestLinuxStartupHasNoManagedActions(t *testing.T) {
	startup := NewStartup(t.TempDir())
	startup.Platform = "linux"
	status, e := startup.Status(context.Background())
	must(t, e)
	if truth(status["supported"]) || str(status["manager"]) != "none" || len(arr(status["actions"])) != 0 {
		t.Fatal(status)
	}
}
