package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/spencer-p/palace/pkg/auth"
)

func main() {
	fmt.Printf("%s", b64(auth.SaltAndHash(os.Args[1])))
}

func b64(in []byte) []byte {
	var b bytes.Buffer
	enc := base64.NewEncoder(base64.URLEncoding, &b)
	if _, err := enc.Write(in); err != nil {
		panic(err)
	}
	if err := enc.Close(); err != nil {
		panic(err)
	}
	return b.Bytes()
}
