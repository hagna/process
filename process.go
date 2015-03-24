// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package socket implements an WebSocket-based playground backend.
// Clients connect to a websocket handler and send run/kill commands, and
// the server sends the output and exit status of the running Processes.
// Multiple clients running multiple Processes may be served concurrently.
// The wire format is JSON and is described by the Message type.
//
// This will not run on App Engine as WebSockets are not supported there.
package process

import (
	"os/exec"
	"errors"

)

const msgLimit = 1000 // max number of messages to send per session

// Message is the wire format for the websocket connection to the browser.
// It is used for both sending output messages and receiving commands, as
// distinguished by the Kind field.
type Message struct {
	Id   string // client-provided unique id for the Process
	Kind string // in: "run", "kill" out: "stdout", "stderr", "end"
	Body string
}

// Process represents a running Process.
type Process struct {
	id   string
	out  chan<- *Message
	Done chan struct{} // closed when wait completes
	run  *exec.Cmd
}

// startProcess builds and runs the given program, sending its output
// and end event as Messages on the provided channel.
func StartProcess(dir *string, args []string, out chan<- *Message) *Process {
	p := &Process{
		id:   string(<-uniq),
		out:  out,
		Done: make(chan struct{}),
	}
	if err := p.start(dir, args); err != nil {
		p.end(err)
		close(out)
		return nil
	}
	go p.wait()
	return p
}

// Kill stops the Process if it is running and waits for it to exit.
func (p *Process) Kill() {
	if p == nil {
		return
	}
	p.run.Process.Kill()
	<-p.Done // block until Process exits
}

// start builds and starts the given program, sending its output to p.out,
// and stores the running *exec.Cmd in the run field.
func (p *Process) start(dir *string, args []string) error {

	if len(args) == 0 {
		return errors.New("No arguments found")
	}
	cmd := p.cmd(dir, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	p.run = cmd
	return nil
}

// wait waits for the running Process to complete
// and sends its error state to the client.
func (p *Process) wait() {
	p.end(p.run.Wait())
	close(p.Done) // unblock waiting Kill calls
}

// end sends an "end" message to the client, containing the Process id and the
// given error value.
func (p *Process) end(err error) {
	m := &Message{Id: p.id, Kind: "end"}
	if err != nil {
		m.Body = err.Error()
	}
	p.out <- m
}

// cmd builds an *exec.Cmd that writes its standard output and error to the
// Process' output channel.
func (p *Process) cmd(dir *string, args ...string) *exec.Cmd {
	cmd := exec.Command(args[0], args[1:]...)
	if dir != nil {
		cmd.Dir = *dir
	}
	cmd.Stdout = &messageWriter{p.id, "stdout", p.out}
	cmd.Stderr = &messageWriter{p.id, "stderr", p.out}
	return cmd
}

// messageWriter is an io.Writer that converts all writes to Message sends on
// the out channel with the specified id and kind.
type messageWriter struct {
	id, kind string
	out      chan<- *Message
}

func (w *messageWriter) Write(b []byte) (n int, err error) {
	w.out <- &Message{Id: w.id, Kind: w.kind, Body: string(b)}
	return len(b), nil
}

// limiter returns a channel that wraps dest. Messages sent to the channel are
// sent to dest. After msgLimit Messages have been passed on, a "kill" Message
// is sent to the kill channel, and only "end" messages are passed.
func limiter(kill chan<- *Message, dest chan<- *Message) chan<- *Message {
	ch := make(chan *Message)
	go func() {
		n := 0
		for m := range ch {
			switch {
			case n < msgLimit || m.Kind == "end":
				dest <- m
				if m.Kind == "end" {
					return
				}
			case n == msgLimit:
				// Process produced too much output. Kill it.
				kill <- &Message{Id: m.Id, Kind: "kill"}
			}
			n++
		}
	}()
	return ch
}



var uniq = make(chan int) // a source of numbers for naming temporary files

func init() {
	go func() {
		for i := 0; ; i++ {
			uniq <- i
		}
	}()
}
