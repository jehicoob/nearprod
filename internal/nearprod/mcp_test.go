package nearprod

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func connectMCP(t *testing.T, info J) *mcpsdk.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	serverSession, err := NewMCPServer(info).Connect(context.Background(), serverTransport, nil)
	must(t, err)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "nearprod-test", Version: "1"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	must(t, err)
	t.Cleanup(func() {
		_ = clientSession.Close()
		_ = serverSession.Close()
	})
	return clientSession
}

func TestMCPListsBoundedToolsAndUsesLocalAgent(t *testing.T) {
	f := newFixture(t, true)
	client := connectMCP(t, f.A.Info)
	tools, err := client.ListTools(context.Background(), nil)
	must(t, err)
	if len(tools.Tools) != 12 {
		t.Fatalf("tools=%d", len(tools.Tools))
	}
	names := []string{}
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
		if tool.Name == "nearprod_api" && (tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint) {
			t.Fatal("advanced API tool must be marked destructive")
		}
	}
	for _, name := range []string{"nearprod_overview", "nearprod_application_action", "nearprod_application_logs", "nearprod_api"} {
		if !contains(names, name) {
			t.Fatalf("missing tool %s", name)
		}
	}

	result, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: "nearprod_overview", Arguments: J{}})
	must(t, err)
	if result.IsError || len(obj(result.StructuredContent)) == 0 || at(obj(result.StructuredContent), "result", "catalog", "version") != Version {
		t.Fatal(result)
	}
}

func TestMCPMutationsStillUseDomainValidation(t *testing.T) {
	f := newFixture(t, true)
	client := connectMCP(t, f.A.Info)
	result, err := client.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: "nearprod_group", Arguments: J{"action": "create", "id": "mcp", "name": "MCP"}})
	must(t, err)
	if result.IsError {
		t.Fatal(result)
	}
	found := false
	for _, raw := range arr(f.S.Store.Get()["groups"]) {
		found = found || str(obj(raw)["id"]) == "mcp"
	}
	if !found {
		t.Fatal("MCP mutation did not pass through the local agent")
	}

	result, err = client.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: "nearprod_group", Arguments: J{"action": "delete", "id": "mcp"}})
	must(t, err)
	if result.IsError || at(obj(result.StructuredContent), "result", "fingerprint") == nil {
		t.Fatal("delete without confirmation must return a preview", result)
	}
	result, err = client.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: "nearprod_group", Arguments: J{"action": "invalid"}})
	must(t, err)
	if !result.IsError {
		t.Fatal("invalid MCP action was accepted")
	}
}
