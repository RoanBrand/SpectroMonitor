package deltaplc

import (
	"fmt"
	"time"

	"github.com/RoanBrand/SpectroMonitor/log"

	"github.com/simonvetter/modbus"
)

type Modbus struct {
	c      *modbus.ModbusClient
	active bool
}

func New(modbusURL string) (*Modbus, error) {
	c, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL:     modbusURL,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid modbus config: %w", err)
	}

	// delta PLC words
	c.SetEncoding(modbus.LITTLE_ENDIAN, modbus.HIGH_WORD_FIRST)

	m := &Modbus{c: c}

	if err = m.c.Open(); err != nil {
		log.Println("error dialing modbus to Delta PLC at", modbusURL)
	} else {
		m.active = true
	}

	return m, nil
}

func (m *Modbus) Close() error {
	if m.active {
		m.active = false
		return m.c.Close()
	}
	return nil
}

// do not return error if connection still broken
func (m *Modbus) WriteBytes(addr uint16, data []byte) error {
	if !m.active {
		if err := m.c.Open(); err != nil {
			return nil
		}

		m.active = true
		log.Println("reconnected to Delta PLC")
	}

	if err := m.c.WriteBytes(addr, data); err != nil {
		m.c.Close()
		m.active = false
		return err
	}

	return nil
}

func (m *Modbus) WriteCoils(addr uint16, values []bool) error {
	if !m.active {
		if err := m.c.Open(); err != nil {
			return nil
		}

		m.active = true
		log.Println("reconnected to Delta PLC")
	}

	if err := m.c.WriteCoils(addr, values); err != nil {
		m.c.Close()
		m.active = false
		return err
	}

	return nil
}
