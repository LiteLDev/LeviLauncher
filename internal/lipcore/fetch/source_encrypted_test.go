package fetch

import (
	"io"
	"testing"
)

// An encrypted 7z entry cannot be read without a password. sevenzip returns
// the raw ciphertext of a stored entry without an error; SharpCompress throws, so lip aborts the
// install. Reading must fail rather than yield the ciphertext.
func TestSevenZipEncryptedEntryFails(t *testing.T) {
	// py7zr, password "secret", plaintext headers: a.txt (220 bytes, stored + AES).
	data := mustHex(t, "377abcaf271c000470dc44774d010000000000001500000000000000ddb166c0e948d72a8f306d96f2a25e0203def9c9584a27b7dff5d9b962025e8c1306c971d809cef3a4fecf0730a5e0ca003c7bbf35ad85f3fcad8c80bf7b6e831fef4ca1b1a48262cbf238171abd3751267c57d23ee2e407e54214b89e86ea3922627b08903086f19c1561a3663b03bed9c89712e00a242eab1fb1c55a2890994f773e242ab5b685639a13dbcdc23996c31c972e247a0b4dd8444b4bd9f50756552c98cb71cd800258afd0e18ad8dadc66ee38fdaa7ab9891f5b0ec1e0d92b95138afe6fc7f2bc98304fc9d00b114946b316b8c19458df797f362b52139892e80d9cbe9601006801040600010980e00a015473d37800070b0100022406f1070112530ff4f96970101d0d1002e54cf3692d71ff010001000c80dc80dc00080a01e22a4b8200000501190100110d0061002e007400780074000000140a01003079a2c2b06ddd011506010020808081000000170680e001096d00070b010001212101180c690000")
	s, err := OpenArchive("zip", writeTemp(t, "enc.7z", data))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rc, err := s.Open("a.txt")
	if err != nil {
		return
	}
	defer rc.Close()
	if n, err := io.Copy(io.Discard, rc); err == nil {
		t.Fatalf("read %d bytes of an encrypted entry without error", n)
	}
}
