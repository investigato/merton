package utils

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/investigato/go-psrp/client"
	"github.com/investigato/go-psrpcore/serialization"
)

func NewUUID() string {
	return strings.ToUpper(uuid.New().String())
}

func ConvertToBase64(input string) string {
	return base64.StdEncoding.EncodeToString([]byte(input))
}

func ConvertToBase64Bytes(input string) []byte {
	return []byte(ConvertToBase64(input))
}
func ParsePort(portStr string) (int, error) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port number: %v", err)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port number must be between 1 and 65535")
	}
	return port, nil
}
func EndpointParser(host string, port int, insecure bool) (*string, error) {
	var enteredString string
	var enteredPort int

	enteredString = strings.ToLower(host)
	enteredPort = port

	// does host already have a scheme? if so, adjust it based on the insecure flag
	if strings.HasPrefix(enteredString, "http://") || strings.HasPrefix(enteredString, "https://") {
		// Endpoint already has a scheme, adjust it based on insecure flag
		if insecure && strings.HasPrefix(enteredString, "https://") {
			enteredString = strings.Replace(enteredString, "https://", "http://", 1)
		} else if !insecure && strings.HasPrefix(enteredString, "http://") {
			enteredString = strings.Replace(enteredString, "http://", "https://", 1)
		}
	} else {
		// No scheme, add it based on insecure flag
		scheme := "https://"
		if insecure {
			scheme = "http://"
		}
		enteredString = scheme + enteredString + ":" + strconv.Itoa(enteredPort)
	}

	if !strings.HasSuffix(enteredString, "/wsman") {
		enteredString += "/wsman"
	}

	return &enteredString, nil
}

func DomainParser(host string, username string, domain string) (string, error) {
	// try to parse the domain from the endpoint if it is not provided as a separate argument
	if domain != "" {
		return domain, nil
	}

	// if host is an IP address, we can't parse a domain from it, so return an empty string
	if net.ParseIP(host) != nil {
		return "", nil
	}

	if strings.Contains(username, "\\") {
		parts := strings.SplitN(username, "\\", 2)
		return parts[0], nil
	}

	if strings.Contains(username, "@") {
		parts := strings.SplitN(username, "@", 2)
		return parts[1], nil
	}

	// if we can't parse a domain from the host or username, return an empty string
	return "", nil
}

func EscapeCommandLineArgs(args []string) string {
	var escapedArgs []string
	for _, arg := range args {
		// Escape double quotes by replacing " with \"
		escapedArg := strings.ReplaceAll(arg, `"`, `\"`)
		// If the argument contains spaces or tabs, wrap it in double quotes
		if strings.ContainsAny(escapedArg, " \t") {
			escapedArg = `"` + escapedArg + `"`
		}
		escapedArgs = append(escapedArgs, escapedArg)
	}
	return strings.Join(escapedArgs, " ")
}

func PrettifyScreenOutput(output string) string {
	// Replace carriage return + newline with just newline for better readability
	return strings.ReplaceAll(output, "\r\n", "\n")
}

func SplitCommandLine(commandLine string) []string {
	var args []string
	var currentArg strings.Builder
	inQuotes := false

	for _, char := range commandLine {
		switch char {
		case '"':
			inQuotes = !inQuotes
		case ' ':
			if inQuotes {
				currentArg.WriteRune(char)
			} else if currentArg.Len() > 0 {
				args = append(args, currentArg.String())
				currentArg.Reset()
			}
		default:
			currentArg.WriteRune(char)
		}
	}

	if currentArg.Len() > 0 {
		args = append(args, currentArg.String())
	}

	return args
}

func SafeJoinCommandAndArgs(command string, args []string) string {
	escapedArgs := EscapeCommandLineArgs(args)
	if escapedArgs != "" {
		return command + " " + escapedArgs
	}
	return command
}

func SafeJoinArgs(args []string) string {
	escapedArgs := EscapeCommandLineArgs(args)
	return escapedArgs
}

func ProcessResult(result *client.Result) string {
	var outputBuilder strings.Builder
	// Process standard output and standard error separately for better formatting
	if len(result.Output) > 0 {
		for _, item := range result.Output {
			outputBuilder.WriteString(FormatObject(item))
			outputBuilder.WriteString("\n")
		}
	}

	if len(result.Errors) > 0 {
		for _, item := range result.Errors {
			outputBuilder.WriteString(color.RedString("ERROR: "))
			outputBuilder.WriteString(FormatObject(item))
			outputBuilder.WriteString("\n")
		}
	}

	return outputBuilder.String()
}

// formatObject converts a deserialized CLIXML object to a human-readable string.
func FormatObject(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	switch val := v.(type) {
	case string:
		return val
	case *serialization.PSObject:
		// For PSObjects, use ToString if available, otherwise format properties
		if val.ToString != "" {
			return val.ToString
		}
		if val.Value != nil {
			return FormatObject(val.Value)
		}
		// Fallback: format as key=value pairs with recursive formatting
		var parts []string
		for k, prop := range val.Properties {
			parts = append(parts, fmt.Sprintf("%s=%s", k, FormatObject(prop)))
		}
		return strings.Join(parts, " ")
	case []interface{}:
		// Format slices recursively
		var items []string
		for _, item := range val {
			items = append(items, FormatObject(item))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case bool:
		return fmt.Sprintf("%t", val)
	case int32, int64, float64:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func EncodePowerShellScript(script string) string {
	u16 := utf16.Encode([]rune(script))
	buf := make([]byte, len(u16)*2)
	for i, u := range u16 {
		binary.LittleEndian.PutUint16(buf[i*2:], u)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func ParseKerberosCredentials(username, domain string) (string, string) {
	// after parsing, the REALM must be returned in uppercase, and the username should be returned without the domain part.
	// username might be DOMAIN\USER, DOMAIN\\USER, DOMAIN/USER, or USER@DOMAIN, or just USER with a separate domain argument.
	const partTwo = 2
	if domain != "" {
		return username, strings.ToUpper(domain)
	}

	if strings.Contains(username, "\\") {
		parts := strings.SplitN(username, "\\", partTwo)
		return parts[1], strings.ToUpper(parts[0])
	}

	if strings.Contains(username, "/") {
		parts := strings.SplitN(username, "/", partTwo)
		return parts[1], strings.ToUpper(parts[0])
	}

	if strings.Contains(username, "@") {
		parts := strings.SplitN(username, "@", partTwo)
		return parts[0], strings.ToUpper(parts[1])
	}

	return username, ""
}
