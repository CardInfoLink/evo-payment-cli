// Package schema provides the `evo-cli schema` command for API introspection.
// It supports three-level navigation:
//
//	evo-cli schema                              → list all services
//	evo-cli schema payment                      → list resources under payment
//	evo-cli schema payment.online               → list methods under payment.online
//	evo-cli schema payment.online.pay           → show full method details
package schema

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/output"
	"github.com/evopayment/evo-cli/internal/registry"
)

// NewCmdSchema creates the schema command.
func NewCmdSchema(f cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema [service[.resource[.method]]]",
		Short: "Introspect API parameter definitions",
		Long: `Browse the API registry to discover services, resources, and method details.

  evo-cli schema                        List all services
  evo-cli schema payment                List resources under payment
  evo-cli schema payment.online         List methods under payment.online
  evo-cli schema payment.online.pay     Show full parameter definition`,
		Args:              cobra.MaximumNArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completeSchemaPath(f),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) == 1 {
				path = args[0]
			}
			format, _ := cmd.Flags().GetString("format")
			if format == "" {
				format = "json"
			}
			return runSchema(f, path, format)
		},
	}

	return cmd
}

// runSchema dispatches to the appropriate handler based on the dot-separated path depth.
func runSchema(f cmdutil.Factory, path, format string) error {
	reg, err := f.Registry()
	if err != nil {
		output.WriteError(f.IOStreams().ErrOut, "cli_error", "", fmt.Sprintf("load registry: %s", err), "")
		return err
	}

	parts := splitPath(path)

	switch len(parts) {
	case 0:
		return listServices(f.IOStreams(), reg, format)
	case 1:
		return listResources(f.IOStreams(), reg, parts[0], format)
	case 2:
		return listMethods(f.IOStreams(), reg, parts[0], parts[1], format)
	case 3:
		return showMethod(f.IOStreams(), reg, parts[0], parts[1], parts[2], format)
	default:
		msg := fmt.Sprintf("invalid path %q: expected service[.resource[.method]]", path)
		output.WriteError(f.IOStreams().ErrOut, "validation", "", msg, "")
		return fmt.Errorf("%s", msg)
	}
}

// splitPath splits a dot-separated path into parts. Returns nil for empty input.
func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}

// --- Level 0: list all services ---

type serviceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func listServices(io *cmdutil.IOStreams, reg *registry.Registry, format string) error {
	items := make([]serviceInfo, 0, len(reg.Services))
	for _, svc := range reg.Services {
		items = append(items, serviceInfo{Name: svc.Name, Description: svc.Description})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return writeJSON(io, items)
}

// --- Level 1: list resources for a service ---

type resourceInfo struct {
	Name    string `json:"name"`
	Methods int    `json:"methods"`
}

func listResources(io *cmdutil.IOStreams, reg *registry.Registry, svcName, format string) error {
	svc := findService(reg, svcName)
	if svc == nil {
		msg := fmt.Sprintf("service %q not found", svcName)
		output.WriteError(io.ErrOut, "validation", "", msg, "")
		return fmt.Errorf("%s", msg)
	}

	items := make([]resourceInfo, 0, len(svc.Resources))
	for resName, res := range svc.Resources {
		items = append(items, resourceInfo{Name: resName, Methods: len(res.Methods)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return writeJSON(io, items)
}

// --- Level 2: list methods for a resource ---

type methodInfo struct {
	Name        string `json:"name"`
	HTTPMethod  string `json:"httpMethod"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

func listMethods(io *cmdutil.IOStreams, reg *registry.Registry, svcName, resName, format string) error {
	svc := findService(reg, svcName)
	if svc == nil {
		msg := fmt.Sprintf("service %q not found", svcName)
		output.WriteError(io.ErrOut, "validation", "", msg, "")
		return fmt.Errorf("%s", msg)
	}
	res, ok := svc.Resources[resName]
	if !ok {
		msg := fmt.Sprintf("resource %q not found in service %q", resName, svcName)
		output.WriteError(io.ErrOut, "validation", "", msg, "")
		return fmt.Errorf("%s", msg)
	}

	items := make([]methodInfo, 0, len(res.Methods))
	for methName, meth := range res.Methods {
		items = append(items, methodInfo{
			Name:        methName,
			HTTPMethod:  meth.HTTPMethod,
			Path:        meth.Path,
			Description: meth.Description,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return writeJSON(io, items)
}

// --- Level 3: show full method details ---

type methodDetail struct {
	Name        string                  `json:"name"`
	HTTPMethod  string                  `json:"httpMethod"`
	Path        string                  `json:"path"`
	Description string                  `json:"description"`
	Parameters  map[string]*paramDetail `json:"parameters,omitempty"`
	RequestBody map[string]*bodyDetail  `json:"requestBody,omitempty"`
}

type paramDetail struct {
	Location   string   `json:"location"`
	Required   bool     `json:"required"`
	Type       string   `json:"type"`
	FromConfig string   `json:"fromConfig,omitempty"`
	Enum       []string `json:"enum,omitempty"`
}

type bodyDetail struct {
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

func showMethod(io *cmdutil.IOStreams, reg *registry.Registry, svcName, resName, methName, format string) error {
	svc := findService(reg, svcName)
	if svc == nil {
		msg := fmt.Sprintf("service %q not found", svcName)
		output.WriteError(io.ErrOut, "validation", "", msg, "")
		return fmt.Errorf("%s", msg)
	}
	res, ok := svc.Resources[resName]
	if !ok {
		msg := fmt.Sprintf("resource %q not found in service %q", resName, svcName)
		output.WriteError(io.ErrOut, "validation", "", msg, "")
		return fmt.Errorf("%s", msg)
	}
	meth, ok := res.Methods[methName]
	if !ok {
		msg := fmt.Sprintf("method %q not found in %s.%s", methName, svcName, resName)
		output.WriteError(io.ErrOut, "validation", "", msg, "")
		return fmt.Errorf("%s", msg)
	}

	detail := methodDetail{
		Name:        methName,
		HTTPMethod:  meth.HTTPMethod,
		Path:        meth.Path,
		Description: meth.Description,
	}

	if len(meth.Parameters) > 0 {
		detail.Parameters = make(map[string]*paramDetail, len(meth.Parameters))
		for pName, p := range meth.Parameters {
			detail.Parameters[pName] = &paramDetail{
				Location:   p.Location,
				Required:   p.Required,
				Type:       p.Type,
				FromConfig: p.FromConfig,
				Enum:       p.Enum,
			}
		}
	}

	if len(meth.RequestBody) > 0 {
		detail.RequestBody = make(map[string]*bodyDetail, len(meth.RequestBody))
		for fName, bf := range meth.RequestBody {
			detail.RequestBody[fName] = &bodyDetail{
				Type:     bf.Type,
				Required: bf.Required,
			}
		}
	}

	return writeJSON(io, detail)
}

// --- helpers ---

func findService(reg *registry.Registry, name string) *registry.Service {
	for i := range reg.Services {
		if reg.Services[i].Name == name {
			return &reg.Services[i]
		}
	}
	return nil
}

func writeJSON(io *cmdutil.IOStreams, v any) error {
	enc := json.NewEncoder(io.Out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// --- Tab completion ---

// completeSchemaPath returns a ValidArgsFunction that provides dynamic
// completions for the dot-separated service.resource.method path.
func completeSchemaPath(f cmdutil.Factory) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			// Only complete the first positional argument.
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		reg, err := f.Registry()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		parts := splitPath(toComplete)
		dotCount := strings.Count(toComplete, ".")

		switch {
		case toComplete == "" || (dotCount == 0 && len(parts) <= 1):
			// Complete service names.
			return completeServices(reg, toComplete), cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace

		case dotCount == 1:
			// Complete resource names: "payment." → "payment.online", "payment.payout", ...
			svcName := parts[0]
			prefix := ""
			if len(parts) > 1 {
				prefix = parts[1]
			}
			return completeResources(reg, svcName, prefix), cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace

		case dotCount == 2:
			// Complete method names: "payment.online." → "payment.online.pay", ...
			svcName := parts[0]
			resName := parts[1]
			prefix := ""
			if len(parts) > 2 {
				prefix = parts[2]
			}
			return completeMethods(reg, svcName, resName, prefix), cobra.ShellCompDirectiveNoFileComp

		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}

func completeServices(reg *registry.Registry, prefix string) []string {
	var completions []string
	for _, svc := range reg.Services {
		if strings.HasPrefix(svc.Name, prefix) {
			completions = append(completions, svc.Name+"\t"+svc.Description)
		}
	}
	sort.Strings(completions)
	return completions
}

func completeResources(reg *registry.Registry, svcName, prefix string) []string {
	svc := findService(reg, svcName)
	if svc == nil {
		return nil
	}
	var completions []string
	for resName := range svc.Resources {
		if strings.HasPrefix(resName, prefix) {
			completions = append(completions, svcName+"."+resName)
		}
	}
	sort.Strings(completions)
	return completions
}

func completeMethods(reg *registry.Registry, svcName, resName, prefix string) []string {
	svc := findService(reg, svcName)
	if svc == nil {
		return nil
	}
	res, ok := svc.Resources[resName]
	if !ok {
		return nil
	}
	var completions []string
	for methName, meth := range res.Methods {
		if strings.HasPrefix(methName, prefix) {
			completions = append(completions, svcName+"."+resName+"."+methName+"\t"+meth.Description)
		}
	}
	sort.Strings(completions)
	return completions
}
