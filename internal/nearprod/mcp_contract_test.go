package nearprod

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func callMCP(t *testing.T, client *mcpsdk.ClientSession, name string, input J) J {
	t.Helper()
	result, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: name, Arguments: input})
	must(t, err)
	if result.IsError {
		t.Fatalf("%s: %v", name, result.Content)
	}
	return obj(at(obj(result.StructuredContent), "result"))
}
func mcpErrorCode(t *testing.T, client *mcpsdk.ClientSession, name string, input J, code string) J {
	t.Helper()
	result, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: name, Arguments: input})
	must(t, err)
	value := obj(at(obj(result.StructuredContent), "result", "error"))
	if !result.IsError || str(value["code"]) != code {
		t.Fatalf("want %s, got %v", code, result)
	}
	return value
}
func fixtureMCP(t *testing.T, f *fixture) *mcpsdk.ClientSession {
	t.Helper()
	h := NewHTTP(f.S, nil)
	ln, err := h.Listen(0)
	must(t, err)
	go h.Server.Serve(ln)
	t.Cleanup(func() { h.Server.Close() })
	return connectMCP(t, J{"url": h.URL, "token": h.Bearer})
}

func TestMCPGuidanceAndSchemasAreDiscoverable(t *testing.T) {
	client := connectMCP(t, J{})
	resources, err := client.ListResources(context.Background(), nil)
	must(t, err)
	if len(resources.Resources) != 2 {
		t.Fatal(resources)
	}
	guide, err := client.ReadResource(context.Background(), &mcpsdk.ReadResourceParams{URI: "nearprod://guide"})
	must(t, err)
	text := guide.Contents[0].Text
	for _, required := range []string{"include", "extends", "filesystem", "PREVIEW_CHANGED", "health", "Compose V2", "acknowledgeDataLoss", "fingerprint", "explain"} {
		if !strings.Contains(text, required) {
			t.Errorf("guide misses %s", required)
		}
	}
	if callMCP(t, client, "nearprod_guide", J{})["guide"] != text {
		t.Fatal("resource/tool guides diverge")
	}
	catalog := callMCP(t, client, "nearprod_capabilities", J{"area": "applications", "operation": "register"})
	ops := arr(catalog["operations"])
	if len(ops) != 1 {
		t.Fatal(catalog)
	}
	properties := obj(at(obj(ops[0]), "parameters", "properties"))
	for _, field := range []string{"product", "slug", "path", "projectName", "modes", "routes", "links"} {
		if properties[field] == nil {
			t.Error("missing field", field)
		}
	}
	tools, err := client.ListTools(context.Background(), nil)
	must(t, err)
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	for _, raw := range arr(callMCP(t, client, "nearprod_capabilities", J{})["operations"]) {
		if !names[str(obj(raw)["tool"])] {
			t.Errorf("catalog tool absent: %v", raw)
		}
	}
	mcpErrorCode(t, client, "nearprod_capabilities", J{"area": "unknown"}, "MCP_OPERATION")
}

func TestMCPInvalidParametersDoNotReachAgent(t *testing.T) {
	calls := 0
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; jsonResponse(w, 200, J{}) }))
	defer httpServer.Close()
	client := connectMCP(t, J{"url": httpServer.URL, "token": "fixture"})
	for _, tc := range []struct {
		name  string
		input J
	}{
		{"nearprod_application_configure", J{"action": "register", "definition": J{"path": "/tmp/app"}}},
		{"nearprod_runtime", J{"operation": "configure", "parameters": J{"memory": 2, "cpus": 2, "confirm": true}}},
		{"nearprod_infrastructure", J{"operation": "create", "parameters": J{"engine": "postgres", "confirm": false, "fingerprint": "x"}}},
		{"nearprod_proxy", J{"operation": "check", "parameters": J{"target": "app/api"}}},
		{"nearprod_applications", J{"operation": "watch-start", "parameters": J{"target": "app/api", "arbitrary": "ignored?"}}},
	} {
		_, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: tc.name, Arguments: tc.input})
		if err == nil {
			t.Errorf("invalid schema accepted: %s", tc.name)
		}
	}
	if calls != 0 {
		t.Fatal("invalid arguments dispatched", calls)
	}
}

func TestMCPDomainToolsForwardReviewedParameters(t *testing.T) {
	type request struct {
		method, path, query string
		body                J
	}
	requests := make(chan request, 32)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := J{}
		if r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&body)
		}
		requests <- request{r.Method, r.URL.Path, r.URL.RawQuery, body}
		jsonResponse(w, 200, J{"id": "fixture-operation", "state": "queued"})
	}))
	defer httpServer.Close()
	client := connectMCP(t, J{"url": httpServer.URL, "token": "fixture"})
	for _, tc := range []struct {
		tool, operation, path, action string
		params                        J
	}{
		{"nearprod_roots", "remove", "/roots/remove", "", J{"root": "/tmp/source", "fingerprint": "reviewed", "confirm": true}},
		{"nearprod_applications", "adopt", "/adopt", "", J{"target": "g/a", "fingerprint": "reviewed", "confirm": true}},
		{"nearprod_applications", "watch-stop", "/watch", "stop", J{"target": "g/a"}},
		{"nearprod_runtime", "configure", "/runtime/actions", "configure", J{"memory": 4, "cpus": 2, "fingerprint": "reviewed", "confirm": true}},
		{"nearprod_proxy", "start", "/proxy/actions", "start", J{"port": 8080, "fingerprint": "reviewed", "confirm": true}},
		{"nearprod_infrastructure", "bind", "/infra/actions", "bind", J{"target": "g/a", "database": "db", "services": A{"api"}, "mapping": J{"url": "DATABASE_URL"}, "fingerprint": "reviewed", "confirm": true}},
		{"nearprod_infrastructure", "purge-database", "/infra/actions", "purge-database", J{"database": "db", "mode": "purge", "typedId": "db", "acknowledgeDataLoss": true, "fingerprint": "reviewed", "confirm": true}},
		{"nearprod_maintenance", "apply", "/tools/actions", "", J{"tool": "compose", "action": "repair", "fingerprint": "reviewed", "confirm": true}},
	} {
		callMCP(t, client, tc.tool, J{"operation": tc.operation, "parameters": tc.params})
		got := <-requests
		want := copyJ(tc.params)
		if tc.action != "" {
			want["action"] = tc.action
		}
		if got.method != "POST" || got.path != "/api"+tc.path || hash(got.body) != hash(want) {
			t.Fatalf("%s: got %+v want %v", tc.operation, got, want)
		}
	}
	callMCP(t, client, "nearprod_diagnostics", J{"operation": "status", "parameters": J{"refresh": false}})
	if got := <-requests; got.method != "GET" || got.query != "" {
		t.Fatal("false must not trigger refresh", got)
	}
	callMCP(t, client, "nearprod_diagnostics", J{"operation": "status", "parameters": J{"refresh": true}})
	if got := <-requests; got.query != "refresh=1" {
		t.Fatal(got)
	}
	callMCP(t, client, "nearprod_api", J{"method": "POST", "route": "/updates"})
	if got := <-requests; got.method != "POST" || got.body == nil {
		t.Fatal("empty POST was not an object", got)
	}
}

func TestMCPRegisterReviewRunAndInspect(t *testing.T) {
	f := newFixture(t, true)
	client := connectMCP(t, f.A.Info)
	path := filepath.Join(f.Root, "mcp-example")
	must(t, os.MkdirAll(path, 0700))
	must(t, writeJSON(filepath.Join(path, "compose.yaml"), J{"services": J{"api": J{"image": "fixture:test", "healthcheck": J{"test": A{"CMD", "true"}}}}}))
	discovered := callMCP(t, client, "nearprod_discover", J{"root": f.Root})
	if len(arr(discovered["candidates"])) != 1 {
		t.Fatal("default discovery failed", discovered)
	}
	options := callMCP(t, client, "nearprod_project_options", J{"path": path})
	if len(arr(options["suggestedFiles"])) != 1 {
		t.Fatal("omitted files lost suggestions", options)
	}
	def := J{"product": "mcp", "slug": "api", "path": path, "projectName": "mcp-api", "modes": J{"dev": J{"files": A{"compose.yaml"}}}}
	app := callMCP(t, client, "nearprod_application_configure", J{"action": "register", "definition": def})
	if app["id"] != "mcp/api" || len(f.F.History()) != 0 {
		t.Fatal("registration ran commands", app)
	}
	review := callMCP(t, client, "nearprod_application_review", J{"target": "mcp/api"})
	callMCP(t, client, "nearprod_application_approve", J{"target": "mcp/api", "fingerprint": review["fingerprint"]})
	op := callMCP(t, client, "nearprod_application_action", J{"target": "mcp/api", "action": "up", "wait": true})
	assertPass(t, waitOp(t, f.S, op))
	final := callMCP(t, client, "nearprod_operation", J{"id": op["id"]})
	if final["state"] != "succeeded" {
		t.Fatal(final)
	}
	status := callMCP(t, client, "nearprod_diagnostics", J{"operation": "status", "parameters": J{"refresh": true}})
	stacks := arr(status["stacks"])
	if len(stacks) != 1 || at(stacks[0], "health") != "healthy" {
		t.Fatal(status)
	}
	logs := callMCP(t, client, "nearprod_application_logs", J{"target": "mcp/api", "tail": 5})
	if len(arr(logs["lines"])) == 0 || logs["target"] != "mcp/api" {
		t.Fatal(logs)
	}
}

func TestMCPLifecycleUsesCallerFingerprintAndPurgesResources(t *testing.T) {
	f, app := purgeFixture(t)
	client := fixtureMCP(t, f)
	target := str(app["id"])
	preview := callMCP(t, client, "nearprod_application_lifecycle", J{"target": target, "action": "purge"})
	req := J{"target": target, "action": "purge", "confirm": true, "typedId": target, "acknowledgeDataLoss": true}
	mcpErrorCode(t, client, "nearprod_application_lifecycle", req, "FINGERPRINT_REQUIRED")
	req["fingerprint"] = "stale"
	op := callMCP(t, client, "nearprod_application_lifecycle", req)
	failed := waitOp(t, f.S, op)
	if at(failed, "error", "code") != "PREVIEW_CHANGED" {
		t.Fatal("stale approval was replaced", failed)
	}
	req["fingerprint"] = preview["fingerprint"]
	req["acknowledgeDataLoss"] = false
	mcpErrorCode(t, client, "nearprod_application_lifecycle", req, "PURGE_CONFIRMATION")
	req["acknowledgeDataLoss"] = true
	op = callMCP(t, client, "nearprod_application_lifecycle", req)
	assertPass(t, waitOp(t, f.S, op))
	if len(allStacks(f.S.Store.Get())) != 0 {
		t.Fatal("app remains")
	}
	f.F.Set(func() {
		if len(f.F.Containers)+len(f.F.Volumes)+len(f.F.Networks)+len(f.F.Images) != 0 {
			t.Error("owned resources remain")
		}
	})
	if _, err := os.Stat(filepath.Join(str(app["path"]), "compose.yaml")); err != nil {
		t.Fatal("source was not preserved", err)
	}
}

func TestMCPGroupAndArchiveDoNotRefreshApproval(t *testing.T) {
	f := newFixture(t, true)
	client := connectMCP(t, f.A.Info)
	callMCP(t, client, "nearprod_group", J{"action": "create", "id": "empty", "name": "Empty"})
	mcpErrorCode(t, client, "nearprod_group", J{"action": "delete", "id": "empty", "confirm": true}, "FINGERPRINT_REQUIRED")
	mcpErrorCode(t, client, "nearprod_group", J{"action": "delete", "id": "empty", "confirm": true, "fingerprint": "stale"}, "PREVIEW_CHANGED")
	preview := callMCP(t, client, "nearprod_group", J{"action": "delete", "id": "empty"})
	callMCP(t, client, "nearprod_group", J{"action": "delete", "id": "empty", "confirm": true, "fingerprint": preview["fingerprint"]})
	app := f.app(t, "example", "api", nil)
	mcpErrorCode(t, client, "nearprod_application_lifecycle", J{"action": "archive", "target": app["id"], "confirm": true, "fingerprint": "stale"}, "PREVIEW_CHANGED")
	preview = callMCP(t, client, "nearprod_application_lifecycle", J{"action": "archive", "target": app["id"]})
	callMCP(t, client, "nearprod_application_lifecycle", J{"action": "archive", "target": app["id"], "confirm": true, "fingerprint": preview["fingerprint"]})
	preview = callMCP(t, client, "nearprod_application_lifecycle", J{"action": "restore", "target": app["id"]})
	callMCP(t, client, "nearprod_application_lifecycle", J{"action": "restore", "target": app["id"], "confirm": true, "fingerprint": preview["fingerprint"]})
}

func TestMCPStructuredErrorsAndStreamingGuidance(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		jsonResponse(w, 422, J{"error": J{"code": "VERIFY_BLOCKED", "message": "Review blockers", "details": J{"blockers": A{"api: bind mount"}}}})
	}))
	defer httpServer.Close()
	client := connectMCP(t, J{"url": httpServer.URL, "token": "fixture"})
	err := mcpErrorCode(t, client, "nearprod_application_review", J{"target": "g/a", "mode": "verify"}, "VERIFY_BLOCKED")
	if len(arr(at(err, "details", "blockers"))) != 1 {
		t.Fatal("details lost", err)
	}
	for _, route := range []string{"/events", "/logs?target=g/a", "/infra/logs?instance=db"} {
		mcpErrorCode(t, client, "nearprod_api", J{"method": "GET", "route": route}, "MCP_STREAM_ROUTE")
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatal("SSE sent to JSON bridge", calls)
	}
}

func TestMCPInfrastructureLogsAreBounded(t *testing.T) {
	f := newFixture(t, true)
	client := connectMCP(t, f.A.Info)
	instance := f.createInstance(t, "postgres")
	logs := callMCP(t, client, "nearprod_infrastructure_logs", J{"instance": instance["id"], "tail": 5})
	if len(arr(logs["lines"])) == 0 || logs["instance"] != instance["id"] {
		t.Fatal(logs)
	}
	for _, tc := range []struct {
		name       string
		line       string
		tail, want int
	}{
		{"line-cap", "hello", 3, 3}, {"byte-cap", strings.Repeat("x", 100000), 500, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("follow") != "false" {
					t.Error("unbounded follow")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				for n := 0; n < 20; n++ {
					if _, err := fmt.Fprintf(w, "event: line\ndata: %s\n\n", mustJSON(J{"text": tc.line})); err != nil {
						return
					}
				}
			}))
			defer h.Close()
			c := connectMCP(t, J{"url": h.URL, "token": "fixture"})
			result := callMCP(t, c, "nearprod_infrastructure_logs", J{"instance": "db", "tail": tc.tail})
			if len(arr(result["lines"])) != tc.want || !truth(result["truncated"]) || integer(result["bytes"]) > 512<<10 {
				t.Fatal("log bound failed", len(arr(result["lines"])), result["bytes"])
			}
		})
	}
}

// The HTTP implementation is the independent source of route coverage. New web
// endpoints must have a discoverable operation, or an explicit transport/UI-only
// exception. A manually maintained count cannot detect this kind of drift.
func TestMCPParityWithHTTPRoutesAndDomainActions(t *testing.T) {
	file, err := parser.ParseFile(gotoken.NewFileSet(), "server.go", nil, 0)
	must(t, err)
	contracts := map[string]bool{}
	routes := map[string]bool{}
	names := map[string]bool{}
	for _, spec := range mcpOperations() {
		key := spec.Area + "/" + spec.Name
		if names[key] {
			t.Error("duplicate operation", key)
		}
		names[key] = true
		contracts[spec.Method+" /api"+spec.Route] = true
		routes["/api"+spec.Route] = true
		if spec.Description == "" || spec.Result == "" || spec.Parameters == nil {
			t.Error("undocumented operation", key)
		}
	}
	exceptions := map[string]string{"/api/session": "browser login", "/api/logout": "browser session", "/api/pair": "CLI/browser pairing", "/api/agent/stop": "administrative CLI", "/api/events": "poll status/operations", "/api/logs": "nearprod_application_logs", "/api/infra/logs": "nearprod_infrastructure_logs"}
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != gotoken.STRING {
			return true
		}
		value, _ := strconv.Unquote(literal.Value)
		if value == "/api/operations/" {
			value = "/api/operations/{id}"
		}
		if strings.HasPrefix(value, "/api/") && value != "/api/" && !routes[value] && exceptions[value] == "" {
			t.Error("HTTP route missing from MCP", value)
		}
		return true
	})
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || (fn.Name.Name != "get" && fn.Name.Name != "post") {
			continue
		}
		method := strings.ToUpper(fn.Name.Name)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			clause, ok := node.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, expr := range clause.List {
				if lit, ok := expr.(*ast.BasicLit); ok {
					route, _ := strconv.Unquote(lit.Value)
					if strings.HasPrefix(route, "/api/") && !contracts[method+" "+route] {
						t.Errorf("missing %s %s", method, route)
					}
				}
			}
			return true
		})
	}
	service, err := parser.ParseFile(gotoken.NewFileSet(), "service.go", nil, 0)
	must(t, err)
	methods := map[string]string{"Action": "/actions", "InfraAction": "/infra/actions", "RuntimeAction": "/runtime/actions", "ProxyAction": "/proxy/actions"}
	for _, decl := range service.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || methods[fn.Name.Name] == "" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			name, ok := call.Fun.(*ast.Ident)
			if !ok || name.Name != "contains" {
				return true
			}
			action, ok := call.Args[1].(*ast.Ident)
			if !ok || action.Name != "action" {
				return true
			}
			list, ok := call.Args[0].(*ast.CompositeLit)
			if !ok {
				return true
			}
			for _, expr := range list.Elts {
				lit, ok := expr.(*ast.BasicLit)
				if !ok {
					continue
				}
				value, _ := strconv.Unquote(lit.Value)
				found := false
				for _, spec := range mcpOperations() {
					found = found || (spec.Route == methods[fn.Name.Name] && spec.Action == value)
				}
				if !found {
					t.Error("domain action missing from MCP", fn.Name.Name, value)
				}
			}
			return true
		})
	}
}

func TestMCPParityWithMaintenanceAndWatchActions(t *testing.T) {
	toolsFile, err := parser.ParseFile(gotoken.NewFileSet(), "tools.go", nil, 0)
	must(t, err)
	allowed := map[string]bool{}
	for _, spec := range mcpOperations() {
		if spec.Route == "/tools/actions" {
			for _, action := range ss(at(spec.Parameters, "properties", "action", "enum")) {
				allowed[action] = true
			}
		}
	}
	ast.Inspect(toolsFile, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		name, ok := call.Fun.(*ast.Ident)
		if !ok || name.Name != "contains" {
			return true
		}
		variable, ok := call.Args[1].(*ast.Ident)
		if !ok || variable.Name != "action" {
			return true
		}
		values, ok := call.Args[0].(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, expr := range values.Elts {
			if lit, ok := expr.(*ast.BasicLit); ok {
				action, _ := strconv.Unquote(lit.Value)
				if !allowed[action] {
					t.Error("maintenance action missing from MCP", action)
				}
			}
		}
		return true
	})
	httpFile, err := parser.ParseFile(gotoken.NewFileSet(), "server.go", nil, 0)
	must(t, err)
	ast.Inspect(httpFile, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok {
			return true
		}
		watch := false
		for _, expr := range clause.List {
			if lit, ok := expr.(*ast.BasicLit); ok {
				value, _ := strconv.Unquote(lit.Value)
				watch = watch || value == "/api/watch"
			}
		}
		if !watch {
			return true
		}
		ast.Inspect(clause, func(node ast.Node) bool {
			comparison, ok := node.(*ast.BinaryExpr)
			if !ok || comparison.Op != gotoken.EQL {
				return true
			}
			call, ok := comparison.X.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			index, ok := call.Args[0].(*ast.IndexExpr)
			if !ok {
				return true
			}
			key, ok := index.Index.(*ast.BasicLit)
			if !ok || key.Value != `"action"` {
				return true
			}
			literal, ok := comparison.Y.(*ast.BasicLit)
			if !ok {
				return true
			}
			action, _ := strconv.Unquote(literal.Value)
			found := false
			for _, spec := range mcpOperations() {
				found = found || (spec.Route == "/watch" && spec.Action == action)
			}
			if !found {
				t.Error("Watch action missing from MCP", action)
			}
			return true
		})
		return false
	})
}

func TestMCPAdvancedPOSTEmptyBody(t *testing.T) {
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || string(raw) != "{}" {
			t.Error(r.Method, string(raw))
		}
		jsonResponse(w, 200, J{"ok": true})
	}))
	defer h.Close()
	callMCP(t, connectMCP(t, J{"url": h.URL, "token": "fixture"}), "nearprod_api", J{"method": "POST", "route": "/updates"})
}
