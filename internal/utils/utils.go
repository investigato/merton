package utils

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/investigato/go-psrp/client"
	"github.com/investigato/go-psrpcore/serialization"
)

type ObjectRenderer func(obj *serialization.PSObject) string

var typeRenderers = map[string]ObjectRenderer{
	"System.ServiceProcess.ServiceController": renderService,
	"Microsoft.Management.Infrastructure.CimInstance#Root/Microsoft/Windows/TaskScheduler/MSFT_ScheduledTask": renderScheduledTask,
}
var tableRenderers = map[string]func([]*serialization.PSObject) string{
	"System.ServiceProcess.ServiceController": renderServiceTable, "System.Diagnostics.Process": renderProcessTable, "Microsoft.Management.Infrastructure.CimInstance#ROOT/StandardCimv2/MSFT_NetTCPConnection": renderTCPConnectionTable,
	"Microsoft.Management.Infrastructure.CimInstance#Root/Microsoft/Windows/TaskScheduler/MSFT_ScheduledTask": renderScheduledTaskTable,
}

func renderPSObject(obj *serialization.PSObject) string {
	for _, typeName := range obj.TypeNames {
		if renderer, ok := typeRenderers[typeName]; ok {
			return renderer(obj)
		}
	}
	// fallback
	return renderGeneric(obj)
}
func renderPSObjects(objs []*serialization.PSObject) string {
	if len(objs) == 0 {
		return ""
	}
	typeName := objs[0].TypeNames[0]
	for _, obj := range objs {
		if obj.TypeNames[0] != typeName {
			return processPSObject(objs)
		}
	}
	// check tableRenderers first — that's what it's for
	if renderer, ok := tableRenderers[typeName]; ok {
		return renderer(objs)
	}
	return processPSObject(objs)
}

func renderService(obj *serialization.PSObject) string {
	status := FormatObject(obj.Properties["Status"])
	name := FormatObject(obj.Properties["ServiceName"])
	display := FormatObject(obj.Properties["DisplayName"])
	return fmt.Sprintf("%-12s %-30s %s", status, name, display)
}

func renderServiceTable(objs []*serialization.PSObject) string {
	var sb strings.Builder

	if _, err := fmt.Fprintf(&sb, "%-10s %-20s %s\n", "Status", "Name", "DisplayName"); err != nil {
		return ""
	}

	if _, err := fmt.Fprintf(&sb, "%-10s %-20s %s\n", "------", "----", "-----------"); err != nil {
		return ""
	}

	for _, obj := range objs {
		status := FormatObject(obj.Properties["Status"])
		name := FormatObject(obj.Properties["ServiceName"])
		display := FormatObject(obj.Properties["DisplayName"])
		if _, err := fmt.Fprintf(&sb, "%-10s %-20s %s\n", status, name, display); err != nil {
			return ""
		}
	}

	return sb.String()
}

func renderScheduledTask(obj *serialization.PSObject) string {
	state := FormatObject(obj.Properties["State"])
	name := FormatObject(obj.Properties["TaskName"])
	path := FormatObject(obj.Properties["TaskPath"])
	return fmt.Sprintf("%-10s %-30s %s", state, name, path)
}

func renderScheduledTaskTable(objs []*serialization.PSObject) string {
	var sb strings.Builder

	if _, err := fmt.Fprintf(&sb, "%-10s %-30s %s\n", "State", "Name", "Path"); err != nil {
		return ""
	}
	if _, err := fmt.Fprintf(&sb, "%-10s %-30s %s\n", "-----", "----", "----"); err != nil {
		return ""
	}

	for _, obj := range objs {
		state := FormatObject(obj.Properties["State"])
		name := FormatObject(obj.Properties["TaskName"])
		path := FormatObject(obj.Properties["TaskPath"])
		if _, err := fmt.Fprintf(&sb, "%-10s %-30s %s\n", state, name, path); err != nil {
			return ""
		}
	}

	return sb.String()
}

func renderProcessTable(objs []*serialization.PSObject) string {
	var sb strings.Builder

	if _, err := fmt.Fprintf(&sb, "%-8s %-10s %-10s %-30s\n", "PID", "CPU", "Memory(MB)", "Name"); err != nil {
		return ""
	}
	if _, err := fmt.Fprintf(&sb, "%-8s %-10s %-10s %-30s\n", "---", "---", "----------", "----"); err != nil {
		return ""
	}

	for _, obj := range objs {
		pid := FormatObject(obj.Properties["Id"])
		cpu := FormatObject(obj.Properties["CPU"])
		name := FormatObject(obj.Properties["ProcessName"])
		// convert to MB
		memStr := ""
		if v, ok := obj.Properties["WorkingSet"]; ok {
			if bytes, ok := toInt64(v); ok {
				memStr = fmt.Sprintf("%.1f", float64(bytes)/1024/1024)
			}
		}
		if _, err := fmt.Fprintf(&sb, "%-8s %-10s %-10s %-30s\n", pid, cpu, memStr, name); err != nil {
			return ""
		}
	}

	return sb.String()
}

func renderTCPConnectionTable(objs []*serialization.PSObject) string {
	var sb strings.Builder

	if _, err := fmt.Fprintf(&sb, "%-22s %-22s %-13s %-13s %-13s %s\n", "Local Address", "Local Port", "Remote Address", "Remote Port", "State", "Process"); err != nil {
		return ""
	}
	if _, err := fmt.Fprintf(&sb, "%-22s %-22s %-13s %-13s %-13s %s\n", "-------------", "--------------", "-------------", "--------------", "-----", "-------"); err != nil {
		return ""
	}

	for _, obj := range objs {
		localAddress := FormatObject(obj.Properties["LocalAddress"])
		localPort := FormatObject(obj.Properties["LocalPort"])
		remoteAddress := FormatObject(obj.Properties["RemoteAddress"])
		remotePort := FormatObject(obj.Properties["RemotePort"])
		state := FormatObject(obj.Properties["State"])
		process := FormatObject(obj.Properties["OwningProcess"])
		if _, err := fmt.Fprintf(&sb, "%-22s %-22s %-13s %-13s %-13s %s\n", localAddress, localPort, remoteAddress, remotePort, state, process); err != nil {
			return ""
		}
	}

	return sb.String()
}
func isCIMDump(s string) bool {
	return strings.HasPrefix(s, "Namespace=") ||
		strings.Contains(s, "MiXml=") ||
		strings.HasPrefix(s, " [Namespace=")
}
func renderGeneric(obj *serialization.PSObject) string {
	if obj.ToString != "" && !isCIMDump(obj.ToString) {
		return obj.ToString
	}
	// the fallbackiest of fallbacks
	if len(obj.OrderedMemberKeys) > 0 {
		var parts []string
		for _, key := range obj.OrderedMemberKeys {
			if v, ok := obj.Properties[key]; ok {
				parts = append(parts, fmt.Sprintf("%s=%v", key, v))
			}
		}
		return strings.Join(parts, " ")
	}
	return fmt.Sprintf("<%s>", obj.TypeNames[0])
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

func ProcessResult(result *client.Result) string {
	var sb strings.Builder
	if objs, ok := asPSObjectSlice(result.Output); ok {
		return renderPSObjects(objs) // hits tableRegistry
	}

	for _, item := range result.Output {
		switch v := item.(type) {
		case string:
			sb.WriteString(v)
			sb.WriteRune('\n')
		case *serialization.PSObject:
			sb.WriteString(renderPSObject(v))
			sb.WriteRune('\n')
		case []any:
			if objs, ok := asPSObjectSlice(v); ok { // v not result.Output
				return renderPSObjects(objs)
			}
		case map[string]any:
			for key, val := range v {
				sb.WriteString(fmt.Sprintf("%s=%v\n", key, val))
			}
			sb.WriteRune('\n')
		case int32, int64, float64, bool:
			sb.WriteString(fmt.Sprintf("%v\n", v))
		default:
			sb.WriteString(fmt.Sprintf("%v\n", v))
		}
	}

	// Errors, Warnings etc. handled separately with appropriate prefixes/colors
	for _, e := range result.Errors {
		sb.WriteString(fmt.Sprintf("[ERROR] %v\n", e))
	}

	return sb.String()
}
func asPSObjectSlice(items []any) ([]*serialization.PSObject, bool) {
	if len(items) == 0 {
		return nil, false
	}
	out := make([]*serialization.PSObject, 0, len(items))
	for _, item := range items {
		obj, ok := item.(*serialization.PSObject)
		if !ok {
			return nil, false
		}
		out = append(out, obj)
	}
	return out, true
}

// ScheduledTask task
type ScheduledTask struct {
	TaskName string
	State    string
	TaskPath string
}

func processPSObject(objs []*serialization.PSObject) string {
	// one pass to collect max width per column from the data, second pass to
	// print with padding. fmt.Sprintf("%-*s", width, value) handles the padding.
	//max column width is 45 characters to prevent excessively wide columns from breaking the output. If a value exceeds this width, it will be truncated with an ellipsis.
	const maxColumnWidth = 45

	if len(objs) == 0 {
		return ""
	}

	columnSet := make(map[string]struct{})
	for _, obj := range objs {
		if obj == nil {
			continue
		}
		for k := range obj.Properties {
			columnSet[k] = struct{}{}
		}
	}

	if len(columnSet) == 0 {
		return ""
	}

	columns := make([]string, 0, len(columnSet))

	// use OrderedMemberKeys from first object if available — PS orders by relevance
	if len(objs[0].OrderedMemberKeys) > 0 {
		seen := make(map[string]struct{})
		for _, k := range objs[0].OrderedMemberKeys {
			if _, ok := columnSet[k]; ok {
				columns = append(columns, k)
				seen[k] = struct{}{}
			}
		}
	} else {
		for col := range columnSet {
			columns = append(columns, col)
		}
		// sort.Strings(columns)
	}

	// cap at something sane regardless
	const maxColumns = 6
	if len(columns) > maxColumns {
		columns = columns[:maxColumns]
	}
	// first pass - find max width per column
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col) // header is minimum width
	}
	for _, obj := range objs {
		if obj == nil {
			continue
		}
		for i, col := range columns {
			v := FormatObject(obj.Properties[col])
			if len(v) > widths[i] {
				if len(v) > maxColumnWidth {
					v = v[:maxColumnWidth-3] + "..."
				}
				widths[i] = len(v)
			}
		}
	}

	// print header
	var b strings.Builder
	for i, col := range columns {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(fmt.Sprintf("%-*s", widths[i], col))
	}
	b.WriteString("\n")
	// print separator
	for i, col := range columns {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(fmt.Sprintf("%-*s", widths[i], strings.Repeat("-", len(col))))
	}
	b.WriteString("\n")
	// print rows
	for _, obj := range objs {
		if obj == nil {
			continue
		}
		for i, col := range columns {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(fmt.Sprintf("%-*s", widths[i], FormatObject(obj.Properties[col])))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func ParseHash(hashStr string) ([]byte, error) { // The hash string can be in the format "LMHASH:NTHASH" or just "NTHASH"
	if strings.Contains(hashStr, ":") {
		parts := strings.SplitN(hashStr, ":", 2)
		decoded, err := hex.DecodeString(parts[1])
		if err != nil {
			return nil, err
		}
		return decoded, nil // Return only the NTHash part
	}
	decoded, err := hex.DecodeString(hashStr)
	if err != nil {
		return nil, err
	}
	return decoded, nil // Assume the entire string is the NTHash
}

// FormatObject converts a deserialized CLIXML object to a human-readable string.
func FormatObject(v any) string {
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
	case []any:
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

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	}
	return 0, false
}
