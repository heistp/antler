// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package antler

import (
	"log"
	"net/http"
)

// Server is the builtin web server.
type Server struct {
	ListenAddr string
	RootDir    string
}

// Run runs the server.
func (s Server) Run() (err error) {
	http.Handle("/", http.FileServer(http.Dir(s.RootDir)))
	log.Printf("Listening on %s...", s.ListenAddr)
	err = http.ListenAndServe(s.ListenAddr, nil)
	return
}
