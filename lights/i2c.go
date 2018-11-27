package lights

import (
	"github.com/RoanBrand/SpectroMonitor/log"
	"github.com/d2r2/go-i2c"
)

var cardOutputs map[uint8]uint8

func init() {
	cardOutputs = make(map[uint8]uint8, 2)
}

func slaveWriteRegU8(slave uint8, addr uint8, data uint8) error {
	conn, err := i2c.NewI2C(slave, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	err = conn.WriteRegU8(addr, data)
	if err != nil {
		return err
	}

	return nil
}

func SetLight(cardAddr, lightAddr uint8) {
	cardOutputs[cardAddr] |= lightAddr
	setAsOutputs(cardAddr)
	outputLights(cardAddr)
}

func ClearLight(cardAddr, lightAddr uint8) {
	cardOutputs[cardAddr] &^= lightAddr
	setAsOutputs(cardAddr)
	outputLights(cardAddr)
}

func clearLights(cardAddr uint8) {
	cardOutputs[cardAddr] = 0
	setAsOutputs(cardAddr)
	outputLights(cardAddr)
}

func setAsOutputs(cardAddr uint8) {
	if err := slaveWriteRegU8(cardAddr, 0, 0); err != nil {
		log.Fatal(err)
	}
}

func outputLights(cardAddr uint8) {
	if err := slaveWriteRegU8(cardAddr, 0x9, cardOutputs[cardAddr]); err != nil {
		log.Fatal(err)
	}
}
