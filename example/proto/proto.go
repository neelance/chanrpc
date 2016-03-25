package proto

type Request struct {
	Exec   *ExecRequest
	Search *SearchRequest
}

type ExecRequest struct {
	Command   string
	Args      []string
	ReplyChan chan<- *ExecReply
}

type ExecReply struct {
	Stdout []byte
	Stderr []byte
	Done   bool
}

type SearchRequest struct {
	// ...
}
