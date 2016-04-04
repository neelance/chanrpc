package chanrpcutil

import "bytes"

func Drain(c <-chan []byte) {
	for range c {
	}
}

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

func SplitToChunks(b []byte) [][]byte {
	return SplitToChunksSize(b, 1024*1024)
}

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

func SendChunksOnChannel(chunks [][]byte) <-chan []byte {
	c := make(chan []byte, len(chunks))
	for _, b := range chunks {
		c <- b
	}
	close(c)
	return c
}
