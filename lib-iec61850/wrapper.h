#ifndef IEC61850_WRAPPER_H
#define IEC61850_WRAPPER_H

#include <stdint.h>
#include <stdbool.h>

/*
 * IEC 61850 CGO 封装层头文件
 * 所有句柄参数使用 void* 以便 CGO 映射为 unsafe.Pointer
 */

/* 数据属性类型常量 (对应 DataAttributeType 枚举) */
#define IEC61850_WRAPPER_BOOLEAN    0
#define IEC61850_WRAPPER_INT8       1
#define IEC61850_WRAPPER_INT16      2
#define IEC61850_WRAPPER_INT32      3
#define IEC61850_WRAPPER_INT64      4
#define IEC61850_WRAPPER_INT8U      6
#define IEC61850_WRAPPER_INT16U     7
#define IEC61850_WRAPPER_INT32U     9
#define IEC61850_WRAPPER_FLOAT32   10
#define IEC61850_WRAPPER_FLOAT64   11
#define IEC61850_WRAPPER_VISIBLE_STRING_255 20
#define IEC61850_WRAPPER_TIMESTAMP  22
#define IEC61850_WRAPPER_QUALITY    23

/* 功能约束常量 */
#define IEC61850_WRAPPER_FC_ST   0   /* 状态 */
#define IEC61850_WRAPPER_FC_MX   1   /* 测量值 */
#define IEC61850_WRAPPER_FC_SP   2   /* 设定值 */
#define IEC61850_WRAPPER_FC_CF   4   /* 配置 */
#define IEC61850_WRAPPER_FC_DC   5   /* 描述 */

/* 触发选项常量 */
#define IEC61850_WRAPPER_TRG_OPT_DATA_CHANGED    1
#define IEC61850_WRAPPER_TRG_OPT_QUALITY_CHANGED 2
#define IEC61850_WRAPPER_TRG_OPT_DATA_UPDATE     4
#define IEC61850_WRAPPER_TRG_OPT_INTEGRITY       8
#define IEC61850_WRAPPER_TRG_OPT_GI              16

/* ==================== 模型创建 API ==================== */

/* 创建空的 IED 模型 */
void* iec61850_wrapper_model_create(const char* name);

/* 销毁 IED 模型 (仅限动态创建的模型) */
void iec61850_wrapper_model_destroy(void* model);

/* 创建逻辑设备 */
void* iec61850_wrapper_ld_create(const char* name, void* model);

/* 创建逻辑节点 */
void* iec61850_wrapper_ln_create(const char* name, void* ld);

/* 创建数据对象 */
void* iec61850_wrapper_do_create(const char* name, void* ln);

/* 创建数据属性
 * type: 数据属性类型 (IEC61850_WRAPPER_* 常量)
 * fc: 功能约束 (IEC61850_WRAPPER_FC_* 常量)
 * triggerOptions: 触发选项 (IEC61850_WRAPPER_TRG_OPT_* 常量的组合)
 */
void* iec61850_wrapper_da_create(const char* name, void* parent,
    int type, int fc, uint8_t triggerOptions);

/* 创建报告控制块 */
void iec61850_wrapper_rcb_create(const char* name, void* ln,
    const char* rptId, bool isBuffered, const char* dataSetName,
    uint32_t confRef, uint8_t trgOps, uint8_t options,
    uint32_t bufTm, uint32_t intgPd);

/* ==================== 服务器生命周期 API ==================== */

/* 创建 IEC 61850 服务器 */
void* iec61850_wrapper_server_create(void* model);

/* 创建带配置的 IEC 61850 服务器 */
void* iec61850_wrapper_server_create_with_config(void* model,
    int maxConnections, int reportBufferSize);

/* 启动服务器，返回 1 成功，0 失败 */
int iec61850_wrapper_server_start(void* server, int tcpPort);

/* 停止服务器 */
void iec61850_wrapper_server_stop(void* server);

/* 销毁服务器 */
void iec61850_wrapper_server_destroy(void* server);

/* 检查服务器是否正在运行 */
bool iec61850_wrapper_server_is_running(void* server);

/* 获取当前打开的连接数 */
int iec61850_wrapper_server_get_connection_count(void* server);

/* ==================== 数据更新 API ==================== */

/* 锁定数据模型 (批量更新前调用) */
void iec61850_wrapper_lock_data_model(void* server);

/* 解锁数据模型 (批量更新后调用) */
void iec61850_wrapper_unlock_data_model(void* server);

/* 更新浮点值 */
void iec61850_wrapper_update_float(void* server, void* dataAttribute, float value);

/* 更新 int32 值 */
void iec61850_wrapper_update_int32(void* server, void* dataAttribute, int32_t value);

/* 更新 uint32 值 */
void iec61850_wrapper_update_uint32(void* server, void* dataAttribute, uint32_t value);

/* 更新布尔值 */
void iec61850_wrapper_update_bool(void* server, void* dataAttribute, bool value);

/* 更新可见字符串值 */
void iec61850_wrapper_update_string(void* server, void* dataAttribute, const char* value);

/* 更新 int64 值 */
void iec61850_wrapper_update_int64(void* server, void* dataAttribute, int64_t value);

/* 更新质量码 */
void iec61850_wrapper_update_quality(void* server, void* dataAttribute, uint16_t quality);

/* 更新时标 (毫秒时间戳) */
void iec61850_wrapper_update_timestamp(void* server, void* dataAttribute, int64_t msTimestamp);

/* ==================== 数据读取 API ==================== */

/* 获取浮点值 */
float iec61850_wrapper_get_float(void* server, void* dataAttribute);

/* 获取 int32 值 */
int32_t iec61850_wrapper_get_int32(void* server, void* dataAttribute);

/* 获取布尔值 */
bool iec61850_wrapper_get_bool(void* server, void* dataAttribute);

/* ==================== 模型查找 API ==================== */

/* 通过完整对象引用查找数据属性 (如 "LD1/MMXU1.TotW.mag.f") */
void* iec61850_wrapper_find_da(void* model, const char* objectRef);

/* 通过对象引用查找数据对象 */
void* iec61850_wrapper_find_do(void* model, const char* objectRef);

/* 获取模型节点名称 */
const char* iec61850_wrapper_get_node_name(void* node);

/* ==================== 读取访问回调 API ==================== */

/* 启用读取访问日志 (客户端读取数据时打印日志) */
void iec61850_wrapper_enable_read_log(void* server);

#endif /* IEC61850_WRAPPER_H */
