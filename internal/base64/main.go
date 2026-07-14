package main

import (
	"encoding/base64"
	"log"
)

func main() {
	data := []byte{0xfb, 0xff, 0xbe}

	stdEncoded := base64.StdEncoding.EncodeToString(data)

	log.Printf("%s\n", stdEncoded)

	stdDecoded, err := base64.StdEncoding.DecodeString(stdEncoded)
	if err != nil {
		log.Fatalf("Decoding failed: %v", err)
	}
	log.Printf("%x\n", stdDecoded)

	urlEncoded := base64.URLEncoding.EncodeToString(stdDecoded)

	log.Printf("%s\n", urlEncoded)
}

// StdBase64 でエンコードしたやつを StdBase64 でデコードして
// それを URLBase64 でエンコードする
