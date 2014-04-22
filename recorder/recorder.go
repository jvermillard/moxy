package recorder

import (
	"encoding/base64"
	"encoding/json"
	"os"
)

type Recorder interface {
	SaveMessage(*MqttMessage) error
	Close() error
}

type FileRecorder struct {
	output *os.File
}

type MqttMessage struct {
	NanoTime int64
	DataB64  string
}

func NewMqttMessage(timestamp int64, data []byte) *MqttMessage {
	return &MqttMessage{timestamp, base64.StdEncoding.EncodeToString(data)}
}

func NewFileRecorder(fileName string) (Recorder, error) {
	var rec FileRecorder
	var err error
	rec.output, err = os.Create(fileName)
	if err != nil {
		return nil, err
	}
	return rec, nil
}

var separator = []byte("#########\n\n")

func (rec FileRecorder) Close() error {
	return rec.output.Close()
}

func (rec FileRecorder) SaveMessage(msg *MqttMessage) error {
	_, err := rec.output.Write(separator)
	if err != nil {
		return err
	}

	data, err := json.Marshal(msg)

	if err != nil {
		return err
	}

	_, err = rec.output.Write(data)
	if err != nil {
		return err
	}
	_, err = rec.output.Write([]byte("\n\n"))
	return err
}
