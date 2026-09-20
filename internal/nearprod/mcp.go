package nearprod

import (
	"context"
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
	Depth      int      `json:"depth,omitempty" jsonschema:"Maximum directory depth, from 1 to 20"`
	MaxEntries int      `json:"maxEntries,omitempty" jsonschema:"Maximum filesystem entries to inspect"`
	Ignore     []string `json:"ignore,omitempty" jsonschema:"Additional directory names to ignore"`
}

type mcpProjectOptionsInput struct {
	Path  string   `json:"path" jsonschema:"Absolute project path under a registered root"`
	Files []string `json:"files,omitempty" jsonschema:"Optional Compose files to inspect"`
}

type mcpGroupInput struct {
	Action  string `json:"action" jsonschema:"Action: create, rename, or delete"`
	ID      string `json:"id,omitempty" jsonschema:"Stable group identifier"`
	Name    string `json:"name,omitempty" jsonschema:"Human-readable group name"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"Confirm deletion after reviewing the returned preview"`
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
}

type mcpApplicationLifecycleInput struct {
	Target  string `json:"target" jsonschema:"group/application identifier"`
	Action  string `json:"action" jsonschema:"Action: archive, restore, or remove"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"Apply the action after NearProd revalidates its preview"`
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
			J{"area": "API avanzada", "access": "total", "detail": "Acceso a todos los endpoints locales, incluidas acciones destructivas con sus confirmaciones existentes."},
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
		return nil, mcpOutput{Result: result}, err
	})
}

func (b *mcpBridge) api(ctx context.Context, route string, body any) (J, error) {
	return requestAPI(ctx, b.info, route, body, 60*time.Second)
}

func NewMCPServer(info J) *mcpsdk.Server {
	bridge := &mcpBridge{info: info}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "nearprod", Version: Version}, &mcpsdk.ServerOptions{
		Instructions: "NearProd controla un entorno local. Revisa previews y fingerprints antes de confirmar acciones destructivas. Las operaciones Docker son asíncronas: conserva el operationId y consulta su estado.",
		Capabilities: &mcpsdk.ServerCapabilities{},
		KeepAlive:    30 * time.Second,
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_overview", Description: "Obtiene catálogo, aplicaciones, grupos, infraestructura, operaciones y estado fresco del Engine local.", Annotations: mcpAnnotations(true, false, false)}, func(ctx context.Context, _ mcpEmptyInput) (any, error) {
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
		return bridge.api(ctx, "/discover", J{"root": input.Root, "depth": input.Depth, "maxEntries": input.MaxEntries, "ignore": input.Ignore})
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_project_options", Description: "Inspecciona opciones Compose, perfiles, servicios y archivos de un proyecto antes de registrarlo o editarlo.", Annotations: mcpAnnotations(true, false, false)}, func(ctx context.Context, input mcpProjectOptionsInput) (any, error) {
		return bridge.api(ctx, "/project-options", J{"path": input.Path, "files": input.Files})
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_group", Description: "Crea, renombra o elimina un grupo vacío. Delete devuelve preview hasta recibir confirm=true.", Annotations: mcpAnnotations(false, true, false)}, func(ctx context.Context, input mcpGroupInput) (any, error) {
		switch input.Action {
		case "create":
			return bridge.api(ctx, "/groups", J{"id": input.ID, "name": input.Name})
		case "rename":
			return bridge.api(ctx, "/groups/rename", J{"id": input.ID, "name": input.Name})
		case "delete":
			preview, err := bridge.api(ctx, "/groups/delete-preview", J{"id": input.ID})
			if err != nil || !input.Confirm {
				return preview, err
			}
			return bridge.api(ctx, "/groups/delete", J{"id": input.ID, "confirm": true, "fingerprint": preview["fingerprint"]})
		default:
			return nil, fail("MCP_INPUT", "Usa create, rename o delete para grupos.", 400)
		}
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_application_configure", Description: "Registra o edita metadata de una aplicación sin iniciar contenedores.", Annotations: mcpAnnotations(false, false, false)}, func(ctx context.Context, input mcpApplicationConfigInput) (any, error) {
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
		return bridge.api(ctx, "/actions", J{"target": input.Target, "action": input.Action, "mode": input.Mode, "service": input.Service, "confirmMode": input.ConfirmMode, "startRuntime": input.StartRuntime})
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_application_lifecycle", Description: "Previsualiza o aplica archive/restore; remove conserva datos pero exige runtime detenido y cero bindings.", Annotations: mcpAnnotations(false, true, false)}, func(ctx context.Context, input mcpApplicationLifecycleInput) (any, error) {
		body := J{"target": input.Target}
		switch input.Action {
		case "archive":
			preview, err := bridge.api(ctx, "/stacks/archive-preview", body)
			if err != nil || !input.Confirm {
				return preview, err
			}
			return bridge.api(ctx, "/stacks/archive", merge(body, J{"confirm": true, "fingerprint": preview["fingerprint"]}))
		case "restore":
			preview, err := bridge.api(ctx, "/stacks/restore-preview", body)
			if err != nil || !input.Confirm {
				return preview, err
			}
			return bridge.api(ctx, "/stacks/restore", merge(body, J{"confirm": true, "fingerprint": preview["fingerprint"]}))
		case "remove":
			return bridge.api(ctx, "/stacks/remove", J{"target": input.Target, "confirm": input.Confirm})
		default:
			return nil, fail("MCP_INPUT", "Usa archive, restore o remove.", 400)
		}
	})

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_application_logs", Description: "Lee una cantidad acotada de logs de contenedores propios. No abre seguimiento infinito.", Annotations: mcpAnnotations(true, false, false)}, func(ctx context.Context, input mcpLogsInput) (any, error) {
		tail := input.Tail
		if tail == 0 {
			tail = 100
		}
		query := url.Values{"target": {input.Target}, "follow": {"false"}, "tail": {str(tail)}, "service": {input.Service}, "container": {input.Container}, "since": {input.Since}}
		lines := A{}
		err := streamAPI(ctx, bridge.info, "/logs", query, func(event string, value J) {
			if event == "line" {
				lines = append(lines, value)
			}
		})
		return J{"target": input.Target, "lines": lines, "truncated": len(lines) >= tail}, err
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

	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_api", Description: "Acceso avanzado a toda la API local de NearProd, incluida infraestructura y acciones destructivas. Las confirmaciones y fingerprints del endpoint siguen siendo obligatorios.", Annotations: mcpAnnotations(false, true, true)}, func(ctx context.Context, input mcpAPIInput) (any, error) {
		method := strings.ToUpper(strings.TrimSpace(input.Method))
		if method == "GET" {
			return bridge.api(ctx, input.Route, nil)
		}
		if method == "POST" {
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
