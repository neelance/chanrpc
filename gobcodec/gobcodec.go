package gobcodec

import (
	"encoding/gob"
	"errors"
	"io"
	"net"
	"reflect"

	"github.com/neelance/chanrpc"
)

type action int

const actionSend action = 1
const actionClose action = 2

// Server serves the chanrpc protocol. Incoming requests get sent on RequestChan.
type Server struct {
	Addr        string
	RequestChan interface{}
}

// ListenAndServe listenes on the given address and serves the chanrpc protocol. Incoming requests
// get sent on the given channel.
func ListenAndServe(addr string, requestChan interface{}) error {
	srv := &Server{Addr: addr, RequestChan: requestChan}
	return srv.ListenAndServe()
}

// ListenAndServe listenes on the Server's address and serves the chanrpc protocol.
func (srv *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	return srv.Serve(ln)
}

// Serve accepts connections on the given listener and serves the chanrpc protocol.
func (srv *Server) Serve(l net.Listener) error {
	defer l.Close()
	for {
		rw, err := l.Accept()
		if err != nil {
			return err
		}

		go func() {
			c := chanrpc.NewConn(NewEncoder(rw), NewDecoder(rw), rw)
			c.SetRequestChannel(reflect.ValueOf(srv.RequestChan))
			c.ReceiveValues()
		}()
	}
}

// DialAndDeliver connects to the chanrpc server at the given address and delivers requests
// received from the request channel to the server.
func DialAndDeliver(addr string, requestChan interface{}) error {
	rw, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	c := chanrpc.NewConn(NewEncoder(rw), NewDecoder(rw), rw)
	c.Deliver(requestChan)
	return nil
}

type Encoder struct {
	enc *gob.Encoder
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{gob.NewEncoder(w)}
}

func (e *Encoder) EncodeSend(chanID int, chanList []*chanrpc.ChanDesc, value reflect.Value) error {
	if err := e.enc.Encode(actionSend); err != nil {
		return err
	}
	if err := e.enc.Encode(chanID); err != nil {
		return err
	}
	if err := e.enc.Encode(chanList); err != nil {
		return err
	}
	if err := e.enc.EncodeValue(value); err != nil {
		return err
	}
	return nil
}

func (e *Encoder) EncodeClose(chanID int) error {
	if err := e.enc.Encode(actionClose); err != nil {
		return err
	}
	if err := e.enc.Encode(chanID); err != nil {
		return err
	}
	return nil
}

type Decoder struct {
	dec *gob.Decoder
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{gob.NewDecoder(r)}
}

func (d *Decoder) Decode() (chanrpc.DecodedMessage, error) {
	var act action
	if err := d.dec.Decode(&act); err != nil {
		return nil, err
	}

	var chanID int
	if err := d.dec.Decode(&chanID); err != nil {
		return nil, err
	}

	switch act {
	case actionSend:
		var chanList []*chanrpc.ChanDesc
		if err := d.dec.Decode(&chanList); err != nil {
			return nil, err
		}

		return &decodedMessage{
			chanID:   chanID,
			chanList: chanList,
			dec:      d.dec,
		}, nil

	case actionClose:
		return &decodedMessage{
			chanID: chanID,
			closed: true,
		}, nil

	default:
		return nil, errors.New("chanrpc: protocol error, invalid action")
	}
}

type decodedMessage struct {
	chanID   int
	closed   bool
	chanList []*chanrpc.ChanDesc
	dec      *gob.Decoder
}

func (m *decodedMessage) ChanID() int {
	return m.chanID
}

func (m *decodedMessage) Closed() bool {
	return m.closed
}

func (m *decodedMessage) ChanList() []*chanrpc.ChanDesc {
	return m.chanList
}

func (m *decodedMessage) DecodeValue(value reflect.Value) error {
	return m.dec.DecodeValue(value)
}
