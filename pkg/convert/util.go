// Package cvt convert different data types.
package cvt

import (
	"encoding/json"
	"reflect"
	"strconv"

	"github.com/yuanyuanxiang/fss/pkg/logger"
)

var log, _ = logger.NewLogger()

func ToString(v interface{}) string {
	if v != nil {
		switch a := v.(type) {
		case []interface{}:
			if len(a) > 0 {
				log.Debugf("ToString: your input is an array \n")
				return ToString(a[0])
			}
		case []map[string]interface{}:
			if len(a) > 0 {
				log.Debugf("ToString: your input is map array \n")
				return ToString(a[0])
			}
		case map[string]interface{}:
			b, _ := json.Marshal(a)
			return string(b)
		case float64:
			return strconv.FormatFloat(a, 'f', -1, 64)
		case string:
			return a
		case bool:
			if a {
				return "true"
			}
			return "false"
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32:
			return ToString(ToNumber(v))
		case []byte:
			return string(a)
		}
		b, err := json.Marshal(v)
		if err != nil {
			log.Warnf("ToString: your input is unsupported \n")
			return ""
		}
		return string(b)
	}
	return ""
}

func ToNumber(v interface{}) float64 {
	if v != nil {
		switch a := v.(type) {
		case []interface{}:
			if len(a) > 0 {
				log.Warnf("ToNumber: your input is an array \n")
				return ToNumber(a[0])
			}
		case []map[string]interface{}:
			if len(a) > 0 {
				log.Warnf("ToNumber: your input is map array \n")
				return ToNumber(a[0])
			}
		case float64:
			return a
		case string:
			if a == "" {
				return 0
			}
			if b, err := strconv.ParseBool(a); err == nil {
				return map[bool]float64{true: 1, false: 0}[b]
			}
			if f, err := strconv.ParseFloat(a, 64); err == nil {
				return f
			}
		case bool:
			if a {
				return 1
			}
			return 0
		case uint:
			return float64(a)
		case uint8:
			return float64(a)
		case uint16:
			return float64(a)
		case uint32:
			return float64(a)
		case uint64:
			return float64(a)
		case int:
			return float64(a)
		case int8:
			return float64(a)
		case int16:
			return float64(a)
		case int32:
			return float64(a)
		case int64:
			return float64(a)
		case float32:
			return float64(a)
		}
		if reflect.TypeOf(v).Kind() == reflect.Ptr {
			ref := reflect.ValueOf(v)
			if ref.IsZero() {
				return 0
			}
			if elem := ref.Elem(); elem.CanInterface() {
				return ToNumber(elem.Interface())
			}
		}
		log.Warnf("ToString: your input is unsupported \n")
	}
	return 0
}

func ToFloat32(v interface{}) float32 {
	return float32(ToNumber(v))
}

func ToFloat64(v interface{}) float64 {
	return ToNumber(v)
}

func ToInt(v interface{}) int {
	return int(ToNumber(v))
}

func ToInt64(v interface{}) int64 {
	return int64(ToNumber(v))
}

func ToBoolean(v interface{}) bool {
	return ToNumber(v) != 0
}
