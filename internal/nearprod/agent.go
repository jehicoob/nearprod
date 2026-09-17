package nearprod

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// A Unix socket is shared with 0.6.x: neither a stale PID nor new file layout can create two writers.
func AcquireAgentLock(home string) (func(), error) {
	if e := privateDir(home); e != nil {
		return nil, e
	}
	guardPath := filepath.Join(home, "agent.guard")
	if _, err := regularOrMissing(guardPath); err != nil {
		return nil, err
	}
	guard, err := os.OpenFile(guardPath, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(guard.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		guard.Close()
		return nil, fail("AGENT_RUNNING", "Otro agente está iniciando o utilizando el catálogo.", 409)
	}
	keep := false
	defer func() {
		if !keep {
			guard.Close()
		}
	}()
	path := filepath.Join(home, "agent.sock")
	if len([]byte(path)) > 100 {
		return nil, fail("HOME_TOO_LONG", "NEARPROD_HOME demasiado largo para socket Unix (máximo 100 bytes).", 400)
	}
	listen := func() (*net.UnixListener, error) {
		return net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	}
	ln, e := listen()
	if e != nil {
		if !errors.Is(e, syscall.EADDRINUSE) {
			return nil, e
		}
		conn, ce := net.DialTimeout("unix", path, 600*time.Millisecond)
		if ce == nil {
			conn.Close()
			return nil, fail("AGENT_RUNNING", "Ya hay un agente NearProd. Detén la versión anterior antes de migrar.", 409)
		}
		if !errors.Is(ce, syscall.ECONNREFUSED) && !os.IsNotExist(ce) {
			return nil, fail("AGENT_LOCK_UNCERTAIN", "No se pudo verificar el socket; no se eliminará.", 409)
		}
		st, se := os.Lstat(path)
		if se == nil {
			if st.Mode()&os.ModeSocket == 0 || !ownedByUser(st) {
				return nil, fail("UNSAFE_LOCK", "agent.sock no es un socket propio; no se elimina.", 409)
			}
			if e = os.Remove(path); e != nil {
				return nil, e
			}
		} else if !os.IsNotExist(se) {
			return nil, se
		}
		ln, e = listen()
		if e != nil {
			return nil, fail("AGENT_RUNNING", "Otro proceso está iniciando el agente.", 409)
		}
	}
	ln.SetUnlinkOnClose(true)
	_ = os.Chmod(path, 0600)
	go func() {
		for {
			c, e := ln.Accept()
			if e != nil {
				return
			}
			c.Close()
		}
	}()
	var once sync.Once
	keep = true
	return func() { once.Do(func() { ln.Close(); guard.Close() }) }, nil
}

type Agent struct {
	Service *Service
	HTTP    *HTTP
	Info    J
	done    chan struct{}
	once    sync.Once
	release func()
}

func StartAgent(ctx context.Context, home string, assets fs.FS, runner Runner, monitor bool, port int) (*Agent, error) {
	release, e := AcquireAgentLock(home)
	if e != nil {
		return nil, e
	}
	store, e := OpenStore(home)
	if e != nil {
		release()
		return nil, e
	}
	service := NewService(ctx, store, runner)
	httpServer := NewHTTP(service, assets)
	a := &Agent{Service: service, HTTP: httpServer, done: make(chan struct{}), release: release}
	httpServer.OnShutdown = a.Close
	ln, e := httpServer.Listen(port)
	if e != nil {
		service.Close()
		release()
		return nil, e
	}
	a.Info = J{"url": httpServer.URL, "port": httpServer.Port, "pid": os.Getpid(), "token": httpServer.Bearer, "version": Version, "runtimeLanguage": "Go", "startedAt": now()}
	if e = writeJSON(filepath.Join(home, "agent.json"), a.Info); e != nil {
		ln.Close()
		service.Close()
		release()
		return nil, e
	}
	go func() { _ = httpServer.Server.Serve(ln); a.Close() }()
	if monitor {
		service.Monitor()
	}
	return a, nil
}
func (a *Agent) Done() <-chan struct{} { return a.done }
func (a *Agent) Close() {
	a.once.Do(func() { // Cancel work and streams before closing network and releasing the catalog lock.
		a.Service.cancel()
		_ = a.HTTP.Server.Close()
		a.Service.Close()
		file := filepath.Join(a.Service.Store.Home, "agent.json")
		if v, e := readJSON(file, 16384); e == nil && equalSecret(str(v["token"]), a.HTTP.Bearer) {
			_ = os.Remove(file)
		}
		a.release()
		close(a.done)
	})
}

var _ = http.ErrServerClosed
