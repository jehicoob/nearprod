package nearprod

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	updateAPIURL    = "https://api.github.com/repos/jehicoob/nearprod/releases/latest"
	updateAssetBase = "https://github.com/jehicoob/nearprod/releases/download"
	maxUpdateAPI    = 2 << 20
	maxChecksums    = 1 << 20
	maxArchive      = 100 << 20
	maxExpanded     = 150 << 20
)

var stableVersionRE = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var sha256RE = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
	State              string `json:"state"`
	Size               int64  `json:"size"`
}

type latestRelease struct {
	TagName    string         `json:"tag_name"`
	HTMLURL    string         `json:"html_url"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

type updatePlan struct {
	Current, Latest, ArchiveName, Provider string
	Archive, Checksums                     releaseAsset
	Available                              bool
	Summary                                J
}

type Updater struct {
	Client                     *http.Client
	APIURL, AssetBase          string
	Executable, Home, OS, Arch string
	AllowedOrigins             map[string]bool
}

func newUpdater() *Updater {
	return &Updater{
		Client:     newUpdateHTTPClient(),
		APIURL:     updateAPIURL,
		AssetBase:  updateAssetBase,
		Executable: binaryPath(),
		Home:       userHome(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		AllowedOrigins: map[string]bool{
			"https://api.github.com": true,
			"https://github.com":     true,
		},
	}
}

func newUpdateHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	return &http.Client{
		Timeout:   2 * time.Minute,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || req.URL.Scheme != "https" || req.URL.User != nil || (req.URL.Port() != "" && req.URL.Port() != "443") || !allowedUpdateHost(req.URL.Hostname()) {
				return fail("UPDATE_REDIRECT", "GitHub devolvió una redirección de descarga no permitida.", 502)
			}
			return nil
		},
	}
}

func allowedUpdateHost(host string) bool {
	return contains([]string{"api.github.com", "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com"}, host)
}

func parseStableVersion(value string) ([3]uint64, error) {
	parts := stableVersionRE.FindStringSubmatch(value)
	if parts == nil {
		return [3]uint64{}, fail("UPDATE_VERSION", "La actualización requiere versiones estables X.Y.Z.", 422)
	}
	var parsed [3]uint64
	for index := range parsed {
		value, err := strconv.ParseUint(parts[index+1], 10, 32)
		if err != nil {
			return [3]uint64{}, fail("UPDATE_VERSION", "La versión publicada está fuera del rango admitido.", 422)
		}
		parsed[index] = value
	}
	return parsed, nil
}

func compareStableVersions(a, b string) (int, error) {
	left, err := parseStableVersion(a)
	if err != nil {
		return 0, err
	}
	right, err := parseStableVersion(b)
	if err != nil {
		return 0, err
	}
	for index := range left {
		if left[index] < right[index] {
			return -1, nil
		}
		if left[index] > right[index] {
			return 1, nil
		}
	}
	return 0, nil
}

func updateProvider(executable, home string) string {
	if _, managed := homebrewPrefix(executable); managed {
		return "homebrew"
	}
	current, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "unmanaged"
	}
	manual, err := filepath.EvalSymlinks(filepath.Join(home, ".local", "bin", "nearprod"))
	if err == nil && current == manual {
		return "manual"
	}
	return "unmanaged"
}

func (u *Updater) requestBytes(ctx context.Context, address, accept string, limit int64) ([]byte, error) {
	parsed, err := url.Parse(address)
	if err != nil || parsed == nil {
		return nil, fail("UPDATE_URL", "La release contiene una URL no permitida.", 422)
	}
	origin := parsed.Scheme + "://" + parsed.Host
	if parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !u.AllowedOrigins[origin] {
		return nil, fail("UPDATE_URL", "La release contiene una URL no permitida.", 422)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "nearprod/"+Version)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := u.Client.Do(req)
	if err != nil {
		return nil, fail("UPDATE_NETWORK", "No se pudo consultar o descargar la release de GitHub.", 503)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fail("UPDATE_HTTP", fmt.Sprintf("GitHub respondió HTTP %d.", response.StatusCode), 502)
	}
	if response.ContentLength > limit {
		return nil, fail("UPDATE_SIZE", "El archivo de actualización supera el tamaño permitido.", 413)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fail("UPDATE_SIZE", "El archivo de actualización supera el tamaño permitido.", 413)
	}
	return data, nil
}

func assetDigest(asset releaseAsset) (string, error) {
	algorithm, value, ok := strings.Cut(asset.Digest, ":")
	if !ok || algorithm != "sha256" || !sha256RE.MatchString(value) {
		return "", fail("UPDATE_DIGEST", "GitHub no publicó un digest SHA-256 válido para "+asset.Name+".", 422)
	}
	return strings.ToLower(value), nil
}

func exactAsset(release latestRelease, name string, max int64) (releaseAsset, error) {
	var found *releaseAsset
	for index := range release.Assets {
		asset := release.Assets[index]
		if asset.Name != name {
			continue
		}
		if found != nil {
			return releaseAsset{}, fail("UPDATE_ASSET", "La release contiene assets duplicados.", 422)
		}
		found = &asset
	}
	if found == nil || found.State != "uploaded" || found.Size <= 0 || found.Size > max {
		return releaseAsset{}, fail("UPDATE_ASSET", "La release no contiene el asset válido "+name+".", 422)
	}
	if _, err := assetDigest(*found); err != nil {
		return releaseAsset{}, err
	}
	return *found, nil
}

func (u *Updater) validateAssetURL(asset releaseAsset, version string) error {
	expected := strings.TrimRight(u.AssetBase, "/") + "/v" + version + "/" + url.PathEscape(asset.Name)
	if asset.BrowserDownloadURL != expected {
		return fail("UPDATE_URL", "La URL del asset no corresponde a la release esperada.", 422)
	}
	return nil
}

func (u *Updater) Check(ctx context.Context) (updatePlan, error) {
	if !contains([]string{"darwin", "linux"}, u.OS) || !contains([]string{"amd64", "arm64"}, u.Arch) {
		return updatePlan{}, fail("UPDATE_PLATFORM", "No hay actualizaciones binarias para esta plataforma.", 400)
	}
	raw, err := u.requestBytes(ctx, u.APIURL, "application/vnd.github+json", maxUpdateAPI)
	if err != nil {
		return updatePlan{}, err
	}
	var release latestRelease
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err = decoder.Decode(&release); err != nil {
		return updatePlan{}, fail("UPDATE_RELEASE", "GitHub no devolvió una release estable válida.", 422)
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF || release.Draft || release.Prerelease || !strings.HasPrefix(release.TagName, "v") {
		return updatePlan{}, fail("UPDATE_RELEASE", "GitHub no devolvió una release estable válida.", 422)
	}
	latest := strings.TrimPrefix(release.TagName, "v")
	if _, err = parseStableVersion(latest); err != nil {
		return updatePlan{}, err
	}
	archiveName := fmt.Sprintf("nearprod_%s_%s_%s.tar.gz", latest, u.OS, u.Arch)
	archive, err := exactAsset(release, archiveName, maxArchive)
	if err != nil {
		return updatePlan{}, err
	}
	checksums, err := exactAsset(release, "SHA256SUMS.txt", maxChecksums)
	if err != nil {
		return updatePlan{}, err
	}
	if err = u.validateAssetURL(archive, latest); err != nil {
		return updatePlan{}, err
	}
	if err = u.validateAssetURL(checksums, latest); err != nil {
		return updatePlan{}, err
	}
	provider := updateProvider(u.Executable, u.Home)
	status := "available"
	available := false
	if _, err = parseStableVersion(Version); err != nil {
		status = "development-build"
	} else {
		comparison, compareErr := compareStableVersions(Version, latest)
		if compareErr != nil {
			return updatePlan{}, compareErr
		}
		switch comparison {
		case -1:
			available = true
		case 0:
			status = "up-to-date"
		case 1:
			status = "newer-than-release"
		}
	}
	instruction := "nearprod update"
	if provider == "homebrew" {
		instruction = "brew update && brew upgrade nearprod"
	} else if provider == "unmanaged" {
		instruction = "Instala primero el binario manual con nearprod install."
	}
	summary := J{
		"status": status, "currentVersion": Version, "latestVersion": latest,
		"available": available, "platform": u.OS + "/" + u.Arch, "asset": archiveName,
		"provider": provider, "canInstall": available && provider == "manual",
		"releaseUrl": release.HTMLURL, "instruction": instruction, "checkedAt": now(),
		"note": "La comprobación no descargó ni instaló el binario.",
	}
	return updatePlan{Current: Version, Latest: latest, ArchiveName: archiveName, Provider: provider, Archive: archive, Checksums: checksums, Available: available, Summary: summary}, nil
}

func checksumEntry(manifest []byte, name string) (string, error) {
	match := ""
	scanner := bufio.NewScanner(bytes.NewReader(manifest))
	scanner.Buffer(make([]byte, 1024), 64<<10)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || fields[1] != name {
			continue
		}
		if match != "" || !sha256RE.MatchString(fields[0]) {
			return "", fail("UPDATE_CHECKSUM", "SHA256SUMS.txt contiene una entrada inválida o duplicada.", 422)
		}
		match = strings.ToLower(fields[0])
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if match == "" {
		return "", fail("UPDATE_CHECKSUM", "SHA256SUMS.txt no contiene el archive esperado.", 422)
	}
	return match, nil
}

func (u *Updater) downloadFile(ctx context.Context, asset releaseAsset, destination string, limit int64) (string, error) {
	data, err := u.requestBytes(ctx, asset.BrowserDownloadURL, "application/octet-stream", limit)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	actual := hex.EncodeToString(digest[:])
	expected, err := assetDigest(asset)
	if err != nil {
		return "", err
	}
	if actual != expected {
		return "", fail("UPDATE_DIGEST", "La descarga no coincide con el digest publicado por GitHub.", 422)
	}
	if err = os.WriteFile(destination, data, 0600); err != nil {
		return "", err
	}
	return actual, nil
}

func extractUpdateBinary(archive, destination string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return fail("UPDATE_ARCHIVE", "El archive descargado no es gzip válido.", 422)
	}
	defer gz.Close()
	limited := &io.LimitedReader{R: gz, N: maxExpanded + 1}
	reader := tar.NewReader(limited)
	found, entries := false, 0
	var total int64
	for {
		header, nextErr := reader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return fail("UPDATE_ARCHIVE", "No se pudo leer el archive descargado.", 422)
		}
		entries++
		clean := path.Clean(header.Name)
		if entries > 64 || header.Name != clean || clean == "." || path.IsAbs(clean) || strings.HasPrefix(clean, "../") || strings.Contains(clean, `\`) || header.Size < 0 || header.Size > maxArchive {
			return fail("UPDATE_ARCHIVE", "El archive contiene una entrada no permitida.", 422)
		}
		total += header.Size
		if total > maxExpanded {
			return fail("UPDATE_ARCHIVE", "El contenido expandido supera el tamaño permitido.", 413)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA && header.Typeflag != tar.TypeDir {
			return fail("UPDATE_ARCHIVE", "El archive contiene enlaces o archivos especiales.", 422)
		}
		if clean != "nearprod" {
			continue
		}
		if found || header.Typeflag == tar.TypeDir || header.Size == 0 {
			return fail("UPDATE_ARCHIVE", "El archive no contiene un ejecutable NearProd único.", 422)
		}
		output, openErr := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
		if openErr != nil {
			return openErr
		}
		written, copyErr := io.Copy(output, reader)
		if copyErr == nil && written != header.Size {
			copyErr = io.ErrUnexpectedEOF
		}
		if copyErr == nil {
			copyErr = output.Sync()
		}
		closeErr := output.Close()
		if copyErr == nil {
			copyErr = closeErr
		}
		if copyErr != nil {
			_ = os.Remove(destination)
			return copyErr
		}
		if err = os.Chmod(destination, 0755); err != nil {
			return err
		}
		found = true
	}
	if !found || limited.N <= 0 {
		return fail("UPDATE_ARCHIVE", "El archive no contiene un ejecutable NearProd válido.", 422)
	}
	return nil
}

func verifyUpdateBinary(ctx context.Context, executable, version, platform string) (J, error) {
	if !nativeOwnBinary(executable) {
		return nil, fail("UPDATE_BINARY", "El ejecutable descargado no contiene la identidad nativa de NearProd.", 422)
	}
	result, err := (&ExecRunner{}).Run(ctx, executable, []string{"--identity"}, RunOptions{Exact: true, Limit: 64 << 10, Timeout: 15 * time.Second})
	if err != nil || result.Code != 0 {
		return nil, fail("UPDATE_BINARY", "El ejecutable descargado no pudo verificar su identidad.", 422)
	}
	identity, err := decodeObject([]byte(result.Stdout))
	if err != nil || str(identity["service"]) != "nearprod" || str(identity["version"]) != version || str(identity["marker"]) != BinaryMarker || str(identity["platform"]) != platform {
		return nil, fail("UPDATE_BINARY", "La identidad del ejecutable no coincide con la release y plataforma solicitadas.", 422)
	}
	return identity, nil
}

func (u *Updater) Apply(ctx context.Context, plan updatePlan) (J, error) {
	if !plan.Available {
		return plan.Summary, nil
	}
	if plan.Provider == "homebrew" {
		return plan.Summary, fail("PACKAGE_MANAGED", "NearProd está gestionado por Homebrew. Usa brew update && brew upgrade nearprod.", 409)
	}
	if plan.Provider != "manual" {
		return plan.Summary, fail("UPDATE_INSTALL_REQUIRED", "El autoactualizador solo reemplaza la instalación manual estable en ~/.local/bin/nearprod.", 409)
	}
	base := filepath.Join(u.Home, ".local", "share", "nearprod")
	if err := safeOwnedDir(base); err != nil {
		return nil, err
	}
	temporary, err := os.MkdirTemp(base, ".update-")
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(temporary, 0700); err != nil {
		_ = os.RemoveAll(temporary)
		return nil, err
	}
	defer os.RemoveAll(temporary)

	checksumsFile := filepath.Join(temporary, "SHA256SUMS.txt")
	if _, err = u.downloadFile(ctx, plan.Checksums, checksumsFile, maxChecksums); err != nil {
		return nil, err
	}
	manifest, err := os.ReadFile(checksumsFile)
	if err != nil {
		return nil, err
	}
	manifestDigest, err := checksumEntry(manifest, plan.ArchiveName)
	if err != nil {
		return nil, err
	}
	apiDigest, err := assetDigest(plan.Archive)
	if err != nil {
		return nil, err
	}
	if manifestDigest != apiDigest {
		return nil, fail("UPDATE_CHECKSUM", "El manifiesto y el digest de GitHub no coinciden para el archive.", 422)
	}
	archiveFile := filepath.Join(temporary, plan.ArchiveName)
	archiveDigest, err := u.downloadFile(ctx, plan.Archive, archiveFile, maxArchive)
	if err != nil {
		return nil, err
	}
	if archiveDigest != manifestDigest {
		return nil, fail("UPDATE_CHECKSUM", "El archive descargado no coincide con SHA256SUMS.txt.", 422)
	}
	candidate := filepath.Join(temporary, "nearprod")
	if err = extractUpdateBinary(archiveFile, candidate); err != nil {
		return nil, err
	}
	platform := u.OS + "/" + u.Arch
	identity, err := verifyUpdateBinary(ctx, candidate, plan.Latest, platform)
	if err != nil {
		return nil, err
	}
	candidateDigest, err := streamHash(candidate)
	if err != nil {
		return nil, err
	}
	var installedIdentity J
	installedDigest := ""
	record, err := installBinaryVersionVerified(u.Home, candidate, false, plan.Latest, func(installed string) error {
		installedIdentity, err = verifyUpdateBinary(ctx, installed, plan.Latest, platform)
		if err != nil {
			return err
		}
		installedDigest, err = streamHash(installed)
		if err != nil {
			return err
		}
		if installedDigest != candidateDigest {
			return fail("UPDATE_VERIFY", "La instalación no coincide byte por byte con el ejecutable verificado.", 500)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	installed := str(record["bin"])
	return merge(plan.Summary, J{
		"status": "updated", "available": false, "canInstall": false, "installed": true,
		"fromVersion": plan.Current, "currentVersion": plan.Latest,
		"sha256": installedDigest, "identity": installedIdentity, "downloadIdentity": identity,
		"bin": installed, "release": record["release"], "previous": record["previous"],
		"temporaryFilesRemoved": true,
		"note":                  "Binario actualizado y verificado. El agente que ya estaba ejecutándose no se reinició; en un momento sin tareas usa nearprod agent stop y nearprod ui.",
	}), nil
}

func confirmUpdate(input io.Reader, output io.Writer, current, latest string) (bool, error) {
	fmt.Fprintf(output, "Actualizar NearProd %s -> %s? Se conservará una copia anterior y no se reiniciará el agente. [s/N]: ", current, latest)
	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return contains([]string{"s", "si", "sí", "y", "yes"}, answer), nil
}
