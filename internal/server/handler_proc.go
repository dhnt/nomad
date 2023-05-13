package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/dhnt/nomad/api"

	"github.com/google/uuid"
)

const defaultTimeout = time.Second * 30
const durationInSecond = 1000000000

var (
	listProcRe   = regexp.MustCompile(`^\/procs[\/]?$`)
	getProcRe    = regexp.MustCompile(`^\/procs\/([-0-9a-fA-F]+)$`)
	deleteProcRe = regexp.MustCompile(`^\/procs\/([-0-9a-fA-F]+)$`)
	createProcRe = regexp.MustCompile(`^\/procs[\/]?$`)
)

type datastore struct {
	m map[string]*api.Proc

	*sync.RWMutex
}

func (r *datastore) Add(p *api.Proc) {
	r.Lock()
	p.Created = time.Now()
	r.m[p.ID] = p
	r.Unlock()
}

func (r *datastore) Remove(id string) {
	r.Lock()
	delete(r.m, id)
	r.Unlock()
}

func (r *datastore) Get(id string) *api.Proc {
	r.RLock()
	defer r.RUnlock()

	p, ok := r.m[id]
	if ok {
		p.Elapsed = (int64)(time.Since(p.Created)) / durationInSecond
		return p
	}
	return nil
}

func (r *datastore) List() []*api.Proc {
	r.RLock()
	defer r.RUnlock()

	now := time.Now()

	procs := make([]*api.Proc, 0, len(r.m))
	for _, p := range r.m {
		p.Elapsed = (int64)(now.Sub(p.Created)) / durationInSecond
		procs = append(procs, p)
	}
	return procs
}

type ProcHandler struct {
	root    string
	baseUrl *url.URL

	store *datastore
}

func NewProcHandler(cfg *ServerConfig) *ProcHandler {
	return &ProcHandler{
		root:    cfg.Root,
		baseUrl: cfg.Url,
		store: &datastore{
			m:       map[string]*api.Proc{},
			RWMutex: &sync.RWMutex{},
		},
	}
}

func (h *ProcHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	switch {
	case r.Method == http.MethodGet && listProcRe.MatchString(r.URL.Path):
		h.List(w, r)
		return
	case r.Method == http.MethodGet && getProcRe.MatchString(r.URL.Path):
		h.Get(w, r)
		return
	case r.Method == http.MethodPost && createProcRe.MatchString(r.URL.Path):
		h.Create(w, r)
		return
	case r.Method == http.MethodDelete && deleteProcRe.MatchString(r.URL.Path):
		h.Remove(w, r)
		return
	default:
		notFound(w, r, r.URL.Path)
		return
	}
}

func (h *ProcHandler) resolvePath(name string) string {
	return filepath.Join(h.root, name)
}

func (h *ProcHandler) List(w http.ResponseWriter, r *http.Request) {
	procs := h.store.List()

	b, err := json.Marshal(procs)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func (h *ProcHandler) Get(w http.ResponseWriter, r *http.Request) {
	matches := getProcRe.FindStringSubmatch(r.URL.Path)
	if len(matches) < 2 {
		notFound(w, r, r.URL.Path)
		return
	}

	p := h.store.Get(matches[1])

	if p == nil {
		notFound(w, r, fmt.Sprintf("proc %s", matches[1]))
		return
	}
	b, err := json.Marshal(p)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func (h *ProcHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p api.Proc
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		internalServerError(w, r, err)
		return
	}

	// add proc id if not provided
	if p.ID == "" {
		id, err := uuid.NewRandom()
		if err != nil {
			internalServerError(w, r, err)
			return
		}
		p.ID = id.String()
	}

	log.Printf("create: %v", p)

	args, err := resolveArgs(h.root, p.Resolve, p.Args)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	p.Args = args

	h.store.Add(&p)

	if p.Background {
		go h.Run(&p)
		u := h.baseUrl.JoinPath("procs", p.ID)
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	// remove after completion if running in sync/foreground
	defer h.store.Remove(p.ID)

	res := h.Run(&p)
	b, err := json.Marshal(res)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func (h *ProcHandler) Remove(w http.ResponseWriter, r *http.Request) {
	matches := deleteProcRe.FindStringSubmatch(r.URL.Path)
	if len(matches) < 2 {
		notFound(w, r, r.URL.Path)
		return
	}

	p := h.store.Get(matches[1])
	if p == nil {
		notFound(w, r, fmt.Sprintf("proc %s", matches[1]))
		return
	}

	if p.Cancel != nil {
		p.Cancel()
	}

	h.store.Remove(p.ID)

	w.WriteHeader(http.StatusNoContent)
}

func (h *ProcHandler) Run(p *api.Proc) *api.RunResult {
	command := p.Command
	args := p.Args

	log.Printf("Run: %s %v\n", command, args)

	res := &api.RunResult{
		ID:         p.ID,
		Command:    command,
		Args:       args,
		Background: p.Background,
		Outfile:    p.Outfile,
		Errfile:    p.Errfile,
	}

	// state transitions
	stateRunning := func() {
		p.State = api.Running
		p.Status = 0
		p.Error = ""
		res.Status = 0
		res.Error = ""
	}

	stateDone := func() {
		p.State = api.Done
		p.Status = 0
		p.Error = ""
		res.Status = 0
		res.Error = ""
	}

	stateFailed := func(err error) {
		p.State = api.Failed
		st := err.Error()
		p.Status = 1
		p.Error = st
		res.Status = 1
		res.Error = st
	}

	var err error

	// setup stdout/stderr
	var stdout, stderr bytes.Buffer
	var outfile, errfile *os.File

	redirectOut, redirectErr := p.Outfile != "", p.Errfile != ""
	if redirectOut {
		outfile, err = os.Create(h.resolvePath(p.Outfile))
		if err != nil {
			log.Printf("failed to create outfile: %q %v", command, err)
			stateFailed(err)
			return res
		}
		defer outfile.Close()
	}
	if redirectErr {
		if p.Errfile == p.Outfile {
			errfile = outfile
		} else {
			errfile, err = os.Create(h.resolvePath(p.Errfile))
			if err != nil {
				log.Printf("failed to create errfile: %q %v", command, err)
				stateFailed(err)
				return res
			}
			defer errfile.Close()
		}
	}

	timeout := func() time.Duration {
		if p.Timeout <= 0 {
			p.Timeout = int64(defaultTimeout) / durationInSecond
			return defaultTimeout
		}
		return time.Duration(p.Timeout * durationInSecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)

	if redirectOut {
		cmd.Stdout = outfile
	} else {
		cmd.Stdout = &stdout
	}

	if redirectErr {
		cmd.Stderr = errfile
	} else {
		cmd.Stderr = &stderr
	}

	// set up working dir and env
	if p.Dir != "" {
		cmd.Dir = p.Dir
	}
	if p.Env != nil {
		cmd.Env = append(os.Environ(), p.Env...)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("start error: %q %v", command, err)
		stateFailed(err)
		return res
	}

	//
	p.Pid = cmd.Process.Pid
	p.Cancel = cancel

	stateRunning()

	//
	err = cmd.Wait()

	res.Stdout = stdout.String()
	res.Stderr = stderr.String()

	if err != nil {
		stateFailed(err)

		if exiterr, ok := err.(*exec.ExitError); ok {
			log.Printf("exit status: %q %v %d", command, err, exiterr.ExitCode())
			// update status code
			p.Status = exiterr.ExitCode()
			res.Status = p.Status
			return res
		} else {
			log.Printf("error: %q %v", command, err)
			return res
		}
	}

	stateDone()

	return res
}
