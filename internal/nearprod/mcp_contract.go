package nearprod

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// This catalog is the published MCP request contract, not a second domain layer.
// Every request still goes through HTTP and the existing service validations.
type mcpOperationSpec struct {
	Area        string `json:"area"`
	Name        string `json:"operation"`
	Method      string `json:"method"`
	Route       string `json:"route"`
	Description string `json:"description"`
	Parameters  J      `json:"parameters"`
	Action      string `json:"action,omitempty"`
	ReadOnly    bool   `json:"readOnly"`
	Async       bool   `json:"async"`
	Result      string `json:"result"`
}

func mcpString(description string) J {
	return J{"type": "string", "minLength": 1, "description": description}
}
func mcpBool(description string) J { return J{"type": "boolean", "description": description} }
func mcpEnum(description string, values ...string) J {
	return J{"type": "string", "enum": values, "description": description}
}
func mcpNumber(description string, minimum, maximum float64, integer bool) J {
	typ := "number"
	if integer {
		typ = "integer"
	}
	schema := J{"type": typ, "minimum": minimum, "description": description}
	if maximum > 0 {
		schema["maximum"] = maximum
	}
	return schema
}
func mcpArray(description string, items J, min, max int) J {
	return J{"type": "array", "items": items, "minItems": min, "maxItems": max, "description": description}
}
func mcpObject(required string, fields J) J {
	return J{"type": "object", "properties": fields, "required": strings.Fields(required), "additionalProperties": false}
}
func mcpConfirmed(schema J, fingerprint bool) J {
	out := copyJ(schema)
	props := obj(out["properties"])
	props["confirm"] = J{"type": "boolean", "const": true, "description": "Explicit authorization to apply the reviewed operation; never infer consent from a preview."}
	out["required"] = append(arr(out["required"]), "confirm")
	if fingerprint {
		props["fingerprint"] = mcpString("Exact fingerprint from the preview the user reviewed. On PREVIEW_CHANGED, review again; do not silently refresh approval.")
		out["required"] = append(arr(out["required"]), "fingerprint")
	}
	return out
}
func mcpModeSchema() J {
	return mcpObject("files", J{
		"files":    mcpArray("Compose paths, relative to project path or absolute, in merge order; explicit -f files only.", mcpString("Compose file path within registered roots"), 1, 16),
		"envFiles": mcpArray("Interpolation environment files in order. Omit for automatic .env; do not send their secret contents.", mcpString("Environment file path"), 0, 16),
		"profiles": mcpArray("Enabled Compose profiles. Omit for default services.", mcpString("Profile name or *"), 0, 32),
	})
}
func mcpApplicationSchema() J {
	target := mcpObject("service port", J{"service": mcpString("Compose HTTP service name"), "port": mcpNumber("Internal container HTTP port, not published host port", 1, 65535, true)})
	route := copyJ(target)
	obj(route["properties"])["host"] = mcpString("Unique local hostname ending in .localhost, without scheme or path")
	obj(route["properties"])["verify"] = target
	route["required"] = append(arr(route["required"]), "host")
	return mcpObject("product slug path projectName modes", J{
		"product":     mcpString("Group ID; lowercase letters, digits, hyphens or underscores"),
		"groupName":   mcpString("Display name used when creating a missing group"),
		"slug":        mcpString("Application ID within its group; target becomes product/slug"),
		"name":        mcpString("Application display name; defaults to slug"),
		"path":        mcpString("Existing absolute source directory under a registered discovery root"),
		"projectName": mcpString("Stable, unique Docker Compose project name; preserve existing identity when adopting"),
		"modes":       mcpObject("dev", J{"dev": mcpModeSchema(), "verify": mcpModeSchema()}),
		"routes":      mcpArray("Optional local HTTP routes. Databases and workers do not need a route.", route, 0, 8),
		"links":       mcpArray("Optional local links; these do not configure routing.", mcpObject("url", J{"label": mcpString("Link label"), "url": mcpString("Local HTTP(S) URL")}), 0, 8),
	})
}

func mcpOperations() []mcpOperationSpec {
	out := []mcpOperationSpec{}
	add := func(area, name, method, route, description string, schema J, read, async bool, action string) {
		result := "JSON snapshot or domain result; errors contain code, message and optional details."
		if strings.Contains(name, "preview") || name == "review" {
			result = "Review with fingerprint and applicable blockers, warnings, affected resources and preservation rules. A preview is not approval."
		}
		if async {
			result = "Operation with id and state. Poll nearprod_operation with that id until succeeded, failed or cancelled; then refresh status and check health separately."
		}
		out = append(out, mcpOperationSpec{area, name, method, route, description, schema, action, read, async, result})
	}
	empty := mcpObject("", J{})
	target := mcpObject("target", J{"target": mcpString("group/application ID from catalog")})
	instance := mcpObject("instance", J{"instance": mcpString("Infrastructure instance ID or UID from inventory")})
	database := mcpObject("database", J{"database": mcpString("Database ID from infrastructure inventory")})
	root := mcpObject("root", J{"root": mcpString("Absolute discovery root directory")})
	mode := mcpEnum("Compose mode; dev by default", "dev", "verify")
	review := copyJ(target)
	obj(review["properties"])["mode"] = mode
	for _, v := range []struct{ name, route, description string }{
		{"health", "/health", "Local agent identity and version; does not check application readiness."},
		{"catalog", "/catalog", "Groups, active and archived applications, runtime settings and operations."},
		{"doctor", "/doctor", "Installed prerequisites, runtime state and host capabilities."},
		{"metrics", "/metrics", "Host/VM/container memory and CPU observations; diagnose OOM before changing limits."},
		{"config-paths", "/config/paths", "Configuration file locations, without contents."},
		{"migration", "/config/migration", "Read-only catalog migration preview; migration execution is CLI-only."},
		{"mcp", "/mcp", "Local MCP connection instructions, without tokens."},
	} {
		add("diagnostics", v.name, "GET", v.route, v.description, empty, true, false, "")
	}
	add("diagnostics", "status", "GET", "/status", "Observed execution, health, routes and watch state. Use refresh=true for fresh Engine observations.", mcpObject("", J{"refresh": mcpBool("Request fresh Engine observations; omitted/false uses cached status")}), true, false, "")
	add("diagnostics", "image", "GET", "/images", "Inspect a selected image platform and digests.", mcpObject("id", J{"id": mcpString("Full sha256: image ID from observed containers")}), true, false, "")
	add("diagnostics", "operation", "GET", "/operations/{id}", "Get an asynchronous operation by its returned id.", mcpObject("id", J{"id": mcpString("Operation id")}), true, false, "")
	add("diagnostics", "cancel", "POST", "/cancel", "Request cancellation; Engine work already accepted may continue. Refresh actual state.", mcpObject("id", J{"id": mcpString("Operation id")}), false, false, "")
	add("roots", "add", "POST", "/roots", "Register an existing root for discovery; does not scan or edit its contents.", root, false, false, "")
	add("roots", "remove-preview", "POST", "/roots/remove-preview", "Inspect dependent catalog entries before removing a root.", root, true, false, "")
	add("roots", "remove", "POST", "/roots/remove", "Remove a root only when unreferenced; preserve its files.", mcpConfirmed(root, true), false, false, "")
	add("applications", "discover", "POST", "/discover", "Discover Compose files offline under a registered root; no containers start.", mcpObject("root", J{"root": mcpString("Registered absolute root"), "depth": mcpNumber("Maximum depth; default 8", 0, 32, true), "maxEntries": mcpNumber("Maximum entries to inspect; default 20000", 1, 100000, true), "ignore": mcpArray("Additional directory names to skip", mcpString("Directory name"), 0, 1000)}), true, false, "")
	add("applications", "project-options", "POST", "/project-options", "Offline hints for Compose files, environment filenames, services and profiles. Final validation requires review.", mcpObject("path", J{"path": mcpString("Absolute project directory"), "files": mcpArray("Explicit Compose files in merge order; omit for suggested files", mcpString("File path"), 0, 16)}), true, false, "")
	add("groups", "create", "POST", "/groups", "Create an organizational group; no networks or containers.", mcpObject("name", J{"name": mcpString("Display name"), "id": mcpString("Stable group ID; defaults to slug of name")}), false, false, "")
	add("groups", "rename", "POST", "/groups/rename", "Change group display name, keeping its ID.", mcpObject("id name", J{"id": mcpString("Group ID"), "name": mcpString("New display name")}), false, false, "")
	group := mcpObject("id", J{"id": mcpString("Group ID")})
	add("groups", "delete-preview", "POST", "/groups/delete-preview", "Check that group contains no active or archived apps.", group, true, false, "")
	add("groups", "delete", "POST", "/groups/delete", "Delete an empty group using its reviewed fingerprint.", mcpConfirmed(group, true), false, false, "")
	add("applications", "register", "POST", "/stacks", "Register metadata without starting containers or writing project files.", mcpApplicationSchema(), false, false, "")
	add("applications", "register-batch", "POST", "/stacks/batch", "Register a set atomically; no partial registration or containers.", mcpObject("definitions", J{"definitions": mcpArray("Complete application definitions", mcpApplicationSchema(), 1, 32), "groupName": mcpString("Name for a missing group")}), false, false, "")
	add("applications", "edit", "POST", "/stacks/edit", "Replace the complete application definition; preserve fields you intend to keep. Running apps impose domain restrictions.", mcpObject("target definition", J{"target": mcpString("Current group/application ID"), "definition": mcpApplicationSchema()}), false, false, "")
	add("applications", "remove", "POST", "/stacks/remove", "Legacy metadata-only removal: preserves data/resources, requires stopped runtime and zero bindings. Use purge for complete deletion.", mcpConfirmed(target, false), false, false, "")
	for _, action := range []string{"archive", "restore", "purge"} {
		add("applications", action+"-preview", "POST", "/stacks/"+action+"-preview", "Review application "+action+". Archive preserves resources; restore re-registers; purge deletes exclusive owned resources and data while preserving source code and shared resources.", target, true, false, "")
		schema := mcpConfirmed(target, true)
		if action == "purge" {
			obj(schema["properties"])["typedId"] = mcpString("Exact target ID, acknowledging the application selected for permanent deletion")
			obj(schema["properties"])["acknowledgeDataLoss"] = J{"type": "boolean", "const": true, "description": "Explicit consent to delete all owned application data"}
			schema["required"] = append(arr(schema["required"]), "typedId", "acknowledgeDataLoss")
		}
		add("applications", action, "POST", "/stacks/"+action, "Apply the reviewed "+action+" with the exact fingerprint. A changed preview requires a new review.", schema, false, action == "purge", "")
	}
	add("applications", "review", "POST", "/preview", "Resolve real Compose and return services, command, warnings, risks, blockers and fingerprint. Never assume running means healthy.", review, true, false, "")
	trust := copyJ(review)
	obj(trust["properties"])["fingerprint"] = mcpString("Exact reviewed Compose fingerprint")
	obj(trust["properties"])["allowUnsafe"] = mcpBool("Accept returned non-blocking risks only within explicit user authorization")
	trust["required"] = append(arr(trust["required"]), "fingerprint")
	add("applications", "approve", "POST", "/trust", "Approve a reviewed Compose fingerprint; does not start containers or bypass blockers.", trust, false, false, "")
	add("applications", "adoption-preview", "POST", "/adoption", "Inspect existing Compose containers and ownership before adopting.", target, true, false, "")
	add("applications", "adopt", "POST", "/adopt", "Adopt matching existing resources using the reviewed fingerprint; preserves names and volumes.", mcpConfirmed(target, true), false, false, "")
	for _, action := range []string{"up", "stop", "restart", "rebuild"} {
		schema := copyJ(review)
		props := obj(schema["properties"])
		props["service"] = mcpString("Optional single Compose service")
		props["confirmMode"] = mcpBool("Explicit consent to switch active mode, which recreates services")
		props["startRuntime"] = mcpBool("Allow starting the configured local runtime")
		props["wait"] = mcpBool("Ask Compose to wait for health when supported; still poll the asynchronous operation")
		add("applications", action, "POST", "/actions", "Run "+action+" on group/application or group. up also applies changed configuration; rebuild forces a build. Review and approve first for up/rebuild.", schema, false, true, action)
	}
	for _, action := range []string{"start", "stop"} {
		add("applications", "watch-"+action, "POST", "/watch", "Compose Watch "+action+"; start requires develop.watch. Bind mounts/dev servers generally reload without Watch.", target, false, false, action)
	}
	add("runtime", "settings", "POST", "/runtime/settings", "Select supported runtime/context/profile. Bound resources can block changes.", mcpObject("kind context", J{"kind": mcpEnum("Runtime kind", "colima", "native"), "context": mcpString("Existing Docker context name"), "profile": mcpString("Colima profile; default default")}), false, false, "")
	resources := mcpObject("memory cpus", J{"memory": mcpNumber("VM memory in GiB; upper bound is actual physical host memory", 0.5, 0, false), "cpus": mcpNumber("CPU count; upper bound is actual host CPU count", 1, 0, true)})
	add("runtime", "preview", "POST", "/runtime/preview", "Preview a Colima resource change and all affected containers, including other projects.", resources, true, false, "")
	add("runtime", "configure", "POST", "/runtime/actions", "Reconfigure and restart Colima. Review impact on all containers; applications may need restarting.", mcpConfirmed(resources, true), false, true, "configure")
	add("runtime", "start", "POST", "/runtime/actions", "Start the configured runtime with explicit authorization.", mcpConfirmed(empty, false), false, true, "start")
	add("proxy", "status", "GET", "/proxy", "Inspect local Traefik proxy and observed routes.", empty, true, false, "")
	port := mcpObject("", J{"port": mcpNumber("Local HTTP listening port; omit for current setting", 1, 65535, true)})
	add("proxy", "preview", "POST", "/proxy/preview", "Review proxy port/configuration and affected routes.", port, true, false, "")
	add("proxy", "start", "POST", "/proxy/actions", "Start/update local proxy with exact reviewed settings.", mcpConfirmed(port, true), false, true, "start")
	add("proxy", "stop", "POST", "/proxy/actions", "Stop the shared proxy; all its local application URLs are affected.", mcpConfirmed(empty, false), false, true, "stop")
	add("proxy", "check", "POST", "/proxy/check", "Check DNS and HTTP access for a configured application hostname; HTTP response is distinct from container health.", mcpObject("target host", J{"target": mcpString("Application ID"), "host": mcpString("Configured .localhost hostname")}), true, false, "")
	addMCPInfrastructure(add, empty, instance, database, mode)
	add("maintenance", "inventory", "GET", "/tools", "Inventory tools and host capabilities before proposing installation.", empty, true, false, "")
	add("maintenance", "startup", "GET", "/startup", "Read configured automatic startup status. Setup changes use CLI.", empty, true, false, "")
	add("maintenance", "versions", "POST", "/tools/versions", "Get available supported package versions/formulas for a tool.", mcpObject("tool", J{"tool": mcpString("Tool ID from inventory")}), true, false, "")
	tool := mcpObject("tool", J{"tool": mcpString("Tool ID from inventory"), "action": mcpEnum("Maintenance action; default install", "install", "upgrade", "repair"), "formula": mcpString("Exact supported formula from versions; no arbitrary commands")})
	add("maintenance", "preview", "POST", "/tools/preview", "Preview host tool maintenance. Repair applies only to Compose/Buildx plugins.", tool, true, false, "")
	add("maintenance", "apply", "POST", "/tools/actions", "Apply reviewed host tool maintenance; stop Watch first and respect platform support.", mcpConfirmed(tool, true), false, true, "")
	add("maintenance", "updates", "POST", "/updates", "Check available tool updates; may access the network.", empty, true, false, "")
	add("maintenance", "config-backup", "POST", "/config/backup", "Create a private configuration backup INCLUDING credentials, excluding SQL data. Never publish the archive.", mcpConfirmed(empty, false), false, false, "")
	return out
}

type mcpAddOperation func(string, string, string, string, string, J, bool, bool, string)

func addMCPInfrastructure(add mcpAddOperation, empty, instance, database, mode J) {
	add("infrastructure", "list", "GET", "/infra", "Instances, databases, bindings, archived records and supported engine options.", empty, true, false, "")
	add("infrastructure", "ports", "POST", "/infra/ports", "Inspect local port availability over a bounded range.", mcpObject("", J{"start": mcpNumber("First port; default 15432", 1024, 65535, true), "end": mcpNumber("Last port; maximum range 128", 1024, 65535, true), "instance": mcpString("Optional instance ID to account for its existing port")}), true, false, "")
	def := mcpObject("engine", J{"engine": mcpEnum("Shared data engine", "postgres", "mysql", "redis"), "id": mcpString("Stable instance ID"), "name": mcpString("Display name"), "image": mcpString("Supported official numeric image tag from inventory; not latest"), "persistence": mcpObject("kind", J{"kind": mcpEnum("SQL requires volume/folder; none is Redis-only", "volume", "folder", "none"), "path": mcpString("Empty dedicated absolute data folder, when kind=folder")}), "hostPort": J{"type": []string{"integer", "null"}, "minimum": 1024, "maximum": 65535, "description": "Optional loopback port; omitted/null means Docker-network-only"}, "memoryMiB": mcpNumber("Container memory limit in MiB, not a reservation; engine minimum applies", 1, 32768, true), "maxConnections": mcpNumber("SQL connection limit; ignored for Redis", 8, 300, true)})
	add("infrastructure", "create-preview", "POST", "/infra/preview", "Preview new data instance, storage ownership, ports and configuration.", def, true, false, "")
	add("infrastructure", "create", "POST", "/infra/actions", "Create the reviewed instance and owned storage.", mcpConfirmed(def, true), false, true, "create")
	add("infrastructure", "start", "POST", "/infra/actions", "Start/check an existing instance.", instance, false, true, "start")
	for _, pair := range []struct {
		name, route string
		schema      J
	}{
		{"stop", "stop-preview", instance}, {"archive-instance", "archive-preview", instance}, {"restore-instance", "restore-instance-preview", instance},
		{"archive-database", "archive-database-preview", database}, {"restore-database", "restore-database-preview", database},
	} {
		add("infrastructure", pair.name+"-preview", "POST", "/infra/"+pair.route, "Review "+pair.name+". Archiving preserves runtime/data; restoring catalog entries does not start them.", pair.schema, true, false, "")
		schema := mcpConfirmed(pair.schema, true)
		if pair.name == "stop" {
			obj(schema["properties"])["allowActive"] = mcpBool("Explicit consent to interrupt active consumers of this shared instance")
		}
		add("infrastructure", pair.name, "POST", "/infra/actions", "Apply reviewed "+pair.name+"; preserve the exact fingerprint.", schema, false, true, pair.name)
	}
	add("infrastructure", "database", "POST", "/infra/actions", "Create database and application user; Redis creates a dedicated application credential. Password is generated, not supplied.", mcpConfirmed(mcpObject("instance name", J{"instance": mcpString("Instance ID"), "name": mcpString("Database/application name: lowercase letter then up to 39 lowercase letters, digits or underscores"), "username": mcpString("Optional application username; reserved admin names are rejected")}), false), false, true, "database")
	add("infrastructure", "connection", "POST", "/infra/connection", "Connection details. Keep reveal=false unless credentials are explicitly needed; do not repeat secrets in chat or logs.", mcpObject("database", J{"database": mcpString("Database ID"), "reveal": mcpBool("Reveal credentials only within authorized scope; default false")}), true, false, "")
	add("infrastructure", "check", "POST", "/infra/actions", "Perform an authenticated read/write probe using a temporary client; does not test business queries.", database, false, true, "check")
	mapping := J{}
	for _, key := range []string{"url", "host", "port", "database", "user", "password"} {
		mapping[key] = mcpString("Environment VARIABLE NAME receiving " + key + ", never the credential value")
	}
	bind := mcpObject("target database services", J{"target": mcpString("Application ID"), "database": mcpString("Database ID"), "mode": mode, "services": mcpArray("Distinct consumer Compose services", mcpString("Service name"), 1, 16), "mapping": mcpObject("", mapping)})
	add("infrastructure", "binding-preview", "POST", "/infra/binding-preview", "Preview service connection and environment mapping; defaults to DATABASE_URL/REDIS_URL. No data migration or .env edit.", bind, true, false, "")
	add("infrastructure", "bind", "POST", "/infra/actions", "Save reviewed binding; then review/approve/up application to apply generated overlay.", mcpConfirmed(bind, true), false, true, "bind")
	binding := mcpObject("binding", J{"binding": mcpString("Binding ID from infrastructure inventory")})
	add("infrastructure", "unbind", "POST", "/infra/actions", "Remove binding metadata; reapply the application to update its containers.", mcpConfirmed(binding, false), false, true, "unbind")
	add("infrastructure", "check-binding", "POST", "/infra/actions", "Check the actual binding configuration and connection.", binding, false, true, "check-binding")
	backup := copyJ(database)
	obj(backup["properties"])["directory"] = mcpString("Private absolute backup directory; omit for NearProd default")
	add("infrastructure", "backup", "POST", "/infra/actions", "Create database backup, not a configuration backup.", mcpConfirmed(backup, false), false, true, "backup")
	restore := copyJ(database)
	obj(restore["properties"])["file"] = mcpString("Absolute backup file path")
	obj(restore["properties"])["trustedBackup"] = J{"type": "boolean", "const": true, "description": "Explicit trust in this backup and consent to restore its contents"}
	restore["required"] = append(arr(restore["required"]), "file", "trustedBackup")
	add("infrastructure", "restore", "POST", "/infra/actions", "Restore a trusted backup to the selected database after explaining data impact.", mcpConfirmed(restore, false), false, true, "restore")
	purge := copyJ(backup)
	obj(purge["properties"])["mode"] = mcpEnum("Backup first, or permanent deletion without backup", "backup-purge", "purge")
	purge["required"] = append(arr(purge["required"]), "mode")
	add("infrastructure", "purge-database-preview", "POST", "/infra/purge-database-preview", "Review permanent SQL database/user deletion and exclusive ownership. Redis cannot isolate data by credential.", purge, true, false, "")
	purge = mcpConfirmed(purge, true)
	obj(purge["properties"])["typedId"] = mcpString("Exact database ID; required when mode=purge")
	obj(purge["properties"])["acknowledgeDataLoss"] = mcpBool("Explicit data-loss acceptance; must be true when mode=purge")
	add("infrastructure", "purge-database", "POST", "/infra/actions", "Apply reviewed database purge; mode=purge requires typedId and acknowledgeDataLoss=true.", purge, false, true, "purge-database")
	add("infrastructure", "logs", "POST", "/infra/actions", "Legacy asynchronous last-200-lines log request. Prefer nearprod_infrastructure_logs for bounded direct logs.", instance, true, true, "logs")
}

// Existing named tools remain available. Domain tools fill the missing routes.
func mcpDedicatedTool(route string) string {
	switch route {
	case "/catalog":
		return "nearprod_overview"
	case "/discover":
		return "nearprod_discover"
	case "/project-options":
		return "nearprod_project_options"
	case "/groups", "/groups/rename", "/groups/delete-preview", "/groups/delete":
		return "nearprod_group"
	case "/stacks", "/stacks/edit":
		return "nearprod_application_configure"
	case "/preview":
		return "nearprod_application_review"
	case "/trust":
		return "nearprod_application_approve"
	case "/actions":
		return "nearprod_application_action"
	case "/stacks/remove", "/stacks/archive-preview", "/stacks/archive", "/stacks/restore-preview", "/stacks/restore", "/stacks/purge-preview", "/stacks/purge":
		return "nearprod_application_lifecycle"
	case "/operations/{id}", "/cancel":
		return "nearprod_operation"
	}
	return ""
}
func (b *mcpBridge) invoke(ctx context.Context, spec mcpOperationSpec, params J) (J, error) {
	route := spec.Route
	if spec.Method == "GET" {
		values := url.Values{}
		for key, value := range params {
			if key == "refresh" {
				if truth(value) {
					values.Set(key, "1")
				}
				continue
			}
			if strings.Contains(route, "{"+key+"}") {
				route = strings.ReplaceAll(route, "{"+key+"}", url.PathEscape(str(value)))
			} else {
				values.Set(key, str(value))
			}
		}
		if len(values) > 0 {
			route += "?" + values.Encode()
		}
		return b.api(ctx, route, nil)
	}
	body := copyJ(params)
	if spec.Action != "" {
		body["action"] = spec.Action
	}
	return b.api(ctx, route, body)
}
func addMCPDomainTools(server *mcpsdk.Server, bridge *mcpBridge) {
	groups := map[string][]mcpOperationSpec{}
	for _, spec := range mcpOperations() {
		if mcpDedicatedTool(spec.Route) == "" {
			groups[spec.Area] = append(groups[spec.Area], spec)
		}
	}
	areas := make([]string, 0, len(groups))
	for area := range groups {
		areas = append(areas, area)
	}
	sort.Strings(areas)
	for _, area := range areas {
		specs := groups[area]
		variants := A{}
		names := []string{}
		readOnly := true
		for _, spec := range specs {
			names = append(names, spec.Name)
			readOnly = readOnly && spec.ReadOnly
			variants = append(variants, mcpObject("operation parameters", J{"operation": J{"type": "string", "const": spec.Name, "description": spec.Description + " " + spec.Result}, "parameters": spec.Parameters}))
		}
		schema := mcpObject("operation parameters", J{"operation": mcpEnum("Operation in this area; each operation has a matching parameters schema", names...), "parameters": J{"type": "object", "description": "Exact parameters for the selected operation, described by the matching anyOf branch"}})
		schema["anyOf"] = variants
		addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_" + area, Description: "NearProd " + area + ": " + strings.Join(names, ", ") + ". Select operation and its matching parameters schema; previews never imply consent. See nearprod_capabilities for result/route details.", InputSchema: schema, Annotations: mcpAnnotations(readOnly, !readOnly, true)}, func(ctx context.Context, input J) (any, error) {
			for _, spec := range specs {
				if str(input["operation"]) == spec.Name {
					return bridge.invoke(ctx, spec, obj(input["parameters"]))
				}
			}
			return nil, fail("MCP_INPUT", "Operación no disponible; consulta nearprod_capabilities.", 400)
		})
	}
}

func mcpCatalog(area, operation string, schemas bool) J {
	entries := A{}
	for _, spec := range mcpOperations() {
		if area != "" && spec.Area != area || operation != "" && spec.Name != operation {
			continue
		}
		raw, _ := json.Marshal(spec)
		entry, _ := decodeObject(raw)
		tool := mcpDedicatedTool(spec.Route)
		if tool == "" {
			tool = "nearprod_" + spec.Area
		}
		entry["tool"] = tool
		if !schemas && operation == "" {
			delete(entry, "parameters")
		}
		entries = append(entries, entry)
	}
	return J{"operations": entries, "note": "For exact request fields, select area/operation or schemas=true. Parameter schemas here describe HTTP requests (injected action is listed separately). Named tools have their own tools/list schemas. Domain tools use {operation, parameters}; nearprod_api uses method, route and body. Inject action for /actions routes as documented. GET parameters belong in the query (operation id in path).", "streaming": A{J{"route": "/logs", "tool": "nearprod_application_logs", "equivalent": "bounded snapshot; no infinite follow"}, J{"route": "/infra/logs", "tool": "nearprod_infrastructure_logs", "equivalent": "bounded snapshot; no infinite follow"}, J{"route": "/events", "equivalent": "poll nearprod_overview / nearprod_operation; no SSE through the JSON bridge"}}, "browserOnly": A{"POST /session", "POST /logout", "POST /pair"}, "cliOnly": A{"POST /agent/stop"}}
}

func mcpConfigureSchema() J {
	schema := mcpObject("action definition", J{"action": mcpEnum("Configuration action", "register", "edit"), "target": mcpString("Current group/application ID; required for edit"), "definition": mcpApplicationSchema()})
	schema["anyOf"] = A{J{"properties": J{"action": J{"const": "register"}}}, J{"properties": J{"action": J{"const": "edit"}}, "required": A{"target"}}}
	return schema
}
