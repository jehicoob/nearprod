package nearprod

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func callHTTP(t *testing.T, f *fixture, method, path string, body any, headers map[string]string) (int, http.Header, []byte) {
	t.Helper()
	var input io.Reader
	if body != nil {
		input = bytes.NewReader(mustJSON(body))
	}
	req, e := http.NewRequest(method, str(f.A.Info["url"])+path, input)
	must(t, e)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		if k == "Host" {
			req.Host = v
		} else {
			req.Header.Set(k, v)
		}
	}
	res, e := localClient(5 * time.Second).Do(req)
	must(t, e)
	defer res.Body.Close()
	data, e := io.ReadAll(res.Body)
	must(t, e)
	return res.StatusCode, res.Header, data
}
func TestHTTPAuthenticationAndIsolation(t *testing.T) {
	f := newFixture(t, true)
	auth := map[string]string{"Authorization": "Bearer " + str(f.A.Info["token"])}
	for _, tc := range []struct {
		path    string
		headers map[string]string
		code    int
	}{{"/api/catalog", nil, 401}, {"/api/catalog", auth, 200}, {"/api/health", nil, 200}, {"/", nil, 200}, {"/../../config/catalog.json", auth, 404}, {"/api/catalog?token=wrong", auth, 400}, {"/api/catalog", map[string]string{"Host": "evil.example"}, 403}, {"/api/catalog", map[string]string{"Origin": "http://evil.localhost", "Authorization": auth["Authorization"]}, 403}, {"/api/catalog", map[string]string{"Sec-Fetch-Site": "cross-site", "Authorization": auth["Authorization"]}, 403}} {
		t.Run(tc.path+string(mustJSON(tc.headers)), func(t *testing.T) {
			status, h, data := callHTTP(t, f, "GET", tc.path, nil, tc.headers)
			if status != tc.code {
				t.Fatalf("%d %s", status, data)
			}
			if h.Get("Content-Security-Policy") == "" || h.Get("X-Frame-Options") != "DENY" {
				t.Fatal("security headers missing")
			}
		})
	}
	status, _, data := callHTTP(t, f, "POST", "/api/pair", J{}, auth)
	if status != 200 {
		t.Fatal(string(data))
	}
	var p J
	must(t, json.Unmarshal(data, &p))
	status, h, data := callHTTP(t, f, "POST", "/api/session", J{"code": p["code"]}, nil)
	if status != 200 {
		t.Fatal(string(data))
	}
	cookie := h.Get("Set-Cookie")
	if !strings.Contains(cookie, "HttpOnly") || !strings.Contains(cookie, "SameSite=Strict") {
		t.Fatal(cookie)
	}
	cookie = strings.Split(cookie, ";")[0]
	status, _, _ = callHTTP(t, f, "POST", "/api/session", J{"code": p["code"]}, nil)
	if status != 401 {
		t.Fatal("pair reused")
	}
	for _, path := range []string{"/api/pair", "/api/agent/stop"} {
		status, _, _ = callHTTP(t, f, "POST", path, J{}, map[string]string{"Cookie": cookie})
		if status != 403 {
			t.Fatal("cookie controlled CLI route")
		}
	}
	status, _, _ = callHTTP(t, f, "GET", "/api/catalog", nil, map[string]string{"Cookie": cookie})
	if status != 200 {
		t.Fatal("valid session")
	}
	status, _, _ = callHTTP(t, f, "POST", "/api/logout", J{}, map[string]string{"Cookie": cookie})
	if status != 200 {
		t.Fatal(status)
	}
	status, _, _ = callHTTP(t, f, "GET", "/api/catalog", nil, map[string]string{"Cookie": cookie})
	if status != 401 {
		t.Fatal("session not invalidated")
	}
}
func TestHTTPPayloadAndRateLimits(t *testing.T) {
	f := newFixture(t, true)
	auth := map[string]string{"Authorization": "Bearer " + str(f.A.Info["token"])}
	status, _, _ := callHTTP(t, f, "POST", "/api/groups", J{"id": strings.Repeat("x", 2<<20)}, auth)
	if status != 413 {
		t.Fatal(status)
	}
	status, _, _ = callHTTP(t, f, "POST", "/api/groups", J{}, map[string]string{"Authorization": auth["Authorization"], "Content-Type": "text/plain"})
	if status != 415 {
		t.Fatal(status)
	}
	for n := 0; n < 11; n++ {
		status, _, _ = callHTTP(t, f, "POST", "/api/session", J{"code": "bad"}, nil)
	}
	if status != 429 {
		t.Fatal("no rate limit")
	}
}
func TestHTTPNativeAssetsAndStorage(t *testing.T) {
	f := newFixture(t, true)
	auth := map[string]string{"Authorization": "Bearer " + str(f.A.Info["token"])}
	for _, path := range []string{"/", "/ui/App.js", "/ui/infrastructure.js", "/ui/tools.js", "/vendor/react.js"} {
		status, _, b := callHTTP(t, f, "GET", path, nil, nil)
		if status != 200 || len(b) < 20 {
			t.Fatalf("asset %s %d", path, status)
		}
	}
	status, _, b := callHTTP(t, f, "GET", "/api/config/paths", nil, auth)
	if status != 200 || !strings.Contains(string(b), "config/catalog.json") {
		t.Fatal(string(b))
	}
	status, _, b = callHTTP(t, f, "POST", "/api/config/backup", J{"confirm": true}, auth)
	if status != 200 {
		t.Fatal(string(b))
	}
}
func TestHTTPAPIRegistrationOperationsAndLogStreaming(t *testing.T) {
	f := newFixture(t, true)
	a := f.app(t, "web", "api", nil)
	id := str(a["id"])
	info := f.A.Info
	p, e := requestAPI(context.Background(), info, "/preview", J{"target": id}, time.Second)
	must(t, e)
	_, e = requestAPI(context.Background(), info, "/trust", J{"target": id, "mode": "dev", "fingerprint": p["fingerprint"], "allowUnsafe": true}, time.Second)
	must(t, e)
	op, e := requestAPI(context.Background(), info, "/actions", J{"target": id, "action": "up"}, time.Second)
	must(t, e)
	assertPass(t, waitOp(t, f.S, op))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", str(info["url"])+"/api/logs?target="+id+"&follow=true", nil)
	must(t, e)
	req.Header.Set("Authorization", "Bearer "+str(info["token"]))
	res, e := localClient(3 * time.Second).Do(req)
	must(t, e)
	defer res.Body.Close()
	buf := make([]byte, 8192)
	n, e := res.Body.Read(buf)
	must(t, e)
	cancel()
	if res.StatusCode != 200 || !strings.Contains(string(buf[:n]), "data:") {
		t.Fatalf("SSE %d %s", res.StatusCode, buf[:n])
	}
}

func TestStopWaitsForSharedSocketBeforeUpgrade(t *testing.T) {
	h := tempHome(t)
	release, e := AcquireAgentLock(h)
	must(t, e)
	done := make(chan error, 1)
	go func() { done <- waitAgentStopped(context.Background(), h) }()
	select {
	case <-done:
		t.Fatal("returned before old agent lock closed")
	case <-time.After(20 * time.Millisecond):
	}
	release()
	select {
	case e := <-done:
		must(t, e)
	case <-time.After(time.Second):
		t.Fatal("did not observe shutdown")
	}
}
