// Package api provides the "evo-cli api" command for raw API calls.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/output"
)

// validMethods lists the accepted HTTP methods.
var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
}

// NewCmdAPI creates the "api" command.
func NewCmdAPI(f cmdutil.Factory) *cobra.Command {
	var (
		data           string
		params         string
		idempotencyKey string
	)

	cmd := &cobra.Command{
		Use:   "api <METHOD> <path>",
		Short: "Send a raw API request to Evo Payment",
		Long:  "Send an arbitrary HTTP request to the Evo Payment API with automatic signing.",
		Args:  cobra.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return []string{"GET", "POST", "PUT", "DELETE"}, cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			apiPath := args[1]

			// Read global flags from root command.
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			outputPath, _ := cmd.Flags().GetString("output")

			if !validMethods[method] {
				io := f.IOStreams()
				output.WriteError(io.ErrOut, "validation", "",
					fmt.Sprintf("unsupported HTTP method: %s (use GET, POST, PUT, DELETE)", method), "")
				return fmt.Errorf("unsupported HTTP method: %s", method)
			}

			// Parse --data flag.
			var body interface{}
			if data != "" {
				parsed, err := parseData(data)
				if err != nil {
					io := f.IOStreams()
					output.WriteError(io.ErrOut, "validation", "", err.Error(), "")
					return err
				}
				body = parsed
			}

			// Parse --params flag.
			var queryParams map[string]string
			if params != "" {
				parsed, err := parseParams(params)
				if err != nil {
					io := f.IOStreams()
					output.WriteError(io.ErrOut, "validation", "", err.Error(), "")
					return err
				}
				queryParams = parsed
			}

			// Validate --output path safety.
			if outputPath != "" {
				if err := validatePathSafety(outputPath); err != nil {
					io := f.IOStreams()
					output.WriteError(io.ErrOut, "validation", "", err.Error(), "")
					return err
				}
			}

			// Handle --dry-run.
			if dryRun {
				if err := runDryRun(f, method, apiPath, queryParams, body, idempotencyKey); err != nil {
					return err
				}
				// Save dry-run output to file if --output specified.
				if outputPath != "" {
					dryRunJSON := buildDryRunJSON(f, method, apiPath, queryParams, body, idempotencyKey)
					if err := os.WriteFile(outputPath, dryRunJSON, 0644); err != nil {
						io := f.IOStreams()
						output.WriteError(io.ErrOut, "cli_error", "",
							fmt.Sprintf("write output file: %s", err), "")
						return err
					}
				}
				return nil
			}

			// Build context with idempotency key if provided.
			ctx := context.Background()
			if idempotencyKey != "" {
				ctx = cmdutil.WithIdempotencyKey(ctx, idempotencyKey)
			}

			// Get EvoClient and send request.
			client, err := f.EvoClient()
			if err != nil {
				io := f.IOStreams()
				handleError(io, err)
				return err
			}

			// Route LinkPay paths to the LinkPay base URL.
			cfg, cfgErr := f.Config()
			var envelope *output.Envelope
			if cfgErr == nil && core.IsLinkPayPath(apiPath) {
				linkPayBaseURL := cfg.ResolveLinkPayBaseURL("")
				envelope, err = client.CallAPIWithBaseURL(linkPayBaseURL, method, apiPath, queryParams, body)
			} else {
				envelope, err = client.CallAPIWithContext(ctx, method, apiPath, queryParams, body)
			}
			if err != nil {
				io := f.IOStreams()
				handleError(io, err)
				return err
			}

			// Save to file if --output specified.
			if outputPath != "" {
				respJSON, err := json.Marshal(envelope)
				if err != nil {
					io := f.IOStreams()
					output.WriteError(io.ErrOut, "cli_error", "",
						fmt.Sprintf("marshal response: %s", err), "")
					return err
				}
				if err := os.WriteFile(outputPath, respJSON, 0644); err != nil {
					io := f.IOStreams()
					output.WriteError(io.ErrOut, "cli_error", "",
						fmt.Sprintf("write output file: %s", err), "")
					return err
				}
			}

			// Write to stdout.
			io := f.IOStreams()
			return output.WriteSuccess(io.Out, envelope.Data, envelope.Meta)
		},
	}

	// Local flags specific to the api command.
	cmd.Flags().StringVar(&data, "data", "", "Request body (JSON string or @filepath)")
	cmd.Flags().StringVar(&params, "params", "", "URL query parameters (JSON object)")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Explicit idempotency key (PUT/DELETE)")

	return cmd
}

// parseData handles --data flag: JSON string or @filepath.
func parseData(data string) (interface{}, error) {
	if strings.HasPrefix(data, "@") {
		filePath := data[1:]
		if err := validatePathSafety(filePath); err != nil {
			return nil, err
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read data file: %w", err)
		}
		var parsed interface{}
		if err := json.Unmarshal(content, &parsed); err != nil {
			return nil, fmt.Errorf("parse data file as JSON: %w", err)
		}
		return parsed, nil
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		return nil, fmt.Errorf("parse --data as JSON: %w", err)
	}
	return parsed, nil
}

// parseParams parses --params JSON string into map[string]string.
func parseParams(params string) (map[string]string, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(params), &raw); err != nil {
		return nil, fmt.Errorf("parse --params as JSON object: %w", err)
	}
	result := make(map[string]string, len(raw))
	for k, v := range raw {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result, nil
}

// validatePathSafety rejects paths containing ".." to prevent path traversal.
func validatePathSafety(path string) error {
	if strings.Contains(path, "..") {
		return fmt.Errorf("path contains '..': path traversal is not allowed")
	}
	return nil
}

// runDryRun prints the request details as JSON without sending.
func runDryRun(f cmdutil.Factory, method, apiPath string, params map[string]string, body interface{}, idempotencyKey string) error {
	io := f.IOStreams()
	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(buildDryRunMap(f, method, apiPath, params, body, idempotencyKey))
}

// buildDryRunMap constructs the dry-run output map.
func buildDryRunMap(f cmdutil.Factory, method, apiPath string, params map[string]string, body interface{}, idempotencyKey string) map[string]interface{} {
	cfg, err := f.Config()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	baseURL := cfg.ResolveBaseURLForPath(apiPath)
	fullURL := baseURL + apiPath

	// Append query params to URL for display.
	if len(params) > 0 {
		parts := make([]string, 0, len(params))
		for k, v := range params {
			parts = append(parts, k+"="+v)
		}
		fullURL += "?" + strings.Join(parts, "&")
	}

	headers := map[string]string{
		"Content-Type": "application/json; charset=utf-8",
		"SignType":     cfg.SignType,
	}
	if cfg.KeyID != "" {
		headers["KeyID"] = cfg.KeyID
	}
	if idempotencyKey != "" && (method == "PUT" || method == "DELETE") {
		headers["Idempotency-Key"] = idempotencyKey
	}

	dryRunOutput := map[string]interface{}{
		"method":  method,
		"url":     fullURL,
		"headers": headers,
	}
	if body != nil {
		dryRunOutput["body"] = body
	}
	return dryRunOutput
}

// buildDryRunJSON returns the dry-run output as formatted JSON bytes.
func buildDryRunJSON(f cmdutil.Factory, method, apiPath string, params map[string]string, body interface{}, idempotencyKey string) []byte {
	m := buildDryRunMap(f, method, apiPath, params, body, idempotencyKey)
	b, _ := json.MarshalIndent(m, "", "  ")
	return b
}

// handleError writes a structured error to stderr based on the error type.
func handleError(io *cmdutil.IOStreams, err error) {
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
