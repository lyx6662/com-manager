package core

import (
	"fmt"
	"testing"

	"github.com/lyx6662/com-manager/pkg/logger"
	"github.com/lyx6662/com-manager/pkg/model"
)

func TestConvertRegisters_Passthrough(t *testing.T) {
	log, _ := logger.New("debug", "")
	c := NewCollector(log, nil)

	regs := []uint16{0x1234, 0x5678, 0xABCD, 0xEF01}

	val, quality := c.convertRegisters(regs, "passthrough")
	if quality != model.QualityGood {
		t.Fatalf("期望质量为Good, 实际: %v", quality)
	}

	result, ok := val.([]uint16)
	if !ok {
		t.Fatalf("期望类型为[]uint16, 实际: %T", val)
	}

	if len(result) != len(regs) {
		t.Fatalf("期望长度%d, 实际: %d", len(regs), len(result))
	}

	for i, v := range regs {
		if result[i] != v {
			t.Errorf("寄存器[%d] 期望0x%04X, 实际0x%04X", i, v, result[i])
		}
	}

	fmt.Printf("✓ passthrough 测试通过: %v -> %v\n", regs, result)
}

func TestConvertRegisters_Int32DCBAPassthrough(t *testing.T) {
	log, _ := logger.New("debug", "")
	c := NewCollector(log, nil)

	regs := []uint16{0x1234, 0x5678}

	val, quality := c.convertRegisters(regs, "int32_dcba_passthrough")
	if quality != model.QualityGood {
		t.Fatalf("期望质量为Good, 实际: %v", quality)
	}

	result, ok := val.([]uint16)
	if !ok {
		t.Fatalf("期望类型为[]uint16, 实际: %T", val)
	}

	if len(result) != 2 {
		t.Fatalf("期望长度2, 实际: %d", len(result))
	}

	// 透传应该保持原样
	if result[0] != 0x1234 || result[1] != 0x5678 {
		t.Errorf("透传值不一致: 期望[0x1234, 0x5678], 实际[0x%04X, 0x%04X]", result[0], result[1])
	}

	// 对比 int32_dcba 会转换
	val2, _ := c.convertRegisters(regs, "int32_dcba")
	intVal := val2.(int32)
	fmt.Printf("✓ int32_dcba_passthrough 测试通过\n")
	fmt.Printf("  原始寄存器: [0x%04X, 0x%04X]\n", regs[0], regs[1])
	fmt.Printf("  透传结果:   [0x%04X, 0x%04X]\n", result[0], result[1])
	fmt.Printf("  int32_dcba: %d (0x%08X)\n", intVal, uint32(intVal))
}

func TestConvertRegisters_AllTypes(t *testing.T) {
	log, _ := logger.New("debug", "")
	c := NewCollector(log, nil)

	// 测试各种数据类型的寄存器数量
	testCases := []struct {
		dataType   string
		regs       []uint16
		expectGood bool
	}{
		{"uint16", []uint16{0x1234}, true},
		{"int16", []uint16{0x1234}, true},
		{"float32", []uint16{0x4120, 0x0000}, true},       // 10.0 in float32
		{"float32_dcba", []uint16{0x0000, 0x4120}, true},   // DCBA format
		{"int32", []uint16{0x0001, 0x0000}, true},          // 65536
		{"int32_dcba", []uint16{0x0000, 0x0001}, true},     // DCBA format
		{"uint32", []uint16{0x0001, 0x0000}, true},         // 65536
		{"uint32_dcba", []uint16{0x0000, 0x0001}, true},    // DCBA format
		{"passthrough", []uint16{0x1234, 0x5678, 0xABCD}, true},
		{"int32_dcba_passthrough", []uint16{0x1234, 0x5678}, true},
	}

	for _, tc := range testCases {
		val, quality := c.convertRegisters(tc.regs, tc.dataType)
		if tc.expectGood && quality != model.QualityGood {
			t.Errorf("%s: 期望Good, 实际%v", tc.dataType, quality)
			continue
		}
		fmt.Printf("✓ %-30s regs=%v -> val=%v\n", tc.dataType, tc.regs, val)
	}
}
