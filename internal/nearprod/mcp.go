package nearprod

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpOutput struct {
	Result any `json:"result"`
}

type mcpEmptyInput struct{}

type mcpDiscoverInput struct {
	Root       string   `json:"root" jsonschema:"Absolute discovery root already registered in NearProd"`
	Depth      *int     `json:"depth,omitempty" jsonschema:"Maximum directory depth, from 0 to 32; default 8"`
	MaxEntries *int     `json:"maxEntries,omitempty" jsonschema:"Maximum filesystem entries to inspect, from 1 to 100000; default 20000"`
	Ignore     []string `json:"ignore,omitempty" jsonschema:"Additional directory names to ignore"`
}

type mcpProjectOptionsInput struct {
	Path  string   `json:"path" jsonschema:"Absolute project path under a registered root"`
	Files []string `json:"files,omitempty" jsonschema:"Optional Compose files to inspect"`
}

type mcpGroupInput struct {
	Action      string `json:"action" jsonschema:"Action: create, rename, or delete"`
	ID          string `json:"id,omitempty" jsonschema:"Stable group identifier"`
	Name        string `json:"name,omitempty" jsonschema:"Human-readable group name"`
	Confirm     bool   `json:"confirm,omitempty" jsonschema:"Confirm deletion after reviewing the returned preview"`
	Fingerprint string `json:"fingerprint,omitempty" jsonschema:"Exact reviewed delete fingerprint; required with confirm=true"`
}

type mcpApplicationConfigInput struct {
	Action     string `json:"action" jsonschema:"Action: register or edit"`
	Target     string `json:"target,omitempty" jsonschema:"Existing group/application identifier for edit"`
	Definition J      `json:"definition" jsonschema:"NearProd application definition"`
}

type mcpApplicationReviewInput struct {
	Target string `json:"target" jsonschema:"group/application identifier"`
	Mode   string `json:"mode,omitempty" jsonschema:"Compose mode: dev or verify"`
}

type mcpApplicationApproveInput struct {
	Target      string `json:"target" jsonschema:"group/application identifier"`
	Mode        string `json:"mode,omitempty" jsonschema:"Compose mode: dev or verify"`
	Fingerprint string `json:"fingerprint" jsonschema:"Exact fingerprint returned by nearprod_application_review"`
	AllowUnsafe bool   `json:"allowUnsafe,omitempty" jsonschema:"Explicitly accept non-blocking Compose risks"`
}

type mcpApplicationActionInput struct {
	Target       string `json:"target" jsonschema:"group or group/application identifier"`
	Action       string `json:"action" jsonschema:"Action: up, stop, restart, or rebuild"`
	Mode         string `json:"mode,omitempty" jsonschema:"Compose mode: dev or verify"`
	Service      string `json:"service,omitempty" jsonschema:"Optional single Compose service"`
	ConfirmMode  bool   `json:"confirmMode,omitempty" jsonschema:"Confirm changing the active Compose mode"`
	StartRuntime bool   `json:"startRuntime,omitempty" jsonschema:"Allow NearProd to start its configured local runtime"`
	Wait         bool   `json:"wait,omitempty" jsonschema:"Ask Compose to wait for readiness where supported; still poll the returned operation id"`
}

type mcpApplicationLifecycleInput struct {
	Target              string `json:"target" jsonschema:"group/application identifier"`
	Action              string `json:"action" jsonschema:"Action: archive, restore, remove (metadata only), or purge (owned resources and data)"`
	Confirm             bool   `json:"confirm,omitempty" jsonschema:"Apply the reviewed action; false returns archive/restore/purge preview"`
	Fingerprint         string `json:"fingerprint,omitempty" jsonschema:"Exact reviewed fingerprint; required when confirming archive, restore or purge"`
	TypedID             string `json:"typedId,omitempty" jsonschema:"Exact application target ID; required for confirmed purge"`
	AcknowledgeDataLoss bool   `json:"acknowledgeDataLoss,omitempty" jsonschema:"Explicit data-loss acceptance; must be true for confirmed purge"`
}

type mcpLogsInput struct {
	Target    string `json:"target" jsonschema:"group/application identifier"`
	Service   string `json:"service,omitempty" jsonschema:"Optional Compose service"`
	Container string `json:"container,omitempty" jsonschema:"Optional owned container ID"`
	Tail      int    `json:"tail,omitempty" jsonschema:"Number of lines, from 1 to 500"`
	Since     string `json:"since,omitempty" jsonschema:"Optional RFC3339 timestamp"`
}

type mcpOperationInput struct {
	ID     string `json:"id" jsonschema:"NearProd operation identifier"`
	Action string `json:"action,omitempty" jsonschema:"Action: get or cancel"`
}

type mcpAPIInput struct {
	Method string `json:"method" jsonschema:"HTTP method: GET or POST"`
	Route  string `json:"route" jsonschema:"NearProd API route without the /api prefix, for example /infra"`
	Body   J      `json:"body,omitempty" jsonschema:"JSON request body for POST routes"`
}

type mcpBridge struct {
	info J
}

func MCPInfo(home string) J {
	command := binaryPath()
	config := J{"mcpServers": J{"nearprod": J{"command": command, "args": A{"mcp", "serve"}, "env": J{"NEARPROD_HOME": home}}}}
	return J{
		"available": true,
		"transport": "stdio",
		"command":   command,
		"args":      A{"mcp", "serve"},
		"home":      home,
		"config":    config,
		"capabilities": A{
			J{"area": "Catálogo y diagnóstico", "access": "total", "detail": "Aplicaciones, grupos, estado, operaciones, runtime, infraestructura y herramientas."},
			J{"area": "Configuración", "access": "total", "detail": "Descubrimiento, registro, edición, revisión y aprobación de aplicaciones y grupos."},
			J{"area": "Operación", "access": "total", "detail": "Iniciar, detener, reiniciar, reconstruir, consultar logs y cancelar operaciones."},
			J{"area": "API avanzada", "access": "total", "detail": "Operaciones JSON documentadas, logs acotados y consultas de estado; acciones destructivas conservan sus confirmaciones."},
		},
		"notes": A{
			"El servidor MCP solo usa stdio y no abre otro puerto.",
			"La configuración no contiene tokens ni credenciales.",
			"Cualquier agente configurado obtiene control total sobre este catálogo local.",
			"NearProd conserva validaciones, fingerprints, ownership y confirmaciones de pérdida de datos.",
		},
	}
}

func boolPointer(value bool) *bool { return &value }

func mcpAnnotations(readOnly, destructive, openWorld bool) *mcpsdk.ToolAnnotations {
	return &mcpsdk.ToolAnnotations{ReadOnlyHint: readOnly, DestructiveHint: boolPointer(destructive), OpenWorldHint: boolPointer(openWorld)}
}

func addNearProdTool[Input any](server *mcpsdk.Server, tool *mcpsdk.Tool, handler func(context.Context, Input) (any, error)) {
	mcpsdk.AddTool(server, tool, func(ctx context.Context, _ *mcpsdk.CallToolRequest, input Input) (*mcpsdk.CallToolResult, mcpOutput, error) {
		result, err := handler(ctx, input)
		if err != nil {
			// Preserve domain details that would otherwise be lost through error.Error().
			payload := J{"error": publicError(err)}
			encoded, _ := json.Marshal(mcpOutput{Result: payload})
			return &mcpsdk.CallToolResult{IsError: true, Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(encoded)}}, StructuredContent: mcpOutput{Result: payload}}, mcpOutput{Result: payload}, nil
		}
		return nil, mcpOutput{Result: result}, nil
	})
}

func (b *mcpBridge) api(ctx context.Context, route string, body any) (J, error) {
	return requestAPI(ctx, b.info, route, body, 60*time.Second)
}

func NewMCPServer(info J) *mcpsdk.Server {
	bridge := &mcpBridge{info: info}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "nearprod", Version: Version}, &mcpsdk.ServerOptions{
		Instructions: mcpInstructions,
		Capabilities: &mcpsdk.ServerCapabilities{},
		KeepAlive:    30 * time.Second,
	})

	addMCPGuidance(server)
	addMCPDomainTools(server, bridge)
	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_overview", Description: "Obtiene catálogo, aplicaciones, grupos, operaciones y estado fresco del Engine local.", Annotations: mcpAnnotations(true, false, false)}, func(ctx context.Context, _ mcpEmptyInput) (any, error) {
		catalog, err := bridge.api(ctx, "/catalog", nil)
		if err != nil {
			return nil, err
		}
		status, err := bridge.api(ctx, "/status?refresh=1", nil)
		if err != nil {
			return nil, err
		}
		return J{"catalog": catalog, "status": status}, nil
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_discover", Description: "Descubre proyectos Compose bajo una raíz registrada sin modificar archivos ni el catálogo.", Annotations: mcpAnnotations(true, false, false)}, func(ctx context.Context, input mcpDiscoverInput) (any, error) {
		body := J{"root": input.Root, "ignore": input.Ignore}
		if input.Depth != nil {
			body["depth"] = *input.Depth
		}
		if input.MaxEntries != nil {
			body["maxEntries"] = *input.MaxEntries
		}
		return bridge.api(ctx, "/discover", body)
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_project_options", Description: "Inspecciona opciones Compose, perfiles, servicios y archivos de un proyecto antes de registrarlo o editarlo.", Annotations: mcpAnnotations(true, false, false)}, func(ctx context.Context, input mcpProjectOptionsInput) (any, error) {
		body := J{"path": input.Path}
		if input.Files != nil {
			body["files"] = input.Files
		}
		return bridge.api(ctx, "/project-options", body)
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_group", Description: "Crea, renombra o elimina un grupo vacío. Delete devuelve preview hasta recibir confirm=true.", Annotations: mcpAnnotations(false, true, false)}, func(ctx context.Context, input mcpGroupInput) (any, error) {
		switch input.Action {
		case "create":
			return bridge.api(ctx, "/groups", J{"id": input.ID, "name": input.Name})
		case "rename":
			return bridge.api(ctx, "/groups/rename", J{"id": input.ID, "name": input.Name})
		case "delete":
			if !input.Confirm {
				return bridge.api(ctx, "/groups/delete-preview", J{"id": input.ID})
			}
			if input.Fingerprint == "" {
				return nil, fail("FINGERPRINT_REQUIRED", "Envía el fingerprint del preview revisado.", 400)
			}
			return bridge.api(ctx, "/groups/delete", J{"id": input.ID, "confirm": true, "fingerprint": input.Fingerprint})
		default:
			return nil, fail("MCP_INPUT", "Usa create, rename o delete para grupos.", 400)
		}
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_application_configure", Description: "Registra o reemplaza la definición completa de una aplicación; no edita Compose ni inicia contenedores. Lee nearprod_guide primero y conserva modos/rutas existentes al editar.", InputSchema: mcpConfigureSchema(), Annotations: mcpAnnotations(false, false, false)}, func(ctx context.Context, input mcpApplicationConfigInput) (any, error) {
		switch input.Action {
		case "register":
			return bridge.api(ctx, "/stacks", input.Definition)
		case "edit":
			return bridge.api(ctx, "/stacks/edit", J{"target": input.Target, "definition": input.Definition})
		default:
			return nil, fail("MCP_INPUT", "Usa register o edit para aplicaciones.", 400)
		}
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_application_review", Description: "Resuelve y revisa Compose; devuelve fingerprint, riesgos, advertencias, bloqueos, servicios y comando exacto.", Annotations: mcpAnnotations(true, false, false)}, func(ctx context.Context, input mcpApplicationReviewInput) (any, error) {
		return bridge.api(ctx, "/preview", J{"target": input.Target, "mode": text(input.Mode, "dev")})
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_application_approve", Description: "Aprueba exactamente un preview Compose. allowUnsafe debe usarse solo tras revisar los riesgos devueltos.", Annotations: mcpAnnotations(false, true, false)}, func(ctx context.Context, input mcpApplicationApproveInput) (any, error) {
		return bridge.api(ctx, "/trust", J{"target": input.Target, "mode": text(input.Mode, "dev"), "fingerprint": input.Fingerprint, "allowUnsafe": input.AllowUnsafe})
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_application_action", Description: "Inicia, detiene, reinicia o reconstruye una aplicación/grupo. Devuelve una operación asíncrona.", Annotations: mcpAnnotations(false, true, false)}, func(ctx context.Context, input mcpApplicationActionInput) (any, error) {
		if !contains([]string{"up", "stop", "restart", "rebuild"}, input.Action) {
			return nil, fail("MCP_INPUT", "Usa up, stop, restart o rebuild.", 400)
		}
		return bridge.api(ctx, "/actions", J{"target": input.Target, "action": input.Action, "mode": input.Mode, "service": input.Service, "confirmMode": input.ConfirmMode, "startRuntime": input.StartRuntime, "wait": input.Wait})
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_application_lifecycle", Description: "Archive/restore conservan recursos. Remove retira solo metadata y conserva datos. Purge elimina recursos/datos propios, conserva código y recursos compartidos; primero preview, después confirm con fingerprint exacto, typedId y acknowledgeDataLoss.", Annotations: mcpAnnotations(false, true, false)}, func(ctx context.Context, input mcpApplicationLifecycleInput) (any, error) {
		body := J{"target": input.Target}
		switch input.Action {
		case "archive", "restore", "purge":
			if !input.Confirm {
				return bridge.api(ctx, "/stacks/"+input.Action+"-preview", body)
			}
			if input.Fingerprint == "" {
				return nil, fail("FINGERPRINT_REQUIRED", "Envía el fingerprint del preview revisado.", 400)
			}
			body = merge(body, J{"confirm": true, "fingerprint": input.Fingerprint})
			if input.Action == "purge" {
				body = merge(body, J{"typedId": input.TypedID, "acknowledgeDataLoss": input.AcknowledgeDataLoss})
			}
			return bridge.api(ctx, "/stacks/"+input.Action, body)
		case "remove":
			return bridge.api(ctx, "/stacks/remove", J{"target": input.Target, "confirm": input.Confirm})
		default:
			return nil, fail("MCP_INPUT", "Usa archive, restore, remove o purge.", 400)
		}
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_application_logs", Description: "Lee una cantidad acotada de logs de contenedores propios. No abre seguimiento infinito.", Annotations: mcpAnnotations(true, false, false)}, func(ctx context.Context, input mcpLogsInput) (any, error) {
		return bridge.logs(ctx, "/logs", url.Values{"target": {input.Target}, "service": {input.Service}, "container": {input.Container}, "since": {input.Since}}, input.Tail)
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_infrastructure_logs", Description: "Lee logs acotados de una instancia de infraestructura propia: 1–500 líneas, máximo 512 KiB y 30 segundos; sin seguimiento continuo.", Annotations: mcpAnnotations(true, false, false)}, func(ctx context.Context, input mcpInfraLogsInput) (any, error) {
		return bridge.logs(ctx, "/infra/logs", url.Values{"instance": {input.Instance}, "since": {input.Since}}, input.Tail)
	})
	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_operation", Description: "Consulta o solicita cancelar una operación NearProd por ID.", Annotations: mcpAnnotations(false, false, false)}, func(ctx context.Context, input mcpOperationInput) (any, error) {
		action := text(input.Action, "get")
		if action == "get" {
			return bridge.api(ctx, "/operations/"+url.PathEscape(input.ID), nil)
		}
		if action == "cancel" {
			return bridge.api(ctx, "/cancel", J{"id": input.ID})
		}
		return nil, fail("MCP_INPUT", "Usa get o cancel para operaciones.", 400)
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_api", Description: "Puente JSON avanzado a la API local. Consulta nearprod_capabilities para rutas y esquemas. GET usa query; POST usa body ({} si vacío). Confirmaciones/fingerprints siguen obligatorios. Para SSE usa herramientas de logs o consulta estado/operaciones.", Annotations: mcpAnnotations(false, true, true)}, func(ctx context.Context, input mcpAPIInput) (any, error) {
		parsed, e := url.Parse(input.Route)
		if e != nil {
			return nil, fail("API_PATH", "Ruta API inválida.", 400)
		}
		if contains([]string{"/events", "/logs", "/infra/logs"}, parsed.Path) {
			return nil, detailed("MCP_STREAM_ROUTE", "Usa las herramientas de logs acotados o consulta estado/operaciones; nearprod_api acepta JSON, no SSE.", 400, J{"applicationLogs": "nearprod_application_logs", "infrastructureLogs": "nearprod_infrastructure_logs", "events": "nearprod_overview / nearprod_operation"})
		}
		method := strings.ToUpper(strings.TrimSpace(input.Method))
		if method == "GET" {
			return bridge.api(ctx, input.Route, nil)
		}
		if method == "POST" {
			if input.Body == nil {
				input.Body = J{}
			}
			return bridge.api(ctx, input.Route, input.Body)
		}
		return nil, fail("MCP_INPUT", "Usa GET o POST para la API local.", 400)
	})

	return server
}

func RunMCPServer(ctx context.Context, home string, transport mcpsdk.Transport) error {
	info, err := EnsureAgent(ctx, home, true, false)
	if err != nil {
		return err
	}
	return NewMCPServer(info).Run(ctx, transport)
}

type mcpInfraLogsInput struct {
	Instance string `json:"instance" jsonschema:"Infrastructure instance ID or UID from nearprod_infrastructure/list"`
	Tail     int    `json:"tail,omitempty" jsonschema:"Maximum lines across the response, 1 to 500; default 100"`
	Since    string `json:"since,omitempty" jsonschema:"Optional RFC3339 timestamp"`
}

func (b *mcpBridge) logs(ctx context.Context, route string, query url.Values, tail int) (J, error) {
	if tail == 0 {
		tail = 100
	}
	if tail < 1 || tail > 500 {
		return nil, fail("LOG_TAIL", "Usa tail entre 1 y 500.", 400)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	query.Set("follow", "false")
	query.Set("tail", str(tail))
	lines := A{}
	bytes := 0
	truncated := false
	err := streamAPI(ctx, b.info, route, query, func(event string, value J) {
		if event != "line" {
			return
		}
		raw, _ := json.Marshal(value)
		if len(lines) >= tail || bytes+len(raw) > 512<<10 {
			truncated = true
			cancel()
			return
		}
		lines = append(lines, value)
		bytes += len(raw)
	})
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		truncated = true
	}
	if err != nil && !(truncated && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded))) {
		return nil, err
	}
	result := J{"lines": lines, "truncated": truncated || len(lines) >= tail, "follow": false, "limit": tail, "bytes": bytes}
	if route == "/logs" {
		result["target"] = query.Get("target")
	} else {
		result["instance"] = query.Get("instance")
	}
	return result, nil
}
