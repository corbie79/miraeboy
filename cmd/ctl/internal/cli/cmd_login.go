package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// CmdLogin authenticates and saves the token to ~/.miraeboy/config.json.
func CmdLogin(client *Client, cfg *Config, args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	user := fs.String("user", "", "사용자 이름")
	pass := fs.String("password", "", "비밀번호")
	server := fs.String("server", cfg.ServerURL, "서버 URL")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *server != "" {
		cfg.ServerURL = *server
		client.cfg.ServerURL = *server
	}

	scanner := bufio.NewScanner(os.Stdin)
	if *user == "" {
		fmt.Print("Username: ")
		scanner.Scan()
		*user = strings.TrimSpace(scanner.Text())
	}
	if *pass == "" {
		fmt.Print("Password: ")
		p, err := readPassword()
		if err != nil {
			// fallback: plain scan
			scanner.Scan()
			p = strings.TrimSpace(scanner.Text())
		}
		*pass = p
		fmt.Println()
	}

	token, err := client.WebLogin(*user, *pass)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	cfg.Token = token
	if err := SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Logged in as %s → token saved to %s\n", *user, configPath())
	return nil
}

// readPassword reads a password from stdin without echoing on POSIX systems.
func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	// Get current terminal state.
	var oldState [2]uint32 // enough space for termios on any POSIX
	type termios struct {
		Iflag, Oflag, Cflag, Lflag uint32
		Cc                         [20]byte
		Ispeed, Ospeed             uint32
	}
	var t termios
	// TCGETS = 0x5401 on Linux
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(fd), 0x5401, uintptr(unsafe.Pointer(&t))); errno != 0 {
		return "", errno
	}
	_ = oldState
	orig := t
	// Turn off echo (ECHO = 0x8, ECHONL = 0x40)
	t.Lflag &^= 0x8
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(fd), 0x5402, uintptr(unsafe.Pointer(&t))); errno != 0 {
		return "", errno
	}
	defer syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(fd), 0x5402, uintptr(unsafe.Pointer(&orig)))

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text()), nil
}

