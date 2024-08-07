// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

//
// launcher interface and related types
//

// A launcher is capable of installing and starting a Node.
type launcher interface {
	launch(Node, logFunc) (transport, error)
}

// ParentNode defines the parent Node (the zero Node value).
var ParentNode = Node{}

// Node represents the information needed to launch a node. This struct must
// remain a valid map key (see https://go.dev/blog/maps#key-types). A zero Node
// value represents the parent node.
type Node struct {
	ID       ID        // identifies the Node
	Platform string    // the Node's platform (e.g. linux-amd64)
	Launcher launchers // union of available launchers
	Netns    Netns     // parameters for Linux network namespaces
	Env      Env       // process environment
}

// ID represents a node identifier. The empty string indicates the parent
// node.
type ID string

// String returns the node ID, or "parent" for the parent node.
func (n ID) String() string {
	if n == "" {
		return "parent"
	}
	return string(n)
}

// launch installs and starts the Node, and returns a transport connected to it
// for communication. The transport must be closed after it's no longer in use,
// so any cleanup operations are also performed.
func (n Node) launch(log logFunc) (transport, error) {
	return n.Launcher.launcher().launch(n, log)
}

func (n Node) String() string {
	return n.ID.String()
}

// launchers is a union of the available launcher implementations.
type launchers struct {
	Local *Local
	SSH   *SSH
}

// launcher returns the launcher implementation for the Node.
func (l *launchers) launcher() (a launcher) {
	switch {
	case l.SSH != nil:
		a = l.SSH
	case l.Local != nil:
		a = l.Local
	default:
		panic("no launcher set in launchers union")
	}
	return
}

// Netns represents the Linux network namespace configuration to use when
// launching a Node (man ip-netns(8)).
type Netns struct {
	// Name is the name of the namespace. If set, this namespace will either be
	// created or used, depending on the value of the Create field.
	Name string

	// Create indicates whether to create a namespace (true) or use an existing
	// one (false). If Create is true with no Name set, the Node ID will be used
	// as the network namespace name.
	Create bool
}

// zero returns true if this Netns is the zero value.
func (n Netns) zero() bool {
	return n == Netns{}
}

// EnvMax is the maximum number of allowed environment variables for a Node.
// This must be kept in sync with the length restriction in config.cue.
const EnvMax = 16

// Env specifies the environment of the node process.
type Env struct {
	// Vars lists the environment variables. Each entry must be of the form
	// "key=value". This field is an array so Node can remain a valid map key.
	Vars [EnvMax]string

	// Inherit indicates whether to include the parent process's environment
	// (true), or not (false).
	Inherit bool
}

// vars returns Vars as a slice, without empty elements, and inheriting the
// parent environment, if Inherit is true.
func (n Env) vars() (s []string) {
	if n.Inherit {
		s = append(s, os.Environ()...)
	}
	for _, e := range n.Vars {
		if e != "" {
			s = append(s, e)
		}
	}
	return
}

// varsSet returns true if any values in the Vars array are non-empty.
func (n Env) varsSet() bool {
	for _, e := range n.Vars {
		if e != "" {
			return true
		}
	}
	return false
}

// An ExeSource provides contents and metadata for node executables.
type ExeSource interface {
	// Reader returns a ReadCloser for the given platform's node executable.
	Reader(platform string) (io.ReadCloser, error)

	// Size returns the size of the given platform's node executable.
	Size(platform string) (int64, error)

	// Platforms returns the platforms for which executables are available.
	Platforms() ([]string, error)
}

//
// nodeCommand
//

// nodeCmd wraps exec.Cmd to create a command that runs a node.
type nodeCmd struct {
	*exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	stderrDone chan struct{}
	cleanup    io.Closer
	log        logFunc
}

// newNodeCmd returns a new instance of nodeCmd.
func newNodeCmd(cmd *exec.Cmd, cleanup io.Closer, log logFunc) (ncmd *nodeCmd,
	err error) {
	ncmd = &nodeCmd{
		cmd,                 // exec.Cmd
		nil,                 // stdin
		nil,                 // stdout
		nil,                 // stderr
		make(chan struct{}), // stderrDone
		cleanup,             // cleanup
		log,                 // log
	}
	if ncmd.stdin, err = ncmd.StdinPipe(); err != nil {
		return
	}
	if ncmd.stdout, err = ncmd.StdoutPipe(); err != nil {
		return
	}
	if ncmd.stderr, err = ncmd.StderrPipe(); err != nil {
		return
	}
	// log each line of stderr, until error or EOF, and discard errors
	go func() {
		defer close(ncmd.stderrDone)
		s := bufio.NewScanner(ncmd.stderr)
		for s.Scan() {
			ncmd.log("stderr: %s", s.Text())
		}
	}()
	return
}

// Write implements io.Writer
func (c *nodeCmd) Write(data []byte) (int, error) {
	return c.stdin.Write(data)
}

// Read implements io.Reader
func (c *nodeCmd) Read(data []byte) (int, error) {
	return c.stdout.Read(data)
}

// Close closes stdin to the underlying command, waits for it to exit, and
// calls the cleanup Closer on defer.
func (c *nodeCmd) Close() (err error) {
	if c.cleanup != nil {
		defer func() {
			if e := c.cleanup.Close(); e != nil && err == nil {
				err = e
			}
		}()
	}
	c.stdin.Close()
	err = c.Wait()
	<-c.stderrDone
	return
}

//
// exeRepo and related types
//

// repo is the package level exeRepo.
var repo = newExeRepo()

// exeRepo provides a package-level source for node executables and metadata.
type exeRepo struct {
	initted bool
	src     map[string]ExeSource
	fileRef map[string]int
	tmpDir  string
	mtx     sync.Mutex
}

// newExeRepo returns a new instance of exeRepo.
func newExeRepo() *exeRepo {
	return &exeRepo{
		false,                      // initted
		make(map[string]ExeSource), // src
		make(map[string]int),       // fileRef
		"",                         // tmpDir
		sync.Mutex{},               // mtx
	}
}

// init detects if we're running as the standalone node executable, and if so,
// adds an ExeSource and fileRef. It must be called, with the mutex locked, by
// any exeRepo method that needs this ExeSource to be available.
func (c *exeRepo) init() (err error) {
	if c.initted {
		return
	}
	var p string
	if p, err = os.Executable(); err != nil {
		return
	}
	n := ExeName(filepath.Base(p))
	if n.Valid() {
		c.fileRef[p] = 1
		c.src[n.Platform()] = &fileExeSource{n.Platform(), p}
		c.tmpDir = filepath.Dir(p)
	}
	c.initted = true
	return
}

// AddSource adds an ExeSource, replacing any prior ExeSources for the same
// platforms.
func (c *exeRepo) AddSource(src ExeSource) (err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	var ps []string
	if ps, err = src.Platforms(); err != nil {
		return
	}
	for _, p := range ps {
		c.src[p] = src
	}
	return
}

// Reader implements ExeSource
func (c *exeRepo) Reader(platform string) (rc io.ReadCloser, err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if err = c.init(); err != nil {
		return
	}
	var s ExeSource
	if s, err = c.source(platform); err != nil {
		return
	}
	rc, err = s.Reader(platform)
	return
}

// Size implements ExeSource
func (c *exeRepo) Size(platform string) (size int64, err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if err = c.init(); err != nil {
		return
	}
	var s ExeSource
	if s, err = c.source(platform); err != nil {
		return
	}
	size, err = s.Size(platform)
	return
}

// File returns an exeFile for the given platform, or an error if a file is not
// available for the given platform. The exeFile must be closed after use, so
// that the underlying file is deleted after it's no longer in use.
func (c *exeRepo) File(platform string) (file *exeFile, err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if err = c.init(); err != nil {
		return
	}
	if c.tmpDir == "" {
		if c.tmpDir, err = os.MkdirTemp("", "antler-*"); err != nil {
			return
		}
	}
	n := PlatformExeName(platform)
	p := filepath.Join(c.tmpDir, n.String())
	// if file already exists, increment ref count
	if _, ok := c.fileRef[p]; ok {
		c.fileRef[p]++
		file = &exeFile{p, c}
		return
	}
	// otherwise, extract it and start ref count at 1
	var s ExeSource
	if s, err = c.source(platform); err != nil {
		return
	}
	var r io.ReadCloser
	if r, err = s.Reader(platform); err != nil {
		return
	}
	defer r.Close()
	var f *os.File
	if f, err = os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0755); err != nil {
		return
	}
	defer f.Close()
	if _, err = io.Copy(f, r); err != nil {
		return
	}
	c.fileRef[p] = 1
	file = &exeFile{p, c}
	return
}

// Platforms implements ExeSource
func (c *exeRepo) Platforms() (s []string, err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if err = c.init(); err != nil {
		return
	}
	for p, _ := range c.src {
		s = append(s, p)
	}
	sort.Strings(s)
	return
}

// source returns the ExeSource for a given platform, or an error if a source is
// not available for the given platform.
func (c *exeRepo) source(platform string) (src ExeSource, err error) {
	var ok bool
	if src, ok = c.src[platform]; !ok {
		var p []string
		if p, err = c.Platforms(); err != nil {
			p = []string{err.Error()}
		}
		err = fmt.Errorf("no executable available for platform %s, "+
			"available platforms: %s", platform, p)
	}
	return
}

// Close is called by exeFile.Close. Extracted files and the temp directory
// are deleted when no longer in use.
func (c *exeRepo) Close(path string, log logFunc) (err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.fileRef[path]--
	if c.fileRef[path] == 0 {
		log("removing %s", path)
		if err = os.Remove(path); err != nil {
			return
		}
		delete(c.fileRef, path)
	}
	if len(c.fileRef) == 0 {
		log("removing %s", c.tmpDir)
		if err = os.Remove(c.tmpDir); err != nil {
			return
		}
		c.tmpDir = ""
	}
	return
}

// fileExeSource is an ExeSource for a given file.
type fileExeSource struct {
	platform string
	path     string
}

// Reader implements ExeSource
func (f *fileExeSource) Reader(platform string) (io.ReadCloser, error) {
	return os.Open(f.path)
}

// Size implements ExeSource
func (f *fileExeSource) Size(platform string) (size int64, err error) {
	var i fs.FileInfo
	if i, err = os.Stat(f.path); err != nil {
		return
	}
	size = i.Size()
	return
}

// Platforms implements ExeSource
func (f *fileExeSource) Platforms() ([]string, error) {
	return []string{f.platform}, nil
}

// exeFile represents an antler node executable that's considered in use until
// the Close method is called.
type exeFile struct {
	Path string
	repo *exeRepo
}

// Close notifies the exeRepo that this exeFile is no longer in use. It must be
// called exactly once.
func (f *exeFile) Close(log logFunc) error {
	return f.repo.Close(f.Path, log)
}

// ExeName represents an antler node executable name.
type ExeName string

// PlatformExeName returns an ExeName for the given platform (e.g. linux-amd64).
func PlatformExeName(platform string) ExeName {
	return ExeName(fmt.Sprintf("antler-node-%s", platform))
}

// Platform returns the platform for the name (e.g. linux-amd64).
func (n ExeName) Platform() string {
	return strings.TrimPrefix(n.String(), "antler-node-")
}

// Valid returns true if this is an executable name for a standalone node.
func (n ExeName) Valid() bool {
	return strings.HasPrefix(n.String(), "antler-node-")
}

func (n ExeName) String() string {
	return string(n)
}

//
// exes
//

// exes is a map of platform to executable contents.
type exes map[string][]byte

// newExes returns an exes for the given platforms. An error is
// returned if any of the platform executables could not be obtained from the
// ExeSource.
func newExes(src ExeSource, platform []string) (xs exes, err error) {
	xs = make(map[string][]byte, len(platform))
	var s []string
	if s, err = src.Platforms(); err != nil {
		return
	}
	for _, p := range platform {
		var a bool
		for _, q := range s {
			if q == p {
				a = true
				break
			}
		}
		if !a {
			err = fmt.Errorf("platform '%s' not one of %s", p, s)
			return
		}
		var r io.ReadCloser
		if r, err = src.Reader(p); err != nil {
			return
		}
		defer func() {
			if e := r.Close(); e != nil && err == nil {
				err = e
			}
		}()
		var b bytes.Buffer
		if io.Copy(&b, r); err != nil {
			return
		}
		xs[p] = b.Bytes()
	}
	return
}

// Platforms returns the sorted list of platforms.
func (x exes) Platforms() (platforms []string, err error) {
	for p := range x {
		platforms = append(platforms, p)
	}
	sort.Strings(platforms)
	return
}

// Bytes returns the executable contents for the given platform as a byte slice.
// An error is returned if an executable is not available for this platform.
func (x exes) Bytes(platform string) (bytes []byte, err error) {
	var ok bool
	if bytes, ok = x[platform]; !ok {
		p, _ := x.Platforms()
		err = fmt.Errorf("platform '%s' not one of %s", platform, p)
		return
	}
	return
}

// Reader returns a Reader for the executable contents for the given platform.
// An error is returned if an executable is not available for this platform.
func (x exes) Reader(platform string) (r io.ReadCloser, err error) {
	var b []byte
	if b, err = x.Bytes(platform); err != nil {
		return
	}
	r = io.NopCloser(bytes.NewReader(b))
	return
}

// Len returns the length of the executable for the given platform. An error is
// returned if an executable is not available for this platform.
func (x exes) Len(platform string) (length int, err error) {
	var b []byte
	if b, err = x.Bytes(platform); err != nil {
		return
	}
	length = len(b)
	return
}

// Size returns the size of the executable for the given platform, as an int64
// for compatibility with file sizes. An error is returned if an executable is
// not available for this platform.
func (x exes) Size(platform string) (size int64, err error) {
	var l int
	if l, err = x.Len(platform); err != nil {
		return
	}
	size = int64(l)
	return
}

// Remove removes the executable for the given platform, if it is present.
func (x exes) Remove(platform string) {
	delete(x, platform)
}
