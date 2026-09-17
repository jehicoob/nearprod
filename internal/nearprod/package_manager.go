package nearprod

import (
	"os"
	"path/filepath"
	"strings"
)

// Recognize a real Homebrew keg without invoking brew or trusting PATH.
// This does not grant permission to modify that package manager's files.
func homebrewPrefix(executable string) (string, bool) {
	real, err := filepath.EvalSymlinks(executable)
	if err != nil || !filepath.IsAbs(real) {
		return "", false
	}
	marker := string(os.PathSeparator) + filepath.Join("Cellar", "nearprod") + string(os.PathSeparator)
	i := strings.LastIndex(real, marker)
	if i < 0 {
		return "", false
	}
	tail := strings.Split(real[i+len(marker):], string(os.PathSeparator))
	if len(tail) != 3 || tail[0] == "" || tail[1] != "bin" || tail[2] != "nearprod" {
		return "", false
	}
	prefix := real[:i]
	if prefix == "" {
		prefix = string(os.PathSeparator)
	}
	return prefix, true
}

func (s *Startup) startupBinary() (string, error) {
	if prefix, managed := homebrewPrefix(s.Executable); managed {
		// opt survives upgrades; the versioned Cellar path does not.
		stable := filepath.Join(prefix, "opt", "nearprod", "bin", "nearprod")
		current, err := filepath.EvalSymlinks(s.Executable)
		if err != nil {
			return "", err
		}
		target, err := filepath.EvalSymlinks(stable)
		if err != nil || target != current || !nativeOwnBinary(target) {
			return "", fail("HOMEBREW_LINK", "Abre la versión vigente de Homebrew; su enlace opt debe apuntar a ese ejecutable.", 409)
		}
		return stable, nil
	}
	manual := filepath.Join(s.Home, ".local", "bin", "nearprod")
	if !nativeOwnBinary(manual) {
		return "", fail("INSTALL_REQUIRED", "Instala el binario manual o usa el ejecutable de Homebrew. No se fijan rutas temporales de Descargas.", 409)
	}
	return manual, nil
}
