package handler

import (
	"encoding/json"
	"github.com/SasukeBo/pmes-data-producer/orm"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

type Form struct {
	DeviceToken string `json:"device_token"`
	PointValues string `json:"point_values"`
	Attributes  string `json:"attributes"`
	Qualified   int    `json:"qualified"`
	BarCode     string `json:"bar_code"`
}

type Response struct {
	Message string `json:"message"`
	Origin  string `json:"originErr"`
}

func DeviceProduce() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, _ := ioutil.ReadAll(c.Request.Body)
		var form Form
		if err := json.Unmarshal(body, &form); err != nil {
			var response = Response{
				Message: "对不起，查询参数不合法，请检查您的输入。Sorry, the search params are illegal, please check your input.",
				Origin:  err.Error(),
			}
			c.AbortWithStatusJSON(http.StatusBadRequest, response)
			return
		}

		deviceToken := form.DeviceToken
		var device orm.Device
		if err := device.GetWithToken(deviceToken); err != nil {
			var response = Response{
				Message: "对不起，查找设备失败.",
				Origin:  err.Error(),
			}
			c.AbortWithStatusJSON(http.StatusNotFound, response)
			return
		}
		ip := c.Request.Header.Get("X-Real-IP")
		if device.IP != ip {
			device.IP = ip
			_ = orm.DB.Save(&device)
		}

		var record orm.ImportRecord
		if err := record.GetDeviceRealtimeRecord(&device); err != nil {
			var response = Response{
				Message: "获取设备实时导入记录失败.",
				Origin:  err.Error(),
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, response)
			return
		}

		qualifiedInt := form.Qualified
		var qualified bool
		if qualifiedInt == 1 {
			qualified = true
		}

		//attributesStr := form.Attributes
		var attribute orm.Map
		var statusCode = 1

		rule := device.GetCurrentTemplateDecodeRule()
		barCode := strings.TrimSpace(form.BarCode)
		if rule != nil {
			decoder := orm.NewBarCodeDecoder(rule)
			attribute, statusCode = decoder.Decode(barCode)
		} else {
			attribute = make(orm.Map)
		}

		pointValuesStr := form.PointValues
		pointValues := make(orm.Map)
		kValues := strings.Split(pointValuesStr, ";")
		for _, item := range kValues {
			sectors := strings.Split(item, ":")
			if len(sectors) < 2 {
				continue
			}
			value, err := strconv.ParseFloat(sectors[1], 64)
			if err != nil {
				value = 0
			}
			pointValues[sectors[0]] = value
		}

		var product = orm.Product{
			MaterialID:        device.MaterialID,
			DeviceID:          device.ID,
			Qualified:         qualified,
			Attribute:         attribute,
			PointValues:       pointValues,
			ImportRecordID:    record.ID,
			MaterialVersionID: record.MaterialVersionID,
			BarCode:           barCode,
			BarCodeStatus:     statusCode,
		}
		if err := orm.DB.Create(&product).Error; err != nil {
			var response = Response{
				Message: "保存产品信息失败.",
				Origin:  err.Error(),
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, response)
			return
		}
		record.Increase(1, 1, qualified)
		c.JSON(http.StatusOK, "ok")
	}
}
