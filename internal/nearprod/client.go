package nearprod

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func validatedAgentInfo(home string) (J, error) {
	v, e := readJSON(filepath.Join(home, "agent.json"), 16384)
	if e != nil {
		return nil, e
	}
	port := integer(v["port"])
	u, e := url.Parse(str(v["url"]))
	if e != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || u.Port() != strconv.Itoa(port) || port < 1 || port > 65535 || len(str(v["token"])) < 16 {
		return nil, fail("AGENT_METADATA", "Metadatos del agente inválidos. No se enviará el token a otra dirección.", 409)
	}
	return v, nil
}
func localClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func requestAPI(ctx context.Context, info J, route string, body any, timeout time.Duration) (J, error) {
	if !strings.HasPrefix(route, "/") || strings.HasPrefix(route, "//") {
		return nil, fail("API_PATH", "Ruta API inválida.", 400)
	}
	method := http.MethodGet
	var data io.Reader
	if body != nil {
		method = http.MethodPost
		b, e := json.Marshal(body)
		if e != nil {
			return nil, e
		}
		data = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, str(info["url"])+"/api"+route, data)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", "Bearer "+str(info["token"]))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, e := localClient(timeout).Do(req)
	if e != nil {
		return nil, fail("AGENT_OFFLINE", "No se pudo contactar con el agente local.", 503)
	}
	defer response.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if e != nil {
		return nil, e
	}
	v, e := decodeObject(raw)
	if e != nil {
		return nil, e
	}
	if response.StatusCode >= 400 {
		x := obj(v["error"])
		return nil, detailed(text(x["code"], "API_ERROR"), text(x["message"], "Error de la API local."), response.StatusCode, x["details"])
	}
	return v, nil
}
func readLive(ctx context.Context, home string, allowMismatch bool) (J, error) {
	info, e := validatedAgentInfo(home)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	res, e := requestAPI(ctx, info, "/health", nil, 2*time.Second)
	if e != nil {
		if str(publicError(e)["code"]) == "AGENT_OFFLINE" {
			return nil, nil
		}
		return nil, e
	}
	if str(res["service"]) != "nearprod" {
		return nil, fail("AGENT_IDENTITY", "El puerto guardado no sirve NearProd. No se iniciará otro escritor.", 409)
	}
	if _, e = requestAPI(ctx, info, "/catalog", nil, 3*time.Second); e != nil {
		return nil, fail("AGENT_AUTH", "El agente rechaza los metadatos guardados; no se iniciará otro sobre el catálogo.", 409)
	}
	if !allowMismatch && str(res["version"]) != Version {
		return nil, fail("AGENT_VERSION_MISMATCH", fmt.Sprintf("Agente %s, CLI %s. Ejecuta nearprod agent stop y luego nearprod ui; los datos permanecen.", str(res["version"]), Version), 409)
	}
	info["version"] = res["version"]
	info["agentUpdateProtocol"] = res["agentUpdateProtocol"]
	return info, nil
}
func EnsureAgent(ctx context.Context, home string, start, allowMismatch bool) (J, error) {
	info, e := readLive(ctx, home, allowMismatch)
	if e != nil || info != nil {
		return info, e
	}
	if !start {
		return nil, fail("AGENT_OFFLINE", "NearProd está detenido.", 503)
	}
	return startAgentBinary(ctx, home, binaryPath(), Version, 0)
}

// An updater continues running the old image after replacing its file. Always
// launch the verified installed path and check the new process's live version.
func startAgentBinary(ctx context.Context, home, executable, version string, port int) (J, error) {
	if info, err := readLive(ctx, home, true); err != nil || info != nil {
		if err == nil && str(info["version"]) != version {
			return nil, fail("AGENT_VERSION_MISMATCH", "El agente activo todavía no utiliza la versión instalada.", 409)
		}
		return info, err
	}
	if e := privateDir(home); e != nil {
		return nil, e
	}
	logpath := filepath.Join(home, "agent.log")
	if _, e := regularOrMissing(logpath); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(logpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	cmd := exec.Command(executable, "serve", "--foreground", "--port", strconv.Itoa(port))
	cmd.Env = append(os.Environ(), "NEARPROD_HOME="+home)
	cmd.Dir = home
	cmd.Stdout = f
	cmd.Stderr = f
	detachProcess(cmd)
	if e = cmd.Start(); e != nil {
		return nil, e
	}
	go func() { _ = cmd.Wait() }()
	for n := 0; n < 100; n++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
		info, e := readLive(ctx, home, true)
		if e != nil || info != nil {
			if e == nil && str(info["version"]) != version {
				return nil, fail("AGENT_VERSION_MISMATCH", "El agente reiniciado no coincide con la versión instalada.", 409)
			}
			return info, e
		}
	}
	return nil, fail("AGENT_START_FAILED", "No inició el agente. Consulta "+logpath+"; no borres el catálogo.", 503)
}
func waitOperation(ctx context.Context, info, op J, onLine func(string)) (J, error) {
	last := ""
	for contains([]string{"queued", "running"}, str(op["state"])) {
		lines := arr(op["lines"])
		start := 0
		if last != "" {
			for n, v := range lines {
				l := obj(v)
				if str(l["time"])+":"+str(l["text"]) == last {
					start = n + 1
				}
			}
		}
		if onLine != nil {
			for _, v := range lines[start:] {
				onLine(str(obj(v)["text"]))
			}
		}
		if len(lines) > 0 {
			l := obj(lines[len(lines)-1])
			last = str(l["time"]) + ":" + str(l["text"])
		}
		select {
		case <-ctx.Done():
			return nil, fail("CLI_INTERRUPTED", "La terminal dejó de esperar; la operación puede seguir. Usa nearprod operation/cancel con su ID.", 409)
		case <-time.After(250 * time.Millisecond):
		}
		var e error
		op, e = requestAPI(ctx, info, "/operations/"+url.PathEscape(str(op["id"])), nil, 10*time.Second)
		if e != nil {
			return nil, e
		}
	}
	return op, nil
}
func streamAPI(ctx context.Context, info J, route string, query url.Values, emit func(string, J)) error {
	req, e := http.NewRequestWithContext(ctx, "GET", str(info["url"])+"/api"+route+"?"+query.Encode(), nil)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+str(info["token"]))
	resp, e := localClient(0).Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		v, _ := decodeObject(b)
		return fail(text(at(v, "error", "code"), "API_ERROR"), text(at(v, "error", "message"), "No se pudieron abrir los logs."), resp.StatusCode)
	}
	scan := bufio.NewScanner(resp.Body)
	scan.Buffer(make([]byte, 4096), 256<<10)
	event := ""
	var failure error
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "event: ") {
			event = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			v, e := decodeObject([]byte(strings.TrimPrefix(line, "data: ")))
			if e != nil {
				return e
			}
			if event == "log-error" {
				failure = fail(text(v["code"], "LOG_ERROR"), str(v["message"]), 422)
			}
			emit(event, v)
			// A bounded consumer may cancel after this event. Do not scan again:
			// closing the HTTP body can leave an incomplete buffered JSON token.
			if err := ctx.Err(); err != nil {
				if !errors.Is(err, context.Canceled) {
					return err
				}
				break
			}
		}
	}
	if e = scan.Err(); e != nil && !errors.Is(e, context.Canceled) {
		return e
	}
	return failure
}

// Wait for both the legacy socket and native guard before an immediate upgrade.
// A successful HTTP response only acknowledges the request; closing the socket
// and releasing the guard are separate operations during normal shutdown.
func waitAgentStopped(ctx context.Context, home string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		conn, e := net.DialTimeout("unix", filepath.Join(home, "agent.sock"), 300*time.Millisecond)
		if e == nil {
			conn.Close()
		} else if errors.Is(e, os.ErrNotExist) || errors.Is(e, syscall.ECONNREFUSED) {
			// Prove the catalog can be acquired instead of racing the old
			// agent's remaining guard.Close(). A concurrent new owner keeps
			// this wait pending; unsafe or uncertain locks still fail closed.
			release, lockErr := AcquireAgentLock(home)
			if lockErr == nil {
				release()
				return nil
			}
			if str(publicError(lockErr)["code"]) != "AGENT_RUNNING" {
				return lockErr
			}
		} else {
			return fail("AGENT_STOP_UNCERTAIN", "Se pidió el cierre pero no se pudo verificar el socket; revisa antes de actualizar.", 409)
		}
		select {
		case <-ctx.Done():
			return fail("AGENT_STOP_PENDING", "El cierre sigue pendiente. No migres el catálogo hasta que termine; no se ha forzado ni matado otro proceso.", 409)
		case <-time.After(50 * time.Millisecond):
		}
	}
}
