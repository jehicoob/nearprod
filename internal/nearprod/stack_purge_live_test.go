package nearprod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Explicit opt-in: only a disposable, uniquely named project and catalog are used.
// The image must already be local; this test never pulls or globally prunes.
func TestStackPurgeLiveDocker(t *testing.T) {
	if os.Getenv("NEARPROD_PURGE_DOCKER") != "1" {
		t.Skip("requires an explicitly selected local Docker context")
	}
	contextName := os.Getenv("NEARPROD_PURGE_CONTEXT")
	if contextName == "" {
		t.Fatal("NEARPROD_PURGE_CONTEXT is required")
	}
	f := newFixture(t, false)
	must(t, f.S.Store.Update(func(state J) error { obj(state["runtime"])["context"] = contextName; return nil }))
	runner := &ExecRunner{}
	f.S.Docker.Runner = runner
	f.S.Compose.Runner = runner
	f.S.Runner = runner
	ctx := context.Background()
	image, e := f.S.Docker.InspectNamed(ctx, "image", "alpine:3.20")
	must(t, e)
	if image == nil {
		t.Fatal("local alpine:3.20 is required")
	}
	name := fmt.Sprintf("run-%d", time.Now().UnixNano())
	app := f.app(t, "purge-acceptance", name, J{"services": J{"api": J{"build": J{"context": "."}, "command": A{"sleep", "3600"}, "mem_limit": "32m", "volumes": A{"data:/data", "/anonymous"}}}, "volumes": J{"data": J{}}})
	must(t, os.WriteFile(filepath.Join(str(app["path"]), "Dockerfile"), []byte("FROM alpine:3.20\nRUN printf 'purge fixture' > /fixture\n"), 0600))
	// Cleanup is bounded by this temporary catalog's identity and exact project labels.
	t.Cleanup(func() {
		stack, _ := purgeStack(f.S.Store.Get(), str(app["id"]))
		if stack == nil {
			return
		}
		cs, err := f.S.Docker.Containers(ctx, true)
		if err != nil {
			t.Log("cleanup inventory:", err)
			return
		}
		for _, raw := range cs {
			c := obj(raw)
			if ownsContainer(stack, c, str(f.S.Store.Get()["owner"])) {
				_, err = f.S.Docker.Call(ctx, []string{"rm", "-f", "-v", str(c["id"])}, RunOptions{})
				if err != nil {
					t.Log("cleanup container:", err)
				}
			}
		}
		for _, kind := range []string{"volume", "network", "image"} {
			out, err := f.S.Docker.Call(ctx, []string{kind, "ls", "--quiet", "--filter", "label=" + LProject + "=" + str(app["projectName"])}, RunOptions{})
			if err != nil {
				t.Log(err)
				continue
			}
			for _, id := range strings.Fields(out.Stdout) {
				_, err = f.S.Docker.Call(ctx, []string{kind, "rm", id}, RunOptions{})
				if err != nil {
					t.Log("cleanup resource:", err)
				}
			}
		}
	})
	f.trust(t, str(app["id"]), "dev")
	assertPass(t, f.act(t, str(app["id"]), "up", J{"mode": "dev"}))
	p, req := reviewedPurge(t, f, app)
	if len(arr(p["blockers"])) > 0 {
		t.Fatal(p["blockers"])
	}
	kinds := map[string]int{}
	for _, r := range arr(p["resources"]) {
		kinds[str(at(r, "kind"))]++
	}
	if kinds["container"] != 1 || kinds["volume"] != 2 || kinds["image"] != 1 || kinds["network"] != 1 {
		t.Fatal("incomplete live inventory", kinds, p["preserved"])
	}
	assertPass(t, executePurge(t, f, req))
	for _, r := range arr(p["resources"]) {
		must(t, f.S.verifyStackResourceAbsent(ctx, obj(r)))
	}
	_, e = os.Stat(filepath.Join(str(app["path"]), "Dockerfile"))
	must(t, e)
	base, e := f.S.Docker.InspectNamed(ctx, "image", str(image["Id"]))
	must(t, e)
	if base == nil {
		t.Fatal("external base image removed")
	}
	t.Logf("Live deletion verified: %v; checkout and original base image preserved", kinds)
}
