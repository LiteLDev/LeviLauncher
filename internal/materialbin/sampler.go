package materialbin

import "fmt"

type SamplerState uint8

const (
	SamplerStateClampPoint SamplerState = iota
	SamplerStateClampLinear
	SamplerStateWrapPoint
	SamplerStateWrapLinear
)

func parseSamplerState(v uint8) (SamplerState, error) {
	switch SamplerState(v) {
	case SamplerStateClampPoint, SamplerStateClampLinear, SamplerStateWrapPoint, SamplerStateWrapLinear:
		return SamplerState(v), nil
	default:
		return 0, fmt.Errorf("invalid sampler state: %d", v)
	}
}

type SamplerDefinition struct {
	RegisterSlot         uint16
	BindingSlot          uint8
	Size                 uint32
	Access               SamplerAccess
	Precision            PrecisionConstraint
	AllowUnorderedAccess uint8
	SamplerType          SamplerType
	TextureFormat        string
	SamplerState         *SamplerState
	DefaultTexture       *string
	TextureURI           *string
	CustomTypeInfo       *CustomTypeInfo
}

func parseSamplerDefinition(d *decoder, materialFileVersion uint64) (SamplerDefinition, error) {
	registerSlot, err := parseSamplerRegisterSlot(d, materialFileVersion)
	if err != nil {
		return SamplerDefinition{}, err
	}

	accessRaw, err := d.readU8()
	if err != nil {
		return SamplerDefinition{}, err
	}
	access, err := parseSamplerAccess(accessRaw)
	if err != nil {
		return SamplerDefinition{}, err
	}
	precisionRaw, err := d.readU8()
	if err != nil {
		return SamplerDefinition{}, err
	}
	precision, err := parsePrecisionConstraint(precisionRaw)
	if err != nil {
		return SamplerDefinition{}, err
	}

	allowUnorderedAccess, err := d.readU8()
	if err != nil {
		return SamplerDefinition{}, err
	}
	samplerTypeRaw, err := d.readU8()
	if err != nil {
		return SamplerDefinition{}, err
	}
	samplerType, err := parseSamplerType(samplerTypeRaw, materialFileVersion)
	if err != nil {
		return SamplerDefinition{}, err
	}

	textureFormat, err := d.readString()
	if err != nil {
		return SamplerDefinition{}, err
	}
	size, err := d.readU32()
	if err != nil {
		return SamplerDefinition{}, err
	}

	bindingSlot, err := parseSamplerBindingSlot(d, materialFileVersion, registerSlot)
	if err != nil {
		return SamplerDefinition{}, err
	}

	samplerState, err := parseOptionalSamplerState(d, materialFileVersion)
	if err != nil {
		return SamplerDefinition{}, err
	}

	defaultTexture, err := readOptional(d, func(d *decoder) (string, error) { return d.readString() })
	if err != nil {
		return SamplerDefinition{}, err
	}

	var textureURI *string
	if supportsTextureURI(materialFileVersion) {
		textureURI, err = readOptional(d, func(d *decoder) (string, error) { return d.readString() })
		if err != nil {
			return SamplerDefinition{}, err
		}
	}

	customTypeInfo, err := readOptional(d, func(d *decoder) (CustomTypeInfo, error) {
		return parseCustomTypeInfo(d, materialFileVersion)
	})
	if err != nil {
		return SamplerDefinition{}, err
	}

	return SamplerDefinition{
		RegisterSlot:         registerSlot,
		BindingSlot:          bindingSlot,
		Size:                 size,
		Access:               access,
		Precision:            precision,
		AllowUnorderedAccess: allowUnorderedAccess,
		SamplerType:          samplerType,
		TextureFormat:        textureFormat,
		SamplerState:         samplerState,
		DefaultTexture:       defaultTexture,
		TextureURI:           textureURI,
		CustomTypeInfo:       customTypeInfo,
	}, nil
}

func (s *SamplerDefinition) write(e *encoder, materialFileVersion uint64) error {
	if err := writeSamplerRegisterSlot(e, materialFileVersion, s.RegisterSlot); err != nil {
		return err
	}

	if err := e.writeU8(uint8(s.Access)); err != nil {
		return err
	}
	if err := e.writeU8(uint8(s.Precision)); err != nil {
		return err
	}
	if err := e.writeU8(s.AllowUnorderedAccess); err != nil {
		return err
	}
	samplerTypeID, err := s.SamplerType.toU8(materialFileVersion)
	if err != nil {
		return err
	}
	if err := e.writeU8(samplerTypeID); err != nil {
		return err
	}
	if err := e.writeString(s.TextureFormat); err != nil {
		return err
	}
	if err := e.writeU32(s.Size); err != nil {
		return err
	}

	if !isLegacyMaterialLayout(materialFileVersion) {
		if err := e.writeU8(s.BindingSlot); err != nil {
			return err
		}
	}

	if err := writeOptionalSamplerState(e, materialFileVersion, s.SamplerState); err != nil {
		return err
	}

	if err := writeOptional(e, s.DefaultTexture, func(e *encoder, v string) error { return e.writeString(v) }); err != nil {
		return err
	}
	if supportsTextureURI(materialFileVersion) {
		if err := writeOptional(e, s.TextureURI, func(e *encoder, v string) error { return e.writeString(v) }); err != nil {
			return err
		}
	}
	return writeOptional(e, s.CustomTypeInfo, func(e *encoder, v CustomTypeInfo) error { return v.write(e, materialFileVersion) })
}

func parseSamplerRegisterSlot(d *decoder, materialFileVersion uint64) (uint16, error) {
	if isLegacyMaterialLayout(materialFileVersion) {
		value, err := d.readU8()
		if err != nil {
			return 0, err
		}
		return uint16(value), nil
	}
	return d.readU16()
}

func parseSamplerBindingSlot(d *decoder, materialFileVersion uint64, registerSlot uint16) (uint8, error) {
	if !isLegacyMaterialLayout(materialFileVersion) {
		return d.readU8()
	}
	if registerSlot > uint16(^uint8(0)) {
		return 0, fmt.Errorf("failed to convert reg %d into u8 for binding_slot", registerSlot)
	}
	return uint8(registerSlot), nil
}

func parseOptionalSamplerState(d *decoder, materialFileVersion uint64) (*SamplerState, error) {
	if !supportsSamplerState(materialFileVersion) {
		return nil, nil
	}

	hasSamplerState, err := d.readBool()
	if err != nil {
		return nil, err
	}
	if !hasSamplerState {
		return nil, nil
	}

	raw, err := d.readU8()
	if err != nil {
		return nil, err
	}
	state := SamplerState(raw)
	return &state, nil
}

func writeSamplerRegisterSlot(e *encoder, materialFileVersion uint64, registerSlot uint16) error {
	if !isLegacyMaterialLayout(materialFileVersion) {
		return e.writeU16(registerSlot)
	}
	if registerSlot > uint16(^uint8(0)) {
		return fmt.Errorf("sampler reg %d exceeds u8 for material file version %d", registerSlot, materialFileVersion)
	}
	return e.writeU8(uint8(registerSlot))
}

func writeOptionalSamplerState(e *encoder, materialFileVersion uint64, state *SamplerState) error {
	if !supportsSamplerState(materialFileVersion) {
		return nil
	}

	var raw *uint8
	if state != nil {
		value := uint8(*state)
		raw = &value
	}
	return writeOptional(e, raw, func(e *encoder, value uint8) error { return e.writeU8(value) })
}

type CustomTypeInfo struct {
	CustomType       string
	CustomTypeStride uint32
}

func parseCustomTypeInfo(d *decoder, _ uint64) (CustomTypeInfo, error) {
	name, err := d.readString()
	if err != nil {
		return CustomTypeInfo{}, err
	}
	size, err := d.readU32()
	if err != nil {
		return CustomTypeInfo{}, err
	}
	return CustomTypeInfo{CustomType: name, CustomTypeStride: size}, nil
}

func (c *CustomTypeInfo) write(e *encoder, _ uint64) error {
	if err := e.writeString(c.CustomType); err != nil {
		return err
	}
	return e.writeU32(c.CustomTypeStride)
}

type SamplerType uint8

const (
	SamplerType2D SamplerType = iota
	SamplerType2DArray
	SamplerType2DExternal
	SamplerType3D
	SamplerTypeCube
	SamplerTypeSamplerCubeArray
	SamplerTypeStructuredBuffer
	SamplerTypeRawBuffer
	SamplerTypeAccelerationStructure
	SamplerType2DShadow
	SamplerType2DArrayShadow
)

func parseSamplerType(v uint8, materialFileVersion uint64) (SamplerType, error) {
	samplerType := v
	if !supportsSamplerState(materialFileVersion) && samplerType >= 5 {
		samplerType++
	}
	switch SamplerType(samplerType) {
	case SamplerType2D,
		SamplerType2DArray,
		SamplerType2DExternal,
		SamplerType3D,
		SamplerTypeCube,
		SamplerTypeSamplerCubeArray,
		SamplerTypeStructuredBuffer,
		SamplerTypeRawBuffer,
		SamplerTypeAccelerationStructure,
		SamplerType2DShadow,
		SamplerType2DArrayShadow:
		return SamplerType(samplerType), nil
	default:
		return 0, fmt.Errorf("invalid sampler type: %d", samplerType)
	}
}

func (s SamplerType) toU8(materialFileVersion uint64) (uint8, error) {
	if !supportsSamplerState(materialFileVersion) {
		switch s {
		case SamplerTypeSamplerCubeArray:
			return 0, fmt.Errorf("sampler type SamplerCubeArray is incompatible with material file versions before %d", 21)
		case SamplerTypeRawBuffer:
			return 5, nil
		case SamplerTypeAccelerationStructure:
			return 6, nil
		case SamplerType2DShadow:
			return 7, nil
		case SamplerType2DArrayShadow:
			return 8, nil
		default:
			return uint8(s), nil
		}
	}
	return uint8(s), nil
}

type SamplerAccess uint8

const (
	SamplerAccessNone SamplerAccess = iota
	SamplerAccessRead
	SamplerAccessWrite
	SamplerAccessReadWrite
)

func parseSamplerAccess(v uint8) (SamplerAccess, error) {
	switch SamplerAccess(v) {
	case SamplerAccessNone, SamplerAccessRead, SamplerAccessWrite, SamplerAccessReadWrite:
		return SamplerAccess(v), nil
	default:
		return 0, fmt.Errorf("invalid sampler access: %d", v)
	}
}
