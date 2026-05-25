package modbus

import (
	"encoding/binary"
	"math"
)

// RegisterType 寄存器类型
type RegisterType int

const (
	RegisterCoil        RegisterType = 1 // 线圈 (0x)
	RegisterDiscrete    RegisterType = 2 // 离散输入 (1x)
	RegisterInput       RegisterType = 3 // 输入寄存器 (3x)
	RegisterHolding     RegisterType = 4 // 保持寄存器 (4x)
)

// RegisterValue 寄存器值
type RegisterValue struct {
	Address uint16
	Type    RegisterType
	Raw     []uint16 // 原始16位值
}

// ToUint16 转换为uint16
func (v *RegisterValue) ToUint16() uint16 {
	if len(v.Raw) == 0 {
		return 0
	}
	return v.Raw[0]
}

// ToInt16 转换为int16
func (v *RegisterValue) ToInt16() int16 {
	return int16(v.ToUint16())
}

// ToUint32 转换为uint32 (两个寄存器合并)
func (v *RegisterValue) ToUint32() uint32 {
	if len(v.Raw) < 2 {
		return 0
	}
	return uint32(v.Raw[0])<<16 | uint32(v.Raw[1])
}

// ToInt32 转换为int32 (两个寄存器合并)
func (v *RegisterValue) ToInt32() int32 {
	return int32(v.ToUint32())
}

// ToFloat32 转换为float32 (两个寄存器合并, 大端)
func (v *RegisterValue) ToFloat32() float32 {
	if len(v.Raw) < 2 {
		return 0
	}
	bits := uint32(v.Raw[0])<<16 | uint32(v.Raw[1])
	return math.Float32frombits(bits)
}

// ToFloat32WordSwap 转换为float32 (两个寄存器合并, 字交换)
func (v *RegisterValue) ToFloat32WordSwap() float32 {
	if len(v.Raw) < 2 {
		return 0
	}
	bits := uint32(v.Raw[1])<<16 | uint32(v.Raw[0])
	return math.Float32frombits(bits)
}

// ToBool 转换为bool (线圈/离散输入)
func (v *RegisterValue) ToBool() bool {
	if len(v.Raw) == 0 {
		return false
	}
	return v.Raw[0] != 0
}

// FromUint16 从uint16创建
func FromUint16(addr uint16, val uint16) *RegisterValue {
	return &RegisterValue{
		Address: addr,
		Type:    RegisterHolding,
		Raw:     []uint16{val},
	}
}

// FromFloat32 从float32创建 (拆分为两个寄存器)
func FromFloat32(addr uint16, val float32) *RegisterValue {
	bits := math.Float32bits(val)
	return &RegisterValue{
		Address: addr,
		Type:    RegisterHolding,
		Raw:     []uint16{uint16(bits >> 16), uint16(bits & 0xFFFF)},
	}
}

// FromBool 从bool创建
func FromBool(addr uint16, val bool) *RegisterValue {
	var v uint16
	if val {
		v = 0xFF00
	}
	return &RegisterValue{
		Address: addr,
		Type:    RegisterCoil,
		Raw:     []uint16{v},
	}
}

// EncodeRegisters 编码寄存器值为字节
func EncodeRegisters(regs []uint16) []byte {
	buf := make([]byte, len(regs)*2)
	for i, reg := range regs {
		binary.BigEndian.PutUint16(buf[i*2:(i+1)*2], reg)
	}
	return buf
}

// DecodeRegisters 解码字节为寄存器值
func DecodeRegisters(data []byte, count int) []uint16 {
	regs := make([]uint16, count)
	for i := 0; i < count && i*2+1 < len(data); i++ {
		regs[i] = binary.BigEndian.Uint16(data[i*2 : (i+1)*2])
	}
	return regs
}
