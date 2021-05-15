package main

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/cloudflare/cfssl/helpers"
	"github.com/joho/godotenv"
	"github.com/mayocream/hath-go/pkg/hath"
	"github.com/spf13/viper"
	"golang.org/x/crypto/pkcs12"
)

func init() {
	godotenv.Load(".env")
	viper.AutomaticEnv()
}

func main() {
	clientKey := viper.GetString("HATH_CLIENT_KEY")
	hc, err := hath.NewClient(hath.Settings{
		ClientID:  viper.GetString("HATH_CLIENT_ID"),
		ClientKey: clientKey,
	})
	if err != nil {
		panic(err)
	}

	data, err := hc.GetRawPKCS12()
	if err != nil {
		panic(err)
	}

	// we should using pkcs12 topem method to remove unsupported tags
	pemBlocks, err := pkcs12.ToPEM(data, clientKey)
	if err != nil {
		panic(err)
	}

	// we failed to decode raw bytes by pkcs12 package
	// for its safe bags not equals 2
	if _, _, err := pkcs12.Decode(data, clientKey); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	for _, block := range pemBlocks {
		log.Println("block type: ", block.Type)
		p := pem.EncodeToMemory(block)
		fmt.Println(string(p))

		if block.Type == "CERTIFICATE" {
			if _, err := helpers.ParseCertificatePEM(p); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		} else if block.Type == "PRIVATE KEY" {
			if _, err := helpers.ParsePrivateKeyPEMWithPassword(p, []byte(clientKey)); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}

	ioutil.WriteFile("hathcert.p12", data, 7777)
}
