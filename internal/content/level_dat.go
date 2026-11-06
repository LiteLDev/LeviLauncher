package content

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/liteldev/LeviLauncher/internal/nbt"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

func toInt64(v any) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int8:
		return int64(t)
	case uint8:
		return int64(t)
	case int16:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case float32:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n
	default:
		return 0
	}
}

func toFloat64(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int8:
		return float64(t)
	case uint8:
		return float64(t)
	case int16:
		return float64(t)
	case int32:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	default:
		return 0
	}
}

func coerceNumberTo(old any, v any) any {
	switch old.(type) {
	case int8:
		return int8(toInt64(v))
	case uint8:
		return uint8(toInt64(v))
	case int16:
		return int16(toInt64(v))
	case int32:
		return int32(toInt64(v))
	case int64:
		return toInt64(v)
	case float32:
		return float32(toFloat64(v))
	case float64:
		return toFloat64(v)
	default:
		return v
	}
}

func coerceSliceToElemType(arr []any, oldElemKind reflect.Kind) any {
	switch oldElemKind {
	case reflect.Int8:
		tmp := make([]int8, 0, len(arr))
		for _, e := range arr {
			tmp = append(tmp, int8(toInt64(e)))
		}
		return tmp
	case reflect.Uint8:
		tmp := make([]uint8, 0, len(arr))
		for _, e := range arr {
			tmp = append(tmp, uint8(toInt64(e)))
		}
		return tmp
	case reflect.Int16:
		tmp := make([]int16, 0, len(arr))
		for _, e := range arr {
			tmp = append(tmp, int16(toInt64(e)))
		}
		return tmp
	case reflect.Int32:
		tmp := make([]int32, 0, len(arr))
		for _, e := range arr {
			tmp = append(tmp, int32(toInt64(e)))
		}
		return tmp
	case reflect.Int64:
		tmp := make([]int64, 0, len(arr))
		for _, e := range arr {
			tmp = append(tmp, toInt64(e))
		}
		return tmp
	case reflect.Float32:
		tmp := make([]float32, 0, len(arr))
		for _, e := range arr {
			tmp = append(tmp, float32(toFloat64(e)))
		}
		return tmp
	case reflect.Float64:
		tmp := make([]float64, 0, len(arr))
		for _, e := range arr {
			tmp = append(tmp, toFloat64(e))
		}
		return tmp
	case reflect.String:
		tmp := make([]string, 0, len(arr))
		for _, e := range arr {
			tmp = append(tmp, fmt.Sprintf("%v", e))
		}
		return tmp
	default:
		return arr
	}
}

func coerceArrayFromSlice(arr []any, elemKind reflect.Kind) any {
	n := len(arr)
	switch elemKind {
	case reflect.Int32:
		a := reflect.New(reflect.ArrayOf(n, reflect.TypeOf(int32(0)))).Elem()
		for i := 0; i < n; i++ {
			a.Index(i).SetInt(int64(int32(toInt64(arr[i]))))
		}
		return a.Interface()
	case reflect.Int64:
		a := reflect.New(reflect.ArrayOf(n, reflect.TypeOf(int64(0)))).Elem()
		for i := 0; i < n; i++ {
			a.Index(i).SetInt(toInt64(arr[i]))
		}
		return a.Interface()
	case reflect.Uint8:
		a := reflect.New(reflect.ArrayOf(n, reflect.TypeOf(uint8(0)))).Elem()
		for i := 0; i < n; i++ {
			a.Index(i).SetUint(uint64(uint8(toInt64(arr[i]))))
		}
		return a.Interface()
	default:
		return arr
	}
}

func coerceMapTo(old map[string]any, m map[string]any) map[string]any {
	for k, v := range m {
		if ov, ok := old[k]; ok && ov != nil {
			switch value := v.(type) {
			case float64, float32, int, int8, uint8, int16, int32, int64, string:
				m[k] = coerceNumberTo(ov, value)
			case []any:
				m[k] = coerceLevelDatList(ov, value)
			case map[string]any:
				if om, ok := ov.(map[string]any); ok {
					m[k] = coerceMapTo(om, value)
				}
			}
		}
	}
	return m
}

func GetLevelDat(worldDir string) ([]byte, error) {
	if strings.TrimSpace(worldDir) == "" {
		return nil, os.ErrNotExist
	}
	p := filepath.Join(worldDir, "level.dat")
	return os.ReadFile(p)
}

func PutLevelDat(worldDir string, levelDatData []byte) error {
	if strings.TrimSpace(worldDir) == "" {
		return os.ErrNotExist
	}
	p := filepath.Join(worldDir, "level.dat")
	backup := filepath.Join(worldDir, "level.dat_leviold")
	if !utils.FileExists(backup) {
		if b, err := os.ReadFile(p); err == nil {
			_ = os.WriteFile(backup, b, 0644)
		}
	}
	return os.WriteFile(p, levelDatData, 0644)
}

func GetLevelDatNbtAndVersion(worldDir string) ([]byte, int32, error) {
	b, err := GetLevelDat(worldDir)
	if err != nil {
		return nil, 0, err
	}
	if len(b) < 8 {
		return nil, 0, io.ErrUnexpectedEOF
	}
	return b[8:], int32(binary.LittleEndian.Uint32(b[:4])), nil
}

func PutLevelDatNbtAndVersion(worldDir string, levelDatData []byte, version int32) error {
	data := make([]byte, 8+len(levelDatData))
	binary.LittleEndian.PutUint32(data[:4], uint32(version))
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(levelDatData)))
	copy(data[8:], levelDatData)
	return PutLevelDat(worldDir, data)
}

func DecodeLevelDat(worldDir string) (map[string]any, int32, error) {
	nbtData, version, err := GetLevelDatNbtAndVersion(worldDir)
	if err != nil {
		return nil, 0, err
	}
	var root map[string]any
	if err = nbt.UnmarshalEncoding(nbtData, &root, nbt.LittleEndian); err != nil {
		return nil, 0, err
	}
	return root, version, nil
}

func EncodeLevelDat(worldDir string, version int32, root map[string]any) error {
	data, err := nbt.MarshalEncoding(root, nbt.LittleEndian)
	if err != nil {
		return err
	}
	return PutLevelDatNbtAndVersion(worldDir, data, version)
}

func ReadLevelDatFields(worldDir string) ([]types.LevelDatField, int32, error) {
	return readLevelDatFields(worldDir, nil, true)
}

func ReadLevelDatFieldsAt(worldDir string, path []string) ([]types.LevelDatField, int32, error) {
	return readLevelDatFields(worldDir, path, false)
}

func readLevelDatFields(worldDir string, path []string, includeCompoundValues bool) ([]types.LevelDatField, int32, error) {
	root, ver, err := DecodeLevelDat(worldDir)
	if err != nil {
		return nil, 0, err
	}
	cur, inData := root["Data"].(map[string]any)
	if !inData {
		cur = root
	}
	for _, seg := range path {
		if m, ok := cur[seg].(map[string]any); ok {
			cur = m
		} else {
			return nil, ver, fmt.Errorf("path not found: %s", seg)
		}
	}
	out := make([]types.LevelDatField, 0, len(cur))
	for k, v := range cur {
		f := types.LevelDatField{Name: k, InData: inData, Path: append([]string{}, path...)}
		switch tv := v.(type) {
		case int8:
			f.Tag = "byte"
			f.ValueString = fmt.Sprintf("%d", tv)
			f.IsBoolLike = tv == 0 || tv == 1
		case uint8:
			f.Tag = "byte"
			f.ValueString = fmt.Sprintf("%d", tv)
			f.IsBoolLike = tv == 0 || tv == 1
		case int16:
			f.Tag = "short"
			f.ValueString = fmt.Sprintf("%d", tv)
		case int32:
			f.Tag = "int"
			f.ValueString = fmt.Sprintf("%d", tv)
			f.IsBoolLike = tv == 0 || tv == 1
		case int64:
			f.Tag = "long"
			f.ValueString = fmt.Sprintf("%d", tv)
		case float32:
			f.Tag = "float"
			f.ValueString = fmt.Sprintf("%g", tv)
		case float64:
			f.Tag = "double"
			f.ValueString = fmt.Sprintf("%g", tv)
		case string:
			f.Tag = "string"
			f.ValueString = tv
		case []any, []int32, []int64, []float32, []float64, []string:
			f.Tag = "list"
			b, _ := json.Marshal(tv)
			f.ValueJSON = string(b)
		case map[string]any:
			f.Tag = "compound"
			if includeCompoundValues {
				b, _ := json.Marshal(tv)
				f.ValueJSON = string(b)
			}
		default:
			f.Tag = "unknown"
			value := v
			rv := reflect.ValueOf(v)
			if rv.Kind() == reflect.Array {
				switch rv.Type().Elem().Kind() {
				case reflect.Int32, reflect.Int64:
					f.Tag = "list"
				case reflect.Uint8:
					// Byte arrays use the same base64 JSON representation as a byte slice.
					buf := make([]byte, rv.Len())
					reflect.Copy(reflect.ValueOf(buf), rv)
					f.Tag = "list"
					value = buf
				}
			}
			if b, err := json.Marshal(value); err == nil {
				f.ValueJSON = string(b)
			} else {
				f.ValueString = fmt.Sprintf("%v", v)
			}
		}
		out = append(out, f)
	}
	return out, ver, nil
}

func ReadLevelDatOrder(worldDir string) ([]string, int32, error) {
	nbtData, version, err := GetLevelDatNbtAndVersion(worldDir)
	if err != nil {
		return nil, 0, err
	}
	keys, err2 := nbt.KeysInOrder(nbtData, nbt.LittleEndian, []string{"Data"})
	if err2 != nil || len(keys) == 0 {
		keys, _ = nbt.KeysInOrder(nbtData, nbt.LittleEndian, nil)
	}
	return keys, version, nil
}

func ReadLevelDatOrderAt(worldDir string, path []string) ([]string, int32, error) {
	nbtData, version, err := GetLevelDatNbtAndVersion(worldDir)
	if err != nil {
		return nil, 0, err
	}
	keys, err2 := nbt.KeysInOrder(nbtData, nbt.LittleEndian, path)
	if err2 != nil {
		return nil, version, err2
	}
	return keys, version, nil
}

func coerceLevelDatList(old any, values []any) any {
	if typ := reflect.TypeOf(old); typ != nil {
		switch typ.Kind() {
		case reflect.Array:
			return coerceArrayFromSlice(values, typ.Elem().Kind())
		case reflect.Slice:
			return coerceSliceToElemType(values, typ.Elem().Kind())
		default:
			return values
		}
	}
	if len(values) > 0 {
		switch values[0].(type) {
		case float64:
			kind := reflect.Int32
			for _, v := range values {
				n, ok := v.(float64)
				if !ok {
					return coerceSliceToElemType(values, reflect.String)
				}
				if math.Trunc(n) != n {
					kind = reflect.Float64
				}
			}
			return coerceSliceToElemType(values, kind)
		case map[string]any:
			compounds := make([]map[string]any, 0, len(values))
			for _, v := range values {
				m, ok := v.(map[string]any)
				if !ok {
					return coerceSliceToElemType(values, reflect.String)
				}
				compounds = append(compounds, m)
			}
			return compounds
		}
	}
	return coerceSliceToElemType(values, reflect.String)
}

func WriteLevelDatFields(worldDir string, fields []types.LevelDatField, version int32) error {
	return WriteLevelDatFieldsAt(worldDir, nil, fields, version)
}

func WriteLevelDatFieldsAt(worldDir string, path []string, fields []types.LevelDatField, version int32) error {
	root, _, err := DecodeLevelDat(worldDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		root = map[string]any{}
	}
	cur := root
	if data, ok := root["Data"].(map[string]any); ok {
		cur = data
	}
	for _, seg := range path {
		if m, ok := cur[seg].(map[string]any); ok {
			cur = m
		} else {
			nm := map[string]any{}
			cur[seg] = nm
			cur = nm
		}
	}
	byName := map[string]types.LevelDatField{}
	for _, f := range fields {
		byName[f.Name] = f
	}
	for k, f := range byName {
		switch f.Tag {
		case "byte":
			n, _ := strconv.Atoi(strings.TrimSpace(f.ValueString))
			cur[k] = uint8(n)
		case "short":
			n, _ := strconv.Atoi(strings.TrimSpace(f.ValueString))
			cur[k] = int16(n)
		case "int":
			n, _ := strconv.Atoi(strings.TrimSpace(f.ValueString))
			cur[k] = int32(n)
		case "long":
			n, _ := strconv.ParseInt(strings.TrimSpace(f.ValueString), 10, 64)
			cur[k] = n
		case "float":
			f32, _ := strconv.ParseFloat(strings.TrimSpace(f.ValueString), 32)
			cur[k] = float32(f32)
		case "double":
			f64, _ := strconv.ParseFloat(strings.TrimSpace(f.ValueString), 64)
			cur[k] = f64
		case "string":
			cur[k] = f.ValueString
		case "list":
			var arr []any
			vj := strings.TrimSpace(f.ValueJSON)
			if vj == "" {
				arr = []any{}
			} else {
				_ = json.Unmarshal([]byte(vj), &arr)
				if arr == nil {
					arr = []any{}
				}
			}
			cur[k] = coerceLevelDatList(cur[k], arr)
		case "compound":
			var obj map[string]any
			vj := strings.TrimSpace(f.ValueJSON)
			if vj == "" {
				obj = map[string]any{}
			} else {
				_ = json.Unmarshal([]byte(vj), &obj)
				if obj == nil {
					obj = map[string]any{}
				}
			}
			if ov, ok := cur[k].(map[string]any); ok {
				cur[k] = coerceMapTo(ov, obj)
			} else {
				cur[k] = obj
			}
		default:
			if strings.TrimSpace(f.ValueJSON) != "" {
				var anyv any
				_ = json.Unmarshal([]byte(f.ValueJSON), &anyv)
				cur[k] = anyv
			} else {
				cur[k] = f.ValueString
			}
		}
	}
	return EncodeLevelDat(worldDir, version, root)
}
