package main

import (
	"flag"
	"fmt"
	"os"
	"encoding/base64"
	"io/ioutil"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/util/crypt"
)

func usage() {
	flag.Usage()
	fmt.Println("")
	fmt.Printf("flexbot-crypt --passphrase=<password phrase> [--sourceString <string to encrypt (STDIN by default)>]\n\n")
}

func encryptString(srcString []byte, passPhrase string) (encrypted string, err error) {
	var b []byte
	if b, err = crypt.Encrypt([]byte(srcString), passPhrase); err != nil {
		err = fmt.Errorf("EncryptString: Encrypt() failure: %s", err)
	} else {
		encrypted = "base64:" + base64.StdEncoding.EncodeToString(b)
	}
	return
}

func main() {
	optPassPhrase := flag.String("passphrase", "", "passphrase to encrypt string")
	optSourceString := flag.String("sourceString", "", "source string to encrypt")
	flag.Parse()
	if len(*optPassPhrase) == 0 {
		usage()
		return
	}
	var err error
	var srcString []byte
	if len(*optSourceString) == 0 {
		if srcString, err = ioutil.ReadAll(os.Stdin); err != nil {
			fmt.Printf("Error: %s\n", err)
			return
		}
	} else {
		srcString = []byte(*optSourceString)
	}
	var dstString string
	if dstString, err = encryptString(srcString, *optPassPhrase); err != nil {
		fmt.Printf("Error: %s\n", err)
	} else {
		fmt.Println(dstString)
	}
}
