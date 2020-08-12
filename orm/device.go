package orm

import (
	"fmt"
	"github.com/SasukeBo/log"
	"github.com/SasukeBo/pmes-data-producer/cache"
	"github.com/jinzhu/copier"
	"github.com/jinzhu/gorm"
)

type Device struct {
	gorm.Model
	UUID           string `gorm:"column:uuid;unique_index;not null"`
	Name           string `gorm:"not null"`                                    // 用于存储用户指定的设备名称，不指定时，默认为Remark的值
	Remark         string `gorm:"not null;unique_index:uidx_name_material_id"` // 用于存储从数据文件解析出的名称
	IP             string `gorm:"column:ip;"`
	MaterialID     uint   `gorm:"column:material_id;not null;unique_index:uidx_name_material_id"` // 同一料号下的设备remark不可重复
	DeviceSupplier string
	IsRealtime     bool `gorm:"default:false;not null"`
	Address        string
}

const deviceCacheKey = "cache_device_%v_%v"

func (d *Device) GetWithToken(token string) error {
	cacheKey := fmt.Sprintf(deviceCacheKey, "token", token)
	cacheValue := cache.Get(cacheKey)
	if cacheValue != nil {
		device, ok := cacheValue.(Device)
		if ok {
			if err := copier.Copy(d, &device); err == nil {
				return nil
			}
		}
	}

	if err := DB.Model(d).Where("uuid = ?", token).First(d).Error; err != nil {
		return fmt.Errorf("device not found with token = %v: %v", token, err)
	}
	_ = cache.Set(cacheKey, *d)
	return nil
}

func (d *Device) genTemplateDecodeRuleKey() string {
	return fmt.Sprintf("device_current_version_template_rule_key_%v_%s", d.ID, nowDateStr())
}

func (d *Device) GetCurrentTemplateDecodeRule() *BarCodeRule {
	key := d.genTemplateDecodeRuleKey()
	value := cache.Get(key)
	if value != nil {
		rule, ok := value.(*BarCodeRule)
		if ok {
			_ = cache.Set(key, rule)
			return rule
		}
	}

	var template DecodeTemplate
	query := DB.Model(&DecodeTemplate{}).Joins("JOIN material_versions ON decode_templates.material_version_id = material_versions.id")
	query = query.Where("decode_templates.material_id = ? AND material_versions.active = true", d.MaterialID)
	if err := query.Find(&template).Error; err != nil {
		log.Errorln(err)
		return nil
	}

	var rule BarCodeRule
	if err := rule.Get(template.BarCodeRuleID); err != nil {
		log.Errorln(err)
		return nil
	}

	_ = cache.Set(key, &rule)
	return &rule
}
