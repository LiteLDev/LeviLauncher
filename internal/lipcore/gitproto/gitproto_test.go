package gitproto

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func pkt(lines ...string) []byte {
	w := new(PktLineWriter)
	for _, l := range lines {
		switch l {
		case "0000":
			w.WriteFlush()
		case "0001":
			w.WriteDelimiter()
		case "0002":
			w.buf.WriteString("0002")
		default:
			w.WriteLine(l)
		}
	}
	return w.Bytes()
}

func band(ch byte, data []byte) []byte {
	return new(PktLineWriter).WriteData(append([]byte{ch}, data...)).Bytes()
}

func TestPktLineRoundTrip(t *testing.T) {
	data := pkt("hello", "0001", "world", "0000", "0002")
	if want := "000ahello\n0001000aworld\n00000002"; string(data) != want {
		t.Fatalf("got %q want %q", data, want)
	}
	r := NewPktLineReader(bytes.NewReader(data))
	var got []string
	for {
		typ, err := r.Read()
		if err != nil {
			t.Fatal(err)
		}
		if typ == PktEndOfStream {
			break
		}
		if typ == PktData {
			got = append(got, r.PayloadText())
		} else {
			got = append(got, typ.String())
		}
	}
	want := []string{"hello", "Delimiter", "world", "Flush", "ResponseEnd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestPktLineErrors(t *testing.T) {
	for _, in := range []string{"00", "zzzz", "0003", "fff1", "0009abc", "000A1234"} {
		r := NewPktLineReader(strings.NewReader(in))
		_, err := r.Read()
		var pe *ProtocolError
		if !errors.As(err, &pe) {
			t.Errorf("%q: expected ProtocolError, got %v", in, err)
		}
	}
	r := NewPktLineReader(strings.NewReader("000A12345\n"))
	if typ, err := r.Read(); err != nil || typ != PktData || r.PayloadText() != "12345" {
		t.Fatalf("upper-case length: %v %v %q", typ, err, r.PayloadText())
	}
}

func TestRepoKey(t *testing.T) {
	cases := map[string]string{
		"https://github.com/LiteLDev/LeviLamina.git":  "https://github.com/liteldev/levilamina",
		"https://GitHub.com/LiteLDev/LeviLamina/":     "https://github.com/liteldev/levilamina",
		"https://gitee.com/LiteLDev/LeviLamina.git":   "https://gitee.com/LiteLDev/LeviLamina.git",
		"https://github.com.evil/LiteLDev/x.git":      "https://github.com.evil/LiteLDev/x.git",
		"https://user@github.com:443/LiteLDev/x.git/": "https://user@github.com:443/liteldev/x",
	}
	for in, want := range cases {
		if got := RepoKey(in); got != want {
			t.Errorf("RepoKey(%q) = %q want %q", in, got, want)
		}
	}
}
