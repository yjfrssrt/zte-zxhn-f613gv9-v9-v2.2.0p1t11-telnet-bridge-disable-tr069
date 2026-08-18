package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
)

var credentialPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func main() {
	macText := flag.String("mac", "", "MAC of the Ethernet interface directly connected to the ONT")
	username := flag.String("username", "CMCCAdmin", "factory-mode Web username")
	password := flag.String("password", "", "factory-mode Web password")
	flag.Parse()

	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.SetPrefix("f613gv9: ")

	if *macText == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "both -mac and -password are required")
		flag.Usage()
		os.Exit(2)
	}
	mac, err := parseMAC(*macText)
	if err != nil {
		log.Fatal(err)
	}
	if !credentialPattern.MatchString(*username) {
		log.Fatal("username contains unsupported characters")
	}

	client := newFactoryClient()
	tempUser, tempPass, err := client.openTempTelnet(*username, *password, mac)
	if err != nil {
		log.Fatal(err)
	}
	if err := verifyTelnet(tempUser, tempPass); err != nil {
		log.Fatalf("factory flow returned credentials, but real Telnet login failed: %v", err)
	}

	fmt.Println()
	fmt.Println("Temporary Telnet verified with a real shell login.")
	fmt.Printf("Temporary username: %s\n", tempUser)
	fmt.Printf("Temporary password: %s\n", tempPass)
}

func parseMAC(text string) ([6]byte, error) {
	var result [6]byte
	normalized := strings.NewReplacer("-", "", ":", "", ".", "", " ", "").Replace(text)
	if len(normalized) != 12 {
		return result, fmt.Errorf("MAC must contain 12 hexadecimal digits")
	}
	hw, err := net.ParseMAC(normalized[0:2] + ":" + normalized[2:4] + ":" + normalized[4:6] + ":" + normalized[6:8] + ":" + normalized[8:10] + ":" + normalized[10:12])
	if err != nil || len(hw) != 6 {
		return result, fmt.Errorf("invalid MAC %q", text)
	}
	copy(result[:], hw)
	return result, nil
}
