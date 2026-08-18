package main

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

func verifyTelnet(username, password string) error {
	var connection net.Conn
	var lastErr error
	for attempt := 1; attempt <= 8; attempt++ {
		connection, lastErr = net.DialTimeout("tcp", net.JoinHostPort(ontAddress, strconv.Itoa(telnetPort)), 3*time.Second)
		if lastErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if lastErr != nil {
		return fmt.Errorf("Telnet service did not open: %w", lastErr)
	}
	defer connection.Close()

	if err := waitForTelnet(connection, 10*time.Second, "ogin:", "sername:"); err != nil {
		return err
	}
	if err := sendTelnetLine(connection, username); err != nil {
		return err
	}
	if err := waitForTelnet(connection, 10*time.Second, "assword:"); err != nil {
		return err
	}
	if err := sendTelnetLine(connection, password); err != nil {
		return err
	}
	if err := waitForTelnet(connection, 10*time.Second, "#", "$"); err != nil {
		return fmt.Errorf("Telnet credentials rejected: %w", err)
	}
	return nil
}

func sendTelnetLine(connection net.Conn, line string) error {
	payload := []byte(line + "\r\n")
	written, err := connection.Write(payload)
	if err != nil {
		return err
	}
	if written != len(payload) {
		return fmt.Errorf("short Telnet write: %d of %d", written, len(payload))
	}
	return nil
}

func waitForTelnet(connection net.Conn, timeout time.Duration, patterns ...string) error {
	deadline := time.Now().Add(timeout)
	buffer := make([]byte, 1024)
	var raw bytes.Buffer
	for {
		visible := stripTelnetControl(raw.Bytes())
		for _, pattern := range patterns {
			if strings.Contains(visible, pattern) {
				return nil
			}
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("timeout waiting for %s", strings.Join(patterns, " or "))
		}
		_ = connection.SetReadDeadline(deadline)
		read, err := connection.Read(buffer)
		if read > 0 {
			raw.Write(buffer[:read])
		}
		if err != nil {
			var networkError net.Error
			if errors.As(err, &networkError) && networkError.Timeout() {
				continue
			}
			return err
		}
	}
}

func stripTelnetControl(input []byte) string {
	const (
		iac  = 0xff
		sb   = 0xfa
		se   = 0xf0
		will = 0xfb
		dont = 0xfe
	)
	visible := make([]byte, 0, len(input))
	for index := 0; index < len(input); {
		if input[index] != iac {
			visible = append(visible, input[index])
			index++
			continue
		}
		if index+1 >= len(input) {
			break
		}
		switch input[index+1] {
		case iac:
			visible = append(visible, iac)
			index += 2
		case sb:
			end := index + 2
			for end+1 < len(input) && !(input[end] == iac && input[end+1] == se) {
				end++
			}
			index = end + 2
		default:
			if input[index+1] >= will && input[index+1] <= dont {
				index += 3
			} else {
				index += 2
			}
		}
	}
	return string(visible)
}
