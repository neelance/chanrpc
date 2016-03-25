package main

import (
	"log"

	"github.com/neelance/chanrpc"
	"github.com/neelance/chanrpc/example/proto"
)

func main() {
	requests := make(chan *proto.Request)
	go processRequests(requests)
	log.Fatal(chanrpc.ListenAndServe(":7000", requests))
}

func processRequests(requests <-chan *proto.Request) {
	for req := range requests {
		go func(req *proto.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Print(err)
				}
			}()

			if execReq := req.Exec; execReq != nil {
				defer close(execReq.ReplyChan)
				execReq.ReplyChan <- &proto.ExecReply{Stdout: []byte("f"), Stderr: []byte("b")}
				execReq.ReplyChan <- &proto.ExecReply{Stdout: []byte("o"), Stderr: []byte("a")}
				execReq.ReplyChan <- &proto.ExecReply{Stdout: []byte("o"), Stderr: []byte("r"), Done: true}
			}

			if searchReq := req.Search; searchReq != nil {
				// ...
			}
		}(req)
	}
}
