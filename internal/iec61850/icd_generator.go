package iec61850

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lyx6662/com-manager/pkg/config"
)

// SCL XML 结构定义

type SCL struct {
	XMLName            xml.Name           `xml:"SCL"`
	XMLNS              string             `xml:"xmlns,attr"`
	Version            string             `xml:"version,attr"`
	Revision           string             `xml:"revision,attr"`
	Header             SCLHeader          `xml:"Header"`
	Communication      SCLCommunication   `xml:"Communication"`
	IED                SCLIed             `xml:"IED"`
	DataTypeTemplates  SCLDataTypes       `xml:"DataTypeTemplates"`
}

type SCLHeader struct {
	ID      string `xml:"id,attr"`
	Version string `xml:"version,attr"`
}

type SCLCommunication struct {
	SubNetworks []SCLSubNetwork `xml:"SubNetwork"`
}

type SCLSubNetwork struct {
	Name         string           `xml:"name,attr"`
	Type         string           `xml:"type,attr"`
	ConnectedAPs []SCLConnectedAP `xml:"ConnectedAP"`
}

type SCLConnectedAP struct {
	IEDName string    `xml:"iedName,attr"`
	APName  string    `xml:"apName,attr"`
	Address SCLAddress `xml:"Address"`
}

type SCLAddress struct {
	Parts []SCLAddressPart `xml:"P"`
}

type SCLAddressPart struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type SCLIed struct {
	Name          string         `xml:"name,attr"`
	Type          string         `xml:"type,attr"`
	Manufacturer  string         `xml:"manufacturer,attr"`
	ConfigVersion string         `xml:"configVersion,attr"`
	Services      SCLServices    `xml:"Services"`
	AccessPoints  []SCLAccessPoint `xml:"AccessPoint"`
}

type SCLServices struct {
	GetDirectory           *SCLService `xml:"GetDirectory"`
	GetDataObjectDefinition *SCLService `xml:"GetDataObjectDefinition"`
	GetDataSetValue        *SCLService `xml:"GetDataSetValue"`
	SetDataSetValue        *SCLService `xml:"SetDataSetValue"`
	DataSetDirectory       *SCLService `xml:"DataSetDirectory"`
	ReadWrite              *SCLService `xml:"ReadWrite"`
	GetCBValues            *SCLService `xml:"GetCBValues"`
	ReportSettings         *SCLReportSettings `xml:"ReportSettings"`
}

type SCLService struct{}

type SCLReportSettings struct {
	RptEnabled string `xml:"rptEnabled,attr"`
	OptFields  string `xml:"optFields,attr"`
	TrgOps     string `xml:"trgOps,attr"`
	IntgPd     string `xml:"intgPd,attr"`
}

type SCLAccessPoint struct {
	Name   string       `xml:"name,attr"`
	Server SCLServer    `xml:"Server"`
}

type SCLServer struct {
	Authentication SCLAuthentication `xml:"Authentication"`
	LDevices       []SCLLDevice      `xml:"LDevice"`
}

type SCLAuthentication struct {
	None string `xml:"none,attr"`
}

type SCLLDevice struct {
	Inst string    `xml:"inst,attr"`
	LN0  SCLLN     `xml:"LN0"`
	LNs  []SCLLN   `xml:"LN"`
}

type SCLLN struct {
	LnClass string      `xml:"lnClass,attr"`
	Inst    string      `xml:"inst,attr"`
	LnType  string      `xml:"lnType,attr"`
	DOIs    []SCLDOI    `xml:"DOI"`
}

type SCLDOI struct {
	Name  string     `xml:"name,attr"`
	DAIs  []SCLDAI   `xml:"DAI"`
	SDIs  []SCLSDI   `xml:"SDI"`
}

type SCLDAI struct {
	Name    string `xml:"name,attr"`
	Value   string `xml:"Val,omitempty"`
	ValKind string `xml:"valKind,attr,omitempty"`
}

type SCLSDI struct {
	Name string    `xml:"name,attr"`
	DAIs []SCLDAI  `xml:"DAI"`
	SDIs []SCLSDI  `xml:"SDI"`
}

// DataTypeTemplates

type SCLDataTypes struct {
	LNodeTypes []SCLLNodeType `xml:"LNodeType"`
	DOTypes    []SCLDOType    `xml:"DOType"`
	DATypes    []SCLDAType    `xml:"DAType"`
	EnumTypes  []SCLEnumType  `xml:"EnumType"`
}

type SCLLNodeType struct {
	ID      string      `xml:"id,attr"`
	LnClass string      `xml:"lnClass,attr"`
	DOs     []SCLDO     `xml:"DO"`
}

type SCLDO struct {
	Name   string `xml:"name,attr"`
	Type   string `xml:"type,attr"`
	CDC    string `xml:"cdc,attr"`
}

type SCLDOType struct {
	ID   string      `xml:"id,attr"`
	CDC  string      `xml:"cdc,attr"`
	DAs  []SCLDA     `xml:"DA"`
	SDOs []SCLSDO    `xml:"SDO"`
}

type SCLSDO struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
	CDC  string `xml:"cdc,attr"`
}

type SCLDA struct {
	Name      string `xml:"name,attr"`
	BType     string `xml:"bType,attr"`
	Type      string `xml:"type,attr,omitempty"` // 当 bType=Struct 或 bType=Enum 时引用类型
	FC        string `xml:"fc,attr"`
	Dchg      string `xml:"dchg,attr,omitempty"`
	Qchg      string `xml:"qchg,attr,omitempty"`
	Dupd      string `xml:"dupd,attr,omitempty"`
}

type SCLDAType struct {
	ID  string       `xml:"id,attr"`
	DAs []SCLDATypeDA `xml:"BDA"`
}

type SCLDATypeDA struct {
	Name  string `xml:"name,attr"`
	BType string `xml:"bType,attr"`
	Type  string `xml:"type,attr,omitempty"` // 当 bType=Struct 或 bType=Enum 时引用类型
}

type SCLEnumType struct {
	ID    string        `xml:"id,attr"`
	Items []SCLEnumItem `xml:"EnumVal"`
}

type SCLEnumItem struct {
	Ordinal int    `xml:"ord,attr"`
	Value   string `xml:",chardata"`
}

// typeCounter 用于生成唯一的类型 ID
type typeCounter struct {
	lnTypes map[string]bool
	doTypes map[string]bool
	daTypes map[string]bool
	lnIdx   int
	doIdx   int
	daIdx   int
}

func newTypeCounter() *typeCounter {
	return &typeCounter{
		lnTypes: make(map[string]bool),
		doTypes: make(map[string]bool),
		daTypes: make(map[string]bool),
	}
}

// GenerateICD 从配置生成 ICD 文件
func GenerateICD(cfg *config.ModbusToIEC61850Config, outputPath string) error {
	if cfg == nil {
		return fmt.Errorf("配置为空")
	}

	iedName := cfg.IEC61850.IEDName
	if iedName == "" {
		iedName = "GW"
	}

	tc := newTypeCounter()

	// 构建 DataTypeTemplates
	dataTypes := buildDataTypeTemplates(iedName, &cfg.Model, tc)

	// 构建 IED 部分
	ied := buildIED(iedName, &cfg.Model)

	// 构建完整 SCL
	scl := SCL{
		XMLNS:    "http://www.iec.ch/61850/2003/SCL",
		Version:  "2007",
		Revision: "B",
		Header: SCLHeader{
			ID:      iedName,
			Version: "1.0",
		},
		Communication: SCLCommunication{
			SubNetworks: []SCLSubNetwork{
				{
					Name: "Ethernet",
					Type: "8-MMS",
					ConnectedAPs: []SCLConnectedAP{
						{
							IEDName: iedName,
							APName:  "S1",
							Address: SCLAddress{
								Parts: []SCLAddressPart{
									{Type: "IP", Value: "127.0.0.1"},
									{Type: "IP-SUBNET", Value: "255.255.255.0"},
									{Type: "IP-GATEWAY", Value: "127.0.0.1"},
									{Type: "OSI-TSEL", Value: "0001"},
									{Type: "OSI-PSEL", Value: "00000001"},
									{Type: "OSI-SSEL", Value: "0001"},
								},
							},
						},
					},
				},
			},
		},
		IED:               ied,
		DataTypeTemplates: dataTypes,
	}

	// 输出 XML
	output, err := xml.MarshalIndent(scl, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 XML 失败: %w", err)
	}

	// 写入文件 (带 XML 声明)
	xmlContent := xml.Header + string(output)

	// 确保目录存在
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(xmlContent), 0644); err != nil {
		return fmt.Errorf("写入 ICD 文件失败: %w", err)
	}

	return nil
}

// buildIED 构建 IED 节点
func buildIED(iedName string, model *config.IEC61850ModelConfig) SCLIed {
	var lDevices []SCLLDevice

	for _, ld := range model.LogicalDevices {
		lDevice := SCLLDevice{
			Inst: ld.Name,
			LN0: SCLLN{
				LnClass: "LLN0",
				Inst:    "",
				LnType:  iedName + ld.Name + "_LLN0",
			},
		}

		for _, ln := range ld.LogicalNodes {
			sclLN := SCLLN{
				LnClass: extractLNClass(ln.Name),
				Inst:    extractLNInst(ln.Name),
				LnType:  iedName + ld.Name + "_" + ln.Name,
			}
			lDevice.LNs = append(lDevice.LNs, sclLN)
		}

		lDevices = append(lDevices, lDevice)
	}

	return SCLIed{
		Name:          iedName,
		Type:          "com-manager",
		Manufacturer:  "com-manager",
		ConfigVersion: "1.0",
		Services: SCLServices{
			GetDirectory:            &SCLService{},
			GetDataObjectDefinition: &SCLService{},
			GetDataSetValue:         &SCLService{},
			SetDataSetValue:         &SCLService{},
			DataSetDirectory:        &SCLService{},
			ReadWrite:               &SCLService{},
			GetCBValues:             &SCLService{},
			ReportSettings: &SCLReportSettings{
				RptEnabled: "Dyn",
				OptFields:  "Dyn",
				TrgOps:     "Dyn",
				IntgPd:     "Dyn",
			},
		},
		AccessPoints: []SCLAccessPoint{
			{
				Name: "S1",
				Server: SCLServer{
					Authentication: SCLAuthentication{None: "true"},
					LDevices:       lDevices,
				},
			},
		},
	}
}

// buildDataTypeTemplates 构建数据类型模板
func buildDataTypeTemplates(iedName string, model *config.IEC61850ModelConfig, tc *typeCounter) SCLDataTypes {
	var lnTypes []SCLLNodeType
	var doTypes []SCLDOType
	var daTypes []SCLDAType
	var enumTypes []SCLEnumType

	// 添加通用 DA 类型 (AnalogueValue, Quality, Timestamp)
	daTypes = append(daTypes, buildCommonDATypes()...)
	// 添加通用 EnumType (Quality, TimeAccuracy)
	enumTypes = append(enumTypes, buildCommonEnumTypes()...)

	for _, ld := range model.LogicalDevices {
		// 为每个 LD 添加 LLN0 类型 (ID 需与 LDevice.LN0.lnType 一致)
		lln0Type := SCLLNodeType{
			ID:      iedName + ld.Name + "_LLN0",
			LnClass: "LLN0",
			DOs: []SCLDO{
				{Name: "Mod", Type: "INC_Mod", CDC: "INC"},
				{Name: "Beh", Type: "INS_Beh", CDC: "INS"},
				{Name: "Health", Type: "INS_Health", CDC: "INS"},
				{Name: "NamPlt", Type: "LPL_NamPlt", CDC: "LPL"},
			},
		}
		lnTypes = append(lnTypes, lln0Type)

		for _, ln := range ld.LogicalNodes {
			lnType := SCLLNodeType{
				ID:      iedName + ld.Name + "_" + ln.Name,
				LnClass: extractLNClass(ln.Name),
			}

			for _, doCfg := range ln.DataObjects {
				doID := "DO_" + ld.Name + "_" + ln.Name + "_" + doCfg.Name
				doType, subDOTypes, subDATypes := buildDOType(doCfg, doID, tc)
				doTypes = append(doTypes, doType)
				doTypes = append(doTypes, subDOTypes...)
				daTypes = append(daTypes, subDATypes...)

				lnType.DOs = append(lnType.DOs, SCLDO{
					Name: doCfg.Name,
					Type: doID,
					CDC:  getCDC(doCfg),
				})
			}

			lnTypes = append(lnTypes, lnType)
		}
	}

	// 添加通用 LNodeType、DOType
	lnTypes = append(lnTypes, buildCommonLNodeTypes()...)
	doTypes = append(doTypes, buildCommonDOTypes()...)

	return SCLDataTypes{
		LNodeTypes: lnTypes,
		DOTypes:    doTypes,
		DATypes:    daTypes,
		EnumTypes:  enumTypes,
	}
}

// buildDOType 为 DataObject 构建 DOType
func buildDOType(doCfg config.DataObjectConfig, doID string, tc *typeCounter) (SCLDOType, []SCLDOType, []SCLDAType) {
	doType := SCLDOType{
		ID:  doID,
		CDC: getCDC(doCfg),
	}

	var subDOTypes []SCLDOType
	var subDATypes []SCLDAType

	for _, daCfg := range doCfg.DataAttributes {
		if len(daCfg.Children) > 0 {
			// 有子属性 → SDO (子数据对象)
			subDOID := doID + "_" + daCfg.Name
			subDO := SCLDOType{
				ID:  subDOID,
				CDC: "CMV", // 子对象默认为 CMV (如 mag)
			}
			for _, child := range daCfg.Children {
				daType := buildDAType(child, subDOID, tc)
				subDO.DAs = append(subDO.DAs, daType)
				subDATypes = append(subDATypes)
			}
			doType.SDOs = append(doType.SDOs, SCLSDO{
				Name: daCfg.Name,
				Type: subDOID,
				CDC:  "CMV",
			})
			subDOTypes = append(subDOTypes, subDO)
		} else {
			// 叶子节点 → DA
			da := buildDA(daCfg)
			doType.DAs = append(doType.DAs, da)
		}
	}

	return doType, subDOTypes, subDATypes
}

// buildDA 构建单个 DA
func buildDA(daCfg config.DataAttributeConfig) SCLDA {
	da := SCLDA{
		Name: daCfg.Name,
		FC:   daCfg.FC,
	}

	// 根据类型设置 bType 属性 (SCL 标准要求)
	switch strings.ToUpper(daCfg.Type) {
	case "FLOAT32":
		da.BType = "FLOAT32"
	case "FLOAT64":
		da.BType = "FLOAT64"
	case "INT32":
		da.BType = "INT32"
	case "INT64":
		da.BType = "INT64"
	case "INT16":
		da.BType = "INT16"
	case "INT8":
		da.BType = "INT8"
	case "BOOLEAN":
		da.BType = "BOOLEAN"
	case "VISIBLE_STRING_255", "STRING":
		da.BType = "VisString255"
	default:
		da.BType = "FLOAT32"
	}

	// 设置触发选项
	triggers := strings.ToUpper(daCfg.Triggers)
	if strings.Contains(triggers, "DATA_CHANGED") {
		da.Dchg = "true"
	}
	if strings.Contains(triggers, "QUALITY_CHANGED") {
		da.Qchg = "true"
	}
	if strings.Contains(triggers, "DATA_UPDATE") {
		da.Dupd = "true"
	}

	return da
}

// buildDAType 构建子属性的 DA (如 mag.f)
func buildDAType(daCfg config.DataAttributeConfig, parentID string, tc *typeCounter) SCLDA {
	return buildDA(daCfg)
}

// getCDC 根据 DataObject 配置推断 CDC (Common Data Class)
func getCDC(doCfg config.DataObjectConfig) string {
	// 根据名称和子属性推断
	for _, da := range doCfg.DataAttributes {
		if len(da.Children) > 0 {
			// 有子属性 (如 mag) → CMV (Complex Measured Value)
			return "CMV"
		}
	}
	// 根据 FC 推断
	for _, da := range doCfg.DataAttributes {
		switch da.FC {
		case "MX":
			return "MV"
		case "ST":
			return "INS"
		case "SP":
			return "ING"
		case "CF":
			return "ING"
		}
	}
	return "ENS"
}

// extractLNClass 从 LN 名称提取 lnClass (如 "MMXU1" → "MMXU")
func extractLNClass(name string) string {
	// 去掉末尾的数字
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] < '0' || name[i] > '9' {
			return name[:i+1]
		}
	}
	return name
}

// extractLNInst 从 LN 名称提取 inst (如 "MMXU1" → "1")
func extractLNInst(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] < '0' || name[i] > '9' {
			return name[i+1:]
		}
	}
	return ""
}

// buildCommonDATypes 构建通用 DA 类型
func buildCommonDATypes() []SCLDAType {
	return []SCLDAType{
		{
			ID: "AnalogueValue",
			DAs: []SCLDATypeDA{
				{Name: "f", BType: "FLOAT32"},
			},
		},
		{
			ID: "Vector",
			DAs: []SCLDATypeDA{
				{Name: "mag", BType: "Struct", Type: "AnalogueValue"},
				{Name: "ang", BType: "Struct", Type: "AnalogueValue"},
			},
		},
		{
			ID: "Quality",
			DAs: []SCLDATypeDA{
				{Name: "validity", BType: "Enum", Type: "QualityValidity"},
			},
		},
		{
			ID: "Timestamp",
			DAs: []SCLDATypeDA{
				{Name: "SecondSinceEpoch", BType: "INT32"},
				{Name: "FractionOfSecond", BType: "INT32"},
				{Name: "TimeQuality", BType: "Struct", Type: "TimeQuality"},
			},
		},
		{
			ID: "TimeQuality",
			DAs: []SCLDATypeDA{
				{Name: "clockNotSync", BType: "BOOLEAN"},
				{Name: "leapSecondsKnown", BType: "BOOLEAN"},
				{Name: "timeAccuracy", BType: "Enum", Type: "TimeAccuracy"},
			},
		},
		{
			ID: "Originator",
			DAs: []SCLDATypeDA{
				{Name: "orCat", BType: "Enum", Type: "OrCat"},
				{Name: "orIdent", BType: "Octet64"},
			},
		},
	}
}

// buildCommonEnumTypes 构建通用枚举类型
func buildCommonEnumTypes() []SCLEnumType {
	return []SCLEnumType{
		{
			ID: "QualityValidity",
			Items: []SCLEnumItem{
				{Ordinal: 0, Value: "good"},
				{Ordinal: 1, Value: "invalid"},
				{Ordinal: 2, Value: "reserved"},
				{Ordinal: 3, Value: "questionable"},
			},
		},
		{
			ID: "TimeAccuracy",
			Items: []SCLEnumItem{
				{Ordinal: 0, Value: "unspecified"},
				{Ordinal: 7, Value: "T0"},
				{Ordinal: 10, Value: "T1"},
				{Ordinal: 14, Value: "T2"},
				{Ordinal: 16, Value: "T3"},
				{Ordinal: 18, Value: "T4"},
				{Ordinal: 20, Value: "T5"},
			},
		},
		{
			ID: "OrCat",
			Items: []SCLEnumItem{
				{Ordinal: 0, Value: "not-supported"},
				{Ordinal: 1, Value: "bay-control"},
				{Ordinal: 2, Value: "station-control"},
				{Ordinal: 3, Value: "remote-control"},
				{Ordinal: 4, Value: "automatic-bay"},
				{Ordinal: 5, Value: "automatic-station"},
				{Ordinal: 6, Value: "automatic-remote"},
				{Ordinal: 7, Value: "maintenance"},
				{Ordinal: 8, Value: "process"},
			},
		},
	}
}

// buildCommonLNodeTypes 构建通用 LNodeType
func buildCommonLNodeTypes() []SCLLNodeType {
	return []SCLLNodeType{
		{
			ID:      "INC_Mod",
			LnClass: "",
			DOs: []SCLDO{
				{Name: "stVal", Type: "INT32", CDC: "INS"},
				{Name: "q", Type: "Quality", CDC: "Quality"},
				{Name: "t", Type: "Timestamp", CDC: "Timestamp"},
				{Name: "ctlModel", Type: "INT32", CDC: "INS"},
			},
		},
		{
			ID:      "INS_Beh",
			LnClass: "",
			DOs: []SCLDO{
				{Name: "stVal", Type: "INT32", CDC: "INS"},
				{Name: "q", Type: "Quality", CDC: "Quality"},
				{Name: "t", Type: "Timestamp", CDC: "Timestamp"},
			},
		},
		{
			ID:      "INS_Health",
			LnClass: "",
			DOs: []SCLDO{
				{Name: "stVal", Type: "INT32", CDC: "INS"},
				{Name: "q", Type: "Quality", CDC: "Quality"},
				{Name: "t", Type: "Timestamp", CDC: "Timestamp"},
			},
		},
		{
			ID:      "LPL_NamPlt",
			LnClass: "",
			DOs: []SCLDO{
				{Name: "vendor", Type: "VisString255", CDC: "LPL"},
				{Name: "swRev", Type: "VisString255", CDC: "LPL"},
				{Name: "d", Type: "VisString255", CDC: "LPL"},
				{Name: "configRev", Type: "VisString255", CDC: "LPL"},
				{Name: "lnNs", Type: "VisString255", CDC: "LPL"},
			},
		},
	}
}

// buildCommonDOTypes 构建通用 DOType
func buildCommonDOTypes() []SCLDOType {
	return []SCLDOType{
		{
			ID:  "INC_Mod",
			CDC: "INC",
			DAs: []SCLDA{
				{Name: "stVal", BType: "INT32", FC: "ST", Dchg: "true"},
				{Name: "q", BType: "Struct", Type: "Quality", FC: "ST", Qchg: "true"},
				{Name: "t", BType: "Struct", Type: "Timestamp", FC: "ST"},
				{Name: "ctlModel", BType: "INT32", FC: "CF"},
			},
		},
		{
			ID:  "INS_Beh",
			CDC: "INS",
			DAs: []SCLDA{
				{Name: "stVal", BType: "INT32", FC: "ST", Dchg: "true"},
				{Name: "q", BType: "Struct", Type: "Quality", FC: "ST", Qchg: "true"},
				{Name: "t", BType: "Struct", Type: "Timestamp", FC: "ST"},
			},
		},
		{
			ID:  "INS_Health",
			CDC: "INS",
			DAs: []SCLDA{
				{Name: "stVal", BType: "INT32", FC: "ST", Dchg: "true"},
				{Name: "q", BType: "Struct", Type: "Quality", FC: "ST", Qchg: "true"},
				{Name: "t", BType: "Struct", Type: "Timestamp", FC: "ST"},
			},
		},
		{
			ID:  "LPL_NamPlt",
			CDC: "LPL",
			DAs: []SCLDA{
				{Name: "vendor", BType: "VisString255", FC: "DC"},
				{Name: "swRev", BType: "VisString255", FC: "DC"},
				{Name: "d", BType: "VisString255", FC: "DC"},
				{Name: "configRev", BType: "VisString255", FC: "DC"},
				{Name: "lnNs", BType: "VisString255", FC: "DC"},
			},
		},
	}
}
