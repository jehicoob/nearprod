package nearprod

import (
	"context"
	"encoding/json"
	"sync"
)

type Bus struct {
	mu      sync.Mutex
	clients map[chan J]struct{}
}

func NewBus() *Bus { return &Bus{clients: map[chan J]struct{}{}} }
func (b *Bus) Subscribe() (chan J, func()) {
	ch := make(chan J, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		if _, ok := b.clients[ch]; ok {
			delete(b.clients, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
}
func (b *Bus) Send(v J) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for c := range b.clients {
		select {
		case c <- copyJ(v):
		default:
			delete(b.clients, c)
			close(c)
		}
	}
}

type activeOp struct {
	ID            string
	Targets       []string
	Global, Build bool
	Cancel        context.CancelFunc
	Value         J
}
type Operations struct {
	mu     sync.Mutex
	active map[string]*activeOp
	recent map[string]J
	store  *Store
	bus    *Bus
	ctx    context.Context
	wg     sync.WaitGroup
	closed bool
}

func NewOperations(ctx context.Context, st *Store, b *Bus) *Operations {
	return &Operations{active: map[string]*activeOp{}, recent: map[string]J{}, store: st, bus: b, ctx: ctx}
}
func (o *Operations) Busy(targets []string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.busyLocked(targets, false, false)
}
func (o *Operations) busyLocked(targets []string, global, build bool) bool {
	for _, a := range o.active {
		if global || a.Global || build && a.Build || len(targets) == 0 {
			return true
		}
		for _, t := range targets {
			if contains(a.Targets, t) {
				return true
			}
		}
	}
	return false
}
func (o *Operations) Submit(action string, targets []string, global, build bool, work func(context.Context, func(string, string)) (any, error)) (J, error) {
	o.mu.Lock()
	if o.closed || o.ctx.Err() != nil {
		o.mu.Unlock()
		return nil, fail("AGENT_STOPPING", "El agente se está cerrando.", 409)
	}
	if o.busyLocked(targets, global, build) {
		o.mu.Unlock()
		return nil, fail("OPERATION_CONFLICT", "Ya hay una operación incompatible activa; espera a que termine.", 409)
	}
	id := token(12)
	ctx, cancel := context.WithCancel(o.ctx)
	v := J{"id": id, "action": action, "targets": cloneStrings(targets), "state": "running", "startedAt": now(), "lines": A{}}
	op := &activeOp{ID: id, Targets: targets, Global: global, Build: build, Cancel: cancel, Value: v}
	o.active[id] = op
	o.wg.Add(1)
	o.mu.Unlock()
	if e := o.store.Update(func(d J) error {
		ops := arr(d["operations"])
		ops = append(ops, copyJ(v))
		if len(ops) > 100 {
			ops = ops[len(ops)-100:]
		}
		d["operations"] = boundHistory(ops)
		return nil
	}); e != nil {
		o.mu.Lock()
		delete(o.active, id)
		o.mu.Unlock()
		cancel()
		o.wg.Done()
		return nil, e
	}
	initial := copyJ(v)
	o.bus.Send(J{"type": "operation", "operation": initial})
	go func() {
		defer o.wg.Done()
		defer cancel()
		redactor := NewRedactor()
		line := func(text, stream string) {
			o.mu.Lock()
			text = bounded(redactor.Text(text), 2000)
			lines := append(arr(op.Value["lines"]), J{"time": now(), "text": text, "stream": stream})
			if len(lines) > 160 {
				lines = lines[len(lines)-160:]
			}
			op.Value["lines"] = lines
			snapshot := copyJ(op.Value)
			o.mu.Unlock()
			o.bus.Send(J{"type": "operation", "operation": snapshot})
		}
		result, e := work(ctx, line)
		o.mu.Lock()
		finished := copyJ(op.Value)
		finished["endedAt"] = now()
		if e != nil {
			finished["state"] = "failed"
			finished["error"] = publicError(e)
			if ctx.Err() != nil {
				finished["state"] = "cancelled"
			}
			if d := obj(at(publicError(e), "details")); d["results"] != nil {
				finished["results"] = d["results"]
			}
		} else {
			finished["state"] = "succeeded"
			finished["results"] = result
		}
		o.mu.Unlock()
		persistErr := o.store.Update(func(d J) error {
			for i, v := range arr(d["operations"]) {
				if str(obj(v)["id"]) == id {
					arr(d["operations"])[i] = finished
					d["operations"] = boundHistory(arr(d["operations"]))
					return nil
				}
			}
			return nil
		})
		if persistErr != nil {
			finished["state"] = "failed"
			finished["error"] = J{"code": "OPERATION_PERSIST", "message": "Docker terminó pero no se guardó el resultado. Revisa el estado y el disco."}
		}
		o.mu.Lock()
		delete(o.active, id)
		o.recent[id] = copyJ(finished)
		if len(o.recent) > 100 {
			for key := range o.recent {
				if key != id {
					delete(o.recent, key)
					break
				}
			}
		}
		o.mu.Unlock()
		o.bus.Send(J{"type": "operation", "operation": finished})
		o.bus.Send(J{"type": "catalog"})
	}()
	return initial, nil
}
func (o *Operations) Get(id string) (J, error) {
	o.mu.Lock()
	if done, ok := o.recent[id]; ok {
		v := copyJ(done)
		o.mu.Unlock()
		return v, nil
	}
	if op, ok := o.active[id]; ok {
		v := copyJ(op.Value)
		o.mu.Unlock()
		return v, nil
	}
	o.mu.Unlock()
	for _, v := range arr(o.store.Get()["operations"]) {
		if str(obj(v)["id"]) == id {
			return obj(v), nil
		}
	}
	return nil, fail("OPERATION_NOT_FOUND", "Operación no encontrada.", 404)
}
func (o *Operations) Cancel(id string) (J, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	op, ok := o.active[id]
	if !ok {
		return nil, fail("OPERATION_NOT_ACTIVE", "La operación ya no está activa.", 409)
	}
	op.Cancel()
	return J{"id": id, "requested": true, "note": "Se cancela el cliente; un build de Docker puede continuar. Revisa el estado."}, nil
}
func (o *Operations) Stop() {
	o.mu.Lock()
	o.closed = true
	for _, op := range o.active {
		op.Cancel()
	}
	o.mu.Unlock()
	o.wg.Wait()
}

// Bound serialized history, not only line counts: escaping/long build output must
// never grow the metadata beyond the reader limit. Keep active operations.
func boundHistory(ops A) A {
	const budget = 4 << 20
	out := append(A{}, ops...)
	for {
		b, err := json.Marshal(out)
		if err != nil || len(b) <= budget {
			return out
		}
		drop := -1
		for n, raw := range out {
			state := str(obj(raw)["state"])
			if state != "running" && state != "queued" {
				drop = n
				break
			}
		}
		if drop < 0 {
			return out
		} // active records do not persist their streaming buffers
		if len(out) == 1 {
			v := copyJ(obj(out[0]))
			v["lines"] = A{}
			v["results"] = J{"truncated": true, "note": "Resultado demasiado grande para el historial; revisa el estado actual."}
			if e := obj(v["error"]); len(e) > 0 {
				v["error"] = J{"code": e["code"], "message": bounded(str(e["message"]), 2000)}
			}
			v["historyTruncated"] = true
			return A{v}
		}
		out = append(out[:drop], out[drop+1:]...)
	}
}
