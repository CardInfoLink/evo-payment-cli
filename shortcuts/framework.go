package shortcuts

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/output"
	"github.com/spf13/cobra"
)

// Risk levels for shortcut commands.
const (
	RiskRead          = "read"
	RiskWrite         = "write"
	RiskHighRiskWrite = "high-risk-write"
)

// Shortcut defines a declarative high-level CLI command.
// The framework automatically handles flag registration, enum validation,
// dry-run routing, high-risk confirmation, execution, and formatted output.
type Shortcut struct {
	Service     string                                              // owning service (payment/linkpay/token)
	Command     string                                              // command name (+pay/+query/+refund)
	Description string                                              // command description
	Flags       []Flag                                              // parameter definitions
	Risk        string                                              // risk level: read / write / high-risk-write
	HasFormat   bool                                                // whether --format is supported
	DryRun      func(ctx context.Context, rt *RuntimeContext) error // dry-run handler (prints request preview)
	Validate    func(ctx context.Context, rt *RuntimeContext) error // optional custom validation
	Execute     func(ctx context.Context, rt *RuntimeContext) error // main execution handler
}

// Flag defines a single flag for a shortcut command.
type Flag struct {
	Name     string   // flag name (e.g. "amount")
	Desc     string   // description
	Required bool     // whether the flag is required
	Default  string   // default value
	Enum     []string // allowed values (auto-validated)
}

// RuntimeContext provides the execution context for shortcut handlers.
type RuntimeContext struct {
	Factory cmdutil.Factory
	Config  *core.CliConfig
	Flags   map[string]string
	IO      *cmdutil.IOStreams
	Format  string
}

// Str returns the flag value as a string.
func (rt *RuntimeContext) Str(name string) string {
	return rt.Flags[name]
}

// Bool returns the flag value as a boolean ("true" → true, everything else → false).
func (rt *RuntimeContext) Bool(name string) bool {
	return rt.Flags[name] == "true"
}

// DoJSON calls EvoClient.CallAPI and returns the parsed response body.
func (rt *RuntimeContext) DoJSON(method, path string, params map[string]string, body interface{}) (map[string]interface{}, error) {
	client, err := rt.Factory.EvoClient()
	if err != nil {
		return nil, err
	}
	envelope, err := client.CallAPI(method, path, params, body)
	if err != nil {
		return nil, err
	}
	if data, ok := envelope.Data.(map[string]interface{}); ok {
		return data, nil
	}
	return nil, nil
}

// DoLinkPayJSON calls EvoClient.CallAPIWithBaseURL using the LinkPay base URL
// and returns the parsed response body.
func (rt *RuntimeContext) DoLinkPayJSON(method, path string, params map[string]string, body interface{}) (map[string]interface{}, error) {
	client, err := rt.Factory.EvoClient()
	if err != nil {
		return nil, err
	}
	baseURL := rt.Config.ResolveLinkPayBaseURL("")
	envelope, err := client.CallAPIWithBaseURL(baseURL, method, path, params, body)
	if err != nil {
		return nil, err
	}
	if data, ok := envelope.Data.(map[string]interface{}); ok {
		return data, nil
	}
	return nil, nil
}

// OutFormat writes the output using the standard Envelope format.
func (rt *RuntimeContext) OutFormat(data interface{}, meta *output.Meta, err error) {
	if err != nil {
		writeShortcutError(rt.IO, err)
		return
	}
	_ = output.WriteSuccess(rt.IO.Out, data, meta)
}

// Mount registers each shortcut as a cobra subcommand under parentCmd.
// Each shortcut is named "+verb" (e.g. "+pay", "+query").
func Mount(parentCmd *cobra.Command, f cmdutil.Factory, shortcuts []Shortcut) {
	for _, sc := range shortcuts {
		sc := sc // capture loop variable
		cmd := buildCobraCommand(f, &sc)
		parentCmd.AddCommand(cmd)
	}
}

// buildCobraCommand creates a cobra.Command from a Shortcut definition.
func buildCobraCommand(f cmdutil.Factory, sc *Shortcut) *cobra.Command {
	cmd := &cobra.Command{
		Use:   sc.Command,
		Short: sc.Description,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShortcut(cmd, f, sc)
		},
	}

	// Register flags from the shortcut definition.
	for _, flag := range sc.Flags {
		cmd.Flags().String(flag.Name, flag.Default, flag.Desc)
	}

	return cmd
}

// runShortcut executes the shortcut pipeline:
// load config → build RuntimeContext → validate required → validate enum →
// custom validate → dry-run route → high-risk confirm → execute
func runShortcut(cmd *cobra.Command, f cmdutil.Factory, sc *Shortcut) error {
	io := f.IOStreams()

	// Load config.
	cfg, err := f.Config()
	if err != nil {
		writeShortcutError(io, err)
		return err
	}

	// Build RuntimeContext with flag values.
	flags := make(map[string]string)
	for _, flag := range sc.Flags {
		val, _ := cmd.Flags().GetString(flag.Name)
		flags[flag.Name] = val
	}

	// Read global flags.
	format, _ := cmd.Flags().GetString("format")
	if format == "" {
		format = "json"
	}

	rt := &RuntimeContext{
		Factory: f,
		Config:  cfg,
		Flags:   flags,
		IO:      io,
		Format:  format,
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Validate required flags.
	if err := validateRequired(sc.Flags, flags); err != nil {
		output.WriteError(io.ErrOut, "validation", "", err.Error(), "")
		return err
	}

	// Validate enum flags.
	if err := validateEnum(sc.Flags, flags); err != nil {
		output.WriteError(io.ErrOut, "validation", "", err.Error(), "")
		return err
	}

	// Custom validation.
	if sc.Validate != nil {
		if err := sc.Validate(ctx, rt); err != nil {
			output.WriteError(io.ErrOut, "validation", "", err.Error(), "")
			return err
		}
	}

	// Dry-run routing.
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		if sc.DryRun != nil {
			return sc.DryRun(ctx, rt)
		}
		// Default dry-run: print a message indicating no dry-run handler.
		fmt.Fprintln(io.Out, `{"dry_run": true, "message": "no dry-run handler defined"}`)
		return nil
	}

	// High-risk confirmation.
	if sc.Risk == RiskHighRiskWrite {
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			confirmed, err := promptConfirmation(io, sc.Command)
			if err != nil {
				return err
			}
			if !confirmed {
				fmt.Fprintln(io.ErrOut, "operation cancelled by user")
				return fmt.Errorf("operation cancelled")
			}
		}
	}

	// Execute.
	return sc.Execute(ctx, rt)
}

// validateRequired checks that all required flags have non-empty values.
func validateRequired(defs []Flag, values map[string]string) error {
	var missing []string
	for _, f := range defs {
		if f.Required && values[f.Name] == "" {
			missing = append(missing, "--"+f.Name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required flag(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

// validateEnum checks that flag values are within the allowed enum list.
func validateEnum(defs []Flag, values map[string]string) error {
	for _, f := range defs {
		if len(f.Enum) == 0 {
			continue
		}
		val := values[f.Name]
		if val == "" {
			continue // empty values are handled by required validation
		}
		found := false
		for _, allowed := range f.Enum {
			if val == allowed {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid value %q for --%s: must be one of [%s]",
				val, f.Name, strings.Join(f.Enum, ", "))
		}
	}
	return nil
}

// promptConfirmation asks the user to type "yes" to confirm a high-risk operation.
// Returns true if the user confirmed, false otherwise.
func promptConfirmation(io *cmdutil.IOStreams, command string) (bool, error) {
	fmt.Fprintf(io.ErrOut, "This is a high-risk operation (%s). Type 'yes' to confirm: ", command)
	scanner := bufio.NewScanner(io.In)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()) == "yes", nil
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}

// writeShortcutError writes a structured error to stderr.
func writeShortcutError(io *cmdutil.IOStreams, err error) {
	type typedError interface {
		Type() string
	}
	type codedError interface {
		Code() string
	}

	errType := "cli_error"
	code := ""
	msg := err.Error()

	if te, ok := err.(typedError); ok {
		errType = te.Type()
	}
	if ce, ok := err.(codedError); ok {
		code = ce.Code()
	}

	output.WriteError(io.ErrOut, errType, code, msg, "")
}

// DryRunOutput is a helper to print a dry-run preview as JSON.
func DryRunOutput(io *cmdutil.IOStreams, method, url string, headers map[string]string, body interface{}) error {
	out := map[string]interface{}{
		"method":  method,
		"url":     url,
		"headers": headers,
	}
	if body != nil {
		out["body"] = body
	}
	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}
