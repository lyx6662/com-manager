#include "wrapper.h"
#include "iec61850_server.h"
#include "iec61850_dynamic_model.h"
#include "iec61850_model.h"
#include "iec61850_common.h"
#include "mms_value.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

/* ==================== 读取访问回调 ==================== */

static IedServer g_read_log_server = NULL;

static void print_da_value(DataAttribute* da, FunctionalConstraint fc) {
    if (!da) return;
    const char* daName = ModelNode_getName((ModelNode*)da);
    if (!daName) return;
    DataAttributeType daType = da->type;

    if (da->fc != fc) return;

    switch (daType) {
        case IEC61850_FLOAT32: {
            float val = IedServer_getFloatAttributeValue(g_read_log_server, da);
            printf("      %s = %f (FLOAT32)\n", daName, val);
            break;
        }
        case IEC61850_INT32: {
            int32_t val = IedServer_getInt32AttributeValue(g_read_log_server, da);
            printf("      %s = %d (INT32)\n", daName, val);
            break;
        }
        case IEC61850_BOOLEAN: {
            bool val = IedServer_getBooleanAttributeValue(g_read_log_server, da);
            printf("      %s = %s (BOOLEAN)\n", daName, val ? "true" : "false");
            break;
        }
        case IEC61850_INT64: {
            int64_t val = IedServer_getInt64AttributeValue(g_read_log_server, da);
            printf("      %s = %lld (INT64)\n", daName, (long long)val);
            break;
        }
        case IEC61850_VISIBLE_STRING_255: {
            /* 直接从 MmsValue 读取字符串 */
            if (da->mmsValue) {
                const char* str = MmsValue_toString(da->mmsValue);
                printf("      %s = \"%s\" (STRING)\n", daName, str ? str : "");
            } else {
                printf("      %s = \"\" (STRING)\n", daName);
            }
            break;
        }
        default:
            printf("      %s (type=%d)\n", daName, daType);
            break;
    }
}

static void print_do_children(ModelNode* node, FunctionalConstraint fc) {
    if (!node) return;
    LinkedList children = ModelNode_getChildren(node);
    if (!children) return;
    LinkedList elem = LinkedList_getNext(children);
    while (elem) {
        ModelNode* child = (ModelNode*)LinkedList_getData(elem);
        if (child) {
            int nodeType = ModelNode_getType(child);
            if (nodeType == DataAttributeModelType) {
                print_da_value((DataAttribute*)child, fc);
            } else if (nodeType == DataObjectModelType) {
                print_do_children(child, fc);
            }
        }
        elem = LinkedList_getNext(elem);
    }
}

static MmsDataAccessError read_access_handler(
    LogicalDevice* ld, LogicalNode* ln, DataObject* dataObject,
    FunctionalConstraint fc, ClientConnection connection, void* parameter)
{
    (void)connection;
    (void)parameter;

    if (!ld || !ln || !dataObject) return DATA_ACCESS_ERROR_SUCCESS;

    const char* ldName = ModelNode_getName((ModelNode*)ld);
    const char* lnName = ModelNode_getName((ModelNode*)ln);
    const char* doName = ModelNode_getName((ModelNode*)dataObject);

    if (!ldName || !lnName || !doName) return DATA_ACCESS_ERROR_SUCCESS;

    const char* fcStr = "??";
    switch (fc) {
        case IEC61850_FC_ST: fcStr = "ST"; break;
        case IEC61850_FC_MX: fcStr = "MX"; break;
        case IEC61850_FC_SP: fcStr = "SP"; break;
        case IEC61850_FC_CF: fcStr = "CF"; break;
        case IEC61850_FC_DC: fcStr = "DC"; break;
        default: break;
    }

    printf("[IEC61850-READ] %s/%s.%s (FC=%s)\n", ldName, lnName, doName, fcStr);
    if (g_read_log_server) {
        print_do_children((ModelNode*)dataObject, fc);
    }
    fflush(stdout);

    return DATA_ACCESS_ERROR_SUCCESS;
}

void iec61850_wrapper_enable_read_log(void* server) {
    if (server) {
        g_read_log_server = (IedServer)server;
        IedServer_setReadAccessHandler((IedServer)server, read_access_handler, NULL);
    }
}

/* ==================== 模型创建 API ==================== */

void* iec61850_wrapper_model_create(const char* name) {
    return (void*)IedModel_create(name);
}

void iec61850_wrapper_model_destroy(void* model) {
    if (model) {
        IedModel_destroy((IedModel*)model);
    }
}

void* iec61850_wrapper_ld_create(const char* name, void* model) {
    if (!model || !name) return NULL;
    return (void*)LogicalDevice_create(name, (IedModel*)model);
}

void* iec61850_wrapper_ln_create(const char* name, void* ld) {
    if (!ld || !name) return NULL;
    return (void*)LogicalNode_create(name, (LogicalDevice*)ld);
}

void* iec61850_wrapper_do_create(const char* name, void* ln) {
    if (!ln || !name) return NULL;
    return (void*)DataObject_create(name, (ModelNode*)ln, 0);
}

void* iec61850_wrapper_da_create(const char* name, void* parent,
    int type, int fc, uint8_t triggerOptions) {
    if (!parent || !name) return NULL;
    return (void*)DataAttribute_create(name, (ModelNode*)parent,
        (DataAttributeType)type, (FunctionalConstraint)fc,
        triggerOptions, 0, 0);
}

void iec61850_wrapper_rcb_create(const char* name, void* ln,
    const char* rptId, bool isBuffered, const char* dataSetName,
    uint32_t confRef, uint8_t trgOps, uint8_t options,
    uint32_t bufTm, uint32_t intgPd) {
    if (!ln || !name) return;
    ReportControlBlock_create(name, (LogicalNode*)ln, rptId,
        isBuffered, dataSetName, confRef, trgOps, options, bufTm, intgPd);
}

/* ==================== 服务器生命周期 API ==================== */

void* iec61850_wrapper_server_create(void* model) {
    if (!model) return NULL;
    return (void*)IedServer_create((IedModel*)model);
}

void* iec61850_wrapper_server_create_with_config(void* model,
    int maxConnections, int reportBufferSize) {
    if (!model) return NULL;

    IedServerConfig config = IedServerConfig_create();
    if (maxConnections > 0) {
        IedServerConfig_setMaxMmsConnections(config, maxConnections);
    }
    if (reportBufferSize > 0) {
        IedServerConfig_setReportBufferSize(config, reportBufferSize);
    }

    IedServer server = IedServer_createWithConfig((IedModel*)model, NULL, config);
    IedServerConfig_destroy(config);
    return (void*)server;
}

int iec61850_wrapper_server_start(void* server, int tcpPort) {
    if (!server) return 0;
    IedServer_start((IedServer)server, tcpPort);
    return IedServer_isRunning((IedServer)server) ? 1 : 0;
}

void iec61850_wrapper_server_stop(void* server) {
    if (server) {
        IedServer_stop((IedServer)server);
    }
}

void iec61850_wrapper_server_destroy(void* server) {
    if (server) {
        IedServer_destroy((IedServer)server);
    }
}

bool iec61850_wrapper_server_is_running(void* server) {
    if (!server) return false;
    return IedServer_isRunning((IedServer)server);
}

int iec61850_wrapper_server_get_connection_count(void* server) {
    if (!server) return 0;
    return IedServer_getNumberOfOpenConnections((IedServer)server);
}

/* ==================== 数据更新 API ==================== */

void iec61850_wrapper_lock_data_model(void* server) {
    if (server) {
        IedServer_lockDataModel((IedServer)server);
    }
}

void iec61850_wrapper_unlock_data_model(void* server) {
    if (server) {
        IedServer_unlockDataModel((IedServer)server);
    }
}

void iec61850_wrapper_update_float(void* server, void* dataAttribute, float value) {
    if (server && dataAttribute) {
        IedServer_updateFloatAttributeValue((IedServer)server,
            (DataAttribute*)dataAttribute, value);
    }
}

void iec61850_wrapper_update_int32(void* server, void* dataAttribute, int32_t value) {
    if (server && dataAttribute) {
        IedServer_updateInt32AttributeValue((IedServer)server,
            (DataAttribute*)dataAttribute, value);
    }
}

void iec61850_wrapper_update_uint32(void* server, void* dataAttribute, uint32_t value) {
    if (server && dataAttribute) {
        IedServer_updateUnsignedAttributeValue((IedServer)server,
            (DataAttribute*)dataAttribute, value);
    }
}

void iec61850_wrapper_update_bool(void* server, void* dataAttribute, bool value) {
    if (server && dataAttribute) {
        IedServer_updateBooleanAttributeValue((IedServer)server,
            (DataAttribute*)dataAttribute, value);
    }
}

void iec61850_wrapper_update_string(void* server, void* dataAttribute, const char* value) {
    if (server && dataAttribute && value) {
        IedServer_updateVisibleStringAttributeValue((IedServer)server,
            (DataAttribute*)dataAttribute, (char*)value);
    }
}

void iec61850_wrapper_update_int64(void* server, void* dataAttribute, int64_t value) {
    if (server && dataAttribute) {
        IedServer_updateInt64AttributeValue((IedServer)server,
            (DataAttribute*)dataAttribute, value);
    }
}

void iec61850_wrapper_update_timestamp(void* server, void* dataAttribute, int64_t msTimestamp) {
    if (server && dataAttribute) {
        Timestamp* ts = Timestamp_create();
        if (ts) {
            Timestamp_setTimeInMilliseconds(ts, (uint64_t)msTimestamp);
            IedServer_updateTimestampAttributeValue((IedServer)server,
                (DataAttribute*)dataAttribute, ts);
            Timestamp_destroy(ts);
        }
    }
}

void iec61850_wrapper_update_quality(void* server, void* dataAttribute, uint16_t quality) {
    if (server && dataAttribute) {
        IedServer_updateQuality((IedServer)server,
            (DataAttribute*)dataAttribute, (Quality)quality);
    }
}

/* ==================== 数据读取 API ==================== */

float iec61850_wrapper_get_float(void* server, void* dataAttribute) {
    if (!server || !dataAttribute) return 0.0f;
    return IedServer_getFloatAttributeValue((IedServer)server,
        (const DataAttribute*)dataAttribute);
}

int32_t iec61850_wrapper_get_int32(void* server, void* dataAttribute) {
    if (!server || !dataAttribute) return 0;
    return IedServer_getInt32AttributeValue((IedServer)server,
        (const DataAttribute*)dataAttribute);
}

bool iec61850_wrapper_get_bool(void* server, void* dataAttribute) {
    if (!server || !dataAttribute) return false;
    return IedServer_getBooleanAttributeValue((IedServer)server,
        (const DataAttribute*)dataAttribute);
}

/* ==================== 模型查找 API ==================== */

void* iec61850_wrapper_find_da(void* model, const char* objectRef) {
    if (!model || !objectRef) return NULL;
    ModelNode* node = IedModel_getModelNodeByObjectReference((IedModel*)model, objectRef);
    if (!node) return NULL;
    if (ModelNode_getType(node) != DataAttributeModelType) return NULL;
    return (void*)node;
}

void* iec61850_wrapper_find_do(void* model, const char* objectRef) {
    if (!model || !objectRef) return NULL;
    ModelNode* node = IedModel_getModelNodeByObjectReference((IedModel*)model, objectRef);
    if (!node) return NULL;
    if (ModelNode_getType(node) != DataObjectModelType) return NULL;
    return (void*)node;
}

const char* iec61850_wrapper_get_node_name(void* node) {
    if (!node) return NULL;
    return ModelNode_getName((ModelNode*)node);
}
