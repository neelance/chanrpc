package chanrpc

import (
	"errors"
	"io"
	"log"
	"reflect"
	"strings"
	"sync"
)

type ChanDesc struct {
	ID      int
	Path    string
	Dir     reflect.ChanDir
	Cap     int
	channel reflect.Value
}

type Encoder interface {
	EncodeSend(chanID int, chanList []*ChanDesc, value reflect.Value) error
	EncodeClose(chanID int) error
}

type Decoder interface {
	Decode() (DecodedMessage, error)
}

type DecodedMessage interface {
	ChanID() int
	Closed() bool
	ChanList() []*ChanDesc
	DecodeValue(reflect.Value) error
}

// Conn encapsulates the state of a connection between two chanrpc endpoints (server or client).
type Conn struct {
	enc            Encoder
	encMutex       sync.Mutex
	dec            Decoder
	closer         io.Closer
	chanMap        map[int]reflect.Value
	nextSendChanID int
	nextRecvChanID int
	chanMutex      sync.RWMutex
}

// NewConn creates a new connection which receives and sends data via the given reader and writer.
func NewConn(enc Encoder, dec Decoder, closer io.Closer) *Conn {
	return &Conn{
		enc:            enc,
		dec:            dec,
		closer:         closer,
		chanMap:        make(map[int]reflect.Value),
		nextSendChanID: 1,
		nextRecvChanID: -1,
	}
}

// Deliver delivers requests received from the request channel to the server.
func (c *Conn) Deliver(requestChan interface{}) {
	go c.ReceiveValues()
	c.deliverChannel(0, reflect.ValueOf(requestChan))
}

func (c *Conn) deliverChannel(chanID int, channel reflect.Value) {
	for {
		x, ok := channel.Recv()
		if !ok {
			break
		}
		if err := c.transmitSend(chanID, x); err != nil {
			c.handleError(err)
			break
		}
	}

	if err := c.transmitClose(chanID); err != nil {
		c.handleError(err)
	}
}

func (c *Conn) transmitSend(chanID int, x reflect.Value) error {
	var chanList []*ChanDesc
	extractChannels(x, "", &chanList)

	c.encMutex.Lock()
	defer c.encMutex.Unlock()

	// registerChannels needs to be protected by encMutex because else a value
	// on a receive channel might be sent before the channel itself has
	// reached the other side.
	if err := c.registerChannels(chanList); err != nil {
		return err
	}

	return c.enc.EncodeSend(chanID, chanList, x)
}

func (c *Conn) transmitClose(chanID int) error {
	c.encMutex.Lock()
	defer c.encMutex.Unlock()

	return c.enc.EncodeClose(chanID)
}

func extractChannels(x reflect.Value, path string, list *[]*ChanDesc) {
	switch x.Kind() {
	case reflect.Ptr:
		extractChannels(x.Elem(), path+".*", list)
	case reflect.Struct:
		for i := 0; i < x.NumField(); i++ {
			extractChannels(x.Field(i), path+"."+x.Type().Field(i).Name, list)
		}
	case reflect.Chan:
		if x.IsNil() {
			return
		}
		*list = append(*list, &ChanDesc{
			Path:    path,
			Dir:     x.Type().ChanDir(),
			Cap:     x.Cap(),
			channel: x,
		})
	}
}

func (c *Conn) registerChannels(chanList []*ChanDesc) error {
	c.chanMutex.Lock()
	defer c.chanMutex.Unlock()

	for _, entry := range chanList {
		if entry.Cap == 0 {
			return errors.New("chanrpc: type error, channel must be buffered")
		}
		switch entry.Dir {
		case reflect.SendDir:
			if c.chanMap == nil { // connection closed
				entry.channel.Close()
				break
			}
			entry.ID = c.nextSendChanID
			c.nextSendChanID++
			c.chanMap[entry.ID] = entry.channel
		case reflect.RecvDir:
			entry.ID = c.nextRecvChanID
			c.nextRecvChanID--
			go c.deliverChannel(entry.ID, entry.channel)
		case reflect.BothDir:
			return errors.New("chanrpc: type error, channel must have a direction")
		}
	}

	return nil
}

func (c *Conn) SetRequestChannel(channel reflect.Value) {
	c.chanMap[0] = channel
}

func (c *Conn) ReceiveValues() {
receiveLoop:
	for {
		m, err := c.dec.Decode()
		if err != nil {
			c.handleError(err)
			break receiveLoop
		}

		c.chanMutex.RLock()
		channel, ok := c.chanMap[m.ChanID()]
		c.chanMutex.RUnlock()
		if !ok {
			c.handleError(errors.New("chanrpc: protocol error, unknown channel id"))
			break receiveLoop
		}

		if m.Closed() {
			channel.Close()
			c.chanMutex.Lock()
			delete(c.chanMap, m.ChanID())
			c.chanMutex.Unlock()
			continue
		}

		x := reflect.New(channel.Type().Elem())
		if err := m.DecodeValue(x); err != nil {
			c.handleError(err)
			break receiveLoop
		}

		for _, entry := range m.ChanList() {
			target := x.Elem()
			for _, field := range strings.Split(entry.Path, ".")[1:] {
				if field == "*" {
					target = target.Elem()
					continue
				}
				target = target.FieldByName(field)
			}
			if target.Kind() != reflect.Chan {
				c.handleError(errors.New("chanrpc: type error, channel expected"))
				break receiveLoop
			}
			if target.Type().ChanDir() != entry.Dir {
				c.handleError(errors.New("chanrpc: type error, wrong channel direction"))
				break receiveLoop
			}
			newChannel := reflect.MakeChan(reflect.ChanOf(reflect.BothDir, target.Type().Elem()), entry.Cap)
			target.Set(newChannel)
			switch entry.Dir {
			case reflect.SendDir:
				go c.deliverChannel(entry.ID, newChannel)
			case reflect.RecvDir:
				c.chanMutex.Lock()
				c.chanMap[entry.ID] = newChannel
				c.chanMutex.Unlock()
			}
		}

		channel.Send(x.Elem())
	}

	c.chanMutex.Lock()
	for id, channel := range c.chanMap {
		if id != 0 {
			channel.Close()
		}
	}
	c.chanMap = nil
	c.chanMutex.Unlock()
}

func (c *Conn) handleError(err error) {
	c.closer.Close()
	if err != io.EOF {
		log.Print(err) // more flexible logging
	}
}
