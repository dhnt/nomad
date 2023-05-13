package shell

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dhnt/nomad/api"
	"github.com/dhnt/nomad/api/cli"
)

type Shell struct {
	mu sync.Mutex

	c *cli.Client

	cwd string
	env []string
}

func New(baseUrl string) (*Shell, error) {
	c, err := cli.NewClient(baseUrl)
	if err != nil {
		return nil, err
	}
	return &Shell{
		c:   c,
		cwd: "/",
	}, nil
}

var builtins = []string{
	"help",
	// environment
	"env",
	"export",
	// process
	"bg",
	"exec",
	"ps",
	"kill",
	// fs
	"pwd",
	"cd",
	"chdir",
	"dirs",
	"ls",
	"mkdir",
	"rmdir",
	"touch",
	"chown",
	"chmod",
	"cp",
	"mv",
	"rm",
	"truncate",
	"echo",
	"cat",
}

func (sh *Shell) Env() []string {
	return sh.env
}

func (sh *Shell) Export(env []string) {
	sh.env = env
}

// Wait polls the status of the process of id at interval until timeout.
// Any errors will be ignored before the timeout is reached.
// Result of the last poll will be returned.
func (sh *Shell) Wait(id string, states []api.RunState, timeout int64, interval int64) (*api.Proc, error) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var r api.Proc
			if err := sh.c.Ps1(id, &r); err != nil {
				if _, ok := err.(api.ErrorNotFound); ok {
					return nil, err
				}
				// continue for other types of errors
			}
			for _, s := range states {
				if r.State == s {
					return &r, nil
				}
			}
		case <-time.After(time.Duration(timeout) * time.Second):
			return nil, fmt.Errorf("timed out after %v seconds", timeout)
		}
	}
}

func (sh *Shell) Exec(req api.RunReq) (*api.RunResult, error) {
	req.Dir = sh.cwd
	req.Env = sh.env

	var result api.RunResult
	err := sh.c.Exec(&req, &result)
	return &result, err
}

func (sh *Shell) Ps(ids ...string) ([]api.Proc, error) {
	if len(ids) == 1 {
		var result api.Proc
		err := sh.c.Ps1(ids[0], &result)
		if err != nil {
			return nil, err
		}
		return []api.Proc{result}, nil
	}
	// get all
	// TODO filter by ids?
	var result []api.Proc
	err := sh.c.Ps(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (sh *Shell) Kill(ids ...string) error {
	if len(ids) == 0 {
		return fmt.Errorf("missing proc id")
	}
	errs := make(map[string]error)
	for _, id := range ids {
		if err := sh.c.Kill(id); err != nil {
			errs[id] = err
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%v", errs)
	}
	return nil
}

func (sh *Shell) KillAll() error {
	ps, err := sh.Ps()
	if err != nil {
		return err
	}
	errs := make(map[string]error)
	for _, p := range ps {
		id := p.ID
		if err := sh.c.Kill(id); err != nil {
			errs[id] = err
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%v", errs)
	}
	return nil
}

func (sh *Shell) Pwd() string {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	return sh.cwd
}

func (sh *Shell) Chdir(s string) error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	dir := sh.resolvePath(s)

	err := sh.c.Opendir(dir)
	if err != nil {
		return err
	}
	sh.cwd = dir
	return nil
}

func (sh *Shell) resolvePath(s string) string {
	if strings.HasPrefix(s, "/") {
		return s
	}
	return fmt.Sprintf("%s/%s", sh.cwd, s)
}
