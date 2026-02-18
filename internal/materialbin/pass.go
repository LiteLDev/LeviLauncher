package materialbin

import "fmt"

type Pass struct {
	PlatformSupport        string
	Fallback               string
	Variants               []Variant
	DefaultFlagValues      []StringPair
	DefaultBlendMode       *BlendMode
	OutputBindingSignature *uint32
}

func parsePass(d *decoder, materialFileVersion uint64) (Pass, error) {
	platformSupport, err := parsePassPlatformSupport(d, materialFileVersion)
	if err != nil {
		return Pass{}, err
	}

	fallback, err := d.readString()
	if err != nil {
		return Pass{}, err
	}

	defaultBlendMode, err := parseOptionalBlendMode(d, materialFileVersion)
	if err != nil {
		return Pass{}, err
	}

	defaultFlagValues, err := parseCountedStringPairsU16(d, materialFileVersion)
	if err != nil {
		return Pass{}, err
	}

	outputBindingSignature, err := parseOutputBindingSignature(d, materialFileVersion)
	if err != nil {
		return Pass{}, err
	}

	variants, err := parseVariants(d, materialFileVersion)
	if err != nil {
		return Pass{}, err
	}

	return Pass{
		PlatformSupport:        platformSupport,
		Fallback:               fallback,
		DefaultBlendMode:       defaultBlendMode,
		DefaultFlagValues:      defaultFlagValues,
		OutputBindingSignature: outputBindingSignature,
		Variants:               variants,
	}, nil
}

func parsePassPlatformSupport(d *decoder, materialFileVersion uint64) (string, error) {
	if !isLegacyMaterialLayout(materialFileVersion) {
		return d.readString()
	}

	firstByte, err := d.peekU8()
	if err != nil {
		return "", err
	}
	if firstByte == 15 {
		return d.readString()
	}
	if err := d.skip(1); err != nil {
		return "", err
	}
	return "", nil
}

func parseOptionalBlendMode(d *decoder, _ uint64) (*BlendMode, error) {
	return readOptional(d, func(d *decoder) (BlendMode, error) {
		raw, err := d.readU16()
		if err != nil {
			return 0, err
		}
		return parseBlendMode(raw)
	})
}

func parseCountedStringPairsU16(d *decoder, materialFileVersion uint64) ([]StringPair, error) {
	count, err := d.readU16()
	if err != nil {
		return nil, err
	}
	return readStringPairs(d, int(count), materialFileVersion)
}

func parseOutputBindingSignature(d *decoder, materialFileVersion uint64) (*uint32, error) {
	if !supportsOutputBindingSignature(materialFileVersion) {
		return nil, nil
	}
	value, err := d.readU32()
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func parseVariants(d *decoder, materialFileVersion uint64) ([]Variant, error) {
	count, err := d.readU16()
	if err != nil {
		return nil, err
	}

	items := make([]Variant, int(count))
	for i := range items {
		variant, err := parseVariant(d, materialFileVersion)
		if err != nil {
			return nil, err
		}
		items[i] = variant
	}
	return items, nil
}

func (p *Pass) write(e *encoder, materialFileVersion uint64) error {
	if p.PlatformSupport == "" {
		return fmt.Errorf("bitset string is empty")
	}
	if err := e.writeString(p.PlatformSupport); err != nil {
		return err
	}
	if err := e.writeString(p.Fallback); err != nil {
		return err
	}
	if err := writeOptional(e, p.DefaultBlendMode, func(e *encoder, value BlendMode) error {
		return e.writeU16(uint16(value))
	}); err != nil {
		return err
	}

	if err := writeCountedStringPairsU16(e, p.DefaultFlagValues, "pass.defaultFlagValues", materialFileVersion); err != nil {
		return err
	}

	if supportsOutputBindingSignature(materialFileVersion) {
		value := uint32(0)
		if p.OutputBindingSignature != nil {
			value = *p.OutputBindingSignature
		}
		if err := e.writeU32(value); err != nil {
			return err
		}
	}

	if err := writeCountU16(e, len(p.Variants), "pass.variants"); err != nil {
		return err
	}
	for i := range p.Variants {
		if err := p.Variants[i].write(e, materialFileVersion); err != nil {
			return err
		}
	}
	return nil
}

func writeCountedStringPairsU16(e *encoder, pairs []StringPair, fieldName string, materialFileVersion uint64) error {
	if err := writeCountU16(e, len(pairs), fieldName); err != nil {
		return err
	}
	return writeStringPairs(e, pairs, materialFileVersion)
}

type Variant struct {
	IsSupported bool
	Flags       []StringPair
	ShaderCodes []PlatformShaderCode
}

func parseVariant(d *decoder, materialFileVersion uint64) (Variant, error) {
	isSupported, err := d.readBool()
	if err != nil {
		return Variant{}, err
	}

	flagCount, err := d.readU16()
	if err != nil {
		return Variant{}, err
	}

	shaderCodeCount, err := d.readU16()
	if err != nil {
		return Variant{}, err
	}

	flags, err := readStringPairs(d, int(flagCount), materialFileVersion)
	if err != nil {
		return Variant{}, err
	}
	shaderCodes := make([]PlatformShaderCode, int(shaderCodeCount))
	for i := range shaderCodes {
		stage, err := parsePlatformShaderStage(d, materialFileVersion)
		if err != nil {
			return Variant{}, err
		}
		code, err := parseShaderCode(d, materialFileVersion)
		if err != nil {
			return Variant{}, err
		}
		shaderCodes[i] = PlatformShaderCode{Stage: stage, Code: code}
	}

	return Variant{
		IsSupported: isSupported,
		Flags:       flags,
		ShaderCodes: shaderCodes,
	}, nil
}

func (v *Variant) write(e *encoder, materialFileVersion uint64) error {
	if err := e.writeBool(v.IsSupported); err != nil {
		return err
	}
	if err := writeCountU16(e, len(v.Flags), "variant.flags"); err != nil {
		return err
	}
	if err := writeCountU16(e, len(v.ShaderCodes), "variant.shaderCodes"); err != nil {
		return err
	}
	if err := writeStringPairs(e, v.Flags, materialFileVersion); err != nil {
		return err
	}
	for _, shaderCode := range v.ShaderCodes {
		if err := shaderCode.Stage.write(e, materialFileVersion); err != nil {
			return err
		}
		if err := shaderCode.Code.write(e, materialFileVersion); err != nil {
			return err
		}
	}
	return nil
}

type BlendMode uint16

const (
	BlendModeNone BlendMode = iota
	BlendModeReplace
	BlendModeAlphaBlend
	BlendModeColorBlendAlphaAdd
	BlendModePreMultiplied
	BlendModeInvertColor
	BlendModeAdditive
	BlendModeAdditiveAlpha
	BlendModeMultiply
	BlendModeMultiplyBoth
	BlendModeInverseSrcAlpha
	BlendModeSrcAlpha
)

func parseBlendMode(v uint16) (BlendMode, error) {
	switch BlendMode(v) {
	case BlendModeNone,
		BlendModeReplace,
		BlendModeAlphaBlend,
		BlendModeColorBlendAlphaAdd,
		BlendModePreMultiplied,
		BlendModeInvertColor,
		BlendModeAdditive,
		BlendModeAdditiveAlpha,
		BlendModeMultiply,
		BlendModeMultiplyBoth,
		BlendModeInverseSrcAlpha,
		BlendModeSrcAlpha:
		return BlendMode(v), nil
	default:
		return 0, fmt.Errorf("invalid blend mode: %d", v)
	}
}

type ShaderCodePlatform uint8

const (
	ShaderCodePlatformDirect3DSm40 ShaderCodePlatform = iota
	ShaderCodePlatformDirect3DSm50
	ShaderCodePlatformDirect3DSm60
	ShaderCodePlatformDirect3DSm65
	ShaderCodePlatformDirect3DXB1
	ShaderCodePlatformDirect3DXBX
	ShaderCodePlatformGlsl120
	ShaderCodePlatformGlsl430
	ShaderCodePlatformEssl100
	ShaderCodePlatformEssl300
	ShaderCodePlatformEssl310
	ShaderCodePlatformMetal
	ShaderCodePlatformVulkan
	ShaderCodePlatformNvn
	ShaderCodePlatformPssl
)

func parseShaderCodePlatform(v uint8, _ uint64) (ShaderCodePlatform, error) {
	switch ShaderCodePlatform(v) {
	case ShaderCodePlatformDirect3DSm40,
		ShaderCodePlatformDirect3DSm50,
		ShaderCodePlatformDirect3DSm60,
		ShaderCodePlatformDirect3DSm65,
		ShaderCodePlatformDirect3DXB1,
		ShaderCodePlatformDirect3DXBX,
		ShaderCodePlatformGlsl120,
		ShaderCodePlatformGlsl430,
		ShaderCodePlatformEssl100,
		ShaderCodePlatformEssl300,
		ShaderCodePlatformEssl310,
		ShaderCodePlatformMetal,
		ShaderCodePlatformVulkan,
		ShaderCodePlatformNvn,
		ShaderCodePlatformPssl:
		return ShaderCodePlatform(v), nil
	default:
		return 0, fmt.Errorf("invalid shader code platform: %d", v)
	}
}

func (p ShaderCodePlatform) String() string {
	switch p {
	case ShaderCodePlatformDirect3DSm40:
		return "Direct3D_SM40"
	case ShaderCodePlatformDirect3DSm50:
		return "Direct3D_SM50"
	case ShaderCodePlatformDirect3DSm60:
		return "Direct3D_SM60"
	case ShaderCodePlatformDirect3DSm65:
		return "Direct3D_SM65"
	case ShaderCodePlatformDirect3DXB1:
		return "Direct3D_XB1"
	case ShaderCodePlatformDirect3DXBX:
		return "Direct3D_XBX"
	case ShaderCodePlatformGlsl120:
		return "GLSL_120"
	case ShaderCodePlatformGlsl430:
		return "GLSL_430"
	case ShaderCodePlatformEssl100:
		return "ESSL_100"
	case ShaderCodePlatformEssl300:
		return "ESSL_300"
	case ShaderCodePlatformEssl310:
		return "ESSL_310"
	case ShaderCodePlatformMetal:
		return "Metal"
	case ShaderCodePlatformVulkan:
		return "Vulkan"
	case ShaderCodePlatformNvn:
		return "Nvn"
	case ShaderCodePlatformPssl:
		return "PSSL"
	default:
		return fmt.Sprintf("unknown(%d)", uint8(p))
	}
}

type ShaderCode struct {
	ShaderInputs   []NamedShaderInput
	SourceHash     uint64
	BgfxShaderData []byte
}

func parseShaderCode(d *decoder, materialFileVersion uint64) (ShaderCode, error) {
	shaderInputs, err := parseNamedShaderInputs(d, materialFileVersion)
	if err != nil {
		return ShaderCode{}, err
	}

	sourceHash, err := d.readU64()
	if err != nil {
		return ShaderCode{}, err
	}

	bgfxShaderLength, err := d.readU32()
	if err != nil {
		return ShaderCode{}, err
	}
	if bgfxShaderLength > uint32(d.remaining()) {
		return ShaderCode{}, fmt.Errorf("invalid bgfx shader length %d at offset %d", bgfxShaderLength, d.offset())
	}
	bgfxShaderData, err := d.read(int(bgfxShaderLength))
	if err != nil {
		return ShaderCode{}, err
	}

	return ShaderCode{
		ShaderInputs:   shaderInputs,
		SourceHash:     sourceHash,
		BgfxShaderData: append([]byte(nil), bgfxShaderData...),
	}, nil
}

func parseNamedShaderInputs(d *decoder, materialFileVersion uint64) ([]NamedShaderInput, error) {
	count, err := d.readU16()
	if err != nil {
		return nil, err
	}

	items := make([]NamedShaderInput, int(count))
	for i := range items {
		name, err := d.readString()
		if err != nil {
			return nil, err
		}
		input, err := parseShaderInput(d, materialFileVersion)
		if err != nil {
			return nil, err
		}
		items[i] = NamedShaderInput{Name: name, Input: input}
	}
	return items, nil
}

func (s *ShaderCode) write(e *encoder, materialFileVersion uint64) error {
	if err := writeCountU16(e, len(s.ShaderInputs), "shaderCode.shaderInputs"); err != nil {
		return err
	}
	for _, input := range s.ShaderInputs {
		if err := e.writeString(input.Name); err != nil {
			return err
		}
		if err := input.Input.write(e, materialFileVersion); err != nil {
			return err
		}
	}
	if err := e.writeU64(s.SourceHash); err != nil {
		return err
	}
	if err := writeCountU32(e, len(s.BgfxShaderData), "shaderCode.bgfxShaderData"); err != nil {
		return err
	}
	return e.writeBytes(s.BgfxShaderData)
}

type ShaderInput struct {
	InputType               ShaderInputType
	Attribute               Attribute
	IsPerInstance           bool
	PrecisionConstraint     *PrecisionConstraint
	InterpolationConstraint *InterpolationConstraint
}

func parseShaderInput(d *decoder, _ uint64) (ShaderInput, error) {
	inputTypeRaw, err := d.readU8()
	if err != nil {
		return ShaderInput{}, err
	}
	inputType, err := parseShaderInputType(inputTypeRaw)
	if err != nil {
		return ShaderInput{}, err
	}

	attributeIndex, err := d.readU8()
	if err != nil {
		return ShaderInput{}, err
	}
	attributeSubIndex, err := d.readU8()
	if err != nil {
		return ShaderInput{}, err
	}
	attribute, err := parseAttribute(attributeIndex, attributeSubIndex)
	if err != nil {
		return ShaderInput{}, err
	}

	isPerInstance, err := d.readBool()
	if err != nil {
		return ShaderInput{}, err
	}

	precisionConstraint, err := readOptional(d, func(d *decoder) (PrecisionConstraint, error) {
		value, err := d.readU8()
		if err != nil {
			return 0, err
		}
		return parsePrecisionConstraint(value)
	})
	if err != nil {
		return ShaderInput{}, err
	}

	interpolationConstraint, err := readOptional(d, func(d *decoder) (InterpolationConstraint, error) {
		value, err := d.readU8()
		if err != nil {
			return 0, err
		}
		return parseInterpolationConstraint(value)
	})
	if err != nil {
		return ShaderInput{}, err
	}

	return ShaderInput{
		InputType:               inputType,
		Attribute:               attribute,
		IsPerInstance:           isPerInstance,
		PrecisionConstraint:     precisionConstraint,
		InterpolationConstraint: interpolationConstraint,
	}, nil
}

func (s *ShaderInput) write(e *encoder, _ uint64) error {
	if err := e.writeU8(uint8(s.InputType)); err != nil {
		return err
	}

	index, subIndex := s.Attribute.Tuple()
	if err := e.writeU8(index); err != nil {
		return err
	}
	if err := e.writeU8(subIndex); err != nil {
		return err
	}

	if err := e.writeBool(s.IsPerInstance); err != nil {
		return err
	}
	if err := writeOptional(e, s.PrecisionConstraint, func(e *encoder, value PrecisionConstraint) error {
		return e.writeU8(uint8(value))
	}); err != nil {
		return err
	}
	if err := writeOptional(e, s.InterpolationConstraint, func(e *encoder, value InterpolationConstraint) error {
		return e.writeU8(uint8(value))
	}); err != nil {
		return err
	}
	return nil
}

type ShaderInputType uint8

const (
	ShaderInputTypeFloat ShaderInputType = iota
	ShaderInputTypeVec2
	ShaderInputTypeVec3
	ShaderInputTypeVec4
	ShaderInputTypeInt
	ShaderInputTypeInt2
	ShaderInputTypeInt3
	ShaderInputTypeInt4
	ShaderInputTypeUInt
	ShaderInputTypeUInt2
	ShaderInputTypeUInt3
	ShaderInputTypeUInt4
	ShaderInputTypeMat4
)

func parseShaderInputType(v uint8) (ShaderInputType, error) {
	switch ShaderInputType(v) {
	case ShaderInputTypeFloat,
		ShaderInputTypeVec2,
		ShaderInputTypeVec3,
		ShaderInputTypeVec4,
		ShaderInputTypeInt,
		ShaderInputTypeInt2,
		ShaderInputTypeInt3,
		ShaderInputTypeInt4,
		ShaderInputTypeUInt,
		ShaderInputTypeUInt2,
		ShaderInputTypeUInt3,
		ShaderInputTypeUInt4,
		ShaderInputTypeMat4:
		return ShaderInputType(v), nil
	default:
		return 0, fmt.Errorf("invalid shader input type: %d", v)
	}
}

type PrecisionConstraint uint8

const (
	PrecisionConstraintLow PrecisionConstraint = iota
	PrecisionConstraintMedium
	PrecisionConstraintHigh
)

func parsePrecisionConstraint(v uint8) (PrecisionConstraint, error) {
	switch PrecisionConstraint(v) {
	case PrecisionConstraintLow, PrecisionConstraintMedium, PrecisionConstraintHigh:
		return PrecisionConstraint(v), nil
	default:
		return 0, fmt.Errorf("invalid precision constraint: %d", v)
	}
}

type InterpolationConstraint uint8

const (
	InterpolationConstraintFlat InterpolationConstraint = iota
	InterpolationConstraintSmooth
	InterpolationConstraintNoPerspective
	InterpolationConstraintCentroid
)

func parseInterpolationConstraint(v uint8) (InterpolationConstraint, error) {
	switch InterpolationConstraint(v) {
	case InterpolationConstraintFlat,
		InterpolationConstraintSmooth,
		InterpolationConstraintNoPerspective,
		InterpolationConstraintCentroid:
		return InterpolationConstraint(v), nil
	default:
		return 0, fmt.Errorf("invalid interpolation constraint: %d", v)
	}
}

type Attribute uint8

const (
	AttributePosition Attribute = iota
	AttributeNormal
	AttributeTangent
	AttributeBitangent
	AttributeColor0
	AttributeColor1
	AttributeColor2
	AttributeColor3
	AttributeIndices
	AttributeWeights
	AttributeTexCoord0
	AttributeTexCoord1
	AttributeTexCoord2
	AttributeTexCoord3
	AttributeTexCoord4
	AttributeTexCoord5
	AttributeTexCoord6
	AttributeTexCoord7
	AttributeTexCoord8
	AttributeFrontFacing
)

var attributeByTuple = map[[2]uint8]Attribute{
	{0, 0}: AttributePosition,
	{1, 0}: AttributeNormal,
	{2, 0}: AttributeTangent,
	{3, 0}: AttributeBitangent,
	{4, 0}: AttributeColor0,
	{4, 1}: AttributeColor1,
	{4, 2}: AttributeColor2,
	{4, 3}: AttributeColor3,
	{5, 0}: AttributeIndices,
	{6, 0}: AttributeWeights,
	{7, 0}: AttributeTexCoord0,
	{7, 1}: AttributeTexCoord1,
	{7, 2}: AttributeTexCoord2,
	{7, 3}: AttributeTexCoord3,
	{7, 4}: AttributeTexCoord4,
	{7, 5}: AttributeTexCoord5,
	{7, 6}: AttributeTexCoord6,
	{7, 7}: AttributeTexCoord7,
	{7, 8}: AttributeTexCoord8,
	{9, 0}: AttributeFrontFacing,
}

var tupleByAttribute = map[Attribute][2]uint8{
	AttributePosition:    {0, 0},
	AttributeNormal:      {1, 0},
	AttributeTangent:     {2, 0},
	AttributeBitangent:   {3, 0},
	AttributeColor0:      {4, 0},
	AttributeColor1:      {4, 1},
	AttributeColor2:      {4, 2},
	AttributeColor3:      {4, 3},
	AttributeIndices:     {5, 0},
	AttributeWeights:     {6, 0},
	AttributeTexCoord0:   {7, 0},
	AttributeTexCoord1:   {7, 1},
	AttributeTexCoord2:   {7, 2},
	AttributeTexCoord3:   {7, 3},
	AttributeTexCoord4:   {7, 4},
	AttributeTexCoord5:   {7, 5},
	AttributeTexCoord6:   {7, 6},
	AttributeTexCoord7:   {7, 7},
	AttributeTexCoord8:   {7, 8},
	AttributeFrontFacing: {9, 0},
}

func parseAttribute(index, subIndex uint8) (Attribute, error) {
	if attribute, ok := attributeByTuple[[2]uint8{index, subIndex}]; ok {
		return attribute, nil
	}
	return 0, fmt.Errorf("invalid attribute tuple (%d,%d)", index, subIndex)
}

func (a Attribute) Tuple() (index, subIndex uint8) {
	if tuple, ok := tupleByAttribute[a]; ok {
		return tuple[0], tuple[1]
	}
	return 0, 0
}

type ShaderStage uint8

const (
	ShaderStageVertex ShaderStage = iota
	ShaderStageFragment
	ShaderStageCompute
	ShaderStageUnknown
)

func parseShaderStage(v uint8) (ShaderStage, error) {
	switch ShaderStage(v) {
	case ShaderStageVertex, ShaderStageFragment, ShaderStageCompute, ShaderStageUnknown:
		return ShaderStage(v), nil
	default:
		return 0, fmt.Errorf("invalid shader stage: %d", v)
	}
}

type PlatformShaderStage struct {
	StageName    string
	PlatformName string
	Stage        ShaderStage
	Platform     ShaderCodePlatform
}

func parsePlatformShaderStage(d *decoder, materialFileVersion uint64) (PlatformShaderStage, error) {
	stageName, err := d.readString()
	if err != nil {
		return PlatformShaderStage{}, err
	}
	platformName, err := d.readString()
	if err != nil {
		return PlatformShaderStage{}, err
	}
	stageRaw, err := d.readU8()
	if err != nil {
		return PlatformShaderStage{}, err
	}
	stage, err := parseShaderStage(stageRaw)
	if err != nil {
		return PlatformShaderStage{}, err
	}
	platformRaw, err := d.readU8()
	if err != nil {
		return PlatformShaderStage{}, err
	}
	platform, err := parseShaderCodePlatform(platformRaw, materialFileVersion)
	if err != nil {
		return PlatformShaderStage{}, err
	}

	return PlatformShaderStage{
		StageName:    stageName,
		PlatformName: platformName,
		Stage:        stage,
		Platform:     platform,
	}, nil
}

func (p *PlatformShaderStage) write(e *encoder, _ uint64) error {
	if err := e.writeString(p.StageName); err != nil {
		return err
	}
	if err := e.writeString(p.Platform.String()); err != nil {
		return err
	}
	if err := e.writeU8(uint8(p.Stage)); err != nil {
		return err
	}
	return e.writeU8(uint8(p.Platform))
}
