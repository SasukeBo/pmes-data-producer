package orm

import "github.com/jinzhu/gorm"

// 数据文件解析模板
// 用于指定文件的必要数据位置

type DecodeTemplate struct {
	gorm.Model
	MaterialID           uint `gorm:"not null"`
	MaterialVersionID    uint `gorm:"not null"` // 料号版本ID
	UserID               uint
	DataRowIndex         int
	CreatedAtColumnIndex int  `gorm:"not null"` // 检测时间位置
	BarCodeIndex         int  // 编码读取位置
	BarCodeRuleID        uint `gorm:"COMMENT:'编码规则ID';column:bar_code_rule_id"`
	ProductColumns       Map  `gorm:"type:JSON;not null"`
}
