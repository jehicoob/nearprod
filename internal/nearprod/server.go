package nearprod

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const csp = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'"

type HTTP struct {
	Service            *Service
	Assets             fs.FS
	Bearer             string
	Server             *http.Server
	URL                string
	Port               int
	OnShutdown         func()
	mu                 sync.Mutex
	pairings, sessions map[string]time.Time
	attempts           int
	attemptUntil       time.Time
	streams            int
}

func NewHTTP(s *Service, assets fs.FS) *HTTP {
	h := &HTTP{Service: s, Assets: assets, Bearer: token(32), pairings: map[string]time.Time{}, sessions: map[string]time.Time{}}
	h.Server = &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32768}
	return h
}
func (h *HTTP) Listen(port int) (net.Listener, error) {
	ln, e := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if e != nil {
		return nil, e
	}
	h.Port = ln.Addr().(*net.TCPAddr).Port
	h.URL = fmt.Sprintf("http://127.0.0.1:%d", h.Port)
	return ln, nil
}
func (h *HTTP) Pair() J {
	h.mu.Lock()
	defer h.mu.Unlock()
	for k, expiry := range h.pairings {
		if time.Now().After(expiry) {
			delete(h.pairings, k)
		}
	}
	if len(h.pairings) >= 10 {
		for k := range h.pairings {
			delete(h.pairings, k)
			break
		}
	}
	code := token(9)
	expiry := time.Now().Add(120 * time.Second)
	h.pairings[code] = expiry
	return J{"code": code, "expiresAt": expiry.UTC().Format(time.RFC3339)}
}
func (h *HTTP) Session(w http.ResponseWriter, req J) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	if now.After(h.attemptUntil) {
		h.attemptUntil = now.Add(time.Minute)
		h.attempts = 0
	}
	if h.attempts >= 10 {
		return fail("PAIR_RATE_LIMIT", "Demasiados intentos; espera un minuto.", 429)
	}
	h.attempts++
	code := strings.TrimSpace(str(req["code"]))
	expiry, ok := h.pairings[code]
	if !ok || !now.Before(expiry) {
		return fail("PAIR_INVALID", "Código inválido o vencido. Ejecuta nearprod ui.", 401)
	}
	delete(h.pairings, code)
	h.attempts = 0
	for k, expiry := range h.sessions {
		if now.After(expiry) {
			delete(h.sessions, k)
		}
	}
	if len(h.sessions) >= 32 {
		return fail("SESSION_LIMIT", "Cierra otra sesión o reinicia el agente.", 429)
	}
	session := token(32)
	h.sessions[session] = now.Add(8 * time.Hour)
	http.SetCookie(w, &http.Cookie{Name: "nearprod_session", Value: session, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 28800})
	return nil
}
func (h *HTTP) authorize(r *http.Request) (bool, bool) {
	auth := r.Header.Get("Authorization")
	bearer := strings.HasPrefix(auth, "Bearer ") && equalSecret(strings.TrimPrefix(auth, "Bearer "), h.Bearer)
	cookie := false
	if c, e := r.Cookie("nearprod_session"); e == nil {
		h.mu.Lock()
		expiry, exists := h.sessions[c.Value]
		cookie = exists && time.Now().Before(expiry)
		h.mu.Unlock()
	}
	return bearer, bearer || cookie
}
func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func bodyJSON(w http.ResponseWriter, r *http.Request, max int64) (J, error) {
	typ, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || typ != "application/json" {
		return nil, fail("CONTENT_TYPE", "Usa application/json.", 415)
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, max))
	if e != nil {
		return nil, fail("BODY_LIMIT", "Petición demasiado grande o interrumpida.", 413)
	}
	return decodeObject(raw)
}
func (h *HTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	headers := w.Header()
	headers.Set("X-Content-Type-Options", "nosniff")
	headers.Set("Referrer-Policy", "no-referrer")
	headers.Set("X-Frame-Options", "DENY")
	headers.Set("Content-Security-Policy", csp)
	headers.Set("Cache-Control", "no-store")
	err := h.handle(w, r)
	if err != nil {
		jsonResponse(w, statusCode(err), J{"error": publicError(err)})
	}
}
func (h *HTTP) handle(w http.ResponseWriter, r *http.Request) error {
	if r.Host != fmt.Sprintf("127.0.0.1:%d", h.Port) {
		return fail("HOST_DENIED", "Host no autorizado.", 403)
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != h.URL {
		return fail("ORIGIN_DENIED", "Origen no autorizado.", 403)
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return fail("ORIGIN_DENIED", "Petición entre sitios rechazada.", 403)
	}
	if r.Method == "OPTIONS" {
		return fail("CORS_DENIED", "Esta API no permite CORS.", 403)
	}
	pathname := r.URL.Path
	if r.Method == "GET" && pathname == "/api/health" {
		jsonResponse(w, 200, J{"service": "nearprod", "version": Version, "runtimeLanguage": "Go"})
		return nil
	}
	if r.Method == "POST" && pathname == "/api/session" {
		req, e := bodyJSON(w, r, 4096)
		if e != nil {
			return e
		}
		if e = h.Session(w, req); e != nil {
			return e
		}
		jsonResponse(w, 200, J{"authenticated": true})
		return nil
	}
	if strings.HasPrefix(pathname, "/api/") {
		bearer, authorized := h.authorize(r)
		if !authorized {
			return fail("AUTH_REQUIRED", "Inicia sesión con el código de nearprod ui.", 401)
		}
		if r.URL.Query().Get("token") != "" || r.URL.Query().Get("code") != "" {
			return fail("URL_CREDENTIALS", "No se admiten credenciales en URLs.", 400)
		}
		if r.Method == "GET" && (pathname == "/api/events" || pathname == "/api/logs" || pathname == "/api/infra/logs") {
			return h.stream(w, r, pathname)
		}
		if r.Method == "GET" {
			v, e := h.get(r.Context(), pathname, r)
			if e != nil {
				return e
			}
			jsonResponse(w, 200, v)
			return nil
		}
		if r.Method != "POST" {
			return fail("METHOD_DENIED", "Método no permitido.", 405)
		}
		req, e := bodyJSON(w, r, 1<<20)
		if e != nil {
			return e
		}
		if pathname == "/api/pair" {
			if !bearer {
				return fail("CLI_ONLY", "Genera códigos desde nearprod ui.", 403)
			}
			jsonResponse(w, 200, h.Pair())
			return nil
		}
		if pathname == "/api/logout" {
			if c, e := r.Cookie("nearprod_session"); e == nil {
				h.mu.Lock()
				delete(h.sessions, c.Value)
				h.mu.Unlock()
			}
			http.SetCookie(w, &http.Cookie{Name: "nearprod_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
			jsonResponse(w, 200, J{"loggedOut": true})
			return nil
		}
		if pathname == "/api/agent/stop" {
			if !bearer {
				return fail("CLI_ONLY", "Detén el agente desde la terminal.", 403)
			}
			if e = h.Service.Idle(); e != nil {
				return e
			}
			jsonResponse(w, 200, J{"stopped": true, "note": "No se detienen contenedores. Se cierran seguimientos Watch del agente."})
			if h.OnShutdown != nil {
				time.AfterFunc(100*time.Millisecond, h.OnShutdown)
			}
			return nil
		}
		v, e := h.post(r.Context(), pathname, req)
		if e != nil {
			return e
		}
		code := 200
		if strings.HasSuffix(pathname, "/actions") || pathname == "/api/actions" {
			code = 202
		}
		jsonResponse(w, code, v)
		return nil
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		return fail("METHOD_DENIED", "Método no permitido.", 405)
	}
	if r.URL.RawQuery != "" {
		return fail("URL_PARAMETERS", "La interfaz no admite parámetros ni credenciales en la URL.", 400)
	}
	if pathname == "/" {
		pathname = "/index.html"
	}
	name := strings.TrimPrefix(pathname, "/")
	if !fs.ValidPath(name) || strings.Contains(name, "\\") || strings.ContainsRune(name, 0) {
		return fail("NOT_FOUND", "Recurso no encontrado.", 404)
	}
	types := map[string]string{".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8", ".svg": "image/svg+xml", ".png": "image/png", ".ico": "image/x-icon"}
	typ := types[path.Ext(name)]
	if typ == "" {
		return fail("NOT_FOUND", "Recurso no disponible.", 404)
	}
	b, e := fs.ReadFile(h.Assets, name)
	if e != nil || len(b) > 10<<20 {
		return fail("NOT_FOUND", "Recurso de la interfaz no encontrado.", 404)
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(200)
	if r.Method != "HEAD" {
		_, _ = w.Write(b)
	}
	return nil
}
func (h *HTTP) get(ctx context.Context, p string, r *http.Request) (any, error) {
	s := h.Service
	switch p {
	case "/api/catalog":
		return s.Catalog(), nil
	case "/api/status":
		if r.URL.Query().Has("refresh") {
			return s.Refresh(ctx), nil
		}
		return s.Status(), nil
	case "/api/proxy":
		return s.Refresh(ctx)["proxy"], nil
	case "/api/doctor":
		return s.Runtime.Doctor(ctx), nil
	case "/api/infra":
		return s.Infra.List(s.Observed()), nil
	case "/api/tools":
		return s.Tools.Inventory(ctx), nil
	case "/api/startup":
		return NewStartup(s.Store.Home).Status(ctx)
	case "/api/metrics":
		return s.Runtime.Metrics(ctx), nil
	case "/api/config/paths":
		return configPaths(s.Store.Home), nil
	case "/api/config/migration":
		return MigrationPreview(s.Store.Home)
	case "/api/mcp":
		return MCPInfo(s.Store.Home), nil
	case "/api/images":
		id := r.URL.Query().Get("id")
		if !strings.HasPrefix(id, "sha256:") || !cidRE.MatchString(strings.TrimPrefix(id, "sha256:")) {
			return nil, fail("IMAGE_ID", "Selecciona una imagen identificada.", 400)
		}
		return s.Docker.Image(ctx, id)
	}
	if strings.HasPrefix(p, "/api/operations/") {
		return s.Ops.Get(strings.TrimPrefix(p, "/api/operations/"))
	}
	return nil, fail("NOT_FOUND", "Ruta no disponible.", 404)
}
func (h *HTTP) post(ctx context.Context, p string, v J) (any, error) {
	s := h.Service
	switch p {
	case "/api/roots":
		return s.AddRoot(str(v["root"]))
	case "/api/roots/remove-preview":
		return s.RemoveRootPreview(str(v["root"]))
	case "/api/roots/remove":
		return s.RemoveRoot(v)
	case "/api/discover":
		return s.Discover(ctx, v)
	case "/api/project-options":
		return s.ProjectOptions(v)
	case "/api/groups":
		return s.Group(v, false)
	case "/api/groups/rename":
		return s.Group(v, true)
	case "/api/groups/delete-preview":
		return s.DeleteGroupPreview(str(v["id"]))
	case "/api/groups/delete":
		return s.DeleteGroup(v)
	case "/api/stacks":
		return s.Register(v, false)
	case "/api/stacks/batch":
		return s.Register(v, true)
	case "/api/stacks/edit":
		return s.Edit(str(v["target"]), obj(v["definition"]))
	case "/api/stacks/remove":
		return s.Remove(ctx, str(v["target"]), truth(v["confirm"]))
	case "/api/stacks/archive-preview":
		return s.ArchiveStackPreview(str(v["target"]))
	case "/api/stacks/archive":
		return s.ArchiveStack(ctx, v)
	case "/api/stacks/restore-preview":
		return s.RestoreStackPreview(str(v["target"]))
	case "/api/stacks/restore":
		return s.RestoreStack(v)
	case "/api/preview":
		return s.Preview(ctx, str(v["target"]), text(v["mode"], "dev"))
	case "/api/trust":
		return s.Trust(ctx, str(v["target"]), v)
	case "/api/adoption":
		return s.Adoption(ctx, str(v["target"]))
	case "/api/adopt":
		return s.Adopt(ctx, str(v["target"]), v)
	case "/api/actions":
		return s.Action(str(v["target"]), str(v["action"]), v)
	case "/api/watch":
		if str(v["action"]) == "start" {
			return s.StartWatch(str(v["target"]))
		}
		if str(v["action"]) == "stop" {
			return s.StopWatch(str(v["target"])), nil
		}
		return nil, fail("ACTION_INVALID", "Usa start/stop para Watch.", 400)
	case "/api/runtime/settings":
		return s.SetRuntime(v)
	case "/api/runtime/preview":
		return s.Runtime.Preview(ctx, v)
	case "/api/runtime/actions":
		return s.RuntimeAction(v)
	case "/api/proxy/preview":
		return s.Proxy.Preview(ctx, v)
	case "/api/proxy/actions":
		return s.ProxyAction(v)
	case "/api/proxy/check":
		st, e := s.Store.Stack(str(v["target"]))
		if e != nil {
			return nil, e
		}
		return s.Proxy.Check(ctx, st, str(v["host"]))
	case "/api/infra/ports":
		return s.Infra.Ports(ctx, v)
	case "/api/infra/preview":
		return s.Infra.Preview(ctx, v)
	case "/api/infra/stop-preview":
		return s.Infra.StopPreview(ctx, str(v["instance"]))
	case "/api/infra/archive-preview":
		return s.Infra.ArchiveInstancePreview(ctx, str(v["instance"]))
	case "/api/infra/restore-instance-preview":
		return s.Infra.RestoreInstancePreview(ctx, str(v["instance"]))
	case "/api/infra/archive-database-preview":
		return s.Infra.ArchiveDatabasePreview(str(v["database"]))
	case "/api/infra/restore-database-preview":
		return s.Infra.RestoreDatabasePreview(str(v["database"]))
	case "/api/infra/purge-database-preview":
		return s.Infra.PurgeDatabasePreview(ctx, v)
	case "/api/infra/binding-preview":
		return s.Infra.BindingPreview(ctx, v)
	case "/api/infra/connection":
		return s.Infra.Connection(str(v["database"]), truth(v["reveal"]))
	case "/api/infra/actions":
		return s.InfraAction(v)
	case "/api/tools/versions":
		return s.Tools.Versions(ctx, str(v["tool"]))
	case "/api/tools/preview":
		return s.Tools.Preview(ctx, v)
	case "/api/tools/actions":
		return s.ToolsAction(v)
	case "/api/updates":
		return s.Runtime.Updates(ctx)
	case "/api/cancel":
		return s.Ops.Cancel(str(v["id"]))
	case "/api/config/backup":
		if !truth(v["confirm"]) {
			return nil, fail("CONFIRM_REQUIRED", "La copia contiene credenciales; confirma guardarla de forma privada.", 409)
		}
		return s.ConfigBackup(ctx)
	}
	return nil, fail("NOT_FOUND", "Ruta no disponible.", 404)
}
func (h *HTTP) stream(w http.ResponseWriter, r *http.Request, p string) error {
	h.mu.Lock()
	if h.streams >= 16 {
		h.mu.Unlock()
		return fail("STREAM_LIMIT", "Demasiadas suscripciones; cierra otras vistas.", 429)
	}
	h.streams++
	h.mu.Unlock()
	defer func() { h.mu.Lock(); h.streams--; h.mu.Unlock() }()
	duration := 30 * time.Minute
	if p != "/api/events" {
		duration = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(r.Context(), duration)
	defer cancel()
	events, unsubscribe := h.Service.Bus.Subscribe()
	defer unsubscribe()
	logch := make(chan J, 64)
	done := make(chan error, 1)
	if p != "/api/events" {
		query := r.URL.Query()
		target := query.Get("target")
		if p == "/api/infra/logs" {
			target = query.Get("instance")
			if _, e := h.Service.Infra.Instance(target); e != nil {
				return e
			}
		} else {
			if _, e := h.Service.Store.Stack(target); e != nil {
				return e
			}
		}
		params := J{"follow": query.Get("follow") != "false", "tail": text(query.Get("tail"), "100"), "service": query.Get("service"), "container": query.Get("container"), "since": text(r.Header.Get("Last-Event-ID"), query.Get("since"))}
		if _, e := logArgs(params); e != nil {
			return e
		}
		go func() {
			emit := func(v J) {
				select {
				case logch <- v:
				case <-ctx.Done():
				default:
					cancel()
				}
			}
			var e error
			if p == "/api/infra/logs" {
				e = h.Service.InfraLogs(ctx, target, params, emit)
			} else {
				e = h.Service.Logs(ctx, target, params, emit)
			}
			done <- e
		}()
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	controller := http.NewResponseController(w)
	send := func(event string, v any) bool {
		b, e := json.Marshal(v)
		if e != nil {
			return false
		}
		_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
		cursor := ""
		if event == "line" {
			if stamp := str(obj(v)["time"]); stamp != "" {
				if _, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
					cursor = "id: " + stamp + "\n"
				}
			}
		}
		_, e = fmt.Fprintf(w, "%sevent: %s\ndata: %s\n\n", cursor, event, b)
		if e != nil {
			return false
		}
		return controller.Flush() == nil
	}
	if p == "/api/events" {
		if !send("message", J{"type": "status", "status": h.Service.Status()}) {
			return nil
		}
	} else {
		if !send("notice", J{"message": "Logs acotados; pueden contener datos sensibles. Al reconectar puede haber solapamientos o huecos."}) {
			return nil
		}
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-done:
			for {
				select {
				case v := <-logch:
					if !send("line", v) {
						return nil
					}
				default:
					if e != nil && !strings.Contains(str(publicError(e)["code"]), "CANCELLED") {
						send("log-error", publicError(e))
					}
					return nil
				}
			}
		case v, ok := <-events:
			if !ok {
				return nil
			}
			if p == "/api/events" && !send("message", v) {
				return nil
			}
		case v := <-logch:
			if !send("line", v) {
				return nil
			}
		case <-heartbeat.C:
			if !send("heartbeat", J{"time": now()}) {
				return nil
			}
		}
	}
}
