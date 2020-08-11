package orm

import (
	"errors"
	"fmt"
	"github.com/SasukeBo/log"
	"github.com/jinzhu/gorm"
	"strconv"
	"strings"
	"time"
)

type BarCodeRule struct {
	gorm.Model
	CodeLength int    `gorm:"COMMENT:'编码长度';not null"`
	Name       string `gorm:"COMMENT:'编码规则名称';not null;unique_index"` // 编码规则名称
	Remark     string `gorm:"COMMENT:'编码规则描述';not null"`              // 规则描述
	UserID     uint   `gorm:"COMMENT:'编码规则创建人'"`                      // 创建人ID
	Items      Map    `gorm:"COMMENT:'解析项配置';type:JSON;not null"`     // 存储解析规则
}

const (
	BarCodeItemTypeCategory = "Category"
	BarCodeItemTypeDatetime = "Datetime"
	BarCodeItemTypeWeekday  = "Weekday"
)

// BarCodeItem 二维码识别规则对象
// - IndexRange 表示识别码索引范围，为整型数组，大于等于两位时，取前两位索引范围的字符，一位时，取该位索引为字符，0位时忽略该规则。
// - Type 值类型，主要分两类Category和Date，前者一律处理为字符串，后者解析为日期
// - 当Type=Date时，有以下字段
// - DayCode - 日期编码，为字符串数组，长度应该大于等于2，前两位表示编码起始字符，按照1-9 A-Z的顺序，从第三个元素开始为剔除字符，即从编码起始
//   字符中剔除这些字符。当位数小于2时，视为无DayCode，则对应日期按照检测时间的日期补全。[1, Y, B, I, O] 表示从1到Y，去除B，I，O。
// - MonthCode - 月份编码，字符串数组，规则同DayCode。 [1, D, A] 表示从1到D，去除A。
type BarCodeItem struct {
	Label           string   `json:"label"`             // 解析项的名称，例如：冲压日期
	Key             string   `json:"key"`               // 解析项的英文标识，例如：ProduceDate
	IndexRange      []int    `json:"index_range"`       // 解析码索引区间，例如：[21,22]
	Type            string   `json:"type"`              // 解析项类型，例如：Datetime
	DayCode         []string `json:"day_code"`          // 日码区间
	DayCodeReject   []string `json:"day_code_reject"`   // 日码区间剔除字段
	MonthCode       []string `json:"month_code"`        // 月码区间
	MonthCodeReject []string `json:"month_code_reject"` // 月码区间剔除字段
	CategorySet     []string `json:"category_set"`      // 类别取值区间
}

func (r *BarCodeRule) Get(id uint) error {
	if err := DB.Model(r).Where("id = ?", id).First(r).Error; err != nil {
		return fmt.Errorf("get bar_code_rule with id = %v failed: %v", id, err)
	}

	return nil
}

const (
	BarCodeStatusSuccess  = 1 + iota
	BarCodeStatusIllegal  // 条码值非法
	BarCodeStatusReadFail // 条码读取错误
	BarCodeStatusTooShort // 条码长度错误
	BarCodeStatusNoRule   // 条码规则无解析项
)

type BarCodeDecoder struct {
	Rules       []BarCodeItem
	BarCodeRule *BarCodeRule
}

// Decode 解析识别码，返回解析结果对象 及 状态码
// 状态码：
// - 1 正确识别
// - 2 识别码不符合编码规则
// - 3 识别码读取失败，为空字符串或ERR
// - 4 识别码长度不正确
func (bdc *BarCodeDecoder) Decode(code string) (out Map, statusCode int) {
	out = make(Map)
	if code == "" || strings.ToUpper(code) == "ERR" {
		statusCode = BarCodeStatusReadFail
		return
	}
	if len(code) != bdc.BarCodeRule.CodeLength {
		statusCode = BarCodeStatusTooShort
		return
	}

	for _, rule := range bdc.Rules {
		var begin, end int
		if len(rule.IndexRange) > 0 {
			begin = rule.IndexRange[0]
		}
		if len(rule.IndexRange) > 1 {
			end = rule.IndexRange[1]
		}
		var childStr string
		if end != 0 {
			childStr = code[begin-1 : end]
		} else {
			childStr = string(code[begin-1])
		}

		// 如果条码段包含*号，表示补位，跳过此解析项
		if strings.Contains(childStr, "*") {
			continue
		}

		switch rule.Type {
		case BarCodeItemTypeCategory:
			if len(rule.CategorySet) > 0 {
				var match = false
				for _, set := range rule.CategorySet {
					if set == childStr {
						out[rule.Key] = childStr
						match = true
						break
					}
				}
				if !match {
					statusCode = BarCodeStatusIllegal
					return
				}
			} else {
				out[rule.Key] = childStr
			}
		case BarCodeItemTypeDatetime:
			timeCode := childStr
			var t *time.Time
			var err error

			if len(timeCode) > 1 {
				t, err = parseCodeDatetime(timeCode[:1], timeCode[1:2], rule)
			} else if len(timeCode) > 0 {
				t, err = parseCodeDatetime("", timeCode, rule)
			}

			if err != nil {
				statusCode = BarCodeStatusIllegal
				return
			}

			if t == nil {
				out[rule.Key] = time.Now()
			} else {
				out[rule.Key] = *t
			}
		case BarCodeItemTypeWeekday:
			weekCode := childStr
			var t *time.Time
			var err error

			if len(weekCode) != 3 {
				statusCode = BarCodeStatusIllegal
				return
			}

			week, err := strconv.ParseInt(weekCode[:2], 10, 64)
			if err != nil {
				statusCode = BarCodeStatusIllegal
				return
			}
			weekDay, err := strconv.ParseInt(weekCode[2:], 10, 64)
			t = parseTimeFromWeekday(int(week), int(weekDay-1))
			out[rule.Key] = *t
		}
	}

	statusCode = BarCodeStatusSuccess
	return
}

func parseCodeDatetime(monthCode, dayCode string, rule BarCodeItem) (*time.Time, error) {
	var month, day int
	var err error

	if monthCode != "" && len(rule.MonthCode) > 1 {
		month, err = parseIndexInCodeRange(monthCode, rule.MonthCode[0], rule.MonthCode[1], rule.MonthCodeReject...)
		if err != nil {
			log.Errorln(err)
			return nil, err
		}
		if month > 12 {
			err = errors.New(fmt.Sprintf("month out range of 1 - 12, got %v", month))
			log.Errorln(err)
			return nil, err
		}
	}

	if dayCode != "" && len(rule.DayCode) > 1 {
		day, err = parseIndexInCodeRange(dayCode, rule.DayCode[0], rule.DayCode[1], rule.DayCodeReject...)
		if err != nil {
			log.Errorln(err)
			return nil, err
		}
		if day > 31 {
			err = errors.New(fmt.Sprintf("month out range of 1 - 31, got %v", day))
			log.Errorln(err)
			return nil, err
		}
	}

	now := time.Now()
	if month == 0 {
		month = int(now.Month())
	}
	if day == 0 {
		day = now.Day()
	}

	t := time.Date(now.Year(), time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return &t, nil
}

func parseIndexInCodeRange(code, begin, end string, rejects ...string) (int, error) {
	ascii := code[0]
	var distance uint8
	if ascii >= begin[0] && ascii <= end[0] {
		distance = ascii - begin[0]
		for _, r := range rejects {
			if r[0] == ascii {
				return 0, errors.New("cannot parse rejected code")
			}
			if r[0] < ascii && r[0] >= begin[0] {
				distance--
			}
		}
		if ascii >= uint8('A') {
			distance = distance - 7
		}
	} else {
		return 0, errors.New("code is out range")
	}

	return int(distance + 1), nil
}

func parseTimeFromWeekday(week, day int) *time.Time {
	now := time.Now()
	t := time.Date(now.Year(), time.January, 7*(week-1), 0, 0, 0, 0, time.UTC)
	weekDay := t.Weekday()
	distance := day - int(weekDay)
	nt := t.AddDate(0, 0, distance)
	return &nt
}

func NewBarCodeDecoder(rule *BarCodeRule) *BarCodeDecoder {
	var decoder BarCodeDecoder
	itemsMapValue, ok := rule.Items["items"]
	if !ok {
		return nil
	}
	items, ok := itemsMapValue.([]interface{})
	if !ok {
		return nil
	}
	var rules []BarCodeItem
	for _, v := range items {
		item, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		out := DecodeBarCodeItemFromDBToStruct(item)
		rules = append(rules, out)
	}

	decoder.Rules = rules
	decoder.BarCodeRule = rule
	return &decoder
}

func DecodeBarCodeItemFromDBToStruct(item map[string]interface{}) BarCodeItem {
	var outItem BarCodeItem
	outItem.Label = fmt.Sprint(item["label"])
	outItem.Type = fmt.Sprint(item["type"])
	outItem.Key = fmt.Sprint(item["key"])
	if codes, ok := item["day_code"].([]interface{}); ok {
		var dayCode []string
		for _, code := range codes {
			dayCode = append(dayCode, fmt.Sprint(code))
		}
		outItem.DayCode = dayCode
	}
	if codes, ok := item["day_code_reject"].([]interface{}); ok {
		var dayCodeReject []string
		for _, code := range codes {
			dayCodeReject = append(dayCodeReject, fmt.Sprint(code))
		}
		outItem.DayCodeReject = dayCodeReject
	}
	if codes, ok := item["month_code"].([]interface{}); ok {
		var monthCode []string
		for _, code := range codes {
			monthCode = append(monthCode, fmt.Sprint(code))
		}
		outItem.MonthCode = monthCode
	}
	if codes, ok := item["month_code_reject"].([]interface{}); ok {
		var monthCodeReject []string
		for _, code := range codes {
			monthCodeReject = append(monthCodeReject, fmt.Sprint(code))
		}
		outItem.MonthCodeReject = monthCodeReject
	}
	if codes, ok := item["index_range"].([]interface{}); ok {
		var indexRange []int
		for _, c := range codes {
			code, err := strconv.Atoi(fmt.Sprint(c))
			if err != nil {
				code = 0
			}
			indexRange = append(indexRange, code)
		}
		outItem.IndexRange = indexRange
	}
	if codes, ok := item["category_set"].([]interface{}); ok {
		var set []string
		for _, code := range codes {
			set = append(set, fmt.Sprint(code))
		}
		outItem.CategorySet = set
	}

	return outItem
}
