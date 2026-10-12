package nearprod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const agentRestartFile = "agent-update.json"

// The handoff is private, short-lived, and consumed only after the new server
// starts successfully. It contains browser sessions, never exposed in CLI JSON.
func readAgentRestart(home string) (J, error) {
	file := filepath.Join(home, agentRestartFile)
	st, err := regularOrMissing(file)
	if err != nil || st == nil {
		return nil, err
	}
	if st.Mode().Perm()&0077 != 0 {
		return nil, fail("UPDATE_SESSION_PERMISSIONS", "El archivo temporal de reconexión debe ser privado.", 409)
	}
	v, err := readJSON(file, 32<<10)
	if err != nil {
		return nil, err
	}
	version := str(v["version"])
	core, _, _ := strings.Cut(version, "+")
	core, _, _ = strings.Cut(core, "-")
	_, versionErr := compareInstalledVersionToStable(version, core)
	_, expiryErr := time.Parse(time.RFC3339Nano, str(v["expiresAt"]))
	if integer(v["port"]) < 1 || integer(v["port"]) > 65535 || versionErr != nil || expiryErr != nil || len(obj(v["sessions"])) > 32 {
		return nil, fail("UPDATE_SESSION_INVALID", "El archivo temporal de reconexión no es válido.", 409)
	}
	return v, nil
}

func (h *HTTP) prepareStop(request J) error {
	if !h.lifecycle.TryLock() {
		return fail("OPERATION_CONFLICT", "Hay una petición activa. Espera a que termine antes de actualizar.", 409)
	}
	defer h.lifecycle.Unlock()
	if !h.Service.mutation.TryLock() {
		return fail("OPERATION_CONFLICT", "Hay una modificación activa. Espera a que termine antes de actualizar.", 409)
	}
	defer h.Service.mutation.Unlock()
	forUpdate := truth(request["forUpdate"])
	err := h.Service.Ops.PrepareStop(func() error {
		if !forUpdate {
			return nil
		}
		if h.Service.HasWatch() {
			return fail("WATCH_ACTIVE", "Hay seguimientos Watch activos. La actualización no los interrumpió ni reemplazó el binario.", 409)
		}
		version := str(request["version"])
		core, _, _ := strings.Cut(version, "+")
		core, _, _ = strings.Cut(core, "-")
		if _, err := compareInstalledVersionToStable(version, core); err != nil {
			return err
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		sessions := J{}
		for id, expiry := range h.sessions {
			if time.Now().Before(expiry) {
				sessions[id] = expiry.UTC().Format(time.RFC3339Nano)
			}
		}
		return writeJSON(filepath.Join(h.Service.Store.Home, agentRestartFile), J{
			"version": version, "port": h.Port, "sessions": sessions,
			"expiresAt": time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339Nano),
		})
	})
	if err == nil {
		h.stopping = true
	}
	return err
}

func (h *HTTP) restoreUpdateSessions(restart J) {
	expiry, err := time.Parse(time.RFC3339Nano, str(restart["expiresAt"]))
	if err != nil || !time.Now().Before(expiry) {
		return
	}
	for id, raw := range obj(restart["sessions"]) {
		until, err := time.Parse(time.RFC3339Nano, str(raw))
		if err == nil && len(id) >= 32 && len(id) <= 128 && time.Now().Before(until) && until.Before(time.Now().Add(8*time.Hour)) {
			h.sessions[id] = until
		}
	}
}

type updateAgent struct {
	home     string
	port     int
	running  bool
	sessions bool
	release  func()
}

func (u *Updater) catalogHome() string {
	if u.CatalogHome != "" {
		return u.CatalogHome
	}
	return filepath.Join(u.Home, ".nearprod")
}

func (u *Updater) prepareAgentUpdate(ctx context.Context, version string) (*updateAgent, error) {
	home := u.catalogHome()
	info, err := readLive(ctx, home, true)
	if err != nil {
		return nil, err
	}
	state := &updateAgent{home: home, release: func() {}}
	if info != nil {
		// Older agents understand the stop route but not the update handoff.
		// Check their Watch state too, before asking them to stop.
		status, err := requestAPI(ctx, info, "/status", nil, 3*time.Second)
		if err != nil {
			return nil, err
		}
		for _, raw := range arr(status["stacks"]) {
			if contains([]string{"starting", "running"}, str(at(raw, "watch", "state"))) {
				return nil, fail("WATCH_ACTIVE", "Hay seguimientos Watch activos. La actualización no los interrumpió ni reemplazó el binario.", 409)
			}
		}
		stopped, err := requestAPI(ctx, info, "/agent/stop", J{"forUpdate": true, "version": version}, 5*time.Second)
		if err != nil {
			return nil, err
		}
		if !truth(stopped["stopped"]) {
			return nil, fail("AGENT_STOP_UNCERTAIN", "El agente no confirmó su cierre; no se reemplazó el binario.", 409)
		}
		state.running, state.sessions, state.port = true, truth(stopped["sessionTransfer"]), integer(info["port"])
		if err = waitAgentStopped(ctx, home); err != nil {
			return state, err
		}
	}
	release, err := AcquireAgentLock(home)
	if err != nil {
		return state, err
	}
	state.release = release
	restart, err := readAgentRestart(home)
	if err != nil {
		return state, err
	}
	if restart != nil {
		state.running, state.port = true, integer(restart["port"])
		state.sessions = state.sessions || len(obj(restart["sessions"])) > 0
		restart["version"] = version
	} else if state.running {
		restart = J{"version": version, "port": state.port, "sessions": J{}, "expiresAt": time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339Nano)}
	}
	if restart != nil {
		if err = writeJSON(filepath.Join(home, agentRestartFile), restart); err != nil {
			return state, err
		}
	}
	return state, nil
}

func (u *Updater) startUpdatedAgent(ctx context.Context, state *updateAgent, executable, version string) (J, error) {
	state.release()
	start := u.StartAgent
	if start == nil {
		start = startAgentBinary
	}
	return start(ctx, state.home, executable, version, state.port)
}

func (u *Updater) finishAgentUpdate(ctx context.Context, state *updateAgent, executable, version string, result J) (J, error) {
	result["agentRestarted"] = false
	if !state.running {
		result["agentStatus"] = "not-running"
		return result, nil
	}
	info, err := u.startUpdatedAgent(ctx, state, executable, version)
	if err != nil {
		result["status"], result["agentStatus"] = "activation-pending", "restart-failed"
		result["note"] = "El binario está instalado, pero no se pudo activar el agente. Consulta agent.log y vuelve a ejecutar nearprod update para reintentar la activación."
		return result, detailed("UPDATE_AGENT_RESTART", str(result["note"]), 503, J{"update": result, "cause": publicError(err)})
	}
	result["agentRestarted"], result["agentStatus"], result["agentVersion"] = true, "running", info["version"]
	if integer(info["agentUpdateProtocol"]) != 1 {
		state.sessions = false
		// A legacy target ignores the handoff. Do not leave it blocking a later
		// start of a protocol-aware version, and do not promise session continuity.
		file := filepath.Join(state.home, agentRestartFile)
		if restart, readErr := readAgentRestart(state.home); readErr == nil && restart != nil && str(restart["version"]) == version {
			if removeErr := os.Remove(file); removeErr != nil {
				return result, removeErr
			}
		}
	}
	result["url"], result["uiSessionsPreserved"] = info["url"], state.sessions
	result["note"] = "Nueva versión instalada y agente activado automáticamente. No se detuvieron contenedores ni se cambiaron sus datos."
	if !state.sessions {
		result["note"] = str(result["note"]) + " Si tu sesión de una versión anterior se cerró, nearprod ui genera un nuevo código de acceso."
	}
	return result, nil
}

func (u *Updater) recoverUpdateAgent(state *updateAgent, version, digest string, cause error) error {
	defer func() { state.release() }()
	if !state.running {
		return cause
	}
	executable := filepath.Join(u.Home, ".local", "bin", "nearprod")
	actual, err := streamHash(executable)
	if err != nil || digest == "" || actual != digest {
		return detailed("UPDATE_RECOVERY_REQUIRED", "Falló la instalación y no se pudo confirmar la copia anterior; no se inició un binario incierto.", 500, publicError(cause))
	}
	// Reacquire after a partial stop/preparation as well as an install failure.
	// Never retarget the handoff while another process owns the catalog.
	state.release()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err = waitAgentStopped(ctx, state.home); err == nil {
		state.release, err = AcquireAgentLock(state.home)
		if err != nil {
			state.release = func() {}
		}
	}
	var restart J
	if err == nil {
		restart, err = readAgentRestart(state.home)
	}
	if err == nil && restart != nil {
		restart["version"] = version
		err = writeJSON(filepath.Join(state.home, agentRestartFile), restart)
	}
	if err == nil {
		_, err = u.startUpdatedAgent(ctx, state, executable, version)
	}
	if err != nil {
		return detailed("UPDATE_AGENT_RECOVERY", "La instalación falló y el agente anterior no pudo reiniciarse. Revisa agent.log; la copia anterior se conserva.", 503, J{"installation": publicError(cause), "restart": publicError(err)})
	}
	return cause
}

// A failed activation can be retried with the same command even when the new
// executable is already installed and there is no newer release to download.
func (u *Updater) activateCurrentAgent(ctx context.Context, plan updatePlan) (J, error) {
	if plan.Provider != "manual" || !contains([]string{"up-to-date", "newer-than-release"}, str(plan.Summary["status"])) {
		return plan.Summary, nil
	}
	info, err := readLive(ctx, u.catalogHome(), true)
	if err != nil {
		return nil, err
	}
	restart, err := readAgentRestart(u.catalogHome())
	if err != nil {
		return nil, err
	}
	if info == nil && restart == nil || info != nil && str(info["version"]) == plan.Current {
		return plan.Summary, nil
	}
	executable := filepath.Join(u.Home, ".local", "bin", "nearprod")
	if _, err = verifyUpdateBinary(ctx, executable, plan.Current, u.OS+"/"+u.Arch); err != nil {
		return nil, err
	}
	state, err := u.prepareAgentUpdate(ctx, plan.Current)
	if err != nil {
		if state != nil {
			state.release()
		}
		return nil, err
	}
	defer state.release()
	result := merge(plan.Summary, J{"installed": true, "bin": executable, "note": fmt.Sprintf("Activando el agente %s ya instalado.", plan.Current)})
	return u.finishAgentUpdate(ctx, state, executable, plan.Current, result)
}
