package profile

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var entropy = []byte("WDTT Plus settings")

func protect(data []byte) ([]byte, error) {
	return cryptData(data, true)
}

func unprotect(data []byte) ([]byte, error) {
	return cryptData(data, false)
}

func cryptData(data []byte, encrypt bool) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	salt := windows.DataBlob{Size: uint32(len(entropy)), Data: &entropy[0]}
	var out windows.DataBlob
	var err error
	if encrypt {
		err = windows.CryptProtectData(&in, nil, &salt, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, &salt, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
