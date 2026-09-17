package nearprod

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

func (s *Service) WatchStatus(id string) J {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	if w := s.watches[id]; w != nil {
		return copyJ(w.Value)
	}
	return J{"state": "off", "lines": A{}}
}
func (s *Service) HasWatch() bool {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	for _, w := range s.watches {
		if contains([]string{"starting", "running"}, str(w.Value["state"])) {
			return true
		}
	}
	return false
}
func (s *Service) StopWatch(id string) J {
	s.watchMu.Lock()
	w := s.watches[id]
	if w != nil {
		w.Cancel()
	}
	s.watchMu.Unlock()
	if w != nil {
		<-w.Done
		s.watchMu.Lock()
		if s.watches[id] == w {
			delete(s.watches, id)
		}
		s.watchMu.Unlock()
	}
	s.Bus.Send(J{"type": "watch", "id": id, "watch": J{"state": "off", "lines": A{}}})
	return J{"id": id, "state": "off"}
}
func (s *Service) StopWatches() {
	s.watchMu.Lock()
	ids := []string{}
	for id := range s.watches {
		ids = append(ids, id)
	}
	s.watchMu.Unlock()
	for _, id := range ids {
		s.StopWatch(id)
	}
}
func (s *Service) StartWatch(id string) (J, error) {
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if e := s.Idle(id); e != nil {
		return nil, e
	}
	s.watchMu.Lock()
	exists := s.watches[id] != nil
	s.watchMu.Unlock()
	if exists {
		return nil, fail("WATCH_ACTIVE", "Detén el seguimiento anterior antes de crear otro.", 409)
	}
	return s.Ops.Submit("watch", []string{id}, false, false, func(ctx context.Context, line func(string, string)) (any, error) {
		st, e := s.Store.Stack(id)
		if e != nil {
			return nil, e
		}
		if str(st["activeMode"]) != "dev" {
			return nil, fail("WATCH_MODE", "Inicia primero la aplicación en modo Desarrollo.", 409)
		}
		if _, e = s.Docker.Owned(ctx, st); e != nil {
			return nil, e
		}
		res, e := s.Compose.Approved(ctx, st, "dev")
		if e != nil {
			return nil, e
		}
		if !s.Compose.Supports(ctx, "watch", "--no-up") || !s.Compose.Supports(ctx, "watch", "--prune") {
			return nil, fail("WATCH_CAPABILITY", "Compose Watch debe soportar --no-up y --prune=false; actualiza Compose.", 409)
		}
		watch := false
		for _, v := range obj(res.Model["services"]) {
			watch = watch || len(arr(at(v, "develop", "watch"))) > 0
		}
		if !watch {
			return nil, fail("WATCH_NOT_CONFIGURED", "El Compose no declara develop.watch. Los bind mounts/recarga del proyecto no necesitan este botón.", 409)
		}
		overlay, e := s.Compose.Overlay(st, res)
		if e != nil {
			return nil, e
		}
		wctx, cancel := context.WithCancel(s.ctx)
		entry := &watchEntry{Value: J{"state": "starting", "lines": A{}}, Cancel: cancel, Done: make(chan struct{})}
		s.watchMu.Lock()
		s.watches[id] = entry
		s.watchMu.Unlock()
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer close(entry.Done)
			defer cancel()
			args := composeArgs(st, "dev", str(at(s.Store.Get(), "runtime", "context")), "watch", []string{"--no-up", "--prune=false"}, overlay)
			_, e := checked(wctx, s.Runner, "docker", args, RunOptions{Dir: str(st["path"]), Stream: true, Redact: res.Redact, Line: func(v, stream string) {
				s.watchMu.Lock()
				entry.Value["state"] = "running"
				lines := append(arr(entry.Value["lines"]), bounded(v, 2000))
				if len(lines) > 40 {
					lines = lines[len(lines)-40:]
				}
				entry.Value["lines"] = lines
				value := copyJ(entry.Value)
				s.watchMu.Unlock()
				s.Bus.Send(J{"type": "watch", "id": id, "watch": value})
			}})
			s.watchMu.Lock()
			entry.Value["state"] = "ended"
			if e != nil {
				entry.Value["state"] = "failed"
				entry.Value["error"] = publicError(e)
			}
			if wctx.Err() != nil {
				entry.Value["state"] = "off"
				delete(entry.Value, "error")
			}
			v := copyJ(entry.Value)
			s.watchMu.Unlock()
			s.Bus.Send(J{"type": "watch", "id": id, "watch": v})
		}()
		return J{"id": id, "state": "starting"}, nil
	})
}
func logArgs(req J) ([]string, error) {
	tail := 100
	if req["tail"] != nil {
		n, e := intRange(req["tail"], 0, 500, "Líneas")
		if e != nil {
			return nil, e
		}
		tail = n
	}
	args := []string{"logs", "--timestamps", "--tail", strconv.Itoa(tail)}
	if since := str(req["since"]); since != "" {
		if len(since) > 50 {
			return nil, fail("INVALID_SINCE", "Cursor temporal inválido.", 400)
		}
		if _, e := time.Parse(time.RFC3339Nano, since); e != nil {
			return nil, fail("INVALID_SINCE", "Usa timestamp RFC3339.", 400)
		}
		args = append(args, "--since", since)
	}
	if truth(req["follow"]) {
		args = append(args, "--follow")
	}
	return args, nil
}
func (s *Service) streamContainers(ctx context.Context, cs A, req J, redact *Redactor, onLine func(J)) error {
	if len(cs) == 0 {
		return fail("NO_CONTAINERS", "No hay contenedores propios para esos logs.", 404)
	}
	if len(cs) > 8 {
		return fail("LOG_LIMIT", "Selecciona un servicio: máximo 8 contenedores por vista.", 400)
	}
	args, e := logArgs(req)
	if e != nil {
		return e
	}
	lctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failure error
	for _, raw := range cs {
		c := obj(raw)
		id := str(c["id"])
		if !cidRE.MatchString(id) {
			return fail("CONTAINER_ID", "ID de contenedor no válido.", 422)
		}
		wg.Add(1)
		go func(c J) {
			defer wg.Done()
			_, e := s.Docker.Call(lctx, append(append([]string{}, args...), str(c["id"])), RunOptions{Stream: truth(req["follow"]), Timeout: 30 * time.Second, Redact: redact, Line: func(v, stream string) {
				stamp, msg := now(), v
				if first, rest, ok := strings.Cut(v, " "); ok {
					if _, e := time.Parse(time.RFC3339Nano, first); e == nil {
						stamp = first
						msg = rest
					}
				}
				onLine(J{"time": stamp, "container": c["id"], "service": text(c["service"], "database"), "stream": stream, "text": msg})
			}})
			if e != nil && ctx.Err() == nil {
				mu.Lock()
				if failure == nil {
					failure = e
					cancel()
				}
				mu.Unlock()
			}
		}(c)
	}
	wg.Wait()
	return failure
}
func (s *Service) Logs(ctx context.Context, id string, req J, onLine func(J)) error {
	st, e := s.Store.Stack(id)
	if e != nil {
		return e
	}
	cs, e := s.Docker.Owned(ctx, st)
	if e != nil {
		return e
	}
	selected := A{}
	for _, v := range cs {
		c := obj(v)
		if svc := str(req["service"]); svc != "" && svc != str(c["service"]) {
			continue
		}
		if id := str(req["container"]); id != "" && id != str(c["id"]) {
			continue
		}
		selected = append(selected, c)
	}
	rd := NewRedactor()
	for _, mode := range obj(st["modes"]) {
		paths := append(ss(obj(mode)["envFiles"]), filepath.Join(str(st["path"]), ".env"))
		for _, p := range paths {
			if b, e := readLimited(p, 2<<20); e == nil {
				rd.Dotenv(string(b))
			}
		}
	}
	for _, v := range arr(s.Infra.State()["bindings"]) {
		b := obj(v)
		if str(b["stackUid"]) == str(st["uid"]) {
			if conn, e := s.Infra.Connection(str(b["databaseId"]), true); e == nil {
				rd.Add(str(conn["password"]))
				rd.Add(str(at(conn, "internal", "url")))
			}
		}
	}
	return s.streamContainers(ctx, selected, req, rd, onLine)
}
func (s *Service) InfraLogs(ctx context.Context, id string, req J, onLine func(J)) error {
	r, e := s.Infra.Instance(id)
	if e != nil {
		return e
	}
	cs, e := s.Infra.Containers(ctx, r)
	if e != nil {
		return e
	}
	return s.streamContainers(ctx, cs, req, s.Infra.redactor(r), onLine)
}

var _ = strings.TrimSpace
