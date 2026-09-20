package nearprod

import (
	"os"
	"runtime"
	"strings"
)

func detectHostEnvironment(platform string) string {
	release := ""
	if platform == "linux" {
		raw, _ := os.ReadFile("/proc/sys/kernel/osrelease")
		release = string(raw)
	}
	return classifyHostEnvironment(platform, release)
}

func classifyHostEnvironment(platform, release string) string {
	switch platform {
	case "darwin":
		return "macos"
	case "linux":
		release = strings.ToLower(release)
		if strings.Contains(release, "wsl2") {
			return "wsl2"
		}
		if strings.Contains(release, "microsoft") {
			return "wsl"
		}
		return "linux"
	default:
		return platform
	}
}

func platformCapabilitiesFor(state J, platform, arch, environment string) J {
	if environment == "" {
		environment = detectHostEnvironment(platform)
	}
	kind := str(at(state, "runtime", "kind"))
	supportedKinds := A{}
	label := platform
	switch platform {
	case "darwin":
		supportedKinds = A{"colima", "native"}
		label = "macOS"
	case "linux":
		supportedKinds = A{"native"}
		label = "Linux"
		if environment == "wsl2" {
			label = "WSL2"
		} else if environment == "wsl" {
			label = "WSL"
		}
	}
	managedVM := platform == "darwin" && kind == "colima"
	runtimeName := "Docker nativo"
	if kind == "colima" {
		runtimeName = "Colima"
	}
	packageProvider := "none"
	packageName := "No administrado"
	if platform == "darwin" {
		packageProvider, packageName = "homebrew", "Homebrew"
	}
	return J{
		"os":           platform,
		"architecture": arch,
		"environment":  environment,
		"displayName":  label,
		"runtime": J{
			"kind":                  kind,
			"displayName":           runtimeName,
			"supportedKinds":        supportedKinds,
			"supported":             contains(ss(supportedKinds), kind),
			"managedVirtualMachine": managedVM,
			"canStart":              managedVM,
			"canConfigureResources": managedVM,
		},
		"packageManagement": J{
			"provider":        packageProvider,
			"displayName":     packageName,
			"managed":         platform == "darwin",
			"canInstall":      platform == "darwin",
			"canCheckUpdates": platform == "darwin",
		},
		"startup": J{
			"supported": platform == "darwin",
			"manager":   map[bool]string{true: "launchd", false: "none"}[platform == "darwin"],
		},
		"proxy": J{
			"supported": platform == "darwin" || platform == "linux",
			"location":  map[bool]string{true: "virtual-machine", false: "host"}[managedVM],
		},
	}
}

func platformCapabilities(state J) J {
	return platformCapabilitiesFor(state, runtime.GOOS, runtime.GOARCH, "")
}

func platformToolPath(state J, platform, name string) string {
	if platform != "darwin" {
		return ""
	}
	return str(at(state, "toolPaths", name))
}
