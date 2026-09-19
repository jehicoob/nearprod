package nearprod

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const launchMarker = "<!-- NearProd managed LaunchAgent v1 -->"
const BinaryMarker = "NearProd native executable; Go control plane; schema 4"

type Startup struct {
	Home, CatalogHome, Platform string
	UID                         int
	Runner                      Runner
	Executable                  string
}

func NewStartup(home string) *Startup {
	return &Startup{Home: userHome(), CatalogHome: home, Platform: runtime.GOOS, UID: os.Getuid(), Runner: &ExecRunner{}, Executable: binaryPath()}
}
func (s *Startup) Label() string { return "dev.nearprod.agent." + hash(s.CatalogHome)[:12] }
func (s *Startup) File() string {
	return filepath.Join(s.Home, "Library", "LaunchAgents", s.Label()+".plist")
}
func (s *Startup) Target() string { return "gui/" + strconv.Itoa(s.UID) + "/" + s.Label() }
func xmlText(v string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(v))
	return b.String()
}
func stablePath() string {
	dirs := []string{filepath.Join(userHome(), ".local", "bin"), "/opt/homebrew/bin", "/opt/homebrew/sbin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin"}
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.IsAbs(d) && !strings.Contains(d, "fnm_multishells") && !strings.ContainsAny(d, "\r\n\x00") {
			dirs = append(dirs, d)
		}
	}
	return strings.Join(unique(dirs), string(os.PathListSeparator))
}
func launchXML(label, bin, home, catalog string, environment J) string {
	env := merge(J{"HOME": home, "NEARPROD_HOME": catalog, "PATH": stablePath()}, environment)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + launchMarker + `\n<plist version="1.0"><dict><key>Label</key><string>` + xmlText(label) + `</string><key>ProgramArguments</key><array><string>` + xmlText(bin) + `</string><string>serve</string><string>--foreground</string><string>--launch-agent</string></array><key>RunAtLoad</key><true/><key>KeepAlive</key><false/><key>ProcessType</key><string>Background</string><key>ThrottleInterval</key><integer>30</integer><key>EnvironmentVariables</key><dict>`)
	for _, k := range keys(env) {
		b.WriteString("<key>" + xmlText(k) + "</key><string>" + xmlText(str(env[k])) + "</string>")
	}
	b.WriteString(`</dict><key>StandardOutPath</key><string>` + xmlText(filepath.Join(catalog, "startup.log")) + `</string><key>StandardErrorPath</key><string>` + xmlText(filepath.Join(catalog, "startup.log")) + `</string><key>Umask</key><integer>63</integer></dict></plist>` + "\n")
	return strings.ReplaceAll(b.String(), `\n<plist`, "\n<plist")
}
func (s *Startup) Status(ctx context.Context) (J, error) {
	if s.Platform != "darwin" {
		return J{"supported": false, "enabled": false, "manager": "none", "actions": A{}, "message": "NearProd no instala ni administra systemd o el inicio de WSL automáticamente."}, nil
	}
	st, e := regularOrMissing(s.File())
	if e != nil {
		return nil, e
	}
	if st != nil {
		b, e := readLimited(s.File(), 1<<20)
		if e != nil {
			return nil, e
		}
		if !strings.Contains(string(b), launchMarker) || !strings.Contains(string(b), "<string>"+s.Label()+"</string>") {
			return nil, fail("STARTUP_FOREIGN", "Ese LaunchAgent no pertenece a NearProd; no se modificará.", 409)
		}
	}
	var loaded any
	r, e := s.Runner.Run(ctx, "/bin/launchctl", []string{"print", s.Target()}, RunOptions{Timeout: 5 * time.Second})
	if e == nil {
		loaded = r.Code == 0
	}
	message := "Inicio automático deshabilitado."
	if st != nil {
		message = "Se inicia SOLO NearProd al entrar en macOS; no Colima ni recursos."
	}
	return J{"supported": true, "enabled": st != nil, "loaded": loaded, "manager": "launchd", "actions": A{"enable", "disable", "status"}, "label": s.Label(), "file": s.File(), "scope": "login", "version": Version, "message": message}, nil
}
func (s *Startup) Enable(ctx context.Context) (J, error) {
	if s.Platform != "darwin" {
		return nil, fail("STARTUP_UNSUPPORTED", "startup enable requiere macOS.", 400)
	}
	if s.UID <= 0 {
		return nil, fail("STARTUP_USER", "Usa tu usuario normal, sin sudo.", 400)
	}
	if _, e := s.Status(ctx); e != nil {
		return nil, e
	}
	if e := privateDir(s.CatalogHome); e != nil {
		return nil, e
	}
	bin, err := s.startupBinary()
	if err != nil {
		return nil, err
	}
	env := J{}
	for _, k := range []string{"COLIMA_HOME", "DOCKER_CONFIG", "XDG_CONFIG_HOME", "XDG_DATA_HOME"} {
		v := os.Getenv(k)
		if filepath.IsAbs(v) && !strings.ContainsAny(v, "\n\r\x00") {
			env[k] = v
		}
	}
	content := launchXML(s.Label(), bin, s.Home, s.CatalogHome, env)
	prior, e := readLimited(s.File(), 1<<20)
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	if e = atomicBytes(s.File(), []byte(content), 0600); e != nil {
		return nil, e
	}
	restore := func() {
		if prior == nil {
			_ = os.Remove(s.File())
		} else {
			_ = atomicBytes(s.File(), prior, 0600)
		}
	}
	if _, e = checked(ctx, s.Runner, "/bin/launchctl", []string{"enable", s.Target()}, RunOptions{}); e != nil {
		restore()
		return nil, e
	}
	res, e := s.Runner.Run(ctx, "/bin/launchctl", []string{"print", s.Target()}, RunOptions{})
	if e != nil {
		restore()
		return nil, e
	}
	if res.Code != 0 {
		if _, e = checked(ctx, s.Runner, "/bin/launchctl", []string{"bootstrap", fmt.Sprintf("gui/%d", s.UID), s.File()}, RunOptions{}); e != nil {
			restore()
			return nil, e
		}
	}
	status, e := s.Status(ctx)
	return merge(status, J{"stableLauncher": true, "note": "No se detiene el agente actual. Si ya estaba cargado, la nueva configuración se aplica al siguiente login; no se duplican escritores."}), e
}
func (s *Startup) Disable(ctx context.Context) (J, error) {
	status, e := s.Status(ctx)
	if e != nil {
		return nil, e
	}
	if !truth(status["supported"]) {
		return nil, fail("STARTUP_UNSUPPORTED", "startup disable requiere macOS.", 400)
	}
	if !truth(status["enabled"]) {
		return merge(status, J{"noOp": true}), nil
	}
	if _, e = checked(ctx, s.Runner, "/bin/launchctl", []string{"disable", s.Target()}, RunOptions{}); e != nil {
		return nil, e
	}
	if e = os.Remove(s.File()); e != nil {
		return nil, e
	}
	status, e = s.Status(ctx)
	return merge(status, J{"note": "Deshabilitado para futuros logins. No detiene el agente actual ni tus contenedores."}), e
}
