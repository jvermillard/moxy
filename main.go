package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
)

var listen string
var server string
var debug bool

func setupFlags() {
	flag.StringVar(&listen, "listen", "0.0.0.0:1883", "the MQTT address and port to listen")
	flag.StringVar(&server, "server", "m2m.eclipse.org:1883", "the target MQTT server to proxify")
	flag.BoolVar(&debug, "v", false, "dumps verbose debug information")
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

	if debug {
		fmt.Printf("new connection: %v\n", conn.RemoteAddr())
		fmt.Printf("Connecting to: %s\n", server)
	}
	// first open a connection to the remote broker
	rConn, err := net.Dial("tcp", server)
	if err != nil {
		panic(err)
	}
	defer rConn.Close()

	go proxifyStream(conn, rConn, func(b *bytes.Buffer) {
		fmt.Print("SENT: ")
		dumpMqttPdu(b)
	})

	//  reverse proxifying
	proxifyStream(rConn, conn, func(b *bytes.Buffer) {
		fmt.Print("RCVD: ")
		dumpMqttPdu(b)
	})
}

func proxifyStream(reader io.Reader, writer io.Writer, dumper func(*bytes.Buffer)) {
	r := bufio.NewReader(reader)
	w := bufio.NewWriter(writer)
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

		// now consume remaining length bytes
		_, err = io.CopyN(buff, r, int64(length))
		if err != nil {
			panic(err)
		}

		// now push the PDU to the remote connection
		dumper(buff)

		count, err := buff.WriteTo(w)
		if err != nil {
			panic(err)
		}

		err = w.Flush()
		if err != nil {
			panic(err)
		}

		if debug {
			fmt.Printf("Wrote %d bytes\n", count)
		}
	}
}

type MsgType byte

/* MsgType */
const (
	/* 0x00 is reserved */
	CONNECT     MsgType = 0x01
	CONNACK     MsgType = 0x02
	PUBLISH     MsgType = 0x03
	PUBACK      MsgType = 0x04
	PUBREC      MsgType = 0x05
	PUBREL      MsgType = 0x06
	PUBCOMP     MsgType = 0x07
	SUBSCRIBE   MsgType = 0x08
	SUBACK      MsgType = 0x09
	UNSUBSCRIBE MsgType = 0x0A
	UNSUBACK    MsgType = 0x0B
	PINGREQ     MsgType = 0x0C
	PINGRESP    MsgType = 0x0D
	DISCONNECT  MsgType = 0x0E
	/* 0x0F is reserved */
)

var msgTypeLookup = map[MsgType]string{
	CONNECT:     "CONNECT",
	CONNACK:     "CONNACK",
	PUBLISH:     "PUBLISH",
	PUBACK:      "PUBACK",
	PUBREC:      "PUBREC",
	PUBREL:      "PUBREL",
	PUBCOMP:     "PUBCOMP",
	SUBSCRIBE:   "SUBSCRIBE",
	SUBACK:      "SUBACK",
	UNSUBSCRIBE: "UNSUBSCRIBE",
	UNSUBACK:    "UNSUBACK",
	PINGREQ:     "PINGREQ",
	PINGRESP:    "PINGRESP",
	DISCONNECT:  "DISCONNECT",
}

func dumpMqttPdu(buff *bytes.Buffer) {
	dump := buff.Bytes()
	msgType := decode_msgtype(dump[0])

	fmt.Printf("%s\n%s\n", msgTypeLookup[msgType], hex.Dump(dump))
}

//decode_msgtype returns the type of the message
func decode_msgtype(header byte) MsgType {
	mtype := (header & 0xF0) >> 4
	return MsgType(mtype)
}

func main() {
	fmt.Println("Moxy: a MQTT proxy")
	setupFlags()
	if debug {
		fmt.Println("verbose mode enabled")
	}
	startServer()
}
