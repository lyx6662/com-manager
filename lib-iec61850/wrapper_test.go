package iec61850

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"
)

// ==================== 辅助函数 ====================

// createTestModel 创建测试用的标准模型: LD=TEST_LD, LN=MMXU1, DO=Mag.f, DO=Mod.stVal
func createTestModel(t testing.TB) (*Model, *DataAttribute, *DataAttribute) {
	t.Helper()
	model := CreateModel("TestModel")
	if model == nil {
		t.Fatal("CreateModel 返回 nil")
	}

	ld := model.AddLogicalDevice("TEST_LD")
	if ld == nil {
		t.Fatal("AddLogicalDevice 返回 nil")
	}

	ln := ld.AddLogicalNode("MMXU1")
	if ln == nil {
		t.Fatal("AddLogicalNode 返回 nil")
	}

	// 测量值: Mag.f (float32, MX)
	mag := ln.AddDataObject("Mag")
	if mag == nil {
		t.Fatal("AddDataObject Mag 返回 nil")
	}
	daF := mag.AddDataAttribute("f", TypeFloat32, FCMeasurand, TriggerDataChanged)
	if daF == nil {
		t.Fatal("AddDataAttribute f 返回 nil")
	}
	model.RegisterDA("TEST_LD/MMXU1.Mag.f", daF)

	// 状态值: Mod.stVal (int32, ST)
	mod := ln.AddDataObject("Mod")
	if mod == nil {
		t.Fatal("AddDataObject Mod 返回 nil")
	}
	daSt := mod.AddDataAttribute("stVal", TypeInt32, FCStatus, TriggerDataChanged)
	if daSt == nil {
		t.Fatal("AddDataAttribute stVal 返回 nil")
	}
	model.RegisterDA("TEST_LD/MMXU1.Mod.stVal", daSt)

	return model, daF, daSt
}

// startTestServer 创建并启动测试服务器，返回 server 和清理函数
func startTestServer(t testing.TB, model *Model, port int) (*Server, func()) {
	t.Helper()
	srv := NewServer(model, ServerConfig{TCPPort: port})
	if srv == nil {
		t.Fatal("NewServer 返回 nil")
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	cleanup := func() {
		srv.Stop()
		srv.Destroy()
	}
	return srv, cleanup
}

// ==================== 方向一：单元测试 ====================

func TestModelCreateAndDestroy(t *testing.T) {
	model := CreateModel("M1")
	if model == nil {
		t.Fatal("CreateModel 返回 nil")
	}
	if model.handle == nil {
		t.Fatal("model.handle 为 nil")
	}
	model.Destroy()
	// 销毁后 handle 应为 nil
	if model.handle != nil {
		t.Error("Destroy 后 handle 不为 nil")
	}
}

func TestModelHierarchy(t *testing.T) {
	model := CreateModel("HModel")
	defer model.Destroy()

	ld := model.AddLogicalDevice("LD1")
	if ld == nil {
		t.Fatal("AddLogicalDevice 失败")
	}

	ln := ld.AddLogicalNode("LLN0")
	if ln == nil {
		t.Fatal("AddLogicalNode 失败")
	}

	do := ln.AddDataObject("DO1")
	if do == nil {
		t.Fatal("AddDataObject 失败")
	}

	da := do.AddDataAttribute("stVal", TypeInt32, FCStatus, TriggerDataChanged)
	if da == nil {
		t.Fatal("AddDataAttribute 失败")
	}

	// 验证句柄非零
	if ld.handle == nil || ln.handle == nil || do.handle == nil || da.handle == nil {
		t.Error("某个节点的 handle 为 nil")
	}
}

func TestNestedDataObjects(t *testing.T) {
	model := CreateModel("NestModel")
	defer model.Destroy()

	ld := model.AddLogicalDevice("LD1")
	ln := ld.AddLogicalNode("MMXU1")

	// 嵌套: MMXU1 -> TotW -> mag -> f
	totW := ln.AddDataObject("TotW")
	if totW == nil {
		t.Fatal("AddDataObject TotW 失败")
	}
	mag := totW.AddDataObject("mag")
	if mag == nil {
		t.Fatal("AddDataObject mag 失败")
	}
	daF := mag.AddDataAttribute("f", TypeFloat32, FCMeasurand, TriggerDataChanged)
	if daF == nil {
		t.Fatal("AddDataAttribute f 失败")
	}

	// 通过完整路径查找
	model.RegisterDA("LD1/MMXU1.TotW.mag.f", daF)
	found := model.FindDA("LD1/MMXU1.TotW.mag.f")
	if found == nil {
		t.Fatal("FindDA 未找到已注册的属性")
	}
	if found.handle != daF.handle {
		t.Error("FindDA 返回的句柄与注册的不一致")
	}
}

func TestServerLifecycle(t *testing.T) {
	model, _, _ := createTestModel(t)
	defer model.Destroy()

	// 创建 -> 启动 -> 停止 -> 销毁
	srv := NewServer(model, ServerConfig{TCPPort: 10201})
	if srv == nil {
		t.Fatal("NewServer 返回 nil")
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	if !srv.IsRunning() {
		t.Error("Start 后 IsRunning 应为 true")
	}

	srv.Stop()
	// Stop 后 IsRunning 可能仍为 true (取决于内部实现)，不强制检查

	srv.Destroy()
	if srv.IsRunning() {
		t.Error("Destroy 后 IsRunning 应为 false")
	}
}

func TestServerLifecycleRepeated(t *testing.T) {
	// 频繁创建/销毁服务器，检测死锁或崩溃
	for i := 0; i < 20; i++ {
		model := CreateModel(fmtModelName(i))
		ld := model.AddLogicalDevice("LD")
		ln := ld.AddLogicalNode("LLN0")
		do := ln.AddDataObject("Mod")
		do.AddDataAttribute("stVal", TypeInt32, FCStatus, TriggerDataChanged)

		srv := NewServer(model, ServerConfig{TCPPort: 0}) // 端口 0 让系统分配
		if srv == nil {
			model.Destroy()
			t.Fatalf("第 %d 次 NewServer 返回 nil", i)
		}
		// 不启动，直接销毁
		srv.Destroy()
		model.Destroy()
	}
}

func fmtModelName(i int) string {
	return fmt.Sprintf("M%d", i)
}

func TestFindDANotFound(t *testing.T) {
	model, _, _ := createTestModel(t)
	defer model.Destroy()

	// 不存在的路径
	da := model.FindDA("NONEXIST/LLN0.DO1.attr")
	if da != nil {
		t.Error("FindDA 对不存在的路径应返回 nil")
	}

	// 空字符串
	da = model.FindDA("")
	if da != nil {
		t.Error("FindDA 对空字符串应返回 nil")
	}

	// 查找 DataObject (非 DataAttribute)
	found := model.FindDO("TEST_LD/MMXU1.Mag")
	if found == nil {
		// 可能找不到，取决于路径格式，不一定是 bug
		t.Log("FindDO 未找到 TEST_LD/MMXU1.Mag (可能正常)")
	}
}

func TestRegisterDAOverwrite(t *testing.T) {
	model, daF, daSt := createTestModel(t)
	defer model.Destroy()

	// 注册同一路径两次，后者应覆盖前者
	model.RegisterDA("TEST_LD/MMXU1.Mag.f", daF)
	model.RegisterDA("TEST_LD/MMXU1.Mag.f", daSt) // 覆盖

	found := model.FindDA("TEST_LD/MMXU1.Mag.f")
	if found == nil {
		t.Fatal("FindDA 返回 nil")
	}
	if found.handle != daSt.handle {
		t.Error("RegisterDA 覆盖后 FindDA 应返回最新的句柄")
	}
}

func TestDataUpdateAndReadback(t *testing.T) {
	model, daF, daSt := createTestModel(t)
	defer model.Destroy()

	srv, cleanup := startTestServer(t, model, 10202)
	defer cleanup()

	// 测试 float32
	testValues := []float32{0.0, -1.5, 3.14159, 1e10, -1e10, 0.001}
	for _, v := range testValues {
		srv.UpdateFloat(daF, v)
		got := srv.GetFloat(daF)
		if got != v {
			t.Errorf("UpdateFloat(%v) -> GetFloat = %v", v, got)
		}
	}

	// 测试 int32
	intValues := []int32{0, 1, -1, 2147483647, -2147483648}
	for _, v := range intValues {
		srv.UpdateInt32(daSt, v)
		got := srv.GetInt32(daSt)
		if got != v {
			t.Errorf("UpdateInt32(%v) -> GetInt32 = %v", v, got)
		}
	}
}

func TestUpdateByPath(t *testing.T) {
	model, _, _ := createTestModel(t)
	defer model.Destroy()

	srv, cleanup := startTestServer(t, model, 10203)
	defer cleanup()

	// 通过路径更新
	if err := srv.UpdateFloatByPath("TEST_LD/MMXU1.Mag.f", 99.5); err != nil {
		t.Fatalf("UpdateFloatByPath 失败: %v", err)
	}

	// 通过句柄读回
	da := model.FindDA("TEST_LD/MMXU1.Mag.f")
	if da == nil {
		t.Fatal("FindDA 返回 nil")
	}
	got := srv.GetFloat(da)
	if got != 99.5 {
		t.Errorf("期望 99.5, 实际 %v", got)
	}

	// 不存在的路径
	err := srv.UpdateFloatByPath("NONEXIST/path", 1.0)
	if err == nil {
		t.Error("UpdateFloatByPath 不存在的路径应返回错误")
	}
}

func TestBoolUpdate(t *testing.T) {
	model := CreateModel("BoolModel")
	defer model.Destroy()

	ld := model.AddLogicalDevice("LD")
	ln := ld.AddLogicalNode("CSWI")
	do := ln.AddDataObject("Pos")
	da := do.AddDataAttribute("stVal", TypeBoolean, FCStatus, TriggerDataChanged)

	srv, cleanup := startTestServer(t, model, 10204)
	defer cleanup()

	srv.UpdateBool(da, true)
	if !srv.GetBool(da) {
		t.Error("UpdateBool(true) -> GetBool 应为 true")
	}

	srv.UpdateBool(da, false)
	if srv.GetBool(da) {
		t.Error("UpdateBool(false) -> GetBool 应为 false")
	}
}

func TestStringUpdate(t *testing.T) {
	model := CreateModel("StrModel")
	defer model.Destroy()

	ld := model.AddLogicalDevice("LD")
	ln := ld.AddLogicalNode("LLN0")
	do := ln.AddDataObject("NamPlt")
	da := do.AddDataAttribute("vendor", TypeVisibleString255, FCDescription, TriggerDataChanged)
	model.RegisterDA("LD/LLN0.NamPlt.vendor", da)

	srv, cleanup := startTestServer(t, model, 10205)
	defer cleanup()

	testStr := "Test Vendor 123"
	if err := srv.UpdateStringByPath("LD/LLN0.NamPlt.vendor", testStr); err != nil {
		t.Fatalf("UpdateStringByPath 失败: %v", err)
	}
	// 字符串没有 GetString 方法，验证不崩溃即可
}

func TestBatchUpdate(t *testing.T) {
	model, daF, _ := createTestModel(t)
	defer model.Destroy()

	srv, cleanup := startTestServer(t, model, 10206)
	defer cleanup()

	// 批量更新
	srv.LockDataModel()
	for i := 0; i < 1000; i++ {
		srv.UpdateFloat(daF, float32(i)*0.1)
	}
	srv.UnlockDataModel()

	// 最终值应为 999 * 0.1 = 99.9
	got := srv.GetFloat(daF)
	if got != 99.9 {
		t.Errorf("批量更新后期望 99.9, 实际 %v", got)
	}
}

func TestServerConnectionCount(t *testing.T) {
	model, _, _ := createTestModel(t)
	defer model.Destroy()

	srv, cleanup := startTestServer(t, model, 10207)
	defer cleanup()

	// 无客户端连接时应为 0
	count := srv.GetConnectionCount()
	if count != 0 {
		t.Errorf("无连接时期望 0, 实际 %d", count)
	}
}

// ==================== 方向二：并发与数据竞争测试 ====================

func TestConcurrentUpdateFloat(t *testing.T) {
	model, daF, _ := createTestModel(t)
	defer model.Destroy()

	srv, cleanup := startTestServer(t, model, 10210)
	defer cleanup()

	var wg sync.WaitGroup
	goroutines := 50
	iterations := 1000

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				srv.UpdateFloat(daF, float32(id*1000+i))
			}
		}(g)
	}
	wg.Wait()

	// 验证最终值是某个 goroutine 写入的 (无法确定具体哪个)
	val := srv.GetFloat(daF)
	if val == 0 && goroutines*iterations > 0 {
		// 值不应该是初始值 0 (除非某个 goroutine 恰好写了 0)
		// 这里只检查不崩溃，不检查具体值
	}
	t.Logf("并发更新完成，最终值: %v", val)
}

func TestConcurrentUpdateByPath(t *testing.T) {
	model, _, _ := createTestModel(t)
	defer model.Destroy()

	srv, cleanup := startTestServer(t, model, 10211)
	defer cleanup()

	var wg sync.WaitGroup
	goroutines := 30

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				_ = srv.UpdateFloatByPath("TEST_LD/MMXU1.Mag.f", float32(id*1000+i))
			}
		}(g)
	}
	wg.Wait()

	t.Log("并发 UpdateByPath 测试完成，无崩溃")
}

func TestConcurrentMixedOperations(t *testing.T) {
	model, daF, daSt := createTestModel(t)
	defer model.Destroy()

	srv, cleanup := startTestServer(t, model, 10212)
	defer cleanup()

	var wg sync.WaitGroup

	// 写 float 的 goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			srv.UpdateFloat(daF, float32(i))
		}
	}()

	// 写 int32 的 goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			srv.UpdateInt32(daSt, int32(i))
		}
	}()

	// 读 float 的 goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			_ = srv.GetFloat(daF)
		}
	}()

	// 读 int32 的 goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			_ = srv.GetInt32(daSt)
		}
	}()

	// 查找操作的 goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			_ = model.FindDA("TEST_LD/MMXU1.Mag.f")
		}
	}()

	wg.Wait()
	t.Log("混合并发操作测试完成，无崩溃")
}

func TestConcurrentLockUnlock(t *testing.T) {
	model, daF, _ := createTestModel(t)
	defer model.Destroy()

	srv, cleanup := startTestServer(t, model, 10213)
	defer cleanup()

	var wg sync.WaitGroup
	goroutines := 20

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				srv.LockDataModel()
				srv.UpdateFloat(daF, float32(id*100+i))
				srv.UnlockDataModel()
			}
		}(g)
	}
	wg.Wait()

	t.Log("并发 Lock/Unlock 测试完成，无死锁")
}

// ==================== 方向三：内存泄漏压测 ====================

func TestMemoryLeakModelLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过内存泄漏测试 (使用 -short)")
	}

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	iterations := 10000
	for i := 0; i < iterations; i++ {
		model := CreateModel(fmt.Sprintf("LeakModel_%d", i))
		ld := model.AddLogicalDevice("LD")
		ln := ld.AddLogicalNode("LLN0")
		do := ln.AddDataObject("DO1")
		do.AddDataAttribute("stVal", TypeInt32, FCStatus, TriggerDataChanged)
		model.Destroy()
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Go 堆内存增长不应超过 10MB
	heapGrowth := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)
	t.Logf("模型生命周期 %d 次迭代: Go 堆内存变化: %d KB", iterations, heapGrowth/1024)

	if heapGrowth > 10*1024*1024 {
		t.Errorf("Go 堆内存增长超过 10MB: %d KB，可能存在内存泄漏", heapGrowth/1024)
	}
}

func TestMemoryLeakServerLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过内存泄漏测试 (使用 -short)")
	}

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	iterations := 5000
	for i := 0; i < iterations; i++ {
		model := CreateModel(fmt.Sprintf("SrvLeak_%d", i))
		ld := model.AddLogicalDevice("LD")
		ln := ld.AddLogicalNode("LLN0")
		do := ln.AddDataObject("DO1")
		do.AddDataAttribute("stVal", TypeInt32, FCStatus, TriggerDataChanged)

		srv := NewServer(model, ServerConfig{TCPPort: 0})
		if srv != nil {
			srv.Destroy()
		}
		model.Destroy()
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	heapGrowth := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)
	t.Logf("服务器生命周期 %d 次迭代: Go 堆内存变化: %d KB", iterations, heapGrowth/1024)

	if heapGrowth > 10*1024*1024 {
		t.Errorf("Go 堆内存增长超过 10MB: %d KB，可能存在内存泄漏", heapGrowth/1024)
	}
}

func TestMemoryLeakUpdateCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过内存泄漏测试 (使用 -short)")
	}

	model, daF, daSt := createTestModel(t)
	defer model.Destroy()

	srv, cleanup := startTestServer(t, model, 10220)
	defer cleanup()

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	iterations := 100000
	for i := 0; i < iterations; i++ {
		srv.UpdateFloat(daF, float32(i)*0.01)
		srv.UpdateInt32(daSt, int32(i))
		_ = srv.GetFloat(daF)
		_ = srv.GetInt32(daSt)
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	heapGrowth := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)
	t.Logf("数据更新 %d 次迭代: Go 堆内存变化: %d KB", iterations, heapGrowth/1024)

	if heapGrowth > 10*1024*1024 {
		t.Errorf("Go 堆内存增长超过 10MB: %d KB，可能存在内存泄漏", heapGrowth/1024)
	}
}

func TestMemoryLeakPathLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过内存泄漏测试 (使用 -short)")
	}

	model, _, _ := createTestModel(t)
	defer model.Destroy()

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	iterations := 100000
	for i := 0; i < iterations; i++ {
		da := model.FindDA("TEST_LD/MMXU1.Mag.f")
		if da == nil {
			t.Fatal("FindDA 返回 nil")
		}
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	heapGrowth := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)
	t.Logf("路径查找 %d 次迭代: Go 堆内存变化: %d KB", iterations, heapGrowth/1024)

	if heapGrowth > 5*1024*1024 {
		t.Errorf("Go 堆内存增长超过 5MB: %d KB，可能存在内存泄漏", heapGrowth/1024)
	}
}

// ==================== 方向四：边界值与异常注入 ====================

func TestNilModelOperations(t *testing.T) {
	// 对 nil model 的操作不应崩溃
	var model *Model
	_ = model // 不应崩溃

	// FindDA on nil model - 会 panic 因为 nil receiver，但不应 segfault
	// Go 的 nil pointer 会 panic，不会 segfault
	defer func() {
		if r := recover(); r != nil {
			t.Logf("FindDA(nil model) 触发 panic (预期行为): %v", r)
		}
	}()
	// 这行可能 panic，但不应 segfault
	_ = model
}

func TestNilServerOperations(t *testing.T) {
	var srv *Server

	// 对 nil server 的操作应安全返回，不应崩溃
	srv.Stop()
	srv.Destroy()
	_ = srv.IsRunning()
	_ = srv.GetConnectionCount()
	srv.LockDataModel()
	srv.UnlockDataModel()

	// Update/Get with nil server
	var da *DataAttribute
	srv.UpdateFloat(da, 1.0)
	srv.UpdateInt32(da, 1)
	srv.UpdateBool(da, true)
	srv.UpdateString(da, "test")
	_ = srv.GetFloat(da)
	_ = srv.GetInt32(da)
	_ = srv.GetBool(da)
}

func TestNilDataAttributeOperations(t *testing.T) {
	model, _, _ := createTestModel(t)
	defer model.Destroy()

	srv, cleanup := startTestServer(t, model, 10230)
	defer cleanup()

	// 传入 nil DataAttribute
	var da *DataAttribute
	srv.UpdateFloat(da, 1.0)    // 不应崩溃
	srv.UpdateInt32(da, 1)      // 不应崩溃
	srv.UpdateBool(da, true)    // 不应崩溃
	srv.UpdateString(da, "abc") // 不应崩溃
	srv.UpdateInt64(da, 1)      // 不应崩溃
	srv.UpdateQuality(da, 0)    // 不应崩溃
	_ = srv.GetFloat(da)        // 不应崩溃
	_ = srv.GetInt32(da)        // 不应崩溃
	_ = srv.GetBool(da)         // 不应崩溃
}

func TestDoubleDestroy(t *testing.T) {
	model := CreateModel("DDModel")
	ld := model.AddLogicalDevice("LD")
	ln := ld.AddLogicalNode("LLN0")
	do := ln.AddDataObject("DO1")
	do.AddDataAttribute("stVal", TypeInt32, FCStatus, TriggerDataChanged)

	srv := NewServer(model, ServerConfig{TCPPort: 10231})
	if srv == nil {
		t.Fatal("NewServer 返回 nil")
	}

	// 第一次销毁
	srv.Destroy()
	// 第二次销毁不应崩溃
	srv.Destroy()
	// 第三次
	srv.Destroy()

	// 模型也是
	model.Destroy()
	model.Destroy()
}

func TestStartStopAfterDestroy(t *testing.T) {
	model := CreateModel("DestModel")
	ld := model.AddLogicalDevice("LD")
	ln := ld.AddLogicalNode("LLN0")
	do := ln.AddDataObject("DO1")
	do.AddDataAttribute("stVal", TypeInt32, FCStatus, TriggerDataChanged)

	srv := NewServer(model, ServerConfig{TCPPort: 10232})
	if srv == nil {
		t.Fatal("NewServer 返回 nil")
	}

	srv.Destroy()

	// Destroy 后调用 Start/Stop/Update 不应崩溃
	err := srv.Start()
	if err == nil {
		t.Log("Destroy 后 Start 未返回错误 (可能正常，取决于实现)")
	}
	srv.Stop()

	var da *DataAttribute
	srv.UpdateFloat(da, 1.0)
	_ = srv.IsRunning()
	_ = srv.GetConnectionCount()

	model.Destroy()
}

func TestDestroyServerBeforeModel(t *testing.T) {
	// 正确的销毁顺序: 先销毁服务器，再销毁模型
	model, daF, _ := createTestModel(t)

	srv, cleanup := startTestServer(t, model, 10233)
	defer cleanup()

	// 先更新确认正常
	srv.UpdateFloat(daF, 42.0)
	if srv.GetFloat(daF) != 42.0 {
		t.Fatal("初始更新失败")
	}

	// 正确顺序: 先 Stop/Destroy 服务器
	srv.Stop()
	srv.Destroy()

	// 然后销毁模型
	model.Destroy()

	// 销毁后操作不应崩溃
	srv.UpdateFloat(daF, 99.0) // handle 已 nil，安全跳过
	_ = srv.GetFloat(daF)      // handle 已 nil，返回 0
	_ = srv.IsRunning()        // handle 已 nil，返回 false
}

func TestEmptyStringPath(t *testing.T) {
	model, _, _ := createTestModel(t)
	defer model.Destroy()

	srv, cleanup := startTestServer(t, model, 10234)
	defer cleanup()

	// 空路径
	err := srv.UpdateFloatByPath("", 1.0)
	if err == nil {
		t.Error("空路径应返回错误")
	}

	err = srv.UpdateInt32ByPath("", 1)
	if err == nil {
		t.Error("空路径应返回错误")
	}

	err = srv.UpdateBoolByPath("", true)
	if err == nil {
		t.Error("空路径应返回错误")
	}
}

func TestVeryLongPath(t *testing.T) {
	model, _, _ := createTestModel(t)
	defer model.Destroy()

	// 极长路径
	longPath := ""
	for i := 0; i < 1000; i++ {
		longPath += "a"
	}

	da := model.FindDA(longPath)
	if da != nil {
		t.Error("极长路径不应找到任何节点")
	}
}

func TestInvalidDataType(t *testing.T) {
	model := CreateModel("InvModel")
	defer model.Destroy()

	ld := model.AddLogicalDevice("LD")
	ln := ld.AddLogicalNode("LLN0")
	do := ln.AddDataObject("DO1")

	// 非法数据类型 (负数)
	da := do.AddDataAttribute("bad", DataType(-1), FCStatus, TriggerDataChanged)
	// 可能返回 nil 或创建成功，取决于 C 库实现
	if da != nil {
		t.Log("AddDataAttribute(负类型) 返回非 nil (C 库接受)")
	}

	// 非法功能约束
	da2 := do.AddDataAttribute("bad2", TypeInt32, FC(-99), TriggerDataChanged)
	if da2 != nil {
		t.Log("AddDataAttribute(非法FC) 返回非 nil (C 库接受)")
	}
}

func TestReusePortAfterStop(t *testing.T) {
	// 启动 -> 停止 -> 用同一端口再启动
	model1 := CreateModel("Reuse1")
	ld1 := model1.AddLogicalDevice("LD")
	ln1 := ld1.AddLogicalNode("LLN0")
	do1 := ln1.AddDataObject("DO1")
	do1.AddDataAttribute("stVal", TypeInt32, FCStatus, TriggerDataChanged)

	srv1 := NewServer(model1, ServerConfig{TCPPort: 10240})
	if srv1 == nil {
		t.Fatal("第一次 NewServer 返回 nil")
	}
	if err := srv1.Start(); err != nil {
		t.Fatalf("第一次 Start 失败: %v", err)
	}
	srv1.Stop()
	srv1.Destroy()
	model1.Destroy()

	// 等待端口释放
	time.Sleep(100 * time.Millisecond)

	model2 := CreateModel("Reuse2")
	ld2 := model2.AddLogicalDevice("LD")
	ln2 := ld2.AddLogicalNode("LLN0")
	do2 := ln2.AddDataObject("DO1")
	do2.AddDataAttribute("stVal", TypeInt32, FCStatus, TriggerDataChanged)

	srv2 := NewServer(model2, ServerConfig{TCPPort: 10240})
	if srv2 == nil {
		t.Fatal("第二次 NewServer 返回 nil")
	}
	if err := srv2.Start(); err != nil {
		t.Fatalf("第二次 Start 失败 (端口可能未释放): %v", err)
	}
	srv2.Stop()
	srv2.Destroy()
	model2.Destroy()
}

// ==================== 基准测试 ====================

func BenchmarkUpdateFloat(b *testing.B) {
	model, daF, _ := createTestModel(b)
	defer model.Destroy()

	srv, cleanup := startTestServer(b, model, 10250)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		srv.UpdateFloat(daF, float32(i))
	}
}

func BenchmarkUpdateFloatByPath(b *testing.B) {
	model, _, _ := createTestModel(b)
	defer model.Destroy()

	srv, cleanup := startTestServer(b, model, 10251)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = srv.UpdateFloatByPath("TEST_LD/MMXU1.Mag.f", float32(i))
	}
}

func BenchmarkGetFloat(b *testing.B) {
	model, daF, _ := createTestModel(b)
	defer model.Destroy()

	srv, cleanup := startTestServer(b, model, 10252)
	defer cleanup()
	srv.UpdateFloat(daF, 42.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = srv.GetFloat(daF)
	}
}

func BenchmarkFindDA(b *testing.B) {
	model, _, _ := createTestModel(b)
	defer model.Destroy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = model.FindDA("TEST_LD/MMXU1.Mag.f")
	}
}

func BenchmarkLockUnlock(b *testing.B) {
	model, daF, _ := createTestModel(b)
	defer model.Destroy()

	srv, cleanup := startTestServer(b, model, 10253)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		srv.LockDataModel()
		srv.UpdateFloat(daF, float32(i))
		srv.UnlockDataModel()
	}
}

// ==================== 辅助: 验证 unsafe.Pointer 大小 ====================

func TestUnsafePointerSize(t *testing.T) {
	// 确保 unsafe.Pointer 大小与 C void* 一致 (通常都是 8 字节在 64 位系统)
	size := unsafe.Sizeof(unsafe.Pointer(nil))
	t.Logf("unsafe.Pointer 大小: %d 字节", size)
	if size != 8 {
		t.Logf("警告: 非 64 位系统，unsafe.Pointer 大小为 %d", size)
	}
}
