package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
)

var listen string
var server string

func setupFlags() {
	flag.StringVar(&listen, "listen", "0.0.0.0:1883", "the MQTT address and port to listen")
	flag.StringVar(&server, "server", "m2m.eclipse.org:1883", "the target MQTT server to proxify")
	flag.Parse()
}

func startServer() {
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		panic(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}

		go serve(conn)
	}
}

const (
	RECV_BUF_LEN = 1200
)

// serve a connected MQTT client
func serve(conn net.Conn) {

	// first open a connection to the remote broker
	rConn, err := net.Dial("tcp", server)
	if err != nil {
		panic(err)
	}
	defer rConn.Close()

	go proxifyStream(conn, rConn)

	//  reverse proxifying
	proxifyStream(rConn, conn)
}

func proxifyStream(reader io.Reader, writer io.Writer) {
	r := bufio.NewReader(reader)
	w := writer
	for {
		// read a whole MQTT PDU
		buff := new(bytes.Buffer)
		header, err := r.ReadByte()

		if err != nil {
			panic(err)
		}

		buff.WriteByte(header)

		// read variable length header
		multiplier := 1
		length := 0

		for {
			b, err := r.ReadByte()
			buff.WriteByte(b)

			if err != nil {
				panic(err)
			}

			length += (int(b) & 127) * multiplier
			multiplier *= 128
			if b&128 == 0 {
				break
			}
		}

		fmt.Printf("remaining lengh: %d\n", length)

		// now consume remaining length bytes
		written, err := io.CopyN(buff, r, int64(length))
		if err != nil {
			panic(err)
		}

		fmt.Printf("%d bytes written on %d\n", written, length)

		// now push the PDU to the remote connection

		count, err := buff.WriteTo(w)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Wrote %d bytes\n", count)
	}
}

func main() {
	fmt.Println("Moxy: a MQTT proxy")
	setupFlags()
	startServer()
}
