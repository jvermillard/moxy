package server

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/jvermillard/moxy/recorder"
)

var debug bool

func StartServer(listen string, server string, isDebug bool, isTrace bool) {
	debug = isDebug
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		panic(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}

		go serve(conn, server, isTrace)
	}
}

const (
	RECV_BUF_LEN = 1200
)

// serve a connected MQTT client
func serve(conn net.Conn, server string, trace bool) {

	if debug {
		fmt.Printf("new connection: %v\n", conn.RemoteAddr())
		fmt.Printf("Connecting to: %s\n", server)
	}

	var rec recorder.Recorder = nil
	if trace {
		var err error
		name := strings.Replace(conn.RemoteAddr().String(), ":", "-", -1)
		rec, err = recorder.NewFileRecorder(name + ".trace")
		if err != nil {
			panic(err)
		}
	}

	// first open a connection to the remote broker
	rConn, err := net.Dial("tcp", server)
	if err != nil {
		panic(err)
	}
	defer rConn.Close()

	//  reverse proxifying
	go proxifyStream(rConn, conn, func(b *bytes.Buffer) {
		fmt.Print("RCVD: ")
		dumpMqttPdu(b)
	}, rec)

	proxifyStream(conn, rConn, func(b *bytes.Buffer) {
		fmt.Print("SENT: ")
		dumpMqttPdu(b)
	}, nil)

	err = conn.Close()
	//err = rConn.Close()

}

func eofOrPanic(err error) bool {
	if err == nil {
		return false
	}

	if err == io.EOF {
		return true
	}

	panic(err)
}

func proxifyStream(reader io.Reader, writer io.Writer, dumper func(*bytes.Buffer), rec recorder.Recorder) {

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
		if rec != nil {
			rec.Close()
		}
	}()

	r := bufio.NewReader(reader)
	w := bufio.NewWriter(writer)
	for {
		// read a whole MQTT PDU
		buff := new(bytes.Buffer)
		header, err := r.ReadByte()

		if eofOrPanic(err) {
			break
		}

		buff.WriteByte(header)

		// read variable length header
		multiplier := 1
		length := 0

		for {
			b, err := r.ReadByte()
			if eofOrPanic(err) {
				break
			}

			buff.WriteByte(b)

			length += (int(b) & 127) * multiplier
			multiplier *= 128
			if b&128 == 0 {
				break
			}
		}

		// now consume remaining length bytes
		_, err = io.CopyN(buff, r, int64(length))
		if eofOrPanic(err) {
			break
		}

		// print the PDU
		dumper(buff)

		// record the PDU
		if rec != nil {
			rec.SaveMessage(recorder.NewMqttMessage(time.Now().UnixNano(), buff.Bytes()))
		}

		// now push the PDU to the remote connection

		count, err := buff.WriteTo(w)
		if eofOrPanic(err) {
			break
		}

		if err != nil {
			panic(err)
		}

		err = w.Flush()
		if eofOrPanic(err) {
			break
		}

		if debug {
			fmt.Printf("Wrote %d bytes\n", count)
		}
	}

	fmt.Println("EoF")
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

	if msgType == CONNECT {

		cnx := decode_connect(dump)
		fmt.Printf("CONNECT: %#v\n", *cnx)
	}
}

//decode_msgtype returns the type of the message
func decode_msgtype(header byte) MsgType {
	mtype := (header & 0xF0) >> 4
	return MsgType(mtype)
}

func getByte(buff *bytes.Buffer) byte {
	v, err := buff.ReadByte()
	if err != nil {
		panic(err)
	}
	return v
}

func decode_connect(buff []byte) *MqttConnect {
	dump := bytes.NewBuffer(buff)

	cnxMsg := MqttConnect{}

	// skip header
	dump.ReadByte()

	// skip remaning len header
	for getByte(dump)&128 != 0 {
	}

	cnxMsg.ProtocolName = decode_string(dump)

	cnxMsg.Version = getByte(dump)

	cnxMsg.Flags = getByte(dump)

	cnxMsg.KeepAlive = uint16(getByte(dump))<<8 | uint16(getByte(dump))

	cnxMsg.ClientId = decode_string(dump)

	if cnxMsg.isWillFlag() {
		// will topic
		cnxMsg.WillTopic = decode_string(dump)
		// will mesage
		cnxMsg.WillMsg = decode_string(dump)
	}

	if cnxMsg.isUserFlag() {
		cnxMsg.Username = decode_string(dump)
	}
	if cnxMsg.isPasswordFlag() {
		cnxMsg.Password = decode_string(dump)
	}

	return &cnxMsg
}

func decode_string(buff *bytes.Buffer) string {
	msb := getByte(buff)
	lsb := getByte(buff)

	pLen := (uint16(msb) << 8) | uint16(lsb)

	return string(buff.Next(int(pLen)))
}

type MqttConnect struct {
	ProtocolName string
	Version      byte
	Flags        byte
	KeepAlive    uint16
	ClientId     string
	Username     string
	Password     string
	WillTopic    string
	WillMsg      string
}

func (cnx *MqttConnect) isWillFlag() bool {
	return cnx.Flags&0x04 > 0
}

func (cnx *MqttConnect) isUserFlag() bool {
	return cnx.Flags&0x80 > 0
}

func (cnx *MqttConnect) isPasswordFlag() bool {
	return cnx.Flags&0x40 > 0
}
