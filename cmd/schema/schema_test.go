package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"testing"
	"testing/quick"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/registry"
	"github.com/spf13/cobra"

	"net/http"
)

// --- test helpers ---

type stubFactory struct {
	config    *core.CliConfig
	configErr error
	io        *cmdutil.IOStreams
	reg       *registry.Registry
	regErr    error
}

func (f *stubFactory) Config() (*core.CliConfig, error)       { return f.config, f.configErr }
func (f *stubFactory) HttpClient() (*http.Client, error)      { return nil, nil }
func (f *stubFactory) EvoClient() (*cmdutil.EvoClient, error) { return nil, nil }
func (f *stubFactory) IOStreams() *cmdutil.IOStreams          { return f.io }
func (f *stubFactory) Registry() (*registry.Registry, error)  { return f.reg, f.regErr }

func newTestIOStreams() (*cmdutil.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &cmdutil.IOStreams{
		In:     io.NopCloser(strings.NewReader("")),
		Out:    out,
		ErrOut: errOut,
	}, out, errOut
}

func newTestRegistry() *registry.Registry {
	return &registry.Registry{
		Version: "1.0.0",
		Services: []registry.Service{
			{
				Name:        "payment",
				Description: "EC Payment APIs",
				Resources: map[string]*registry.Resource{
					"online": {
						Methods: map[string]*registry.Method{
							"pay": {
								HTTPMethod:  "POST",
								Path:        "/g2/v1/payment/mer/{sid}/payment",
								Description: "Create a payment",
								Parameters: map[string]*registry.Parameter{
									"sid": {Location: "path", Required: true, Type: "string", FromConfig: "merchantSid"},
								},
								RequestBody: map[string]*registry.BodyField{
									"merchantTransInfo": {Type: "object", Required: true},
									"transAmount":       {Type: "object", Required: true},
								},
							},
							"query": {
								HTTPMethod:  "GET",
								Path:        "/g2/v1/payment/mer/{sid}/payment",
								Description: "Query payment status",
								Parameters: map[string]*registry.Parameter{
									"sid":             {Location: "path", Required: true, Type: "string", FromConfig: "merchantSid"},
									"merchantTransID": {Location: "query", Required: true, Type: "string"},
								},
							},
						},
					},
				},
			},
			{
				Name:        "linkpay",
				Description: "LinkPay hosted payment page",
				Resources: map[string]*registry.Resource{
					"order": {
						Methods: map[string]*registry.Method{
							"create": {
								HTTPMethod:  "POST",
								Path:        "/g2/v0/payment/mer/{sid}/evo.e-commerce.linkpay",
								Description: "Create a LinkPay order",
								Parameters: map[string]*registry.Parameter{
									"sid": {Location: "path", Required: true, Type: "string", FromConfig: "merchantSid"},
								},
							},
						},
					},
				},
			},
		},
	}
}

// --- Unit Tests ---

func TestSchemaCommand_ListServices(t *testing.T) {
	ios, out, _ := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	rootCmd.PersistentFlags().String("format", "json", "")
	rootCmd.AddCommand(NewCmdSchema(f))

	rootCmd.SetArgs([]string{"schema"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var services []serviceInfo
	if err := json.Unmarshal(out.Bytes(), &services); err != nil {
		t.Fatalf("stdout not valid JSON: %v\noutput: %s", err, out.String())
	}
	if len(services) != 2 {
		t.Fatalf("got %d services, want 2", len(services))
	}
	// Sorted alphabetically: linkpay, payment
	if services[0].Name != "linkpay" {
		t.Errorf("services[0].Name = %q, want linkpay", services[0].Name)
	}
	if services[1].Name != "payment" {
		t.Errorf("services[1].Name = %q, want payment", services[1].Name)
	}
}

func TestSchemaCommand_ListResources(t *testing.T) {
	ios, out, _ := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	rootCmd.PersistentFlags().String("format", "json", "")
	rootCmd.AddCommand(NewCmdSchema(f))

	rootCmd.SetArgs([]string{"schema", "payment"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var resources []resourceInfo
	if err := json.Unmarshal(out.Bytes(), &resources); err != nil {
		t.Fatalf("stdout not valid JSON: %v\noutput: %s", err, out.String())
	}
	if len(resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(resources))
	}
	if resources[0].Name != "online" {
		t.Errorf("resources[0].Name = %q, want online", resources[0].Name)
	}
	if resources[0].Methods != 2 {
		t.Errorf("resources[0].Methods = %d, want 2", resources[0].Methods)
	}
}

func TestSchemaCommand_ListMethods(t *testing.T) {
	ios, out, _ := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	rootCmd.PersistentFlags().String("format", "json", "")
	rootCmd.AddCommand(NewCmdSchema(f))

	rootCmd.SetArgs([]string{"schema", "payment.online"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var methods []methodInfo
	if err := json.Unmarshal(out.Bytes(), &methods); err != nil {
		t.Fatalf("stdout not valid JSON: %v\noutput: %s", err, out.String())
	}
	if len(methods) != 2 {
		t.Fatalf("got %d methods, want 2", len(methods))
	}
	// Sorted: pay, query
	if methods[0].Name != "pay" {
		t.Errorf("methods[0].Name = %q, want pay", methods[0].Name)
	}
	if methods[0].HTTPMethod != "POST" {
		t.Errorf("methods[0].HTTPMethod = %q, want POST", methods[0].HTTPMethod)
	}
}

func TestSchemaCommand_ShowMethodDetail(t *testing.T) {
	ios, out, _ := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	rootCmd.PersistentFlags().String("format", "json", "")
	rootCmd.AddCommand(NewCmdSchema(f))

	rootCmd.SetArgs([]string{"schema", "payment.online.pay"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var detail methodDetail
	if err := json.Unmarshal(out.Bytes(), &detail); err != nil {
		t.Fatalf("stdout not valid JSON: %v\noutput: %s", err, out.String())
	}
	if detail.Name != "pay" {
		t.Errorf("name = %q, want pay", detail.Name)
	}
	if detail.HTTPMethod != "POST" {
		t.Errorf("httpMethod = %q, want POST", detail.HTTPMethod)
	}
	if detail.Path != "/g2/v1/payment/mer/{sid}/payment" {
		t.Errorf("path = %q, want /g2/v1/payment/mer/{sid}/payment", detail.Path)
	}
	if detail.Parameters == nil || detail.Parameters["sid"] == nil {
		t.Fatal("expected sid parameter")
	}
	if !detail.Parameters["sid"].Required {
		t.Error("sid.required = false, want true")
	}
	if detail.RequestBody == nil || detail.RequestBody["merchantTransInfo"] == nil {
		t.Fatal("expected merchantTransInfo in requestBody")
	}
	if !detail.RequestBody["merchantTransInfo"].Required {
		t.Error("merchantTransInfo.required = false, want true")
	}
}

func TestSchemaCommand_ServiceNotFound(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	rootCmd.PersistentFlags().String("format", "json", "")
	rootCmd.AddCommand(NewCmdSchema(f))

	rootCmd.SetArgs([]string{"schema", "nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent service")
	}

	var envelope map[string]any
	if jsonErr := json.Unmarshal(errOut.Bytes(), &envelope); jsonErr != nil {
		t.Fatalf("stderr not valid JSON: %v\noutput: %s", jsonErr, errOut.String())
	}
	if envelope["ok"] != false {
		t.Errorf("ok = %v, want false", envelope["ok"])
	}
}

func TestSchemaCommand_ResourceNotFound(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	rootCmd.PersistentFlags().String("format", "json", "")
	rootCmd.AddCommand(NewCmdSchema(f))

	rootCmd.SetArgs([]string{"schema", "payment.nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent resource")
	}
	if !strings.Contains(errOut.String(), "not found") {
		t.Errorf("stderr = %q, want 'not found'", errOut.String())
	}
}

func TestSchemaCommand_MethodNotFound(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	rootCmd.PersistentFlags().String("format", "json", "")
	rootCmd.AddCommand(NewCmdSchema(f))

	rootCmd.SetArgs([]string{"schema", "payment.online.nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent method")
	}
	if !strings.Contains(errOut.String(), "not found") {
		t.Errorf("stderr = %q, want 'not found'", errOut.String())
	}
}

func TestSchemaCommand_InvalidPath(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	rootCmd.PersistentFlags().String("format", "json", "")
	rootCmd.AddCommand(NewCmdSchema(f))

	rootCmd.SetArgs([]string{"schema", "a.b.c.d"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for too many path segments")
	}
	if !strings.Contains(errOut.String(), "invalid path") {
		t.Errorf("stderr = %q, want 'invalid path'", errOut.String())
	}
}

func TestSchemaCommand_RegistryError(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	f := &stubFactory{io: ios, regErr: fmt.Errorf("registry load failed")}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	rootCmd.PersistentFlags().String("format", "json", "")
	rootCmd.AddCommand(NewCmdSchema(f))

	rootCmd.SetArgs([]string{"schema"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when registry fails")
	}
	if !strings.Contains(errOut.String(), "load registry") {
		t.Errorf("stderr = %q, want 'load registry'", errOut.String())
	}
}

// --- Completion Tests ---

func TestCompleteSchemaPath_Empty(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	fn := completeSchemaPath(f)
	completions, dir := fn(nil, nil, "")

	if dir&cobra.ShellCompDirectiveNoFileComp == 0 {
		t.Error("expected NoFileComp directive")
	}
	// Should return service names.
	if len(completions) != 2 {
		t.Fatalf("got %d completions, want 2", len(completions))
	}
	found := strings.Join(completions, " ")
	if !strings.Contains(found, "linkpay") || !strings.Contains(found, "payment") {
		t.Errorf("completions = %v, want linkpay and payment", completions)
	}
}

func TestCompleteSchemaPath_ServicePrefix(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	fn := completeSchemaPath(f)
	completions, _ := fn(nil, nil, "pay")

	if len(completions) != 1 {
		t.Fatalf("got %d completions, want 1", len(completions))
	}
	if !strings.HasPrefix(completions[0], "payment") {
		t.Errorf("completions[0] = %q, want to start with payment", completions[0])
	}
}

func TestCompleteSchemaPath_ResourceLevel(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	fn := completeSchemaPath(f)
	completions, _ := fn(nil, nil, "payment.")

	if len(completions) != 1 {
		t.Fatalf("got %d completions, want 1", len(completions))
	}
	if completions[0] != "payment.online" {
		t.Errorf("completions[0] = %q, want payment.online", completions[0])
	}
}

func TestCompleteSchemaPath_MethodLevel(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	fn := completeSchemaPath(f)
	completions, _ := fn(nil, nil, "payment.online.")

	if len(completions) != 2 {
		t.Fatalf("got %d completions, want 2", len(completions))
	}
	found := strings.Join(completions, " ")
	if !strings.Contains(found, "payment.online.pay") || !strings.Contains(found, "payment.online.query") {
		t.Errorf("completions = %v, want pay and query", completions)
	}
}

func TestCompleteSchemaPath_MethodPrefix(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	fn := completeSchemaPath(f)
	completions, _ := fn(nil, nil, "payment.online.p")

	if len(completions) != 1 {
		t.Fatalf("got %d completions, want 1", len(completions))
	}
	if !strings.Contains(completions[0], "payment.online.pay") {
		t.Errorf("completions[0] = %q, want payment.online.pay", completions[0])
	}
}

func TestCompleteSchemaPath_AlreadyHasArg(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{io: ios, reg: reg}

	fn := completeSchemaPath(f)
	completions, _ := fn(nil, []string{"payment"}, "")

	if len(completions) != 0 {
		t.Errorf("got %d completions, want 0 (already has arg)", len(completions))
	}
}

func TestCompleteSchemaPath_RegistryError(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	f := &stubFactory{io: ios, regErr: fmt.Errorf("fail")}

	fn := completeSchemaPath(f)
	completions, dir := fn(nil, nil, "")

	if len(completions) != 0 {
		t.Errorf("got %d completions, want 0 on error", len(completions))
	}
	if dir&cobra.ShellCompDirectiveError == 0 {
		t.Error("expected Error directive on registry failure")
	}
}

// --- splitPath tests ---

func TestSplitPath(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"payment", 1},
		{"payment.online", 2},
		{"payment.online.pay", 3},
		{"a.b.c.d", 4},
	}
	for _, tt := range tests {
		parts := splitPath(tt.input)
		if len(parts) != tt.want {
			t.Errorf("splitPath(%q) = %d parts, want %d", tt.input, len(parts), tt.want)
		}
	}
}

// =============================================================================
// Property-Based Tests (testing/quick)
// =============================================================================

// Feature: evo-payment-cli, Property 18: Schema 自省完整性
// **Validates: Requirements 6.1, 6.2, 6.3**
//
// For any random Registry with services/resources/methods, the schema command
// should correctly output the corresponding level of information:
// - service level: lists all resources
// - resource level: lists all methods
// - method level: outputs complete parameter definitions
func TestProperty18_SchemaIntrospectionCompleteness(t *testing.T) {
	f := func(seed uint32) bool {
		rng := rand.New(rand.NewSource(int64(seed)))

		httpMethods := []string{"GET", "POST", "PUT", "DELETE"}

		// Generate random registry: 1-3 services, 1-3 resources each, 1-3 methods each.
		numServices := 1 + rng.Intn(3)
		services := make([]registry.Service, numServices)

		type methExpect struct {
			httpMethod string
			path       string
		}

		// Track expected structure.
		svcNames := make([]string, numServices)
		// svcName -> list of resource names
		svcResources := make(map[string][]string)
		// svcName.resName -> list of method names
		resMethods := make(map[string][]string)
		// svcName.resName.methName -> expected method detail
		methDetails := make(map[string]methExpect)

		for si := 0; si < numServices; si++ {
			svcName := fmt.Sprintf("svc%d", si)
			svcNames[si] = svcName

			numResources := 1 + rng.Intn(3)
			resources := make(map[string]*registry.Resource)
			resNames := make([]string, 0, numResources)

			for ri := 0; ri < numResources; ri++ {
				resName := fmt.Sprintf("res%d", ri)
				resNames = append(resNames, resName)

				numMethods := 1 + rng.Intn(3)
				methods := make(map[string]*registry.Method)
				methNames := make([]string, 0, numMethods)

				for mi := 0; mi < numMethods; mi++ {
					methName := fmt.Sprintf("m%d", mi)
					methNames = append(methNames, methName)
					hm := httpMethods[rng.Intn(len(httpMethods))]
					p := fmt.Sprintf("/api/%s/%s/{sid}/%s", svcName, resName, methName)

					methods[methName] = &registry.Method{
						HTTPMethod:  hm,
						Path:        p,
						Description: fmt.Sprintf("desc %s", methName),
						Parameters: map[string]*registry.Parameter{
							"sid": {Location: "path", Required: true, Type: "string", FromConfig: "merchantSid"},
						},
					}
					methDetails[svcName+"."+resName+"."+methName] = methExpect{
						httpMethod: hm,
						path:       p,
					}
				}
				resources[resName] = &registry.Resource{Methods: methods}
				resMethods[svcName+"."+resName] = methNames
			}

			services[si] = registry.Service{
				Name:        svcName,
				Description: fmt.Sprintf("Service %d", si),
				Resources:   resources,
			}
			svcResources[svcName] = resNames
		}

		reg := &registry.Registry{Version: "1.0.0", Services: services}

		// Helper to run schema command and capture stdout.
		runSchema := func(args []string) (string, error) {
			ios, out, _ := newTestIOStreams()
			sf := &stubFactory{io: ios, reg: reg}
			rootCmd := &cobra.Command{Use: "evo-cli"}
			rootCmd.PersistentFlags().String("format", "json", "")
			rootCmd.AddCommand(NewCmdSchema(sf))
			rootCmd.SetArgs(args)
			err := rootCmd.Execute()
			return out.String(), err
		}

		// 1. Run `schema` (no args) → verify all service names appear.
		out, err := runSchema([]string{"schema"})
		if err != nil {
			t.Logf("schema (no args) error: %v", err)
			return false
		}
		var gotServices []serviceInfo
		if err := json.Unmarshal([]byte(out), &gotServices); err != nil {
			t.Logf("schema (no args) invalid JSON: %v\noutput: %s", err, out)
			return false
		}
		if len(gotServices) != numServices {
			t.Logf("schema: got %d services, want %d", len(gotServices), numServices)
			return false
		}
		gotSvcSet := make(map[string]bool)
		for _, s := range gotServices {
			gotSvcSet[s.Name] = true
		}
		for _, name := range svcNames {
			if !gotSvcSet[name] {
				t.Logf("schema: service %q not found in output", name)
				return false
			}
		}

		// 2. For each service, run `schema <service>` → verify all resource names appear.
		for _, svcName := range svcNames {
			out, err := runSchema([]string{"schema", svcName})
			if err != nil {
				t.Logf("schema %s error: %v", svcName, err)
				return false
			}
			var gotResources []resourceInfo
			if err := json.Unmarshal([]byte(out), &gotResources); err != nil {
				t.Logf("schema %s invalid JSON: %v", svcName, err)
				return false
			}
			expectedRes := svcResources[svcName]
			if len(gotResources) != len(expectedRes) {
				t.Logf("schema %s: got %d resources, want %d", svcName, len(gotResources), len(expectedRes))
				return false
			}
			gotResSet := make(map[string]bool)
			for _, r := range gotResources {
				gotResSet[r.Name] = true
			}
			for _, name := range expectedRes {
				if !gotResSet[name] {
					t.Logf("schema %s: resource %q not found", svcName, name)
					return false
				}
			}
		}

		// 3. For each service.resource, run `schema <service>.<resource>` → verify all method names appear.
		for _, svcName := range svcNames {
			for _, resName := range svcResources[svcName] {
				path := svcName + "." + resName
				out, err := runSchema([]string{"schema", path})
				if err != nil {
					t.Logf("schema %s error: %v", path, err)
					return false
				}
				var gotMethods []methodInfo
				if err := json.Unmarshal([]byte(out), &gotMethods); err != nil {
					t.Logf("schema %s invalid JSON: %v", path, err)
					return false
				}
				expectedMeth := resMethods[path]
				if len(gotMethods) != len(expectedMeth) {
					t.Logf("schema %s: got %d methods, want %d", path, len(gotMethods), len(expectedMeth))
					return false
				}
				gotMethSet := make(map[string]bool)
				for _, m := range gotMethods {
					gotMethSet[m.Name] = true
				}
				for _, name := range expectedMeth {
					if !gotMethSet[name] {
						t.Logf("schema %s: method %q not found", path, name)
						return false
					}
				}
			}
		}

		// 4. For each service.resource.method, run `schema <service>.<resource>.<method>` → verify httpMethod and path match.
		for key, expect := range methDetails {
			out, err := runSchema([]string{"schema", key})
			if err != nil {
				t.Logf("schema %s error: %v", key, err)
				return false
			}
			var detail methodDetail
			if err := json.Unmarshal([]byte(out), &detail); err != nil {
				t.Logf("schema %s invalid JSON: %v", key, err)
				return false
			}
			if detail.HTTPMethod != expect.httpMethod {
				t.Logf("schema %s: httpMethod=%q, want %q", key, detail.HTTPMethod, expect.httpMethod)
				return false
			}
			if detail.Path != expect.path {
				t.Logf("schema %s: path=%q, want %q", key, detail.Path, expect.path)
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 18 failed: %v", err)
	}
}
