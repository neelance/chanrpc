package chanrpcutil

import "bytes"

// Drain consumes and discards all byte slices from the given channel until it gets closed.
func Drain(c <-chan []byte) {
	for range c {
	}
}

// ReadAll immediately returns a new channel. In a separate goroutine it buffers the data of all
// incoming byte slices from the given channel until it gets closed. Then it sends all data as a
// single slice to the returned channel and closes it.
func ReadAll(c <-chan []byte) <-chan []byte {
	c2 := make(chan []byte, 1)
	go func() {
		var buf bytes.Buffer
		for b := range c {
			buf.Write(b)
		}
		c2 <- buf.Bytes()
		close(c2)
	}()
	return c2
}

// SplitToChunks splits a byte slice into multiple slices of 1MB each. The last slice might be
// shorter.
func SplitToChunks(b []byte) [][]byte {
	return SplitToChunksSize(b, 1024*1024)
}

// SplitToChunksSize splits a byte slice into multiple slices of the given chunk size. The last
// slice might be shorter.
func SplitToChunksSize(b []byte, chunkSize int) [][]byte {
	chunks := make([][]byte, (len(b)+chunkSize-1)/chunkSize)
	for i := range chunks {
		if i == len(chunks)-1 {
			chunks[i] = b[i*chunkSize:]
			break
		}
		chunks[i] = b[i*chunkSize : (i+1)*chunkSize]
	}
	return chunks
}

// SendChunksOnChannel creates a new channel and sends all given byte slice chunks on that channel.
// The channel then gets closed and returned.
func SendChunksOnChannel(chunks [][]byte) <-chan []byte {
	c := make(chan []byte, len(chunks))
	for _, b := range chunks {
		c <- b
	}
	close(c)
	return c
}
