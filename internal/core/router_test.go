package core

import (
	"fmt"
	"testing"

	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
	"github.com/lyx6662/com-manager/pkg/model"
)

func TestApplyMapping_Passthrough(t *testing.T) {
	log, _ := logger.New("debug", "")
	router := NewRouter(log)

	// 测试透传 uint16 数组
	pt := model.DataPoint{
		DeviceID:  "device-1",
		Name:      "test",
		Value:     []uint16{0x1234, 0x5678, 0xABCD, 0xEF01},
		Quality:   model.QualityGood,
		DataType:  model.DataType("uint16"),
	}

	rule := config.MappingRule{
		DataType: "passthrough",
	}

	result := router.applyMapping(rule, pt)
	if len(result) != 4 {
		t.Fatalf("期望4个寄存器, 实际: %d", len(result))
	}

	expected := []uint16{0x1234, 0x5678, 0xABCD, 0xEF01}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("寄存器[%d] 期望0x%04X, 实际0x%04X", i, v, result[i])
		}
	}

	fmt.Printf("✓ Router passthrough 测试通过: %v -> %v\n", pt.Value, result)
}

func TestApplyMapping_Uint16Pair(t *testing.T) {
	log, _ := logger.New("debug", "")
	router := NewRouter(log)

	// 测试 int32_dcba_passthrough 生成的 uint16_pair
	pt := model.DataPoint{
		DeviceID:  "device-1",
		Name:      "test",
		Value:     []uint16{0x1234, 0x5678},
		Quality:   model.QualityGood,
		DataType:  model.DataType("uint16_pair"),
	}

	rule := config.MappingRule{
		DataType: "int32_dcba_passthrough",
	}

	result := router.applyMapping(rule, pt)
	if len(result) != 2 {
		t.Fatalf("期望2个寄存器, 实际: %d", len(result))
	}

	if result[0] != 0x1234 || result[1] != 0x5678 {
		t.Errorf("透传值不一致: 期望[0x1234, 0x5678], 实际[0x%04X, 0x%04X]", result[0], result[1])
	}

	fmt.Printf("✓ Router uint16_pair 测试通过: %v -> %v\n", pt.Value, result)
}

func TestApplyMapping_NormalTypes(t *testing.T) {
	log, _ := logger.New("debug", "")
	router := NewRouter(log)

	testCases := []struct {
		name     string
		value    interface{}
		dataType string
		expected int // 期望的寄存器数量
	}{
		{"uint16", uint16(100), "uint16", 1},
		{"int16", int16(-100), "int16", 1},
		{"float32", float32(10.5), "float32", 2},
		{"int32", int32(100000), "int32", 2},
	}

	for _, tc := range testCases {
		pt := model.DataPoint{
			DeviceID: "device-1",
			Name:     tc.name,
			Value:    tc.value,
			Quality:  model.QualityGood,
			DataType: model.DataType(tc.dataType),
		}

		rule := config.MappingRule{
			DataType: tc.dataType,
			Scale:    1.0,
			Offset:   0.0,
		}

		result := router.applyMapping(rule, pt)
		if len(result) != tc.expected {
			t.Errorf("%s: 期望%d个寄存器, 实际%d", tc.name, tc.expected, len(result))
		}
		fmt.Printf("✓ %-10s val=%v -> regs=%v\n", tc.name, tc.value, result)
	}
}
