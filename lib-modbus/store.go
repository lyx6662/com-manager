package modbus

import "sync"

// RegisterStore Modbus寄存器存储（TCP/RTU服务端共用）
type RegisterStore struct {
	mu             sync.RWMutex
	registers      map[uint16]uint16
	coils          map[uint16]bool
	discreteInputs map[uint16]bool
}

// NewRegisterStore 创建寄存器存储
func NewRegisterStore() *RegisterStore {
	return &RegisterStore{
		registers:      make(map[uint16]uint16),
		coils:          make(map[uint16]bool),
		discreteInputs: make(map[uint16]bool),
	}
}

// UpdateRegisters 更新寄存器值
func (s *RegisterStore) UpdateRegisters(startAddr uint16, values []uint16) {
	// 边界检查: startAddr + len(values) 不能超过 65536
	if uint32(startAddr)+uint32(len(values)) > 65536 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, val := range values {
		s.registers[startAddr+uint16(i)] = val
	}
}

// UpdateCoils 更新线圈值
func (s *RegisterStore) UpdateCoils(startAddr uint16, values []bool) {
	// 边界检查: startAddr + len(values) 不能超过 65536
	if uint32(startAddr)+uint32(len(values)) > 65536 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, val := range values {
		s.coils[startAddr+uint16(i)] = val
	}
}

// ReadRegisters 读保持/输入寄存器，返回响应数据或异常码
func (s *RegisterStore) ReadRegisters(startAddr, quantity uint16) ([]byte, ExceptionCode) {
	if quantity == 0 || quantity > 125 {
		return nil, ExceptionIllegalDataValue
	}
	// 边界检查: startAddr + quantity 不能超过 65536 (uint16 地址空间)
	if uint32(startAddr)+uint32(quantity) > 65536 {
		return nil, ExceptionIllegalDataAddress
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	byteCount := quantity * 2
	data := make([]byte, 1+byteCount)
	data[0] = byte(byteCount)

	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		val := s.registers[addr]
		data[1+i*2] = byte(val >> 8)
		data[1+i*2+1] = byte(val & 0xFF)
	}

	return data, 0
}

// ReadCoils 读线圈，返回响应数据或异常码
func (s *RegisterStore) ReadCoils(startAddr, quantity uint16) ([]byte, ExceptionCode) {
	if quantity == 0 || quantity > 2000 {
		return nil, ExceptionIllegalDataValue
	}
	// 边界检查: startAddr + quantity 不能超过 65536 (uint16 地址空间)
	if uint32(startAddr)+uint32(quantity) > 65536 {
		return nil, ExceptionIllegalDataAddress
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	byteCount := (quantity + 7) / 8
	data := make([]byte, 1+byteCount)
	data[0] = byte(byteCount)

	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		if s.coils[addr] {
			byteIndex := 1 + i/8
			bitIndex := i % 8
			data[byteIndex] |= 1 << bitIndex
		}
	}

	return data, 0
}

// ReadDiscreteInputs 读离散输入，返回响应数据或异常码
func (s *RegisterStore) ReadDiscreteInputs(startAddr, quantity uint16) ([]byte, ExceptionCode) {
	if quantity == 0 || quantity > 2000 {
		return nil, ExceptionIllegalDataValue
	}
	// 边界检查: startAddr + quantity 不能超过 65536 (uint16 地址空间)
	if uint32(startAddr)+uint32(quantity) > 65536 {
		return nil, ExceptionIllegalDataAddress
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	byteCount := (quantity + 7) / 8
	data := make([]byte, 1+byteCount)
	data[0] = byte(byteCount)

	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		if s.discreteInputs[addr] {
			byteIndex := 1 + i/8
			bitIndex := i % 8
			data[byteIndex] |= 1 << bitIndex
		}
	}

	return data, 0
}

// WriteSingleReg 写单个寄存器，返回回显数据或异常码
func (s *RegisterStore) WriteSingleReg(addr, value uint16) ([]byte, ExceptionCode) {
	s.mu.Lock()
	s.registers[addr] = value
	s.mu.Unlock()

	data := make([]byte, 4)
	data[0] = byte(addr >> 8)
	data[1] = byte(addr & 0xFF)
	data[2] = byte(value >> 8)
	data[3] = byte(value & 0xFF)
	return data, 0
}

// WriteMultiRegs 写多个寄存器，返回确认数据或异常码
func (s *RegisterStore) WriteMultiRegs(requestData []byte) ([]byte, ExceptionCode) {
	if len(requestData) < 5 {
		return nil, ExceptionIllegalDataValue
	}

	startAddr := uint16(requestData[0])<<8 | uint16(requestData[1])
	quantity := uint16(requestData[2])<<8 | uint16(requestData[3])
	byteCount := int(requestData[4])

	if len(requestData) < 5+byteCount {
		return nil, ExceptionIllegalDataValue
	}
	// 边界检查: startAddr + quantity 不能超过 65536
	if uint32(startAddr)+uint32(quantity) > 65536 {
		return nil, ExceptionIllegalDataAddress
	}

	s.mu.Lock()
	for i := uint16(0); i < quantity; i++ {
		offset := 5 + i*2
		if offset+1 < uint16(len(requestData)) {
			value := uint16(requestData[offset])<<8 | uint16(requestData[offset+1])
			s.registers[startAddr+i] = value
		}
	}
	s.mu.Unlock()

	data := make([]byte, 4)
	data[0] = byte(startAddr >> 8)
	data[1] = byte(startAddr & 0xFF)
	data[2] = byte(quantity >> 8)
	data[3] = byte(quantity & 0xFF)
	return data, 0
}

// WriteSingleCoil 写单个线圈，返回回显数据或异常码
func (s *RegisterStore) WriteSingleCoil(addr, value uint16) ([]byte, ExceptionCode) {
	s.mu.Lock()
	s.coils[addr] = (value == 0xFF00)
	s.mu.Unlock()

	data := make([]byte, 4)
	data[0] = byte(addr >> 8)
	data[1] = byte(addr & 0xFF)
	data[2] = byte(value >> 8)
	data[3] = byte(value & 0xFF)
	return data, 0
}

// WriteMultiCoils 写多个线圈，返回确认数据或异常码
func (s *RegisterStore) WriteMultiCoils(requestData []byte) ([]byte, ExceptionCode) {
	if len(requestData) < 5 {
		return nil, ExceptionIllegalDataValue
	}

	startAddr := uint16(requestData[0])<<8 | uint16(requestData[1])
	quantity := uint16(requestData[2])<<8 | uint16(requestData[3])

	// 边界检查: startAddr + quantity 不能超过 65536
	if uint32(startAddr)+uint32(quantity) > 65536 {
		return nil, ExceptionIllegalDataAddress
	}

	s.mu.Lock()
	for i := uint16(0); i < quantity; i++ {
		byteIndex := 5 + i/8
		bitIndex := i % 8
		if byteIndex < uint16(len(requestData)) {
			s.coils[startAddr+i] = (requestData[byteIndex] & (1 << bitIndex)) != 0
		}
	}
	s.mu.Unlock()

	data := make([]byte, 4)
	data[0] = byte(startAddr >> 8)
	data[1] = byte(startAddr & 0xFF)
	data[2] = byte(quantity >> 8)
	data[3] = byte(quantity & 0xFF)
	return data, 0
}
