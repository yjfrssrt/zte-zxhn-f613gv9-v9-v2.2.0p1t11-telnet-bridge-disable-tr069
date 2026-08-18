package main

import (
	"bytes"
	"crypto/aes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	ontAddress = "192.168.1.1"
	httpsBase  = "https://192.168.1.1:443"
	telnetPort = 23
	clientRand = 0
)

// newKeyPool is the 96-byte interoperability constant used by the re_rand
// FactoryMode flow. The 24-byte session key starts at the derived index and
// each byte is XORed with 0xA5.
var newKeyPool = [...]byte{
	0x9c, 0x33, 0x75, 0xd1, 0x1c, 0x42, 0x45, 0x37, 0x18, 0x48,
	0x91, 0x73, 0x17, 0x45, 0x79, 0x44, 0x43, 0xd7, 0xd5, 0x73,
	0x33, 0x54, 0x76, 0xd2, 0xc5, 0xf1, 0x2c, 0x4f, 0x7a, 0xba,
	0x61, 0xd9, 0x5c, 0x69, 0xdf, 0x8c, 0xd2, 0x1c, 0xde, 0x3b,
	0x35, 0x2d, 0x2f, 0xe1, 0xde, 0x4c, 0x77, 0xf5, 0x1a, 0x65,
	0xd1, 0xfe, 0x18, 0x43, 0x8e, 0xa7, 0x42, 0x08, 0x04, 0x78,
	0xd5, 0xe4, 0xf3, 0x34, 0xa4, 0xd3, 0xf2, 0x36, 0x47, 0x6d,
	0x86, 0x9d, 0x42, 0x65, 0x13, 0x42, 0xdc, 0x42, 0x99, 0x48,
	0xdc, 0x67, 0x9f, 0x9e, 0xdc, 0x46, 0x37, 0x5f, 0x84, 0x9f,
	0x6f, 0x76, 0xce, 0x79, 0x4f, 0x49,
}

type factoryClient struct {
	http *http.Client
}

func newFactoryClient() *factoryClient {
	transport := &http.Transport{
		Proxy:               nil,
		DisableCompression:  true,
		ForceAttemptHTTP2:   false,
		TLSHandshakeTimeout: 5 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 15 * time.Second,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS10,
			InsecureSkipVerify: true, // Fixed private ONT address with a self-signed certificate.
		},
	}
	return &factoryClient{http: &http.Client{Transport: transport, Timeout: 12 * time.Second}}
}

func (c *factoryClient) openTempTelnet(username, password string, mac [6]byte) (string, string, error) {
	if _, _, err := c.request(http.MethodGet, "/", nil); err != nil {
		return "", "", fmt.Errorf("activate ONT Web service: %w", err)
	}

	resetBody, resetStatus, err := c.request(http.MethodPost, "/webFac", []byte("SendSq.gch"))
	if err != nil {
		return "", "", fmt.Errorf("reset factory session: %w", err)
	}
	if resetStatus != http.StatusBadRequest && !(resetStatus == http.StatusOK && len(resetBody) == 0) {
		return "", "", fmt.Errorf("reset factory session: unexpected HTTP %d", resetStatus)
	}

	_, _, err = c.request(http.MethodPost, "/webFac", []byte("RequestFactoryMode.gch"))
	if err != nil && !transportEndedAsExpected(err) {
		return "", "", fmt.Errorf("request factory mode: %w", err)
	}

	randRequest := []byte(fmt.Sprintf("SendSq.gch?rand=%d\r\n", clientRand))
	randBody, randStatus, err := c.request(http.MethodPost, "/webFac", randRequest)
	if err != nil {
		return "", "", fmt.Errorf("request re_rand: %w", err)
	}
	if randStatus != http.StatusOK {
		return "", "", fmt.Errorf("request re_rand: HTTP %d", randStatus)
	}
	serverRand, _, serverMAC, err := parseReRand(randBody)
	if err != nil {
		return "", "", err
	}
	key, index := deriveSessionKey(clientRand, serverRand)
	logf("re_rand accepted: index=%d, server_mac=%x", index, serverMAC)

	infoCommand := append([]byte("SendInfo.gch?info=12|"), macMagicPayload(mac)...)
	infoCipher, err := encryptZeroPadded(infoCommand, key)
	if err != nil {
		return "", "", err
	}
	_, infoStatus, err := c.request(http.MethodPost, "/webFacEntry", infoCipher)
	if err != nil {
		return "", "", fmt.Errorf("send client MAC proof: %w", err)
	}
	if infoStatus != http.StatusOK {
		return "", "", fmt.Errorf("send client MAC proof: HTTP %d", infoStatus)
	}

	loginCommand := []byte("CheckLoginAuth.gch?version50&user=" + username + "&pass=" + password)
	loginCipher, err := encryptZeroPadded(loginCommand, key)
	if err != nil {
		return "", "", err
	}
	loginBody, loginStatus, err := c.request(http.MethodPost, "/webFacEntry", loginCipher)
	if err != nil {
		return "", "", fmt.Errorf("check factory login: %w", err)
	}
	if loginStatus != http.StatusOK {
		return "", "", fmt.Errorf("check factory login: HTTP %d", loginStatus)
	}
	if err := verifyFactoryMarker(loginBody, key); err != nil {
		return "", "", err
	}

	modeCipher, err := encryptZeroPadded([]byte("FactoryMode.gch?mode=2&user=notused"), key)
	if err != nil {
		return "", "", err
	}
	modeBody, modeStatus, err := c.request(http.MethodPost, "/webFacEntry", modeCipher)
	if err != nil {
		return "", "", fmt.Errorf("enter factory mode: %w", err)
	}
	if modeStatus != http.StatusOK {
		return "", "", fmt.Errorf("enter factory mode: HTTP %d", modeStatus)
	}
	plain, err := decryptAligned(modeBody, key)
	if err != nil {
		return "", "", fmt.Errorf("decrypt factory credentials: %w", err)
	}
	return parseTemporaryCredentials(bytes.TrimRight(plain, "\x00"))
}

func (c *factoryClient) request(method, path string, body []byte) ([]byte, int, error) {
	request, err := http.NewRequest(method, httpsBase+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("User-Agent", "f613gv9-factory-telnet/1")
	request.Header.Set("Connection", "close")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	return responseBody, response.StatusCode, err
}

func transportEndedAsExpected(err error) bool {
	return errors.Is(err, io.EOF) || strings.Contains(strings.ToLower(err.Error()), "eof")
}

func parseReRand(body []byte) (int, int, [6]byte, error) {
	var serverMAC [6]byte
	parts := bytes.SplitN(body, []byte("&"), 3)
	if len(parts) != 3 || !bytes.HasPrefix(parts[0], []byte("re_rand=")) {
		return 0, 0, serverMAC, fmt.Errorf("unexpected re_rand response")
	}
	serverRand, err := strconv.Atoi(string(bytes.TrimPrefix(parts[0], []byte("re_rand="))))
	if err != nil || serverRand < 0 {
		return 0, 0, serverMAC, fmt.Errorf("invalid server random")
	}
	seed, err := strconv.Atoi(string(parts[1]))
	if err != nil || seed < 0 {
		return 0, 0, serverMAC, fmt.Errorf("invalid random seed")
	}
	if len(parts[2]) != 6 {
		return 0, 0, serverMAC, fmt.Errorf("server MAC has length %d, want 6", len(parts[2]))
	}
	copy(serverMAC[:], parts[2])
	return serverRand, seed, serverMAC, nil
}

func deriveSessionKey(clientRandom, serverRandom int) ([]byte, int) {
	mixed := uint32(0x1000193) * uint32(clientRandom)
	masked := mixed & uint32(0x8000003f)
	index := int((masked ^ uint32(serverRandom)) % 60)
	key := make([]byte, 24)
	for i := range key {
		key[i] = newKeyPool[index+i] ^ 0xA5
	}
	return key, index
}

func encryptZeroPadded(plain, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	padding := block.BlockSize() - len(plain)%block.BlockSize()
	padded := append(append([]byte(nil), plain...), make([]byte, padding)...)
	ciphertext := make([]byte, len(padded))
	for offset := 0; offset < len(padded); offset += block.BlockSize() {
		block.Encrypt(ciphertext[offset:offset+block.BlockSize()], padded[offset:offset+block.BlockSize()])
	}
	return ciphertext, nil
}

func decryptAligned(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) == 0 || len(ciphertext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("ciphertext length %d is not a positive AES block multiple", len(ciphertext))
	}
	plain := make([]byte, len(ciphertext))
	for offset := 0; offset < len(ciphertext); offset += block.BlockSize() {
		block.Decrypt(plain[offset:offset+block.BlockSize()], ciphertext[offset:offset+block.BlockSize()])
	}
	return plain, nil
}

func verifyFactoryMarker(response, key []byte) error {
	remainder := len(response) % aes.BlockSize
	if remainder != 0 && remainder != 12 {
		return fmt.Errorf("unsupported webFacEntry suffix length %d", remainder)
	}
	aligned := len(response) - remainder
	plain, err := decryptAligned(response[:aligned], key)
	if err != nil {
		return fmt.Errorf("decrypt factory marker: %w", err)
	}
	marker := bytes.TrimRight(plain, "\x00")
	if !bytes.Equal(marker, []byte("FactoryMode.gch")) {
		return fmt.Errorf("factory marker mismatch")
	}
	if remainder == 12 {
		logf("accepted F613GV9 compatibility suffix: raw=%d, AES=%d, suffix=12", len(response), aligned)
	}
	return nil
}

func parseTemporaryCredentials(plain []byte) (string, string, error) {
	question := bytes.IndexByte(plain, '?')
	if question < 0 || question == len(plain)-1 {
		return "", "", fmt.Errorf("factory response has no query credentials")
	}
	values, err := url.ParseQuery(string(plain[question+1:]))
	if err != nil {
		return "", "", fmt.Errorf("parse factory credentials: %w", err)
	}
	user := values.Get("user")
	pass := values.Get("pass")
	if !credentialPattern.MatchString(user) || !credentialPattern.MatchString(pass) {
		return "", "", fmt.Errorf("factory response contains invalid credentials")
	}
	return user, pass, nil
}

func logf(format string, args ...any) {
	fmt.Printf("[protocol] "+format+"\n", args...)
}
