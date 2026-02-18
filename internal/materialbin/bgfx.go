package materialbin

import "fmt"

type BGFXShader struct {
	Magic      uint32
	Hash       uint32
	Uniforms   []Uniform
	Code       []byte
	Attributes []uint16
	Size       *uint16
}

func ParseBGFXShader(data []byte, materialFileVersion uint64) (*BGFXShader, error) {
	d := newDecoder(data)
	shader, err := parseBGFXShader(d, materialFileVersion)
	if err != nil {
		return nil, err
	}
	return &shader, nil
}

func parseBGFXShader(d *decoder, materialFileVersion uint64) (BGFXShader, error) {
	magic, err := d.readU32()
	if err != nil {
		return BGFXShader{}, err
	}
	hash, err := d.readU32()
	if err != nil {
		return BGFXShader{}, err
	}

	uniforms, err := parseBGFXUniforms(d, materialFileVersion)
	if err != nil {
		return BGFXShader{}, err
	}

	code, err := parseBGFXCode(d, materialFileVersion)
	if err != nil {
		return BGFXShader{}, err
	}

	if _, err := d.readU8(); err != nil {
		return BGFXShader{}, err
	}

	attributes, size, err := parseBGFXAttributes(d, materialFileVersion)
	if err != nil {
		return BGFXShader{}, err
	}

	return BGFXShader{
		Magic:      magic,
		Hash:       hash,
		Uniforms:   uniforms,
		Code:       append([]byte(nil), code...),
		Attributes: attributes,
		Size:       size,
	}, nil
}

func parseBGFXUniforms(d *decoder, materialFileVersion uint64) ([]Uniform, error) {
	uniformCount, err := d.readU16()
	if err != nil {
		return nil, err
	}

	uniforms := make([]Uniform, int(uniformCount))
	for i := range uniforms {
		uniform, err := parseUniform(d, materialFileVersion)
		if err != nil {
			return nil, err
		}
		uniforms[i] = uniform
	}
	return uniforms, nil
}

func parseBGFXCode(d *decoder, _ uint64) ([]byte, error) {
	codeLen, err := d.readU32()
	if err != nil {
		return nil, err
	}
	if codeLen > uint32(d.remaining()) {
		return nil, fmt.Errorf("bgfx code length %d exceeds remaining %d", codeLen, d.remaining())
	}
	return d.read(int(codeLen))
}

func parseBGFXAttributes(d *decoder, _ uint64) ([]uint16, *uint16, error) {
	if d.remaining() == 0 {
		return nil, nil, nil
	}

	attrCount, err := d.readU8()
	if err != nil {
		return nil, nil, err
	}
	if attrCount == 0 {
		return nil, nil, nil
	}

	attributes := make([]uint16, int(attrCount))
	for i := range attributes {
		attr, err := d.readU16()
		if err != nil {
			return nil, nil, err
		}
		attributes[i] = attr
	}

	size, err := d.readU16()
	if err != nil {
		return nil, nil, err
	}
	return attributes, &size, nil
}

func (b *BGFXShader) MarshalBinary(materialFileVersion uint64) ([]byte, error) {
	e := &encoder{}
	if err := e.writeU32(b.Magic); err != nil {
		return nil, err
	}
	if err := e.writeU32(b.Hash); err != nil {
		return nil, err
	}
	if err := writeCountU16(e, len(b.Uniforms), "bgfx.uniforms"); err != nil {
		return nil, err
	}
	for i := range b.Uniforms {
		if err := b.Uniforms[i].write(e, materialFileVersion); err != nil {
			return nil, err
		}
	}
	if err := writeCountU32(e, len(b.Code), "bgfx.code"); err != nil {
		return nil, err
	}
	if err := e.writeBytes(b.Code); err != nil {
		return nil, err
	}
	if err := e.writeU8(0); err != nil {
		return nil, err
	}
	if len(b.Attributes) > 0 {
		if err := writeCountU8(e, len(b.Attributes), "bgfx.attributes"); err != nil {
			return nil, err
		}
		for _, attr := range b.Attributes {
			if err := e.writeU16(attr); err != nil {
				return nil, err
			}
		}
		if b.Size != nil {
			if err := e.writeU16(*b.Size); err != nil {
				return nil, err
			}
		}
	}
	return e.bytes(), nil
}

type Uniform struct {
	Name     string
	Type     uint8
	Num      uint8
	RegIndex uint16
	RegCount uint16
}

func parseUniform(d *decoder, _ uint64) (Uniform, error) {
	nameLen, err := d.readU8()
	if err != nil {
		return Uniform{}, err
	}
	nameBytes, err := d.read(int(nameLen))
	if err != nil {
		return Uniform{}, err
	}
	utype, err := d.readU8()
	if err != nil {
		return Uniform{}, err
	}
	num, err := d.readU8()
	if err != nil {
		return Uniform{}, err
	}
	regIndex, err := d.readU16()
	if err != nil {
		return Uniform{}, err
	}
	regCount, err := d.readU16()
	if err != nil {
		return Uniform{}, err
	}
	return Uniform{
		Name:     string(nameBytes),
		Type:     utype,
		Num:      num,
		RegIndex: regIndex,
		RegCount: regCount,
	}, nil
}

func (u *Uniform) write(e *encoder, _ uint64) error {
	if len(u.Name) > int(^uint8(0)) {
		return fmt.Errorf("uniform name length %d exceeds u8", len(u.Name))
	}
	if err := e.writeU8(uint8(len(u.Name))); err != nil {
		return err
	}
	if err := e.writeBytes([]byte(u.Name)); err != nil {
		return err
	}
	if err := e.writeU8(u.Type); err != nil {
		return err
	}
	if err := e.writeU8(u.Num); err != nil {
		return err
	}
	if err := e.writeU16(u.RegIndex); err != nil {
		return err
	}
	return e.writeU16(u.RegCount)
}
