package nearprod

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ConfigBackup includes metadata and credentials, NEVER live database files or Docker volumes.
func (s *Service) ConfigBackup(ctx context.Context) (J, error) {
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if e := s.Idle(); e != nil {
		return nil, e
	}
	home := s.Store.Home
	dir := filepath.Join(home, "backups", "config")
	if e := safeOwnedDir(dir); e != nil {
		return nil, e
	}
	file := filepath.Join(dir, "nearprod-config-"+time.Now().UTC().Format("20060102T150405")+"-"+token(6)+".tar.gz")
	f, e := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	good := false
	defer func() {
		f.Close()
		if !good {
			os.Remove(file)
		}
	}()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	names := []string{"config/catalog.json", "config/migration.json", "catalog.json"}
	for _, raw := range arr(s.Infra.State()["instances"]) {
		r := obj(raw)
		rel, e := filepath.Rel(home, s.Infra.SecretFile(r))
		if e != nil || !within(home, filepath.Join(home, rel)) {
			return nil, fail("CONFIG_PATH", "Vault fuera del HOME administrado.", 409)
		}
		names = append(names, filepath.ToSlash(rel))
	}
	manifest := J{"version": Version, "createdAt": now(), "containsSecrets": true, "containsDatabaseData": false, "files": J{}}
	for _, name := range unique(names) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		p := filepath.Join(home, filepath.FromSlash(name))
		if !within(home, p) {
			return nil, fail("BACKUP_PATH", "Ruta fuera del catálogo.", 409)
		}
		st, e := regularOrMissing(p)
		if e != nil {
			return nil, e
		}
		if st == nil {
			continue
		}
		data, e := readLimited(p, 16<<20)
		if e != nil {
			return nil, e
		}
		if e = tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(data)), ModTime: time.Now()}); e != nil {
			return nil, e
		}
		if _, e = tw.Write(data); e != nil {
			return nil, e
		}
		obj(manifest["files"])[name] = hash(string(data))
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if e = tw.WriteHeader(&tar.Header{Name: "MANIFEST.json", Mode: 0600, Size: int64(len(data))}); e != nil {
		return nil, e
	}
	if _, e = tw.Write(data); e != nil {
		return nil, e
	}
	if e = tw.Close(); e != nil {
		return nil, e
	}
	if e = gz.Close(); e != nil {
		return nil, e
	}
	if e = f.Sync(); e != nil {
		return nil, e
	}
	if e = f.Close(); e != nil {
		return nil, e
	}
	sum, e := streamHash(file)
	if e != nil {
		return nil, e
	}
	st, e := os.Stat(file)
	if e != nil {
		return nil, e
	}
	size := st.Size()
	good = true
	return J{"file": file, "sha256": sum, "bytes": size, "containsSecrets": true, "containsDatabaseData": false, "note": "Copia privada de configuración y credenciales. No sustituye exportar las bases SQL. No compartir este archivo."}, nil
}

var _ = fmt.Sprintf
var _ = io.EOF
var _ = strings.TrimSpace
