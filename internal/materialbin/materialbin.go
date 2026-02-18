package materialbin

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	startEndMagic    uint64 = 0x0A11DA1A
	definitionClass         = "RenderDragon.CompiledMaterialDefinition"
	coreBuiltinsName        = "Core/Builtins"
)

type decoder struct {
	buf []byte
	off int
}

func newDecoder(buf []byte) *decoder {
	return &decoder{buf: buf}
}

func (d *decoder) offset() int {
	return d.off
}

func (d *decoder) remaining() int {
	return len(d.buf) - d.off
}

func (d *decoder) read(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("negative read length: %d", n)
	}
	if d.remaining() < n {
		return nil, fmt.Errorf("unexpected EOF at offset %d: need %d bytes, remaining %d", d.off, n, d.remaining())
	}
	out := d.buf[d.off : d.off+n]
	d.off += n
	return out, nil
}

func (d *decoder) skip(n int) error {
	_, err := d.read(n)
	return err
}

func (d *decoder) peekU8() (uint8, error) {
	if d.remaining() < 1 {
		return 0, fmt.Errorf("unexpected EOF at offset %d while peeking u8", d.off)
	}
	return d.buf[d.off], nil
}

func (d *decoder) readU8() (uint8, error) {
	b, err := d.read(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (d *decoder) readU16() (uint16, error) {
	b, err := d.read(2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(b), nil
}

func (d *decoder) readU32() (uint32, error) {
	b, err := d.read(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (d *decoder) readU64() (uint64, error) {
	b, err := d.read(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b), nil
}

func (d *decoder) readBool() (bool, error) {
	v, err := d.readU8()
	if err != nil {
		return false, err
	}
	return v != 0, nil
}

func (d *decoder) readString() (string, error) {
	strLen, err := d.readU32()
	if err != nil {
		return "", err
	}
	if strLen > uint32(d.remaining()) {
		return "", fmt.Errorf("invalid string length %d at offset %d (remaining %d)", strLen, d.off, d.remaining())
	}
	b, err := d.read(int(strLen))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type encoder struct {
	buf bytes.Buffer
}

func (e *encoder) bytes() []byte {
	return e.buf.Bytes()
}

func (e *encoder) writeU8(v uint8) error {
	return e.buf.WriteByte(v)
}

func (e *encoder) writeU16(v uint16) error {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	_, err := e.buf.Write(b[:])
	return err
}

func (e *encoder) writeU32(v uint32) error {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	_, err := e.buf.Write(b[:])
	return err
}

func (e *encoder) writeU64(v uint64) error {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	_, err := e.buf.Write(b[:])
	return err
}

func (e *encoder) writeBool(v bool) error {
	if v {
		return e.writeU8(1)
	}
	return e.writeU8(0)
}

func (e *encoder) writeBytes(v []byte) error {
	_, err := e.buf.Write(v)
	return err
}

func (e *encoder) writeString(v string) error {
	if len(v) > int(^uint32(0)) {
		return fmt.Errorf("string length %d exceeds uint32", len(v))
	}
	if err := e.writeU32(uint32(len(v))); err != nil {
		return err
	}
	_, err := e.buf.WriteString(v)
	return err
}

func writeOptional[T any](e *encoder, value *T, writeValue func(*encoder, T) error) error {
	hasValue := value != nil
	if err := e.writeBool(hasValue); err != nil {
		return err
	}
	if !hasValue {
		return nil
	}
	return writeValue(e, *value)
}

func readOptional[T any](d *decoder, readValue func(*decoder) (T, error)) (*T, error) {
	has, err := d.readBool()
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	v, err := readValue(d)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func writeCountU8(e *encoder, n int, name string) error {
	if n < 0 || n > int(^uint8(0)) {
		return fmt.Errorf("%s count %d exceeds u8", name, n)
	}
	return e.writeU8(uint8(n))
}

func writeCountU16(e *encoder, n int, name string) error {
	if n < 0 || n > int(^uint16(0)) {
		return fmt.Errorf("%s count %d exceeds u16", name, n)
	}
	return e.writeU16(uint16(n))
}

func writeCountU32(e *encoder, n int, name string) error {
	if n < 0 || n > int(^uint32(0)) {
		return fmt.Errorf("%s count %d exceeds u32", name, n)
	}
	return e.writeU32(uint32(n))
}

type StringPair struct {
	Key   string
	Value string
}

type NamedSamplerDefinition struct {
	Name              string
	SamplerDefinition SamplerDefinition
}

type NamedPropertyField struct {
	Name          string
	PropertyField PropertyField
}

type NamedPass struct {
	Name string
	Pass Pass
}

type NamedShaderInput struct {
	Name  string
	Input ShaderInput
}

type PlatformShaderCode struct {
	Stage PlatformShaderStage
	Code  ShaderCode
}

type CompiledMaterialDefinition struct {
	Version           uint64
	EncryptionVariant EncryptionVariant
	Name              string
	ParentName        *string

	SamplerDefinitions []NamedSamplerDefinition
	PropertyFields     []NamedPropertyField
	UniformOverrides   *[]StringPair
	Passes             []NamedPass
}

func Parse(data []byte, materialFileVersion uint64) (*CompiledMaterialDefinition, error) {
	return (materialCodec{materialFileVersion: materialFileVersion}).parse(newDecoder(data))
}

func ParseAuto(data []byte) (*CompiledMaterialDefinition, uint64, error) {
	parsed, err := Parse(data, 0)
	if err != nil {
		return nil, 0, err
	}
	return parsed, parsed.Version, nil
}

func (c *CompiledMaterialDefinition) MarshalBinary(materialFileVersion uint64) ([]byte, error) {
	e := &encoder{}
	if err := (materialCodec{materialFileVersion: materialFileVersion}).write(e, c); err != nil {
		return nil, err
	}
	return e.bytes(), nil
}

func (c *CompiledMaterialDefinition) WriteTo(w io.Writer, materialFileVersion uint64) (int64, error) {
	b, err := c.MarshalBinary(materialFileVersion)
	if err != nil {
		return 0, err
	}
	n, err := w.Write(b)
	return int64(n), err
}

type materialCodec struct {
	materialFileVersion uint64
}

func (c materialCodec) parse(d *decoder) (*CompiledMaterialDefinition, error) {
	header, err := c.parseHeader(d)
	if err != nil {
		return nil, err
	}
	if c.materialFileVersion != 0 && c.materialFileVersion != header.FileVersion {
		return nil, fmt.Errorf("material file version mismatch: file=%d requested=%d", header.FileVersion, c.materialFileVersion)
	}
	c.materialFileVersion = header.FileVersion

	samplerDefinitions, err := c.parseSamplerDefinitions(d)
	if err != nil {
		return nil, err
	}

	propertyFields, err := c.parsePropertyFields(d)
	if err != nil {
		return nil, err
	}

	uniformOverrides, err := c.parseUniformOverrides(d, header.Name)
	if err != nil {
		return nil, err
	}

	passes, err := c.parsePasses(d)
	if err != nil {
		return nil, err
	}

	if err := parseMaterialFooter(d, c.materialFileVersion); err != nil {
		return nil, err
	}

	return &CompiledMaterialDefinition{
		Version:            header.FileVersion,
		EncryptionVariant:  header.EncryptionVariant,
		Name:               header.Name,
		ParentName:         header.ParentName,
		SamplerDefinitions: samplerDefinitions,
		PropertyFields:     propertyFields,
		UniformOverrides:   uniformOverrides,
		Passes:             passes,
	}, nil
}

func (c materialCodec) write(e *encoder, material *CompiledMaterialDefinition) error {
	if err := c.writeHeader(e, material); err != nil {
		return err
	}
	if err := c.writeSamplerDefinitions(e, material.SamplerDefinitions); err != nil {
		return err
	}
	if err := c.writePropertyFields(e, material.PropertyFields); err != nil {
		return err
	}
	if err := c.writeUniformOverrides(e, material.Name, material.UniformOverrides); err != nil {
		return err
	}
	if err := c.writePasses(e, material.Passes); err != nil {
		return err
	}
	return writeMaterialFooter(e, c.materialFileVersion)
}

type materialHeader struct {
	FileVersion       uint64
	EncryptionVariant EncryptionVariant
	Name              string
	ParentName        *string
}

func (c materialCodec) parseHeader(d *decoder) (materialHeader, error) {
	if err := parseMaterialHeader(d, c.materialFileVersion); err != nil {
		return materialHeader{}, err
	}

	fileVersion, err := d.readU64()
	if err != nil {
		return materialHeader{}, err
	}

	rawEncryptionVariant, err := d.readU32()
	if err != nil {
		return materialHeader{}, err
	}
	encryptionVariant, err := parseEncryptionVariant(rawEncryptionVariant)
	if err != nil {
		return materialHeader{}, err
	}
	if encryptionVariant.IsEncrypted() {
		return materialHeader{}, fmt.Errorf("encrypted files are not supported")
	}

	name, err := d.readString()
	if err != nil {
		return materialHeader{}, err
	}
	parentName, err := readOptional(d, func(d *decoder) (string, error) { return d.readString() })
	if err != nil {
		return materialHeader{}, err
	}

	return materialHeader{
		FileVersion:       fileVersion,
		EncryptionVariant: encryptionVariant,
		Name:              name,
		ParentName:        parentName,
	}, nil
}

func parseMaterialHeader(d *decoder, _ uint64) error {
	magic, err := d.readU64()
	if err != nil {
		return err
	}
	if magic != startEndMagic {
		return fmt.Errorf("invalid starting magic: got 0x%X", magic)
	}

	klass, err := d.readString()
	if err != nil {
		return err
	}
	if klass != definitionClass {
		return fmt.Errorf("invalid definition string: %q", klass)
	}
	return nil
}

func parseMaterialFooter(d *decoder, _ uint64) error {
	magic, err := d.readU64()
	if err != nil {
		return err
	}
	if magic != startEndMagic {
		return fmt.Errorf("invalid ending magic: got 0x%X", magic)
	}
	return nil
}

func (c materialCodec) writeHeader(e *encoder, material *CompiledMaterialDefinition) error {
	fileVersion := resolveMaterialFileVersion(c.materialFileVersion, material.Version)
	if fileVersion == 0 {
		return fmt.Errorf("material file version is required")
	}

	if err := e.writeU64(startEndMagic); err != nil {
		return err
	}
	if err := e.writeString(definitionClass); err != nil {
		return err
	}
	if err := e.writeU64(fileVersion); err != nil {
		return err
	}
	if err := e.writeU32(uint32(material.EncryptionVariant)); err != nil {
		return err
	}
	if err := e.writeString(material.Name); err != nil {
		return err
	}
	return writeOptional(e, material.ParentName, func(e *encoder, value string) error { return e.writeString(value) })
}

func writeMaterialFooter(e *encoder, _ uint64) error {
	return e.writeU64(startEndMagic)
}

func resolveMaterialFileVersion(version, fallback uint64) uint64 {
	if version != 0 {
		return version
	}
	return fallback
}

func isLegacyMaterialLayout(version uint64) bool {
	return version <= 18
}

func supportsTextureURI(version uint64) bool {
	return version >= 20
}

func supportsSamplerState(version uint64) bool {
	return version >= 21
}

func supportsUniformOverrides(version uint64) bool {
	return version >= 22
}

func supportsOutputBindingSignature(version uint64) bool {
	return version >= 23
}

func (c materialCodec) parseSamplerDefinitions(d *decoder) ([]NamedSamplerDefinition, error) {
	count, err := d.readU8()
	if err != nil {
		return nil, err
	}

	items := make([]NamedSamplerDefinition, int(count))
	for i := range items {
		name, err := d.readString()
		if err != nil {
			return nil, err
		}
		sampler, err := parseSamplerDefinition(d, c.materialFileVersion)
		if err != nil {
			return nil, err
		}
		items[i] = NamedSamplerDefinition{Name: name, SamplerDefinition: sampler}
	}
	return items, nil
}

func (c materialCodec) writeSamplerDefinitions(e *encoder, samplers []NamedSamplerDefinition) error {
	if err := writeCountU8(e, len(samplers), "sampler definitions"); err != nil {
		return err
	}
	for _, sampler := range samplers {
		if err := e.writeString(sampler.Name); err != nil {
			return err
		}
		if err := sampler.SamplerDefinition.write(e, c.materialFileVersion); err != nil {
			return err
		}
	}
	return nil
}

func (c materialCodec) parsePropertyFields(d *decoder) ([]NamedPropertyField, error) {
	count, err := d.readU16()
	if err != nil {
		return nil, err
	}

	items := make([]NamedPropertyField, int(count))
	for i := range items {
		name, err := d.readString()
		if err != nil {
			return nil, err
		}
		field, err := parsePropertyField(d, c.materialFileVersion)
		if err != nil {
			return nil, err
		}
		items[i] = NamedPropertyField{Name: name, PropertyField: field}
	}
	return items, nil
}

func (c materialCodec) writePropertyFields(e *encoder, fields []NamedPropertyField) error {
	if err := writeCountU16(e, len(fields), "property fields"); err != nil {
		return err
	}
	for _, field := range fields {
		if err := e.writeString(field.Name); err != nil {
			return err
		}
		if err := field.PropertyField.write(e, c.materialFileVersion); err != nil {
			return err
		}
	}
	return nil
}

func (c materialCodec) parseUniformOverrides(d *decoder, materialName string) (*[]StringPair, error) {
	if !c.hasUniformOverrides(materialName) {
		return nil, nil
	}

	count, err := d.readU16()
	if err != nil {
		return nil, err
	}
	items, err := readStringPairs(d, int(count), c.materialFileVersion)
	if err != nil {
		return nil, err
	}
	return &items, nil
}

func (c materialCodec) writeUniformOverrides(e *encoder, materialName string, overrides *[]StringPair) error {
	if !c.hasUniformOverrides(materialName) {
		return nil
	}

	if overrides == nil {
		return e.writeU16(0)
	}
	if err := writeCountU16(e, len(*overrides), "uniform overrides"); err != nil {
		return err
	}
	return writeStringPairs(e, *overrides, c.materialFileVersion)
}

func (c materialCodec) hasUniformOverrides(materialName string) bool {
	return supportsUniformOverrides(c.materialFileVersion) && materialName != coreBuiltinsName
}

func (c materialCodec) parsePasses(d *decoder) ([]NamedPass, error) {
	count, err := d.readU16()
	if err != nil {
		return nil, err
	}

	items := make([]NamedPass, int(count))
	for i := range items {
		name, err := d.readString()
		if err != nil {
			return nil, err
		}
		pass, err := parsePass(d, c.materialFileVersion)
		if err != nil {
			return nil, err
		}
		items[i] = NamedPass{Name: name, Pass: pass}
	}
	return items, nil
}

func (c materialCodec) writePasses(e *encoder, passes []NamedPass) error {
	if err := writeCountU16(e, len(passes), "passes"); err != nil {
		return err
	}
	for _, pass := range passes {
		if err := e.writeString(pass.Name); err != nil {
			return err
		}
		if err := pass.Pass.write(e, c.materialFileVersion); err != nil {
			return err
		}
	}
	return nil
}

func readStringPair(d *decoder, _ uint64) (StringPair, error) {
	key, err := d.readString()
	if err != nil {
		return StringPair{}, err
	}
	value, err := d.readString()
	if err != nil {
		return StringPair{}, err
	}
	return StringPair{Key: key, Value: value}, nil
}

func readStringPairs(d *decoder, count int, materialFileVersion uint64) ([]StringPair, error) {
	if count < 0 {
		return nil, fmt.Errorf("negative string pair count: %d", count)
	}

	items := make([]StringPair, count)
	for i := range items {
		item, err := readStringPair(d, materialFileVersion)
		if err != nil {
			return nil, err
		}
		items[i] = item
	}
	return items, nil
}

func writeStringPair(e *encoder, pair StringPair, _ uint64) error {
	if err := e.writeString(pair.Key); err != nil {
		return err
	}
	return e.writeString(pair.Value)
}

func writeStringPairs(e *encoder, pairs []StringPair, materialFileVersion uint64) error {
	for _, pair := range pairs {
		if err := writeStringPair(e, pair, materialFileVersion); err != nil {
			return err
		}
	}
	return nil
}

type EncryptionVariant uint32

const (
	EncryptionVariantNone             EncryptionVariant = 0x4E4F4E45
	EncryptionVariantSimplePassphrase EncryptionVariant = 0x534D504C
	EncryptionVariantKeyPair          EncryptionVariant = 0x4B595052
)

func parseEncryptionVariant(v uint32) (EncryptionVariant, error) {
	switch EncryptionVariant(v) {
	case EncryptionVariantNone, EncryptionVariantSimplePassphrase, EncryptionVariantKeyPair:
		return EncryptionVariant(v), nil
	default:
		return 0, fmt.Errorf("invalid encryption variant: 0x%X", v)
	}
}

func (e EncryptionVariant) IsEncrypted() bool {
	return e != EncryptionVariantNone
}
