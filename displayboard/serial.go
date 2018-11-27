package displayboard

import (
	"github.com/tarm/serial"
	"time"
)

var port *serial.Port

func Start(portName string, baudRate int) error {
	s, err := serial.OpenPort(&serial.Config{Name: portName, Baud: baudRate})
	if err != nil {
		return err
	}

	port = s
	return nil
}

func Shutdown() error {
	return port.Close()
}

func Write(displayAddress uint8, data []byte) error {
	b := make([]byte, 0, 16)
	b = append(b, 0x0, 0x53, displayAddress, 0x3)

	b = append(b, data...)
	b = append(b, 0x4)

	var newXor byte
	for _, dataByte := range b {
		newXor ^= dataByte
	}
	b = append(b, newXor)

	_, err := port.Write(b)
	return err
}

func WriteCurrentTime(addr uint8) error {
	return Write(addr, []byte(time.Now().Format("15:04")))
}
