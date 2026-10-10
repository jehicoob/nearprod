package nearprod

import (
	"context"
	_ "embed"
	"encoding/json"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed mcpdocs/agent-guide.md
var mcpAgentGuide string

const mcpInstructions = "NearProd controls the selected local environment. First read nearprod_guide (also nearprod://guide), then nearprod_capabilities for exact operation schemas. Inspect, explain missing configuration, edit/create files with your client's filesystem tools within existing authorization, review and approve the exact fingerprint, apply, poll the returned operation id, and verify observed health separately. Availability/previews are not consent. Never silently refresh a destructive approval; never expose credentials. Use dedicated bounded log tools, not SSE through nearprod_api."

type mcpCapabilitiesInput struct {
	Area      string `json:"area,omitempty" jsonschema:"Filter: diagnostics, roots, groups, applications, runtime, proxy, infrastructure or maintenance"`
	Operation string `json:"operation,omitempty" jsonschema:"Exact operation name from the index; selecting it includes its complete parameter schema"`
	Schemas   bool   `json:"schemas,omitempty" jsonschema:"Include complete schemas for all selected operations; default false for a compact index"`
}

func addMCPGuidance(server *mcpsdk.Server) {
	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_guide", Description: "Read NearProd requirements and the inspect/explain/edit/validate/apply/verify workflow before configuring projects. Includes Compose restrictions, examples, file-edit boundaries, health, OOM and safe deletion semantics.", Annotations: mcpAnnotations(true, false, false)}, func(context.Context, mcpEmptyInput) (any, error) { return J{"guide": mcpAgentGuide}, nil })
	addNearProdTool(server, &mcpsdk.Tool{Name: "nearprod_capabilities", Description: "Discover every web/API operation, matching MCP tool, exact request fields, result expectations and confirmation requirements. With no arguments returns an index; select area/operation or schemas=true for full schemas.", Annotations: mcpAnnotations(true, false, false)}, func(_ context.Context, input mcpCapabilitiesInput) (any, error) {
		result := mcpCatalog(input.Area, input.Operation, input.Schemas)
		if len(arr(result["operations"])) == 0 {
			return nil, fail("MCP_OPERATION", "No hay operaciones coincidentes; consulta el índice sin filtros.", 400)
		}
		return result, nil
	})
	for _, entry := range []struct {
		uri, name, mime string
		read            func() string
	}{
		{"nearprod://guide", "NearProd agent guide", "text/markdown", func() string { return mcpAgentGuide }},
		{"nearprod://operations", "NearProd operation schemas", "application/json", func() string { raw, _ := json.Marshal(mcpCatalog("", "", true)); return string(raw) }},
	} {
		server.AddResource(&mcpsdk.Resource{URI: entry.uri, Name: entry.name, MIMEType: entry.mime}, func(_ context.Context, _ *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
			return &mcpsdk.ReadResourceResult{Contents: []*mcpsdk.ResourceContents{{URI: entry.uri, MIMEType: entry.mime, Text: entry.read()}}}, nil
		})
	}
}
