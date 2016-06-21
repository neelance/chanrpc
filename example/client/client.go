package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"

	"github.com/neelance/chanrpc/example/proto"
	"github.com/neelance/chanrpc/gobcodec"
)

var requestsToServer = make(chan *proto.Request, 100)

func main() {
	go startClient()

	stdout, stderr, err := Exec("test", []string{"abc"})
	fmt.Println(string(stdout), string(stderr), err)
}

func startClient() {
	for {
		err := gobcodec.DialAndDeliver(":7000", requestsToServer)
		log.Printf("DialAndDeliver error, reconnecting: %v", err)
	}
}

func Exec(command string, args []string) ([]byte, []byte, error) {
	replyChan := make(chan *proto.ExecReply, 10)
	requestsToServer <- &proto.Request{Exec: &proto.ExecRequest{
		Command:   command,
		Args:      args,
		ReplyChan: replyChan,
	}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	done := false
	for reply := range replyChan {
		stdout.Write(reply.Stdout)
		stderr.Write(reply.Stderr)
		done = reply.Done
	}
	if !done {
		return nil, nil, errors.New("rpc failed")
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}
