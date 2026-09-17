package nearprod

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type RunOptions struct {
	Dir     string
	Timeout time.Duration
	Limit   int
	Exact   bool
	Stream  bool
	Input   io.Reader
	Output  io.Writer
	Env     map[string]string
	Line    func(string, string)
	Redact  *Redactor
}
type Result struct {
	Code           int
	Stdout, Stderr string
	Truncated      bool
}
type Runner interface {
	Run(context.Context, string, []string, RunOptions) (Result, error)
}
type ExecRunner struct{ ToolPath func(string) string }

func findExecutable(name string) string {
	if filepath.IsAbs(name) {
		if st, e := os.Stat(name); e == nil && st.Mode().IsRegular() && st.Mode()&0111 != 0 {
			return name
		}
		return ""
	}
	if p, e := exec.LookPath(name); e == nil {
		return p
	}
	for _, d := range []string{"/opt/homebrew/bin", "/usr/local/bin", filepath.Join(userHome(), ".local/bin"), filepath.Join(userHome(), ".cargo/bin")} {
		p := filepath.Join(d, name)
		if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() && st.Mode()&0111 != 0 {
			return p
		}
	}
	return ""
}
func (r *ExecRunner) Run(parent context.Context, name string, args []string, o RunOptions) (Result, error) {
	if r.ToolPath != nil {
		if p := r.ToolPath(name); p != "" {
			name = p
		}
	}
	bin := findExecutable(name)
	if bin == "" {
		return Result{Code: 127}, fail("TOOL_MISSING", "No se encuentra "+filepath.Base(name)+". Revisa Herramientas y PATH.", 503)
	}
	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return Result{}, fail("INVALID_ARGUMENT", "Argumento contiene NUL.", 400)
		}
	}
	ctx := parent
	var cancel context.CancelFunc
	if !o.Stream {
		timeout := o.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		ctx, cancel = context.WithTimeout(parent, timeout)
	} else {
		ctx, cancel = context.WithCancel(parent)
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = o.Dir
	cmd.Stdin = o.Input
	cmd.Env = os.Environ()
	for k, v := range o.Env {
		prefix := k + "="
		kept := cmd.Env[:0]
		for _, line := range cmd.Env {
			if !strings.HasPrefix(line, prefix) {
				kept = append(kept, line)
			}
		}
		cmd.Env = append(kept, prefix+v)
	}
	childProcessGroup(cmd)
	cmd.WaitDelay = 2 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			killGroup(cmd.Process.Pid)
		}
		return nil
	}
	limit := o.Limit
	if limit <= 0 {
		limit = 16000
	}
	out := &capture{limit: limit, exact: o.Exact, line: o.Line, stream: "stdout", redact: o.Redact, sink: o.Output}
	errout := &capture{limit: limit, exact: false, line: o.Line, stream: "stderr", redact: o.Redact}
	cmd.Stdout = out
	cmd.Stderr = errout
	e := cmd.Run()
	out.flush()
	errout.flush()
	result := Result{Stdout: out.String(), Stderr: errout.String(), Truncated: out.truncated || errout.truncated}
	if ctx.Err() != nil {
		if parent.Err() != nil {
			return result, fail("CANCELLED", "Se canceló el proceso cliente. Docker puede seguir trabajando; revisa el estado.", 409)
		}
		return result, fail("PROCESS_TIMEOUT", "Se agotó el tiempo del comando; revisa estado y logs.", 504)
	}
	if e != nil {
		var exit *exec.ExitError
		if errors.As(e, &exit) {
			result.Code = exit.ExitCode()
		} else {
			return result, fail("PROCESS_START", "No se pudo iniciar "+filepath.Base(bin)+".", 503)
		}
	}
	if o.Exact && out.truncated {
		return result, fail("OUTPUT_LIMIT", "La salida estructurada supera el límite; no se interpretará truncada.", 413)
	}
	return result, nil
}
func checked(ctx context.Context, r Runner, name string, args []string, o RunOptions) (Result, error) {
	res, e := r.Run(ctx, name, args, o)
	if e != nil {
		return res, e
	}
	if res.Code != 0 {
		msg := bounded(strings.TrimSpace(res.Stderr+"\n"+res.Stdout), 4000)
		if o.Redact != nil {
			msg = o.Redact.Text(msg)
		} else {
			msg = NewRedactor().Text(msg)
		}
		return res, detailed("COMMAND_FAILED", fmt.Sprintf("%s terminó con código %d. %s", filepath.Base(name), res.Code, msg), 422, J{"exitCode": res.Code})
	}
	return res, nil
}

type capture struct {
	mu        sync.Mutex
	b         []byte
	pending   []byte
	limit     int
	exact     bool
	truncated bool
	line      func(string, string)
	stream    string
	redact    *Redactor
	sink      io.Writer
}

func (c *capture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(p)
	if c.sink != nil {
		return c.sink.Write(p)
	}
	if c.exact {
		left := c.limit - len(c.b)
		if left < n {
			c.truncated = true
			if left > 0 {
				c.b = append(c.b, p[:left]...)
			}
		} else {
			c.b = append(c.b, p...)
		}
	} else {
		c.b = append(c.b, p...)
		if len(c.b) > c.limit {
			c.truncated = true
			c.b = append([]byte{}, c.b[len(c.b)-c.limit:]...)
		}
	}
	if c.line != nil {
		c.pending = append(c.pending, p...)
		for {
			idx := bytes.IndexByte(c.pending, '\n')
			if idx < 0 && len(c.pending) < 8192 {
				break
			}
			if idx < 0 || idx > 8192 {
				idx = 8192
			}
			s := string(c.pending[:idx])
			skip := idx
			if skip < len(c.pending) && c.pending[skip] == '\n' {
				skip++
			}
			c.pending = c.pending[skip:]
			if c.redact != nil {
				s = c.redact.Text(s)
			}
			c.line(s, c.stream)
		}
	}
	return n, nil
}
func (c *capture) String() string { c.mu.Lock(); defer c.mu.Unlock(); return string(c.b) }
func (c *capture) flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.line != nil && len(c.pending) > 0 {
		s := string(c.pending)
		if c.redact != nil {
			s = c.redact.Text(s)
		}
		c.line(s, c.stream)
	}
	c.pending = nil
}

type Redactor struct {
	mu     sync.RWMutex
	values []string
}

var secretKey = regexp.MustCompile(`(?i)(password|passwd|secret|token|api.?key|credential|database_url|redis_url)`)
var urlSecret = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^\s/:]+:)[^\s@]+@`)
var assignmentSecret = regexp.MustCompile(`(?i)((?:password|passwd|secret|token|api_key)\s*[=:]\s*)[^\s,;]+`)

func NewRedactor() *Redactor {
	r := &Redactor{}
	for _, e := range os.Environ() {
		if i := strings.IndexByte(e, '='); i > 0 && secretKey.MatchString(e[:i]) {
			r.Add(e[i+1:])
		}
	}
	return r
}
func (r *Redactor) Add(s string) {
	if len(s) < 4 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !contains(r.values, s) {
		r.values = append(r.values, s)
	}
}
func (r *Redactor) Env(v J) {
	for k, v := range v {
		if secretKey.MatchString(k) {
			r.Add(str(v))
		}
	}
}
func (r *Redactor) Dotenv(s string) {
	for _, line := range strings.Split(s, "\n") {
		parts := strings.SplitN(strings.TrimSpace(strings.TrimPrefix(line, "export ")), "=", 2)
		if len(parts) == 2 && secretKey.MatchString(parts[0]) {
			r.Add(strings.Trim(parts[1], "\"' \r"))
		}
	}
}
func (r *Redactor) Text(s string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, v := range r.values {
		s = strings.ReplaceAll(s, v, "[REDACTED]")
	}
	s = urlSecret.ReplaceAllString(s, "${1}[REDACTED]@")
	return assignmentSecret.ReplaceAllString(s, "${1}[REDACTED]")
}
