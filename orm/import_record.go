package orm

import (
	"errors"
	"fmt"
	"github.com/SasukeBo/log"
	"github.com/SasukeBo/pmes-data-producer/cache"
	"github.com/jinzhu/copier"
	"github.com/jinzhu/gorm"
	"time"
)

const (
	ImportRecordTypeSystem   = "SYSTEM"
	ImportRecordTypeRealtime = "REALTIME"
	ImportRecordTypeUser     = "USER"

	ImportStatusLoading   ImportStatus = "Loading"
	ImportStatusImporting ImportStatus = "Importing"
	ImportStatusFinished  ImportStatus = "Finished"
	ImportStatusFailed    ImportStatus = "Failed"
	ImportStatusReverted  ImportStatus = "Reverted"
)

func nowDateStr() string {
	var tStr = time.Now().String()
	return tStr[:10]
}

func (i *ImportRecord) genKey(deviceID uint) string {
	return fmt.Sprintf("device_realtime_key_%v_%s", deviceID, nowDateStr())
}

type ImportStatus string

type ImportRecord struct {
	gorm.Model
	FileID             uint         `gorm:"column:file_id"'` // 关联文件的ID
	FileName           string       `gorm:"not null"`        // 文件名称
	Path               string       `gorm:"not null"`        // 存储路径
	MaterialID         uint         `gorm:"not null;index"`  // 关联料号ID
	DeviceID           uint         `gorm:"not null;index"`  // 关联设备ID
	RowCount           int          // 数据行数
	RowFinishedCount   int          // 完成行数
	RowInvalidCount    int          // 无效数据行
	Status             ImportStatus `gorm:"not null;default:false"` // 导入状态
	ErrorCode          string       // 错误码
	OriginErrorMessage string       // 原始错误信息
	FileSize           int
	UserID             uint
	ImportType         string  `gorm:"not null;default:'SYSTEM'"` // 导入方式，默认为系统
	DecodeTemplateID   uint    `gorm:"not null"`                  // 文件解析模板ID
	MaterialVersionID  uint    `gorm:"not null;default: 0"`       // 料号版本信息
	Blocked            bool    `gorm:"default:false"`             // 屏蔽导入的数据
	Yield              float64 // 单次导入记录的良率
}

// 获取实时设备的导入记录
// 生成以当前时间日期为结尾的key，通过key缓存获取数据
// 当缓存中没有该日期的实时导入记录时，从数据库获取
// 当数据库中没有该日期的实时导入记录时，创建新纪录
func (i *ImportRecord) GetDeviceRealtimeRecord(device *Device) error {
	// 生成key
	cacheKey := i.genKey(device.ID)
	log.Info("cacheKey is %s", cacheKey)

	// 获取缓存
	cacheValue := cache.Get(cacheKey)
	if cacheValue != nil {
		log.Info("cacheValue is not nil")
		record, ok := cacheValue.(ImportRecord)
		if ok {
			if err := copier.Copy(i, &record); err == nil {
				if err := cache.Set(cacheKey, *i); err != nil {
					log.Error("cache import record failed: %v", err)
				}
				log.Info("find record in cache")
				return nil
			}
		}
		log.Info("cacheValue is not ImportRecord type")
	}

	// 查询数据库 [当前设备的 实时导入的 当前日期的 正在导入的] 导入记录
	var query = "device_id = ? AND import_type = ? AND DATE(created_at) = DATE(?) AND import_records.status = ?"

	var record ImportRecord
	if err := DB.Model(&ImportRecord{}).Where(
		query, device.ID, ImportRecordTypeRealtime, time.Now(), ImportStatusImporting,
	).Find(&record).Error; err == nil {
		if err := copier.Copy(i, &record); err == nil {
			log.Info("find record in db")
			if err := cache.Set(cacheKey, *i); err != nil {
				log.Error("cache import record failed: %v", err)
			}
			return nil
		}
	}

	// 如果没有找到，更新当前版本的总数及良率统计
	{
		var lastRealtimeRecord ImportRecord
		if err := DB.Model(&ImportRecord{}).Where(
			query, device.ID, ImportRecordTypeRealtime, time.Now().AddDate(0, 0, -1), ImportStatusImporting,
		).Find(&lastRealtimeRecord).Error; err == nil {
			lastRealtimeRecord.Status = ImportStatusFinished
			_ = DB.Save(&lastRealtimeRecord)

			var version MaterialVersion
			if err := version.Get(lastRealtimeRecord.MaterialVersionID); err == nil {
				_ = version.UpdateWithRecord(&lastRealtimeRecord)
			}
		}
	}

	// 获取料号的当前版本信息
	var version MaterialVersion
	if err := version.GetActiveWithMaterialID(device.MaterialID); err != nil {
		return err
	}

	// 创建新记录
	i.MaterialID = device.MaterialID
	i.Status = ImportStatusImporting
	i.DeviceID = device.ID
	i.Path = "realtime"
	i.ImportType = ImportRecordTypeRealtime
	i.MaterialVersionID = version.ID
	if err := DB.Create(i).Error; err != nil {
		return err
	}

	if err := cache.Set(cacheKey, *i); err != nil {
		log.Error("cache import record failed: %v", err)
	}
	return nil
}

func (i *ImportRecord) Increase(tc, fc int, qualified bool) error {
	if i == nil {
		return errors.New("cannot increase nil import record")
	}
	i.RowCount = i.RowCount + tc
	ok := float64(i.RowFinishedCount) * i.Yield
	i.RowFinishedCount = i.RowFinishedCount + fc
	if qualified {
		ok = ok + float64(fc)
		i.Yield = ok / float64(i.RowFinishedCount)
	}
	cacheKey := i.genKey(i.DeviceID)
	if err := cache.Set(cacheKey, *i); err != nil {
		log.Error("cache import record failed: %v", err)
	}

	return DB.Save(i).Error
}
