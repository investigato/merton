package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/investigato/go-psrp/client"
	"github.com/investigato/go-psrp/winrs"
	"github.com/investigato/prompt"
	flag "github.com/spf13/pflag"

	"github.com/investigato/merton/internal/utils"
)

var (
	version   = "dev"
	codename  = "unknown"
	buildTime = "unknown"
)

type ShellType int

const (
	PsrpShell ShellType = iota
	WinRsShell
)

type MertonShell struct {
	shellType ShellType
	cmdCWD    string
	servePort int
}

func (s ShellType) String() string {
	switch s {
	case PsrpShell:
		return "PS"
	case WinRsShell:
		return "CMD"
	default:
		return fmt.Sprintf("Unknown ShellType(%d)", s)
	}
}

func (sh *MertonShell) changeServerPort(port int) {
	sh.servePort = port
}

// setCWD takes in a Result object, extracts the Output, and sets the saved variable cmdCWD. this is only for WinRS
func (sh *MertonShell) setCWD(newCWD string) error {
	switch sh.shellType {
	case PsrpShell:
		sh.cmdCWD = newCWD
		return nil
	case WinRsShell:
		sh.cmdCWD = newCWD
		return nil
	default:
		return fmt.Errorf("unsupported shell type: %s", sh.shellType.String())
	}
}

func (sh *MertonShell) getCWD(ctx context.Context, c *client.Client) (string, error) {
	switch sh.shellType {
	case PsrpShell:
		cwdResult, err := c.Execute(ctx, "$pwd.Path")
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		currentDir := cwdResult.Output[0]
		if str, ok := currentDir.(string); ok {
			// remove leading and trailing whitespace, newlines, brackets, braces
			currentDirClean := strings.Trim(str, " \n\r\t[]{}")
			err = sh.setCWD(currentDirClean)
			if err != nil {
				return "", fmt.Errorf("failed to set current working directory: %w", err)
			}
			return currentDirClean, nil
		}
	case WinRsShell:
		if sh.cmdCWD != "" {
			return sh.cmdCWD, nil
		}
		// initial round only
		cwdResult, err := c.ExecuteCmd(ctx, "cd")
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		cmdCWDToResult := sh.toResult(cwdResult)
		if len(cmdCWDToResult.Output) == 0 {
			return "", fmt.Errorf("failed to get current working directory: no output from    command")
		}
		currentDir := cmdCWDToResult.Output[0]
		if str, ok := currentDir.(string); ok {
			currentDirClean := strings.Trim(str, " \n\r\t[]{}")
			currentDirClean = strings.ReplaceAll(currentDirClean, "/", "\\")
			err = sh.setCWD(currentDirClean)
			if err != nil {
				return "", fmt.Errorf("failed to set current working directory: %w", err)
			}
			return currentDirClean, nil
		}
	default:
		return "", fmt.Errorf("unsupported shell type: %s", sh.shellType.String())
	}
	return "", fmt.Errorf("failed to get current working directory: no output from command")
}
func (sh *MertonShell) toResult(cmd *client.CmdResult) *client.Result {
	// remap Stdout and Stderr to []interface{}
	output := make([]any, 0, 1)
	if cmd.Stdout != "" {
		output = append(output, cmd.Stdout)
	}

	errorList := make([]any, 0, 1)
	if cmd.Stderr != "" {
		errorList = append(errorList, cmd.Stderr)
	}

	return &client.Result{
		Output: output,
		Errors: errorList,
	}
}

func main() {
	var hostname string
	var port int
	var domain string
	var username string
	var password string
	var hashes string
	var usetls bool
	var insecure bool
	var targetspn string
	var realm string
	var loglevel string
	var kerberos bool
	var krb5conf string
	var krb5ccachepath string
	var usewinrs bool
	var enablecbt bool
	var kdcIP string
	var logFilePath string
	var serveport int
	var versionFlag bool
	flag.StringVarP(&hostname, "host", "i", "", "Hostname or IP address of the target")
	flag.IntVarP(&port, "port", "P", 5985, "Port number to connect to")
	flag.StringVarP(&domain, "domain", "d", "", "Domain name for authentication")
	flag.StringVarP(&username, "username", "u", "", "Username for authentication")
	flag.StringVarP(&password, "password", "p", "", "Password for authentication")
	flag.StringVarP(&hashes, "hashes", "H", "", "NT hash for authentication")
	flag.BoolVarP(&usetls, "tls", "t", false, "Use TLS for the connection")
	flag.BoolVarP(&insecure, "insecure", "", false, "Skip TLS certificate verification")
	flag.StringVarP(&targetspn, "target-spn", "", "", "Target Service Principal Name (SPN) for Kerberos authentication")
	flag.StringVarP(&realm, "realm", "r", "", "Kerberos realm for authentication")
	flag.StringVarP(&loglevel, "log-level", "", "", "Logging level (debug, info, warn, error)")
	flag.BoolVarP(&kerberos, "kerberos", "k", false, "Use Kerberos authentication")
	flag.StringVarP(&krb5conf, "krb5conf", "", "", "Path to krb5.conf file for Kerberos authentication")
	flag.StringVarP(&krb5ccachepath, "krb5ccache", "", "", "Path to Kerberos credential cache (ccache) file")
	flag.BoolVarP(&usewinrs, "winrs", "", false, "Use WinRS protocol instead of PSRP")
	flag.BoolVarP(&enablecbt, "enable-cbt", "", false, "Enable Client-to-Server Binding (CBT) for Kerberos authentication")
	flag.StringVarP(&kdcIP, "kdc-ip", "", "", "IP address of the Key Distribution Center (KDC) for Kerberos authentication")
	flag.StringVarP(&logFilePath, "log-file", "", "", "Path to log file for output")
	flag.IntVarP(&serveport, "serveport", "", 8080, "Port to use for serving files during upload/download operations")
	flag.BoolVarP(&versionFlag, "version", "V", false, "Display version information and exit")
	flag.Parse()
	// Configure the client
	if versionFlag {
		fmt.Printf("App Version: %s\nCodename: %s\nBuild Time: %s\n", version, codename, buildTime)
		os.Exit(0)
	}
	var shellTypeStarter ShellType
	if usewinrs {
		shellTypeStarter = WinRsShell
	} else {
		shellTypeStarter = PsrpShell
	}

	mertonShell := MertonShell{
		shellType: shellTypeStarter,
		cmdCWD:    "",
		servePort: serveport,
	}

	cfg := client.DefaultConfig()
	cfg.Hostname = hostname
	cfg.Username = username
	if password != "" {
		cfg.Password = password
	}
	if hashes != "" {
		parsedHash, err := utils.ParseHash(hashes)
		if err != nil {
			log.Fatalf("failed to parse hashes: %v", err)
		}
		cfg.NTHash = parsedHash

	}
	if usetls {
		cfg.UseTLS = true
	}
	if port != 5985 {
		cfg.Port = port
	}
	if insecure {
		cfg.InsecureSkipVerify = true
	}
	if targetspn != "" {
		cfg.TargetSPN = targetspn
	}
	if realm != "" {
		cfg.Realm = realm
	}

	if kerberos {
		cfg.AuthType = client.AuthKerberos
	}
	if enablecbt {
		cfg.EnableCBT = true
	}
	if kdcIP != "" {
		cfg.KdcIP = kdcIP
	}
	if krb5conf != "" {
		cfg.Krb5ConfPath = krb5conf
	}
	if krb5ccachepath != "" {
		cfg.CCachePath = krb5ccachepath
	}
	signals := make(chan os.Signal, 1)

	// Register the channel to receive SIGINT (Ctrl+C) and SIGTERM (standard termination signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Create the client
	c, err := client.New(hostname, cfg, loglevel)
	if err != nil {
		log.Fatal(err)
	}

	// Connect to the server
	if connectErr := c.Connect(ctx); connectErr != nil {
		log.Fatal(connectErr)
	}
	displayLogo(version, codename)
	var once sync.Once
	doCleanup := func() {
		once.Do(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cleanupCancel()
			if err = c.Close(cleanupCtx); err != nil {
				log.Printf("failed to close client: %v", err)
			}
		})
	}
	go func() {
		sig := <-signals
		log.Printf("Received signal: %s, initiating cleanup", sig.String())
		doCleanup() // no messes!
		os.Exit(0)
	}()

	mertonShell.cmdCWD, err = mertonShell.getCWD(ctx, c)
	if err != nil {
		log.Fatalf("failed to get initial working directory: %v", err)
	}
	historyConfig := &prompt.HistoryConfig{
		Enabled:     true,
		MaxEntries:  1000,
		File:        "./merton_history.log",
		MaxFileSize: 1024 * 1024, // 1MB
		MaxBackups:  3,
	}

	pr, err := prompt.New(mertonShell.formatPrompt(mertonShell.cmdCWD), prompt.WithHistory(historyConfig), prompt.WithMultiline(true), prompt.WithColorScheme(prompt.ThemeNightOwl))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if closeErr := pr.Close(); closeErr != nil {
			log.Printf("failed to close prompt: %v", closeErr)
		}
	}()

	line := ""
mainloop:
	for {
		if mertonShell.needsCWDRefresh(line) {
			cwd, cwdErr := mertonShell.getCWD(ctx, c)
			if cwdErr != nil {
				log.Printf("failed to get current working directory: %v", cwdErr)
			}
			pr.SetPrefix(mertonShell.formatPrompt(cwd))
		}

		line, err = pr.Run()
		if err != nil {
			if errors.Is(err, prompt.ErrInterrupted) {
				if len(line) == 0 {
					break
				}
			} else {
				log.Fatal(err)
			}
		}
		parts := utils.SplitCommandLine(line)
		if len(parts) == 0 {
			continue
		}
		pr.AddHistory(line)
		switch parts[0] {
		case "exit", "quit":
			fmt.Println("Thank you!")
			break mainloop
		case "serveport":
			if len(parts) != 2 {
				fmt.Println("Usage: serveport <port>")
				continue
			}
			newPort, err := utils.ParsePort(parts[1])
			if err != nil {
				fmt.Printf("Invalid port: %v\n", err)
				continue
			}
			mertonShell.changeServerPort(newPort)
			fmt.Printf("Server port changed to %d\n", newPort)
		case "chsh":
			if mertonShell.shellType == PsrpShell {
				mertonShell.shellType = WinRsShell
				cwd, cwdErr := mertonShell.getCWD(ctx, c)
				if cwdErr != nil {
					log.Fatalf("failed to get current working directory: %v", cwdErr)
				}
				pr.SetPrefix(mertonShell.formatPrompt(cwd))

			} else {
				mertonShell.shellType = PsrpShell
				cwd, cwdErr := mertonShell.getCWD(ctx, c)
				if cwdErr != nil {
					log.Fatalf("failed to get current working directory: %v", cwdErr)
				}
				pr.SetPrefix(mertonShell.formatPrompt(cwd))
			}
		case "download":
			Download(ctx, &mertonShell, c, line)
		case "upload":
			Upload(ctx, &mertonShell, c, line, hostname, serveport)
		default:
			result, err := mertonShell.dispatch(ctx, c, line)
			if err != nil {
				log.Fatal(err)
			}

			output := utils.ProcessResult(result)
			if output != "" {
				fmt.Println(output)
			}
		}

	}
	doCleanup()
}

func (sh *MertonShell) needsCWDRefresh(previous string) bool {
	if previous == "" {
		return true
	}
	first := strings.ToLower(strings.Fields(previous)[0])
	switch first {
	case "cd", "chdir", "set-location", "sl":
		return true
	}
	return false
}

func (sh *MertonShell) dispatch(ctx context.Context, c *client.Client, commandLine string) (*client.Result, error) {
	switch sh.shellType {
	case PsrpShell:
		command := commandHandler(commandLine)
		result, err := c.Execute(ctx, command)
		if err != nil {

			// we need to output the error and continue the shell
			return &client.Result{
				Output: nil,
				Errors: []any{err.Error()},
			}, nil

		}
		return result, nil
	case WinRsShell:
		parts := utils.SplitCommandLine(commandLine)
		switch strings.ToLower(parts[0]) {
		case "cd", "chdir":
			if len(parts) < 2 {
				return &client.Result{Output: []any{sh.cmdCWD}}, nil
			}
			if err := sh.setCWD(parts[1]); err != nil {
				return nil, err
			}
			return &client.Result{}, nil
		}
		cmdResult, err := c.ExecuteCmd(ctx, commandLine,
			winrs.WithWorkingDirectory(sh.cmdCWD))
		if err != nil {
			return &client.Result{
				Output: nil,
				Errors: []any{err.Error()},
			}, nil
		}
		return sh.toResult(cmdResult), nil
	default:
		return nil, fmt.Errorf("unsupported shell type: %s", sh.shellType.String())
	}
}

func commandHandler(commandLine string) string {
	// handle any necessary conversions from PowerShell to WinRS, specifically gci because it doesn't deserialize correctly over psrp
	var psrpCommandMap = map[string]string{
		"dir":                  "cmd /c dir /a",
		"ls":                   "cmd /c dir /a",
		"gci":                  "cmd /c dir /a",
		"get-childitem":        "cmd /c dir /a",
		"get-nettcpconnection": "cmd /c netstat -ano",
		"get-netipinterface":   "netsh interface show interface",
		"get-netadapter":       "ipconfig /all",
	}
	parts := utils.SplitCommandLine(commandLine)
	command := parts[0]
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}

	// check against the map, if not there, then return commandLine unchanged
	if cmd, ok := psrpCommandMap[strings.ToLower(command)]; ok {
		// remap together into a single command string to be executed by winrs
		if len(args) > 0 {
			// check if args has an /a in it and remove it since we add it automagically
			for i, arg := range args {
				if arg == "/a" {
					args = append(args[:i], args[i+1:]...)
					break
				}
				// if it has -recurse, then remove it and add /s to the winrs command
				if arg == "-recurse" {
					args = append(args[:i], args[i+1:]...)
					cmd += " /s"
					break
				}
			}
			if len(args) > 0 {
				lastArg := args[len(args)-1]
				if strings.HasSuffix(lastArg, "\\") && !strings.HasSuffix(lastArg, "\\\\") {
					args[len(args)-1] += "\\"
				}
			}
			return utils.SafeJoinCommandAndArgs(cmd, args)
		}
		return utils.SafeJoinCommandAndArgs(cmd, nil)
	}

	return commandLine
}

func (sh *MertonShell) formatPrompt(cwd string) string {
	return fmt.Sprintf("(%s)> %s ", sh.shellType.String(), cwd)
}

func displayLogo(version string, codename string) {
	logo := `
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⡠⣤⣤⣤⠤⣄⡀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⡤⠊⣡⡾⠛⠉⠛⢿⣿⠛⢶⡶⢦⡀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⡠⠴⠒⢺⣉⣉⡑⠒⠢⢄⡜⢠⢾⣿⡇⠀⠀⠀⠀⢻⣿⡿⠷⠤⣹⡄⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣠⠞⠁⢀⣠⠔⠉⠀⠀⠈⠉⠻⣉⡟⡉⣾⣿⣷⣄⣀⢀⣠⡿⠁⠀⠐⠄⠀⣷⡀
⠀⠀⠀⠀⠀⠀⠀⠀⣠⠊⢳⠔⠋⠁⠀⢱⠀⠀⠀⠀⠀⣀⢼⢣⠗⠉⠉⣙⠻⠿⠟⠋⠀⠀⠀⠀⠀⠀⠀⡇
⠀⠀⠀⠀⠀⠀⠀⡴⠁⢀⠏⠀⠀⠀⠀⡸⠀⠀⢀⡴⠛⢲⢊⣳⠀⠀⠋⠙⢦⣤⣀⣀⣀⣀⣀⡀⠀⠀⢀⡇
⠀⠀⠀⠀⠀⠀⢰⠁⠀⡸⠀⠀⠀⠀⢀⣇⣀⡴⠛⠂⣤⣾⣻⣿⣦⣀⠀⠀⠀⠉⠂⠬⠽⠿⠛⡩⠗⠔⠊⠀
⠀⠀⢀⡀⠀⡠⡏⠀⢀⡇⠀⠀⠀⣠⡯⠖⠙⢄⣠⣶⣿⣿⡟⣟⢫⡪⢫⣶⣶⡤⠤⠤⠤⠒⠋⠀⠀⠀⠀⠀
⠀⠀⠀⠻⢍⡯⠝⢢⠮⠬⡦⠒⠋⢹⠀⣀⣴⣾⣏⠀⠈⢧⣧⢽⣶⣬⣿⣿⣟⣀⡀⠤⣀⡀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⢀⣑⡶⣅⣀⣠⣁⣤⣤⣾⠿⠛⢞⣿⡿⡗⠂⠘⡄⠉⢚⠟⠛⡝⣉⠈⣀⣄⠀⢉⢢⡀⠀⠀⠀⠀
⠀⠀⡠⠞⢁⡛⠿⠉⠉⢹⢯⡽⠧⡄⢐⡮⡟⣭⢂⠁⠀⢠⡣⠴⣥⡴⢚⣾⡾⠦⢿⢉⠁⠀⠈⠝⡄⠀⠀⠀
⢠⣎⡀⠠⣴⣣⣀⣀⡰⠋⠀⢉⣳⣞⠁⠟⠷⠁⠀⠀⢠⠮⠔⠊⠁⣠⢎⠉⠋⠮⣼⣩⠁⠀⢀⡱⡇⠀⠀⠀
⠘⢢⣼⢤⣙⣻⠟⠋⠀⠀⠀⠘⢄⣼⣀⡀⠀⠀⣠⠔⠁⠀⠀⠀⠀⠙⠪⠶⠒⠊⠁⠙⠤⢴⡡⠟⠀⠀⠀⠀
⠀⠀⠈⠉⠁⠀⠀⠀⠀⠀⠀⠀⠈⠓⠒⠛⠒⠉⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠀⠀⠀⠀⠀⠀
   Merton %s - %s
`
	_, _ = fmt.Fprintf(os.Stdout, logo, version, codename)
}
