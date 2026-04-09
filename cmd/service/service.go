// Package service provides auto-generated CLI commands from the API Registry.
// Each API endpoint in meta_data.json becomes a cobra command:
//
//	evo-cli <service> <resource> <method> [flags]
package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/output"
	"github.com/evopayment/evo-cli/internal/registry"
)

// RegisterServiceCommands iterates over the Registry and creates a cobra
// command hierarchy: service → resource → method. Each generated command
// is added directly to rootCmd.
func RegisterServiceCommands(rootCmd *cobra.Command, f cmdutil.Factory) error {
	reg, err := f.Registry()
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	for _, svc := range reg.Services {
		svcCmd := newServiceCmd(svc)

		for resName, res := range svc.Resources {
			resCmd := newResourceCmd(resName)

			for methName, meth := range res.Methods {
				methCmd := newMethodCmd(f, svc.Name, resName, methName, meth)
				resCmd.AddCommand(methCmd)
			}

			svcCmd.AddCommand(resCmd)
		}

		rootCmd.AddCommand(svcCmd)
	}

	return nil
}

// newServiceCmd creates the top-level service command (e.g., "payment", "linkpay").
func newServiceCmd(svc registry.Service) *cobra.Command {
	return &cobra.Command{
		Use:   svc.Name,
		Short: svc.Description,
	}
}

// newResourceCmd creates a resource sub-command (e.g., "online", "order").
func newResourceCmd(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Manage %s resources", name),
	}
}

// newMethodCmd creates a leaf method command that sends the actual API request.
func newMethodCmd(f cmdutil.Factory, _, _ string, methName string, meth *registry.Method) *cobra.Command {
	var (
		data   string
		params string
	)

	cmd := &cobra.Command{
		Use:          methName,
		Short:        meth.Description,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			return runMethod(f, cmd, meth, data, params, dryRun)
		},
	}

	cmd.Flags().StringVar(&data, "data", "", "Request body (JSON string)")
	cmd.Flags().StringVar(&params, "params", "", "Query/path parameters (JSON object)")

	return cmd
}

// runMethod executes the API call for a service method command.
// It validates required parameters, resolves path templates, and sends the request.
func runMethod(f cmdutil.Factory, cmd *cobra.Command, meth *registry.Method, dataFlag, paramsFlag string, dryRun bool) error {
	io := f.IOStreams()

	// Load config for merchantSid and other fromConfig values.
	cfg, err := f.Config()
	if err != nil {
		output.WriteError(io.ErrOut, "config_missing", "", err.Error(), "")
		return err
	}

	// Parse --params flag into a map.
	userParams, err := parseParamsFlag(paramsFlag)
	if err != nil {
		output.WriteError(io.ErrOut, "validation", "", err.Error(), "")
		return err
	}

	// Validate required parameters and resolve path template.
	resolvedPath, queryParams, err := resolveParameters(meth, cfg, userParams)
	if err != nil {
		exitCode := output.WriteError(io.ErrOut, "validation", "", err.Error(), "")
		return &validationError{msg: err.Error(), exitCode: exitCode}
	}

	// Parse --data flag.
	var body interface{}
	if dataFlag != "" {
		if err := json.Unmarshal([]byte(dataFlag), &body); err != nil {
			output.WriteError(io.ErrOut, "validation", "", fmt.Sprintf("parse --data as JSON: %s", err), "")
			return fmt.Errorf("parse --data as JSON: %w", err)
		}
	}

	// Handle --dry-run: print request details without sending.
	if dryRun {
		baseURL := resolveBaseURLForService(cfg, meth)
		fullURL := baseURL + resolvedPath
		if len(queryParams) > 0 {
			parts := make([]string, 0, len(queryParams))
			for k, v := range queryParams {
				parts = append(parts, k+"="+v)
			}
			fullURL += "?" + strings.Join(parts, "&")
		}
		headers := map[string]string{
			"Content-Type": "application/json; charset=utf-8",
			"SignType":     cfg.SignType,
		}
		dryRunOutput := map[string]interface{}{
			"method":  meth.HTTPMethod,
			"url":     fullURL,
			"headers": headers,
		}
		if body != nil {
			dryRunOutput["body"] = body
		}
		enc := json.NewEncoder(io.Out)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(dryRunOutput)
	}

	// Get EvoClient and send request.
	client, err := f.EvoClient()
	if err != nil {
		output.WriteError(io.ErrOut, "cli_error", "", err.Error(), "")
		return err
	}

	// Use LinkPay base URL for LinkPay API paths.
	var envelope *output.Envelope
	if core.IsLinkPayPath(meth.Path) {
		linkPayBaseURL := cfg.ResolveLinkPayBaseURL("")
		envelope, err = client.CallAPIWithBaseURL(linkPayBaseURL, meth.HTTPMethod, resolvedPath, queryParams, body)
	} else {
		envelope, err = client.CallAPI(meth.HTTPMethod, resolvedPath, queryParams, body)
	}
	if err != nil {
		handleMethodError(io, err)
		return err
	}

	return output.WriteSuccess(io.Out, envelope.Data, envelope.Meta)
}

// resolveParameters validates required parameters and builds the resolved URL path
// and query parameter map. Returns an error if any required parameter is missing.
func resolveParameters(meth *registry.Method, cfg *core.CliConfig, userParams map[string]string) (string, map[string]string, error) {
	resolvedPath := meth.Path
	queryParams := make(map[string]string)

	// Copy user-provided params that aren't path params into query params.
	for k, v := range userParams {
		queryParams[k] = v
	}

	for paramName, param := range meth.Parameters {
		var value string

		// Check if value comes from config.
		if param.FromConfig == "merchantSid" {
			value = cfg.MerchantSid
		} else if v, ok := userParams[paramName]; ok {
			value = v
		}

		// Validate required parameters.
		if param.Required && value == "" {
			return "", nil, fmt.Errorf("missing required parameter: %s", paramName)
		}

		// Replace path parameters in the URL template.
		if param.Location == "path" && value != "" {
			// Try exact match first, then try simplified name (strip " of ..." suffix)
			// because swagger param names like "merchantOrderID of LinkPay" map to
			// path placeholders like {merchantOrderID}.
			placeholder := "{" + paramName + "}"
			if strings.Contains(resolvedPath, placeholder) {
				resolvedPath = strings.ReplaceAll(resolvedPath, placeholder, value)
			} else if idx := strings.Index(paramName, " of "); idx > 0 {
				shortName := paramName[:idx]
				shortPlaceholder := "{" + shortName + "}"
				resolvedPath = strings.ReplaceAll(resolvedPath, shortPlaceholder, value)
			}
			// Remove path params from query params (they belong in the URL).
			delete(queryParams, paramName)
		}

		// Add query parameters.
		if param.Location == "query" && value != "" {
			queryParams[paramName] = value
		}
	}

	return resolvedPath, queryParams, nil
}

// parseParamsFlag parses the --params JSON flag into a map[string]string.
// Returns an empty map if the flag is empty.
func parseParamsFlag(paramsFlag string) (map[string]string, error) {
	if paramsFlag == "" {
		return make(map[string]string), nil
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(paramsFlag), &raw); err != nil {
		return nil, fmt.Errorf("parse --params as JSON object: %w", err)
	}

	result := make(map[string]string, len(raw))
	for k, v := range raw {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result, nil
}

// resolveBaseURLForService returns the appropriate base URL for the given method.
// LinkPay endpoints use a separate base URL from the main payment API.
func resolveBaseURLForService(cfg *core.CliConfig, meth *registry.Method) string {
	return cfg.ResolveBaseURLForPath(meth.Path)
}

// validationError wraps a validation failure with an exit code.
type validationError struct {
	msg      string
	exitCode int
}

func (e *validationError) Error() string { return e.msg }

// handleMethodError writes a structured error to stderr.
func handleMethodError(io *cmdutil.IOStreams, err error) {
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
