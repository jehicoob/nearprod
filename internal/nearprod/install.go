package nearprod

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

func nativeOwnBinary(p string) bool {
	st, e := regularOrMissing(p)
	if e != nil || st == nil || st.Size() > 100<<20 {
		return false
	}
	b, e := os.ReadFile(p)
	return e == nil && hasKnownBinaryMarker(b) && st.Mode()&0111 != 0
}
func hasKnownBinaryMarker(b []byte) bool {
	return bytes.Contains(b, []byte(BinaryMarker)) || bytes.Contains(b, []byte(transitionalBinaryMarker))
}
func safeOwnedDir(p string) error {
	if e := noSymlinkAncestors(p); e != nil {
		return e
	}
	if e := os.MkdirAll(p, 0700); e != nil {
		return e
	}
	st, e := os.Lstat(p)
	if e != nil {
		return e
	}
	if !st.IsDir() || !ownedByUser(st) {
		return fail("DIRECTORY_OWNER", "La carpeta no pertenece a tu usuario.", 409)
	}
	current, e := filepath.EvalSymlinks(p)
	if e != nil {
		return e
	}
	for ; ; current = filepath.Dir(current) {
		st, e := os.Lstat(current)
		if e != nil {
			return e
		}
		if !st.IsDir() {
			return fail("DIRECTORY_OWNER", "La ruta de instalación contiene un elemento que no es una carpeta.", 409)
		}
		if st.Mode().Perm()&0022 != 0 && st.Mode()&os.ModeSticky == 0 {
			return fail("DIRECTORY_PERMISSIONS", "La ruta de instalación contiene una carpeta escribible por otros usuarios.", 409)
		}
		if parent := filepath.Dir(current); parent == current {
			break
		}
	}
	return nil
}
func noSymlinkAncestors(p string) error {
	p = filepath.Clean(p)
	for {
		st, e := os.Lstat(p)
		if e == nil && st.Mode()&os.ModeSymlink != 0 && !(runtime.GOOS == "darwin" && (p == "/var" || p == "/tmp" || p == "/etc")) {
			return fail("SYMLINK_PATH", "La ruta atraviesa un enlace simbólico; elige una carpeta real.", 409)
		}
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		parent := filepath.Dir(p)
		if parent == p {
			return nil
		}
		p = parent
	}
}
func copyFileNew(src, dst string, mode os.FileMode) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	if e == nil {
		e = out.Sync()
	}
	ce := out.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		_ = os.Remove(dst)
	}
	return e
}
func InstallBinary(home, source string, configureShell bool) (J, error) {
	return installBinaryVersion(home, source, configureShell, Version)
}

func installBinaryVersion(home, source string, configureShell bool, version string) (J, error) {
	return installBinaryVersionVerified(home, source, configureShell, version, nil)
}

func installBinaryVersionVerified(home, source string, configureShell bool, version string, verify func(string) error) (J, error) {
	source, err := filepath.EvalSymlinks(source)
	if err != nil {
		return nil, err
	}
	if _, managed := homebrewPrefix(source); managed {
		return nil, fail("PACKAGE_MANAGED", "NearProd está gestionado por Homebrew. Usa brew upgrade nearprod; no ejecutes nearprod install para crear otra copia.", 409)
	}
	if !nativeOwnBinary(source) {
		return nil, fail("BINARY_INVALID", "No es el ejecutable nativo NearProd de esta distribución.", 400)
	}
	base := filepath.Join(home, ".local", "share", "nearprod")
	bindir := filepath.Join(home, ".local", "bin")
	for _, dir := range []string{base, bindir, filepath.Join(base, "releases")} {
		if e := safeOwnedDir(dir); e != nil {
			return nil, e
		}
	}
	dest := filepath.Join(bindir, "nearprod")
	prior, e := regularOrMissing(dest)
	if e != nil {
		return nil, e
	}
	var priorBytes []byte
	priorMode := os.FileMode(0755)
	if prior != nil {
		priorBytes, e = os.ReadFile(dest)
		if e != nil {
			return nil, e
		}
		if !hasKnownBinaryMarker(priorBytes) && !bytes.Contains(priorBytes, []byte("# NearProd stable launcher")) {
			return nil, fail("INSTALL_FOREIGN", "No se sobrescribe un comando ajeno en "+dest, 409)
		}
		priorMode = prior.Mode().Perm()
	}
	// Prepare all shell validation before switching the executable.
	rc := filepath.Join(home, ".zshrc")
	var shell []byte
	var shellBackup any
	shellExists := false
	modifyShell := false
	if configureShell {
		var e error
		shell, e = readLimited(rc, 2<<20)
		if e != nil && !os.IsNotExist(e) {
			return nil, e
		}
		rcState, stateErr := regularOrMissing(rc)
		if stateErr != nil {
			return nil, stateErr
		}
		shellExists = rcState != nil
		if !bytes.Contains(shell, []byte("# NearProd: comando estable")) {
			re := regexp.MustCompile(`(?m)^\s*(alias\s+nearprod=|(function\s+)?nearprod\s*\(\)|function\s+nearprod\b)`)
			if re.Match(shell) {
				return nil, fail("SHELL_CONFLICT", ".zshrc ya define nearprod. Conserva esa configuración o instala sin --configure-shell y revisa tu alias.", 409)
			}
			modifyShell = true
		}
	}
	release := filepath.Join(base, "releases", version+"-"+token(6))
	if e = safeOwnedDir(release); e != nil {
		return nil, e
	}
	var previous any
	installationComplete := false
	preserveRecovery := false
	defer func() {
		if installationComplete || preserveRecovery {
			return
		}
		_ = os.RemoveAll(release)
		if p := str(previous); p != "" {
			_ = os.Remove(p)
		}
		if p := str(shellBackup); p != "" {
			_ = os.Remove(p)
		}
	}()
	copy := filepath.Join(release, "nearprod")
	if e = copyFileNew(source, copy, 0755); e != nil {
		return nil, e
	}
	st, e := os.Stat(copy)
	if e != nil || st.Size() == 0 {
		return nil, fail("INSTALL_COPY", "La copia no se completó.", 500)
	}
	checksum, e := streamHash(copy)
	if e != nil {
		return nil, e
	}
	temp := dest + "." + token(8) + ".tmp"
	if e = copyFileNew(copy, temp, 0755); e != nil {
		return nil, e
	}
	defer os.Remove(temp)
	current, e := regularOrMissing(dest)
	if e != nil {
		return nil, e
	}
	if (prior == nil) != (current == nil) {
		return nil, fail("INSTALL_CHANGED", "El ejecutable cambió durante la instalación; no se reemplazó.", 409)
	}
	if prior != nil {
		currentBytes, readErr := os.ReadFile(dest)
		if readErr != nil {
			return nil, readErr
		}
		if !bytes.Equal(currentBytes, priorBytes) {
			return nil, fail("INSTALL_CHANGED", "El ejecutable cambió durante la instalación; no se reemplazó.", 409)
		}
	}
	if prior != nil {
		backup := filepath.Join(base, "releases", "previous-"+token(8))
		if e = atomicBytes(backup, priorBytes, 0700); e != nil {
			return nil, e
		}
		previous = backup
	}
	if e = os.Rename(temp, dest); e != nil {
		return nil, e
	}
	shellChanged := false
	rollback := func(cause error) (J, error) {
		var rollbackErr error
		if prior != nil {
			rollbackErr = atomicBytes(dest, priorBytes, priorMode)
		} else {
			rollbackErr = os.Remove(dest)
			if os.IsNotExist(rollbackErr) {
				rollbackErr = nil
			}
		}
		if rollbackErr == nil && shellChanged {
			if shellExists {
				rollbackErr = atomicBytes(rc, shell, 0600)
			} else {
				rollbackErr = os.Remove(rc)
				if os.IsNotExist(rollbackErr) {
					rollbackErr = nil
				}
			}
		}
		if rollbackErr != nil {
			preserveRecovery = true
			return nil, detailed("INSTALL_ROLLBACK", "Falló la instalación y no se pudo restaurar automáticamente el ejecutable anterior.", 500, J{"cause": publicError(cause), "rollback": publicError(rollbackErr), "previous": previous})
		}
		if p := str(previous); p != "" {
			_ = os.Remove(p)
		}
		_ = os.RemoveAll(release)
		return nil, cause
	}
	if verify != nil {
		if e = verify(dest); e != nil {
			return rollback(e)
		}
	}
	if modifyShell {
		if len(shell) > 0 {
			p := rc + ".nearprod-" + token(8) + ".bak"
			if e = atomicBytes(p, shell, 0600); e != nil {
				return rollback(e)
			}
			shellBackup = p
		}
		block := `\n# NearProd: comando estable; binario Go, sin dependencia de Node\nexport PATH="$HOME/.local/bin:$PATH"\nnearprod() { "$HOME/.local/bin/nearprod" "$@"; }\n`
		b := append(shell, []byte(strings.ReplaceAll(block, `\n`, "\n"))...)
		if e = atomicBytes(rc, b, 0600); e != nil {
			return rollback(e)
		}
		shellChanged = true
	}
	record := J{"version": version, "language": "Go", "bin": dest, "release": release, "previous": previous, "sha256": checksum, "installedAt": now(), "requiresNode": false}
	if e = writeJSON(filepath.Join(base, "installation.json"), record); e != nil {
		return rollback(e)
	}
	installationComplete = true
	return merge(record, J{"shellBackup": shellBackup, "note": "No se tocó ~/.nearprod ni tus bases. Cierra el agente anterior antes de abrir la nueva versión. No activa startup; usa nearprod startup enable."}), nil
}

var _ = fmt.Sprintf
