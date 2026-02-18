package materialbin

import "fmt"

const (
	propertyVec4Bytes = 16
	propertyMat3Bytes = 36
	propertyMat4Bytes = 64
)

type PropertyField struct {
	FieldType  PropertyType
	Num        uint32
	VectorData []byte
	MatrixData []byte
}

func parsePropertyField(d *decoder, _ uint64) (PropertyField, error) {
	fieldTypeRaw, err := d.readU16()
	if err != nil {
		return PropertyField{}, err
	}
	fieldType, err := parsePropertyType(fieldTypeRaw)
	if err != nil {
		return PropertyField{}, err
	}
	num, err := d.readU32()
	if err != nil {
		return PropertyField{}, err
	}
	hasData, err := d.readBool()
	if err != nil {
		return PropertyField{}, err
	}
	field := PropertyField{
		FieldType: fieldType,
		Num:       num,
	}
	if !hasData {
		return field, nil
	}

	switch fieldType {
	case PropertyTypeVec4:
		data, err := d.read(propertyVec4Bytes)
		if err != nil {
			return PropertyField{}, err
		}
		field.VectorData = append([]byte(nil), data...)
	case PropertyTypeMat3:
		data, err := d.read(propertyMat3Bytes)
		if err != nil {
			return PropertyField{}, err
		}
		field.MatrixData = append([]byte(nil), data...)
	case PropertyTypeMat4:
		data, err := d.read(propertyMat4Bytes)
		if err != nil {
			return PropertyField{}, err
		}
		field.MatrixData = append([]byte(nil), data...)
	case PropertyTypeExternal:
	}
	return field, nil
}

func (p *PropertyField) write(e *encoder, materialFileVersion uint64) error {
	if err := e.writeU16(uint16(p.FieldType)); err != nil {
		return err
	}
	if p.FieldType != PropertyTypeExternal {
		if err := e.writeU32(p.Num); err != nil {
			return err
		}
	}
	switch p.FieldType {
	case PropertyTypeVec4:
		return writeOptionalPropertyData(e, p.VectorData, materialFileVersion)
	case PropertyTypeMat3, PropertyTypeMat4:
		return writeOptionalPropertyData(e, p.MatrixData, materialFileVersion)
	case PropertyTypeExternal:
		return nil
	}
	return nil
}

func writeOptionalPropertyData(e *encoder, data []byte, _ uint64) error {
	hasData := len(data) > 0
	if err := e.writeBool(hasData); err != nil {
		return err
	}
	if !hasData {
		return nil
	}
	return e.writeBytes(data)
}

type PropertyType uint16

const (
	PropertyTypeVec4     PropertyType = 2
	PropertyTypeMat3     PropertyType = 3
	PropertyTypeMat4     PropertyType = 4
	PropertyTypeExternal PropertyType = 5
)

func parsePropertyType(v uint16) (PropertyType, error) {
	switch PropertyType(v) {
	case PropertyTypeVec4, PropertyTypeMat3, PropertyTypeMat4, PropertyTypeExternal:
		return PropertyType(v), nil
	default:
		return 0, fmt.Errorf("invalid property type: %d", v)
	}
}
