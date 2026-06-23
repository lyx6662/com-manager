// IEC 61850 客户端测试工具
// 使用 libiec61850 的 C 客户端 API 通过 CGO 连接服务器
// 编译: go build -o test-client.exe ./cmd/test-iec61850-client/
// 运行: test-client.exe [hostname] [port] [ld_prefix]
//   ld_prefix: IED+LD 前缀，如 "GWGRID_GATEWAY" (default)
package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/libiec61850/src/iec61850/inc
#cgo CFLAGS: -I${SRCDIR}/../../third_party/libiec61850/src/mms/inc
#cgo CFLAGS: -I${SRCDIR}/../../third_party/libiec61850/src/common/inc
#cgo CFLAGS: -I${SRCDIR}/../../third_party/libiec61850/hal/inc
#cgo CFLAGS: -I${SRCDIR}/../../third_party/libiec61850/src/mms/iso_mms/asn1c
#cgo CFLAGS: -I${SRCDIR}/../../third_party/libiec61850/config
#cgo CFLAGS: -I${SRCDIR}/../../third_party/libiec61850/src/logging
#cgo CFLAGS: -I${SRCDIR}/../../third_party/libiec61850/src/r_session
#cgo LDFLAGS: -L${SRCDIR}/../../build -liec61850-x86_64-windows -lhal-x86_64-windows -lws2_32 -lwinmm -static
#include "iec61850_client.h"
#include "hal_thread.h"
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

// build_ref 构建对象引用: ldPrefix + "/" + path
static void build_ref(char* buf, int bufSize, const char* ldPrefix, const char* path) {
    snprintf(buf, bufSize, "%s/%s", ldPrefix, path);
}

// printQuality 打印品质码含义
static void printQuality(int quality) {
    printf("      品质码: 0x%04X -> ", quality);
    if (quality == 0) {
        printf("Good (正常)\n");
    } else {
        if (quality & 0x80) printf("[无效] ");
        if (quality & 0x40) printf("[可疑] ");
        if (quality & 0x20) printf("[被取代] ");
        if (quality & 0x10) printf("[溢出] ");
        if (quality & 0x01) printf("[运维闭锁] ");
        printf("\n");
    }
}

// printTimestamp 打印时间戳
static void printTimestamp(uint64_t timestampMs) {
    // 毫秒时间戳转换为可读时间
    uint64_t seconds = timestampMs / 1000;
    uint64_t ms = timestampMs % 1000;

    // 简单计算日期时间 (UTC)
    uint64_t days = seconds / 86400;
    uint64_t secs = seconds % 86400;
    int hour = secs / 3600;
    int min = (secs % 3600) / 60;
    int sec = secs % 60;

    // 从1970-01-01开始的天数计算年月日 (简化算法)
    int year = 1970;
    int month = 1;
    int day = 1;

    // 累加年份
    while (days >= 365) {
        int isLeap = (year % 4 == 0 && (year % 100 != 0 || year % 400 == 0));
        if (days < (isLeap ? 366 : 365)) break;
        days -= isLeap ? 366 : 365;
        year++;
    }

    // 累加月份
    int daysInMonth[] = {31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31};
    if (year % 4 == 0 && (year % 100 != 0 || year % 400 == 0)) daysInMonth[1] = 29;
    while (days >= daysInMonth[month-1]) {
        days -= daysInMonth[month-1];
        month++;
    }
    day += days;

    printf("      时标: %04d-%02d-%02d %02d:%02d:%02d.%03llu (UTC)\n",
           year, month, day, hour, min, sec, (unsigned long long)ms);
    printf("      时间戳: %llu ms\n", (unsigned long long)timestampMs);
}

// buildDataObjectPath 构建数据对象级别的路径
// IEC 61850 结构: LN.DO.subDO.leaf (如 MMXU1.TotW.mag.f)
// q 和 t 在 DO 级别: LN.DO.q, LN.DO.t
// 规则: 保留 LN.DO 部分，去掉后面的 subDO.leaf
// "MMXU1.TotW.mag.f" -> "MMXU1.TotW"
// "CSWI1.Mod.stVal" -> "CSWI1.Mod"
static void buildDataObjectPath(const char* basePath, char* doPath, int bufSize) {
    strncpy(doPath, basePath, bufSize);
    doPath[bufSize-1] = '\0';

    // 按 "." 分割，保留前两段 (LN.DO)
    int dotCount = 0;
    char* p = doPath;
    while (*p) {
        if (*p == '.') {
            dotCount++;
            if (dotCount >= 2) {
                *p = '\0';  // 截断在第二个点之前
                break;
            }
        }
        p++;
    }
}

// readDataPoint 读取一个完整的数据点 (值 + 品质 + 时标)
static void readDataPoint(IedConnection con, const char* ldPrefix, const char* basePath, const char* dataName, FunctionalConstraint fc) {
    IedClientError error;
    char ref[256];
    char doPath[256];
    char fullPath[256];

    printf("    [%s]\n", dataName);

    // 读取值
    build_ref(ref, sizeof(ref), ldPrefix, basePath);
    error = IED_ERROR_OK;
    MmsValue* val = IedConnection_readObject(con, &error, ref, fc);
    if (val) {
        if (MmsValue_getType(val) == MMS_FLOAT) {
            printf("      数值: %f (FLOAT32)\n", MmsValue_toFloat(val));
        } else if (MmsValue_getType(val) == MMS_INTEGER) {
            printf("      数值: %d (INT32)\n", MmsValue_toInt32(val));
        } else if (MmsValue_getType(val) == MMS_DATA_ACCESS_ERROR) {
            printf("      数值: DATA_ACCESS_ERROR (错误码: %d)\n", MmsValue_getDataAccessError(val));
        } else {
            printf("      数值: 类型=%d\n", MmsValue_getType(val));
        }
        MmsValue_delete(val);
    } else {
        printf("      数值: 读取失败 (错误码: %d)\n", error);
    }

    // 构建数据对象级别的路径 (如 MMXU1.TotW)
    buildDataObjectPath(basePath, doPath, sizeof(doPath));

    // 读取品质 q (在 DO 级别)
    snprintf(fullPath, sizeof(fullPath), "%s.q", doPath);
    build_ref(ref, sizeof(ref), ldPrefix, fullPath);
    error = IED_ERROR_OK;
    MmsValue* qVal = IedConnection_readObject(con, &error, ref, fc);
    if (qVal) {
        if (MmsValue_getType(qVal) == MMS_BIT_STRING) {
            int quality = MmsValue_getBitStringAsInteger(qVal);
            printQuality(quality);
        } else if (MmsValue_getType(qVal) == MMS_DATA_ACCESS_ERROR) {
            printf("      品质: DATA_ACCESS_ERROR\n");
        } else {
            printf("      品质: 类型=%d\n", MmsValue_getType(qVal));
        }
        MmsValue_delete(qVal);
    } else {
        printf("      品质: 读取失败 (错误码: %d)\n", error);
    }

    // 读取时标 t (在 DO 级别)
    snprintf(fullPath, sizeof(fullPath), "%s.t", doPath);
    build_ref(ref, sizeof(ref), ldPrefix, fullPath);
    error = IED_ERROR_OK;
    MmsValue* tVal = IedConnection_readObject(con, &error, ref, fc);
    if (tVal) {
        if (MmsValue_getType(tVal) == MMS_UTC_TIME) {
            uint64_t timestamp = MmsValue_getUtcTimeInMs(tVal);
            printTimestamp(timestamp);
        } else if (MmsValue_getType(tVal) == MMS_DATA_ACCESS_ERROR) {
            printf("      时标: DATA_ACCESS_ERROR\n");
        } else {
            printf("      时标: 类型=%d\n", MmsValue_getType(tVal));
        }
        MmsValue_delete(tVal);
    } else {
        printf("      时标: 读取失败 (错误码: %d)\n", error);
    }
    printf("\n");
}

// client_test 连接服务器，浏览模型并读取数据
static int client_test(const char* hostname, int tcpPort, const char* ldPrefix) {
    IedClientError error;
    char ref[256];

    printf("========================================\n");
    printf("  IEC 61850 Client 测试工具\n");
    printf("========================================\n");
    printf("连接目标: %s:%d\n", hostname, tcpPort);
    printf("LD 前缀: %s\n\n", ldPrefix);

    IedConnection con = IedConnection_create();

    // 1. 建立连接
    printf("[1] 正在连接...\n");
    IedConnection_connect(con, &error, hostname, tcpPort);

    if (error != IED_ERROR_OK) {
        printf("    连接失败! 错误码: %d\n", error);
        IedConnection_destroy(con);
        return 1;
    }
    printf("    连接成功!\n\n");

    // 2. 获取逻辑设备列表
    printf("[2] 获取逻辑设备列表:\n");
    LinkedList deviceList = IedConnection_getLogicalDeviceList(con, &error);
    if (error == IED_ERROR_OK && deviceList) {
        LinkedList deviceElem = LinkedList_getNext(deviceList);
        while (deviceElem != NULL) {
            char* ldName = (char*)LinkedList_getData(deviceElem);
            printf("    LD: %s\n", ldName);
            deviceElem = LinkedList_getNext(deviceElem);
        }
        LinkedList_destroy(deviceList);
    }
    printf("\n");

    // 3. 读取所有数据点 (值 + 品质 + 时标)
    printf("[3] 读取数据点 (值 + 品质 + 时标):\n\n");

    // 读取所有 MMXU1-4 的 TotW0-TotW4
    char pathBuf[128];
    char nameBuf[128];
    for (int mmxu = 1; mmxu <= 4; mmxu++) {
        for (int tw = 0; tw <= 4; tw++) {
            snprintf(pathBuf, sizeof(pathBuf), "MMXU%d.TotW%d.mag.f", mmxu, tw);
            snprintf(nameBuf, sizeof(nameBuf), "MMXU%d.TotW%d", mmxu, tw);
            readDataPoint(con, ldPrefix, pathBuf, nameBuf, IEC61850_FC_MX);
        }
    }

    // 4. 连续读取观察变化 (读取 MMXU1.TotW0)
    printf("[4] 连续读取 5 次 MMXU1.TotW0 (每秒 1 次):\n");
    for (int i = 0; i < 5; i++) {
        printf("    --- 第 %d 次 ---\n", i+1);
        readDataPoint(con, ldPrefix, "MMXU1.TotW0.mag.f", "MMXU1.TotW0", IEC61850_FC_MX);
        Thread_sleep(1000);
    }

    // 5. 关闭连接
    printf("[5] 关闭连接...\n");
    IedConnection_close(con);
    IedConnection_destroy(con);

    printf("========================================\n");
    printf("  测试完成\n");
    printf("========================================\n");
    return 0;
}
*/
import "C"

import (
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

func main() {
	// 设置控制台输出为 UTF-8 编码
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setConsoleOutputCP := kernel32.NewProc("SetConsoleOutputCP")
	setConsoleOutputCP.Call(65001) // CP_UTF8

	hostname := "localhost"
	port := 102
	ldPrefix := "GWWG1"

	if len(os.Args) > 1 {
		hostname = os.Args[1]
	}
	if len(os.Args) > 2 {
		if p, err := strconv.Atoi(os.Args[2]); err == nil {
			port = p
		}
	}
	if len(os.Args) > 3 {
		ldPrefix = os.Args[3]
	}

	cHost := C.CString(hostname)
	defer C.free(unsafe.Pointer(cHost))
	cPrefix := C.CString(ldPrefix)
	defer C.free(unsafe.Pointer(cPrefix))

	C.client_test(cHost, C.int(port), cPrefix)
}
