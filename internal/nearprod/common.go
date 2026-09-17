// Package nearprod contains the local coordinator. No component starts Node.
package nearprod

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const SchemaVersion = 4
const TraefikImage = "traefik:v3.7.13"

// J preserves unknown legacy catalog properties. Runtime and boundary values are
// explicitly validated; migrations never deserialize/recreate resource identities.
type J map[string]any
type A = []any

func obj(v any) J {
	switch x := v.(type) {
	case J:
		return x
	case map[string]any:
		return J(x)
	}
	return J{}
}
func arr(v any) A {
	if x, ok := v.([]any); ok && x != nil {
		return x
	}
	if x, ok := v.([]string); ok {
		r := A{}
		for _, s := range x {
			r = append(r, s)
		}
		return r
	}
	return A{}
}
func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return string(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	}
	return ""
}
func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		n, _ := x.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(x, 64)
		return n
	}
	return 0
}
func integer(v any) int { return int(num(v)) }
func truth(v any) bool  { x, _ := v.(bool); return x }
func ss(v any) []string {
	r := []string{}
	for _, x := range arr(v) {
		r = append(r, str(x))
	}
	return r
}
func at(v any, path ...string) any {
	for _, p := range path {
		v = obj(v)[p]
	}
	return v
}
func text(v any, fallback string) string {
	if s := str(v); s != "" {
		return s
	}
	return fallback
}
func list(v any) A { return append(A{}, arr(v)...) }
func copyJ(v J) J {
	b, _ := json.Marshal(v)
	var out J
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	_ = decoder.Decode(&out)
	if out == nil {
		return J{}
	}
	return out
}
func merge(a, b J) J {
	out := copyJ(a)
	for k, v := range b {
		out[k] = v
	}
	return out
}
func keys(v J) []string {
	r := []string{}
	for k := range v {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}
func contains(xs []string, x string) bool {
	for _, a := range xs {
		if a == x {
			return true
		}
	}
	return false
}
func unique(xs []string) []string {
	r := []string{}
	for _, x := range xs {
		if !contains(r, x) {
			r = append(r, x)
		}
	}
	return r
}
func now() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") }
func token(n int) string {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func secretHex() string {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}

// hash is byte-compatible with the old JS helper for strings and arrays.
// Map fingerprints intentionally use deterministic Go key ordering and require review once.
func hash(v any) string {
	var b []byte
	if s, ok := v.(string); ok {
		b = []byte(s)
	} else {
		b, _ = json.Marshal(v)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func equalSecret(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
func homeDir() string {
	if h := os.Getenv("NEARPROD_HOME"); h != "" {
		p, _ := filepath.Abs(expandHome(h))
		return p
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".nearprod")
}
func userHome() string { h, _ := os.UserHomeDir(); return h }
func expandHome(p string) string {
	if p == "~" {
		return userHome()
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(userHome(), p[2:])
	}
	return p
}
func within(root, p string) bool {
	rel, e := filepath.Rel(root, p)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}
func bounded(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
func cloneStrings(xs []string) A {
	r := A{}
	for _, x := range xs {
		r = append(r, x)
	}
	return r
}
func slug(v string) string {
	r := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", "ü", "u").Replace(strings.ToLower(v))
	r = regexp.MustCompile(`[^a-z0-9_-]+`).ReplaceAllString(r, "-")
	r = strings.Trim(r, "-_")
	if len(r) > 48 {
		r = r[:48]
	}
	if r == "" {
		r = "proyecto"
	}
	return r
}

var safeTokenRE = regexp.MustCompile(`^[A-Za-z0-9_-]{8,80}$`)
var idRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,79}$`)
var svcRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)
var cidRE = regexp.MustCompile(`^[a-f0-9]{12,64}$`)

func validID(v string) bool { return idRE.MatchString(v) }
func requireText(v any, name string, max int) (string, error) {
	s, ok := v.(string)
	if !ok || len(s) == 0 || len(s) > max || strings.ContainsAny(s, "\x00\r\n") {
		return "", fail("INVALID_INPUT", name+" no válido.", 400)
	}
	return s, nil
}
func intRange(v any, min, max int, name string) (int, error) {
	f := num(v)
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) || f < float64(min) || f > float64(max) {
		return 0, fail("INVALID_NUMBER", fmt.Sprintf("%s: usa un entero entre %d y %d.", name, min, max), 400)
	}
	return int(f), nil
}

type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
	Details any    `json:"details,omitempty"`
}

func (e *AppError) Error() string { return e.Code + ": " + e.Message }
func fail(code, msg string, status int) *AppError {
	return &AppError{Code: code, Message: msg, Status: status}
}
func detailed(code, msg string, status int, d any) *AppError {
	return &AppError{Code: code, Message: msg, Status: status, Details: d}
}
func publicError(err error) J {
	if err == nil {
		return nil
	}
	var a *AppError
	if errors.As(err, &a) {
		m := J{"code": a.Code, "message": a.Message}
		if a.Details != nil {
			m["details"] = a.Details
		}
		return m
	}
	return J{"code": "INTERNAL_ERROR", "message": "No se completó la operación. Revisa el diagnóstico local."}
}
func statusCode(err error) int {
	var a *AppError
	if errors.As(err, &a) {
		return a.Status
	}
	return 500
}

// Atomic writes reject symlinks, preserve parent permissions and fsync before rename.
func regularOrMissing(file string) (os.FileInfo, error) {
	st, e := os.Lstat(file)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() {
		return nil, fail("UNSAFE_FILE", "No se sobrescriben enlaces ni archivos especiales: "+file, 409)
	}
	if !ownedByUser(st) {
		return nil, fail("FILE_OWNER", "El archivo no pertenece al usuario actual.", 409)
	}
	return st, nil
}
func privateDir(dir string) error {
	if e := noSymlinkAncestors(dir); e != nil {
		return e
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	st, e := os.Lstat(dir)
	if e != nil {
		return e
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || !ownedByUser(st) {
		return fail("UNSAFE_HOME", "La carpeta debe ser real y del usuario actual.", 409)
	}
	return os.Chmod(dir, 0700)
}
func atomicBytes(file string, data []byte, mode os.FileMode) error {
	if e := noSymlinkAncestors(filepath.Dir(file)); e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(file), 0700); e != nil {
		return e
	}
	if _, e := regularOrMissing(file); e != nil {
		return e
	}
	f, e := os.OpenFile(file+"."+token(6)+".tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(mode); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	if _, e = regularOrMissing(file); e != nil {
		return e
	}
	if e = os.Rename(tmp, file); e != nil {
		return e
	}
	if d, e := os.Open(filepath.Dir(file)); e == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
func writeJSON(file string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return atomicBytes(file, append(b, '\n'), 0600)
}
func readJSON(file string, max int64) (J, error) {
	st, e := regularOrMissing(file)
	if e != nil {
		return nil, e
	}
	if st == nil {
		return nil, os.ErrNotExist
	}
	if st.Size() > max {
		return nil, fail("FILE_LIMIT", "Archivo demasiado grande.", 413)
	}
	b, e := readLimited(file, max)
	if e != nil {
		return nil, e
	}
	return decodeObject(b)
}
func decodeObject(b []byte) (J, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v J
	if e := dec.Decode(&v); e != nil || v == nil {
		return nil, fail("INVALID_JSON", "Se esperaba un objeto JSON válido.", 400)
	}
	var rest any
	if e := dec.Decode(&rest); e != io.EOF {
		return nil, fail("INVALID_JSON", "JSON contiene datos adicionales.", 400)
	}
	return v, nil
}
func canonical(p string, roots []string, kind string) (string, error) {
	if len(p) > 4096 || strings.ContainsRune(p, 0) {
		return "", fail("INVALID_PATH", "Ruta inválida.", 400)
	}
	full, e := filepath.Abs(expandHome(p))
	if e != nil {
		return "", e
	}
	full, e = filepath.EvalSymlinks(full)
	if e != nil {
		return "", fail("PATH_MISSING", "La ruta no está disponible: "+p, 422)
	}
	if roots != nil {
		ok := false
		for _, root := range roots {
			if within(root, full) {
				ok = true
			}
		}
		if !ok {
			return "", fail("PATH_OUTSIDE_ROOT", "Ruta fuera de las raíces autorizadas.", 403)
		}
	}
	st, e := os.Stat(full)
	if e != nil {
		return "", e
	}
	if kind == "directory" && !st.IsDir() || kind == "file" && !st.Mode().IsRegular() {
		return "", fail("INVALID_PATH", "Tipo de archivo o carpeta no válido.", 400)
	}
	return full, nil
}
func sortedStrings(xs []string) []string { r := append([]string{}, xs...); sort.Strings(r); return r }
