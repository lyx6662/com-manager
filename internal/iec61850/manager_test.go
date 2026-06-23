package iec61850

import (
	"testing"
	"time"

	"github.com/lyx6662/com-manager/pkg/config"
)

func TestManagerBuildAndStart(t *testing.T) {
	cfg := &config.ModbusToIEC61850Config{
		IEC61850: config.IEC61850Config{
			Enabled:        true,
			Port:           10201,
			IEDName:        "TestModel",
			MaxConnections: 5,
		},
		Model: config.IEC61850ModelConfig{
			LogicalDevices: []config.LogicalDeviceConfig{
				{
					Name: "TEST_LD",
					LogicalNodes: []config.LogicalNodeConfig{
						{
							Name: "MMXU1",
							DataObjects: []config.DataObjectConfig{
								{
									Name: "TotW",
									DataAttributes: []config.DataAttributeConfig{
										{
											Name: "mag",
											Children: []config.DataAttributeConfig{
												{
													Name:     "f",
													Type:     "FLOAT32",
													FC:       "MX",
													Triggers: "DATA_CHANGED",
												},
											},
										},
									},
								},
								{
									Name: "Mod",
									DataAttributes: []config.DataAttributeConfig{
										{
											Name:     "stVal",
											Type:     "INT32",
											FC:       "ST",
											Triggers: "DATA_CHANGED",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Mappings: []config.IEC61850MappingRule{
			{
				SourceDevice: "device-1",
				SourceName:   "power",
				DataType:     "float32",
				IEC61850Path: "TEST_LD/MMXU1.TotW.mag.f",
				Scale:        1.0,
			},
			{
				SourceDevice: "device-1",
				SourceName:   "mode",
				DataType:     "int32",
				IEC61850Path: "TEST_LD/MMXU1.Mod.stVal",
				Scale:        1.0,
			},
		},
	}

	mgr := NewManager(cfg)

	// 构建模型
	if err := mgr.BuildModel(); err != nil {
		t.Fatalf("BuildModel 失败: %v", err)
	}
	defer mgr.Stop()

	// 启动服务器
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}

	// 验证服务器运行
	if !mgr.IsRunning() {
		t.Fatal("服务器未运行")
	}

	// 更新数据
	// 注意: UpdateData 接收原始路径 (不含 IED 前缀)
	// updateValue 内部会自动添加 IED 前缀: IED="TestModel" + "TEST_LD" → "TestModelTEST_LD"
	if err := mgr.UpdateData("TEST_LD/MMXU1.TotW.mag.f", float32(25.5), 0); err != nil {
		t.Fatalf("UpdateFloat 失败: %v", err)
	}
	if err := mgr.UpdateData("TEST_LD/MMXU1.Mod.stVal", int32(1), 0); err != nil {
		t.Fatalf("UpdateInt32 失败: %v", err)
	}

	// 等待一下让服务器处理
	time.Sleep(100 * time.Millisecond)

	t.Log("=== 集成测试通过 ===")
	t.Log("服务器启动成功，数据更新成功")
	t.Log("可使用 test-client.exe localhost 10201 连接验证")
}

func TestManagerNotEnabled(t *testing.T) {
	cfg := &config.ModbusToIEC61850Config{
		IEC61850: config.IEC61850Config{
			Enabled: false,
			Port:    10202,
		},
	}

	mgr := NewManager(cfg)
	if err := mgr.BuildModel(); err != nil {
		t.Fatalf("BuildModel 失败: %v", err)
	}
	defer mgr.Stop()

	// 启动服务器 (即使 enabled=false，服务器仍可启动)
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}

	if !mgr.IsRunning() {
		t.Fatal("服务器未运行")
	}
	t.Log("disabled 配置下服务器仍可正常启动")
}
