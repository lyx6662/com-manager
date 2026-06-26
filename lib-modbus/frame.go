package modbus

import (
	"encoding/binary"
	"fmt"
)

// FunctionCode Modbus功能码
type FunctionCode byte

const (
	FuncReadCoils          FunctionCode = 0x01 // 读线圈
	FuncReadDiscreteInputs FunctionCode = 0x02 // 读离散输入
	FuncReadHoldingRegs    FunctionCode = 0x03 // 读保持寄存器
	FuncReadInputRegs      FunctionCode = 0x04 // 读输入寄存器
	FuncWriteSingleCoil    FunctionCode = 0x05 // 写单个线圈
	FuncWriteSingleReg     FunctionCode = 0x06 // 写单个寄存器
	FuncWriteMultiCoils    FunctionCode = 0x0F // 写多个线圈
	FuncWriteMultiRegs     FunctionCode = 0x10 // 写多个寄存器
)

// ExceptionCode Modbus异常码
type ExceptionCode byte

const (
	ExceptionIllegalFunction    ExceptionCode = 0x01
	ExceptionIllegalDataAddress ExceptionCode = 0x02
	ExceptionIllegalDataValue   ExceptionCode = 0x03
	ExceptionSlaveFailure       ExceptionCode = 0x04
)

// ModbusException Modbus异常
type ModbusException struct {
	Function  FunctionCode
	Exception ExceptionCode
}

func (e *ModbusException) Error() string {
	return fmt.Sprintf("modbus exception: function=0x%02X, exception=0x%02X", e.Function, e.Exception)
}

// RTUFrame Modbus RTU帧
type RTUFrame struct {
	SlaveID      byte
	FunctionCode FunctionCode
	Data         []byte
	CRC          uint16
}

// Encode 编码RTU帧
func (f *RTUFrame) Encode() []byte {
	frame := make([]byte, 0, len(f.Data)+4)
	frame = append(frame, f.SlaveID)
	frame = append(frame, byte(f.FunctionCode))
	frame = append(frame, f.Data...)
	crc := CRC16(frame)
	frame = append(frame, byte(crc&0xFF), byte(crc>>8))
	return frame
}

// TCPFrame Modbus TCP帧 (MBAP头)
type TCPFrame struct {
	TransactionID uint16
	ProtocolID    uint16 // 固定 0x0000
	Length        uint16
	UnitID        byte
	FunctionCode  FunctionCode
	Data          []byte
}

// Encode 编码TCP帧
func (f *TCPFrame) Encode() []byte {
	f.Length = uint16(len(f.Data) + 2)
	frame := make([]byte, 0, 6+len(f.Data)+2)

	// MBAP头
	buf := make([]byte, 6)
	binary.BigEndian.PutUint16(buf[0:2], f.TransactionID)
	binary.BigEndian.PutUint16(buf[2:4], f.ProtocolID)
	binary.BigEndian.PutUint16(buf[4:6], f.Length)
	frame = append(frame, buf...)
	frame = append(frame, f.UnitID)
	frame = append(frame, byte(f.FunctionCode))
	frame = append(frame, f.Data...)
	return frame
}

// CRC16 Modbus CRC16校验
func CRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// ParseRTUFrame 解析RTU帧
func ParseRTUFrame(data []byte) (*RTUFrame, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("RTU帧长度不足: %d", len(data))
	}

	// 过滤无效的从站ID（有效范围1-247）
	slaveID := data[0]
	if slaveID < 1 || slaveID > 247 {
		return nil, fmt.Errorf("无效的从站ID: 0x%02X，可能是噪声数据", slaveID)
	}

	// 过滤无效的功能码（有效范围1-127）
	funcCode := data[1]
	if funcCode < 1 || funcCode > 127 {
		return nil, fmt.Errorf("无效的功能码: 0x%02X，可能是噪声数据", funcCode)
	}

	frame := &RTUFrame{
		SlaveID:      slaveID,
		FunctionCode: FunctionCode(funcCode),
		Data:         data[2 : len(data)-2],
	}

	// 校验CRC
	receivedCRC := binary.LittleEndian.Uint16(data[len(data)-2:])
	calculatedCRC := CRC16(data[:len(data)-2])
	if receivedCRC != calculatedCRC {
		return nil, fmt.Errorf("CRC校验失败: 收到=0x%04X, 计算=0x%04X", receivedCRC, calculatedCRC)
	}

	return frame, nil
}

// ParseTCPFrame 解析TCP帧
func ParseTCPFrame(data []byte) (*TCPFrame, error) {
	if len(data) < 7 {
		return nil, fmt.Errorf("TCP帧长度不足: %d", len(data))
	}

	frame := &TCPFrame{
		TransactionID: binary.BigEndian.Uint16(data[0:2]),
		ProtocolID:    binary.BigEndian.Uint16(data[2:4]),
		Length:        binary.BigEndian.Uint16(data[4:6]),
		UnitID:        data[6],
		FunctionCode:  FunctionCode(data[7]),
		Data:          data[8:],
	}

	if frame.ProtocolID != 0x0000 {
		return nil, fmt.Errorf("协议ID错误: 0x%04X", frame.ProtocolID)
	}

	return frame, nil
}
