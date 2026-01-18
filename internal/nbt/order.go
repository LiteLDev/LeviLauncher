package nbt

import (
	"bytes"
)

// KeysInOrder returns the ordered list of keys for a compound at the given path.
// The data parameter must be the raw NBT payload (without any custom headers).
func KeysInOrder(data []byte, encoding Encoding, path []string) ([]string, error) {
	r := newOffsetReader(bytes.NewReader(data))
	// Read root tag type
	tByte, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	t := tagType(tByte)
	if !t.IsValid() {
		return nil, UnknownTagError{Off: r.off, Op: "KeysInOrder", TagType: t}
	}
	// Read root name if not End
	if t != tagEnd {
		if _, err := encoding.String(r); err != nil {
			return nil, err
		}
	}
	// We expect a root compound
	if t != tagStruct {
		return nil, InvalidTypeError{Off: r.off, FieldType: nil, Field: "<root>", TagType: t}
	}
	// Traverse to target compound
	keys, err := keysInCompound(r, encoding, path)
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// keysInCompound scans a compound body and either returns its keys order, or finds a nested compound by path.
func keysInCompound(r *offsetReader, encoding Encoding, path []string) ([]string, error) {
	// If path is empty, collect keys at current level
	if len(path) == 0 {
		out := []string{}
		for {
			tByte, err := r.ReadByte()
			if err != nil {
				return nil, err
			}
			tt := tagType(tByte)
			if tt == tagEnd {
				break
			}
			name, err := encoding.String(r)
			if err != nil {
				return nil, err
			}
			out = append(out, name)
			if err := skipTagValue(r, encoding, tt); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	// Otherwise, find child compound with matching name and descend
	next := path[0]
	rest := path[1:]
	for {
		tByte, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		tt := tagType(tByte)
		if tt == tagEnd {
			break
		}
		name, err := encoding.String(r)
		if err != nil {
			return nil, err
		}
		if tt == tagStruct && name == next {
			// Found target child: recurse into its body
			return keysInCompound(r, encoding, rest)
		}
		// Skip value for non-targets
		if err := skipTagValue(r, encoding, tt); err != nil {
			return nil, err
		}
	}
	return []string{}, nil
}

// skipTagValue advances the reader past the value body of a tag of type tt.
func skipTagValue(r *offsetReader, encoding Encoding, tt tagType) error {
	switch tt {
	case tagByte:
		_, err := r.ReadByte()
		return err
	case tagInt16:
		_, err := encoding.Int16(r)
		return err
	case tagInt32:
		_, err := encoding.Int32(r)
		return err
	case tagInt64:
		_, err := encoding.Int64(r)
		return err
	case tagFloat32:
		_, err := encoding.Float32(r)
		return err
	case tagFloat64:
		_, err := encoding.Float64(r)
		return err
	case tagString:
		_, err := encoding.String(r)
		return err
	case tagByteArray:
		// length followed by bytes
		n, err := encoding.Int32(r)
		if err != nil {
			return err
		}
		_ = r.Next(int(n))
		return nil
	case tagInt32Array:
		n, err := encoding.Int32(r)
		if err != nil {
			return err
		}
		_ = r.Next(int(n) * 4)
		return nil
	case tagInt64Array:
		n, err := encoding.Int32(r)
		if err != nil {
			return err
		}
		_ = r.Next(int(n) * 8)
		return nil
	case tagSlice:
		// listType + length + elements
		listTypeByte, err := r.ReadByte()
		if err != nil {
			return err
		}
		listType := tagType(listTypeByte)
		if !listType.IsValid() {
			return UnknownTagError{Off: r.off, Op: "SkipList", TagType: listType}
		}
		n, err := encoding.Int32(r)
		if err != nil {
			return err
		}
		switch listType {
		case tagByte:
			_ = r.Next(int(n))
			return nil
		case tagInt32:
			_ = r.Next(int(n) * 4)
			return nil
		case tagInt64:
			_ = r.Next(int(n) * 8)
			return nil
		case tagFloat32:
			_ = r.Next(int(n) * 4)
			return nil
		case tagFloat64:
			_ = r.Next(int(n) * 8)
			return nil
		case tagString:
			for i := int32(0); i < n; i++ {
				if _, err := encoding.String(r); err != nil {
					return err
				}
			}
			return nil
		case tagStruct, tagSlice, tagByteArray, tagInt32Array, tagInt64Array:
			for i := int32(0); i < n; i++ {
				if err := skipTagValue(r, encoding, listType); err != nil {
					return err
				}
			}
			return nil
		default:
			return nil
		}
	case tagStruct:
		// Nested compound: read until TAG_End, skipping contents
		for {
			tByte, err := r.ReadByte()
			if err != nil {
				return err
			}
			nt := tagType(tByte)
			if nt == tagEnd {
				break
			}
			if _, err := encoding.String(r); err != nil {
				return err
			}
			if err := skipTagValue(r, encoding, nt); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}
