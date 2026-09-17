package nearprod

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestPortPermissionRegression(t *testing.T) {
	for _, tc := range []struct {
		name       string
		bind, dial error
		conn       bool
		want       bool
	}{
		{"permission-refused-defer-docker", syscall.EACCES, syscall.ECONNREFUSED, false, true},
		{"eperm-refused", syscall.EPERM, syscall.ECONNREFUSED, false, true},
		{"in-use", syscall.EADDRINUSE, nil, false, false},
		{"permission-listener", syscall.EACCES, nil, true, false},
		{"permission-timeout-failclosed", syscall.EACCES, os.ErrDeadlineExceeded, false, false},
		{"unknown-bind-failclosed", syscall.EIO, syscall.ECONNREFUSED, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := checkPortWith(80, func(string, string) (net.Listener, error) { return nil, &net.OpError{Err: tc.bind} }, func(string) (net.Conn, error) {
				if tc.conn {
					a, b := net.Pipe()
					defer b.Close()
					return a, nil
				}
				return nil, tc.dial
			})
			if truth(r["available"]) != tc.want {
				t.Fatal(r)
			}
		})
	}
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	must(t, e)
	port := ln.Addr().(*net.TCPAddr).Port
	if truth(availablePort(port)["available"]) {
		t.Fatal("occupied real port accepted")
	}
	ln.Close()
	if !truth(availablePort(port)["available"]) {
		t.Fatal("free port rejected")
	}
}
func TestValidationBoundaryCases(t *testing.T) {
	for _, s := range []string{"https://x.localhost", "x.nearprod", "x.localhost:80", "x.localhost/path", "*.localhost", "-a.localhost", "a..localhost", strings.Repeat("a", 64) + ".localhost"} {
		t.Run("host-"+s, func(t *testing.T) {
			if _, e := routeHost(s); e == nil {
				t.Fatal("accepted")
			}
		})
	}
	for _, s := range []string{"app.localhost", "api-app.localhost", "legacy.nearprod.localhost"} {
		t.Run(s, func(t *testing.T) { _, e := routeHost(s); must(t, e) })
	}
	for _, n := range []any{math.NaN(), math.Inf(1), -1, 65536, 1.5, "abc", nil} {
		t.Run("number-"+str(n), func(t *testing.T) {
			if _, e := intRange(n, 1, 65535, "port"); e == nil {
				t.Fatal("accepted")
			}
		})
	}
	for _, s := range []string{`[]`, `null`, `{} {}`, `{"a":1} junk`, `{`} {
		t.Run("json-"+s, func(t *testing.T) {
			if _, e := decodeObject([]byte(s)); e == nil {
				t.Fatal("accepted")
			}
		})
	}
	for _, s := range []string{"include: x", `{"include":"remote"}`, "services:\n api:\n  extends: x", `{"\u0069nclude":"x"}`} {
		t.Run("transitive-"+s, func(t *testing.T) { expectCode(t, validateSource([]byte(s)), "COMPOSE_TRANSITIVE") })
	}
}
func TestAtomicFileAndCanonicalRootSafety(t *testing.T) {
	h := tempHome(t)
	outside := tempHome(t)
	must(t, os.WriteFile(filepath.Join(outside, "config"), []byte("keep"), 0600))
	must(t, os.Symlink(filepath.Join(outside, "config"), filepath.Join(h, "target")))
	expectCode(t, atomicBytes(filepath.Join(h, "target"), []byte("bad"), 0600), "UNSAFE_FILE")
	must(t, os.Symlink(outside, filepath.Join(h, "sub")))
	expectCode(t, atomicBytes(filepath.Join(h, "sub", "new"), []byte("bad"), 0600), "SYMLINK_PATH")
	if _, e := canonical(filepath.Join(h, "sub", "config"), []string{h}, "file"); str(publicError(e)["code"]) != "PATH_OUTSIDE_ROOT" {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(filepath.Join(outside, "config"))
	if string(b) != "keep" {
		t.Fatal("outside changed")
	}
	if within(h, filepath.Join(h, "..", "foreign")) {
		t.Fatal("outside allowed")
	}
}
func TestRunnerArgumentsAreNotShell(t *testing.T) {
	r := &ExecRunner{}
	h := tempHome(t)
	payload := "$(touch " + filepath.Join(h, "owned") + "); ' $PATH"
	v, e := r.Run(context.Background(), "/usr/bin/printf", []string{"%s", payload}, RunOptions{Exact: true})
	must(t, e)
	if v.Stdout != payload {
		t.Fatal(v)
	}
	if _, e := os.Stat(filepath.Join(h, "owned")); !os.IsNotExist(e) {
		t.Fatal("shell executed")
	}
	_, e = r.Run(context.Background(), "/bin/echo", []string{"\x00"}, RunOptions{})
	expectCode(t, e, "INVALID_ARGUMENT")
	_, e = r.Run(context.Background(), "nonexistent-nearprod-tool", nil, RunOptions{})
	expectCode(t, e, "TOOL_MISSING")
}
func TestRunnerTimeoutCancelAndBounds(t *testing.T) {
	r := &ExecRunner{}
	_, e := r.Run(context.Background(), "/bin/sleep", []string{"5"}, RunOptions{Timeout: 20 * time.Millisecond})
	expectCode(t, e, "PROCESS_TIMEOUT")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = r.Run(ctx, "/bin/sleep", []string{"5"}, RunOptions{})
	expectCode(t, e, "CANCELLED")
	v, e := r.Run(context.Background(), "/usr/bin/printf", []string{"%s", strings.Repeat("x", 100000)}, RunOptions{Limit: 100})
	must(t, e)
	if !v.Truncated || len(v.Stdout) != 100 {
		t.Fatal("unbounded output")
	}
	_, e = r.Run(context.Background(), "/usr/bin/printf", []string{"%s", strings.Repeat("x", 10000)}, RunOptions{Limit: 100, Exact: true})
	expectCode(t, e, "OUTPUT_LIMIT")
	var dest bytes.Buffer
	raw := []byte{0, 1, 2, 255, 0, 128}
	_, e = r.Run(context.Background(), "/bin/cat", nil, RunOptions{Input: bytes.NewReader(raw), Output: &dest})
	must(t, e)
	if !bytes.Equal(raw, dest.Bytes()) {
		t.Fatal("binary corrupted")
	}
}
func TestRedactionAndStreamingLimits(t *testing.T) {
	red := NewRedactor()
	red.Add("super-secret-value")
	red.Dotenv("DATABASE_URL=postgres://alice:password@db/x\nAPI_TOKEN='abcd12345'")
	for _, v := range []string{"super-secret-value", "postgres://alice:password@db/x", "abcd12345", "PASSWORD=private123", "https://alice:othersecret@server/x"} {
		t.Run(v, func(t *testing.T) {
			out := red.Text(v)
			if out == v {
				t.Fatal("not redacted", out)
			}
		})
	}
	count, maxLen := 0, 0
	c := capture{limit: 99, line: func(s, _ string) { count++; maxLen = max(maxLen, len(s)) }, redact: red}
	_, e := c.Write([]byte(strings.Repeat("a", 100000)))
	must(t, e)
	c.flush()
	if count < 10 || maxLen > 8192 || len(c.String()) > 99 {
		t.Fatal(count, maxLen, len(c.String()))
	}
}
func TestOperationsConcurrencyCancellationAndBounds(t *testing.T) {
	f := newFixture(t, false)
	o := f.S.Ops
	block := make(chan struct{})
	job := func(ctx context.Context, line func(string, string)) (any, error) {
		line("start", "stdout")
		select {
		case <-block:
			return J{"ok": true}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	first, e := o.Submit("up", []string{"a"}, false, true, job)
	must(t, e)
	_, e = o.Submit("up", []string{"a"}, false, false, job)
	expectCode(t, e, "OPERATION_CONFLICT")
	_, e = o.Submit("up", []string{"b"}, false, true, job)
	expectCode(t, e, "OPERATION_CONFLICT")
	_, e = o.Submit("runtime", nil, true, false, job)
	expectCode(t, e, "OPERATION_CONFLICT")
	second, e := o.Submit("logs", []string{"b"}, false, false, job)
	must(t, e)
	_, e = o.Cancel(str(first["id"]))
	must(t, e)
	v := waitOp(t, f.S, first)
	if str(v["state"]) != "cancelled" {
		t.Fatal(v)
	}
	close(block)
	assertPass(t, waitOp(t, f.S, second))
	last, e := o.Submit("bounded", []string{"a"}, false, false, func(_ context.Context, line func(string, string)) (any, error) {
		for n := 0; n < 300; n++ {
			line(strings.Repeat("x", 4000), "stdout")
		}
		return nil, nil
	})
	must(t, e)
	v = waitOp(t, f.S, last)
	if len(arr(v["lines"])) != 160 || len(str(at(arr(v["lines"])[0], "text"))) > 2000 {
		t.Fatal("unbounded")
	}
	o.Stop()
	_, e = o.Submit("after-close", nil, false, false, job)
	expectCode(t, e, "AGENT_STOPPING")
}
func TestBusDropsSlowConsumer(t *testing.T) {
	b := NewBus()
	ch, close := b.Subscribe()
	defer close()
	for n := 0; n < 100; n++ {
		b.Send(J{"n": n})
	}
	count := 0
	for range ch {
		count++
	}
	if count != 64 {
		t.Fatal(count)
	}
}
func TestDockerStatesAndSafePortLinks(t *testing.T) {
	for _, tc := range []struct {
		name     string
		running  bool
		health   string
		exit     int
		oom      bool
		expected string
	}{{"healthy", true, "healthy", 0, false, "running"}, {"running-no-health", true, "", 0, false, "running"}, {"stopped", false, "", 0, false, "stopped"}, {"crashed", false, "", 1, false, "failed"}, {"oom", false, "", 137, true, "failed"}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			v := f.Add("p", "api", "/test", nil, tc.running, tc.health)
			obj(v["State"])["ExitCode"] = tc.exit
			obj(v["State"])["OOMKilled"] = tc.oom
			c := summarizeContainer(v)
			s := stackState(A{c}, []string{"api"}, true)
			if str(s["execution"]) != tc.expected {
				t.Fatal(s)
			}
		})
	}
	for _, p := range []string{"5432/tcp", "3306/tcp", "6379/tcp", "8000/udp"} {
		if publishedURL(p, J{"HostIp": "127.0.0.1", "HostPort": "15000"}) != nil {
			t.Fatal("SQL/UDP as HTTP", p)
		}
	}
	if publishedURL("80/tcp", J{"HostIp": "0.0.0.0", "HostPort": "8080"}) != "http://127.0.0.1:8080" {
		t.Fatal("web link")
	}
}
func TestJSONComposeEscapesLiteralDollar(t *testing.T) {
	h := tempHome(t)
	v := J{"services": J{"api": J{"environment": J{"PASSWORD": "abc${NOT_INTERPOLATED}"}, "command": A{"sh", "-c", "echo $HOME"}}}}
	file := filepath.Join(h, "compose.json")
	must(t, writeCompose(file, v))
	b, _ := os.ReadFile(file)
	if !strings.Contains(string(b), "$${NOT_INTERPOLATED}") || !strings.Contains(string(b), "$$HOME") {
		t.Fatal(string(b))
	}
	var model J
	must(t, json.Unmarshal(b, &model))
	if hash(unescape(model)) != hash(v) {
		t.Fatal("roundtrip")
	}
}
func TestConcurrentStoreOperationsStopRace(t *testing.T) {
	f := newFixture(t, false)
	var wg sync.WaitGroup
	for n := 0; n < 20; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, e := f.S.Ops.Submit("r", []string{str(n)}, false, false, func(ctx context.Context, _ func(string, string)) (any, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Millisecond):
					return nil, nil
				}
			})
			if e != nil && !errors.Is(e, context.Canceled) && str(publicError(e)["code"]) != "AGENT_STOPPING" {
				t.Error(e)
			}
		}(n)
	}
	f.S.Ops.Stop()
	wg.Wait()
}

func TestHistoryHasSerializedBudgetPreservingActiveOperations(t *testing.T) {
	ops := A{J{"id": "running", "state": "running", "lines": A{}}}
	for n := 0; n < 25; n++ {
		ops = append(ops, J{"id": str(n), "state": "succeeded", "lines": A{J{"text": strings.Repeat("<", 50000)}}})
	}
	out := boundHistory(ops)
	encoded, _ := json.Marshal(out)
	if len(encoded) > 4<<20 || str(at(out[0], "id")) != "running" || str(at(out[len(out)-1], "id")) != "24" {
		t.Fatal("unbounded history or active/latest record lost")
	}
	only := boundHistory(A{J{"id": "large", "state": "succeeded", "results": strings.Repeat("x", 5<<20)}})
	if !truth(at(only[0], "historyTruncated")) {
		t.Fatal("single oversized result not bounded")
	}
}
