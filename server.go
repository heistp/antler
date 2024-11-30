// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package antler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

////go:embed admin
//var admin embed.FS

// Server is the builtin web server.
type Server struct {
	ListenAddr string
	RootDir    string
}

// Run runs the server.
func (s Server) Run(ctx context.Context) (err error) {
	ec := make(chan error)

	m := http.NewServeMux()
	m.Handle("/", http.FileServer(http.Dir(s.RootDir)))
	//m.Handle("/admin/", http.FileServer(http.FS(admin)))
	var v http.Server
	v.Addr = s.ListenAddr
	v.Handler = m

	go func(ec chan error) {
		var e error
		defer func() {
			if p := recover(); p != nil {
				e = fmt.Errorf("server panic: %s\n%s", p, string(debug.Stack()))
			}
			if e != nil {
				ec <- e
			}
			close(ec)
		}()
		e = v.ListenAndServe()
	}(ec)

	log.Printf("Listening on %s...", s.ListenAddr)

	d := ctx.Done()
	for ec != nil {
		select {
		case <-d:
			c, x := context.WithTimeout(context.Background(), 1*time.Second)
			defer x()
			if e := v.Shutdown(c); e != nil && err == nil {
				err = e
			}
			d = nil
		case e := <-ec:
			if e != nil && e != http.ErrServerClosed && err == nil {
				err = e
			}
			ec = nil
		}
	}

	return
}
