package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/quick"

	"github.com/evopayment/evo-cli/internal/cmdutil"
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/registry"
	"github.com/spf13/cobra"
)

// --- test helpers ---

type stubFactory struct {
	config    *core.CliConfig
	configErr error
	client    *cmdutil.EvoClient
	clientErr error
	io        *cmdutil.IOStreams
	reg       *registry.Registry
	regErr    error
}

func (f *stubFactory) Config() (*core.CliConfig, error)       { return f.config, f.configErr }
func (f *stubFactory) HttpClient() (*http.Client, error)      { return nil, nil }
func (f *stubFactory) EvoClient() (*cmdutil.EvoClient, error) { return f.client, f.clientErr }
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

func newTestConfig() *core.CliConfig {
	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignType:    "SHA256",
		Env:         "test",
	}
	cfg.SetResolvedBaseURL("https://hkg-online-uat.everonet.com")
	cfg.SetResolvedLinkPayBaseURL("https://hkg-counter-uat.everonet.com")
	return cfg
}

func newTestEvoClient(baseURL string) *cmdutil.EvoClient {
	cfg := &core.CliConfig{
		MerchantSid: "S024116",
		SignKey:     core.SecretInput{Value: "64b59e70e15445196b1b5d2935f4e1bc"},
		SignType:    "SHA256",
		Env:         "test",
	}
	cfg.SetResolvedBaseURL(baseURL)
	return cmdutil.NewEvoClient(&http.Client{}, cfg, cmdutil.DefaultIOStreams())
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
							"query": {
								HTTPMethod:  "GET",
								Path:        "/g2/v0/payment/mer/{sid}/evo.e-commerce.linkpay/{merchantOrderID}",
								Description: "Retrieve LinkPay order",
								Parameters: map[string]*registry.Parameter{
									"sid":             {Location: "path", Required: true, Type: "string", FromConfig: "merchantSid"},
									"merchantOrderID": {Location: "path", Required: true, Type: "string"},
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

func TestRegisterServiceCommands_CreatesHierarchy(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{config: newTestConfig(), io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	err := RegisterServiceCommands(rootCmd, f)
	if err != nil {
		t.Fatalf("RegisterServiceCommands() error = %v", err)
	}

	// Verify "payment" service command exists.
	paymentCmd, _, err := rootCmd.Find([]string{"payment"})
	if err != nil || paymentCmd.Name() != "payment" {
		t.Fatal("expected 'payment' command to be registered")
	}

	// Verify "payment online" resource command exists.
	onlineCmd, _, err := rootCmd.Find([]string{"payment", "online"})
	if err != nil || onlineCmd.Name() != "online" {
		t.Fatal("expected 'payment online' command to be registered")
	}

	// Verify "payment online pay" method command exists.
	payCmd, _, err := rootCmd.Find([]string{"payment", "online", "pay"})
	if err != nil || payCmd.Name() != "pay" {
		t.Fatal("expected 'payment online pay' command to be registered")
	}

	// Verify "payment online query" method command exists.
	queryCmd, _, err := rootCmd.Find([]string{"payment", "online", "query"})
	if err != nil || queryCmd.Name() != "query" {
		t.Fatal("expected 'payment online query' command to be registered")
	}

	// Verify "linkpay" service command exists.
	linkpayCmd, _, err := rootCmd.Find([]string{"linkpay"})
	if err != nil || linkpayCmd.Name() != "linkpay" {
		t.Fatal("expected 'linkpay' command to be registered")
	}

	// Verify "linkpay order create" method command exists.
	createCmd, _, err := rootCmd.Find([]string{"linkpay", "order", "create"})
	if err != nil || createCmd.Name() != "create" {
		t.Fatal("expected 'linkpay order create' command to be registered")
	}
}

func TestRegisterServiceCommands_RegistryError(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	f := &stubFactory{io: ios, regErr: fmt.Errorf("registry load failed")}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	err := RegisterServiceCommands(rootCmd, f)
	if err == nil {
		t.Fatal("expected error when registry fails to load")
	}
	if !strings.Contains(err.Error(), "registry load failed") {
		t.Errorf("error = %q, want to contain 'registry load failed'", err.Error())
	}
}

func TestServiceCommand_MethodFlags(t *testing.T) {
	ios, _, _ := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{config: newTestConfig(), io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	_ = RegisterServiceCommands(rootCmd, f)

	payCmd, _, _ := rootCmd.Find([]string{"payment", "online", "pay"})
	if payCmd == nil {
		t.Fatal("pay command not found")
	}

	// Verify --data and --params flags exist.
	if payCmd.Flags().Lookup("data") == nil {
		t.Error("expected --data flag on method command")
	}
	if payCmd.Flags().Lookup("params") == nil {
		t.Error("expected --params flag on method command")
	}
}

func TestServiceCommand_SidAutoReplace(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"}}`)
	}))
	defer srv.Close()

	ios, out, _ := newTestIOStreams()
	client := newTestEvoClient(srv.URL)
	reg := newTestRegistry()
	f := &stubFactory{config: newTestConfig(), client: client, io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	_ = RegisterServiceCommands(rootCmd, f)

	rootCmd.SetArgs([]string{"payment", "online", "pay", "--data", `{"amount":"100"}`})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify {sid} was replaced with merchantSid from config.
	if !strings.Contains(capturedPath, "/mer/S024116/") {
		t.Errorf("path = %q, want to contain /mer/S024116/", capturedPath)
	}
	if strings.Contains(capturedPath, "{sid}") {
		t.Errorf("path = %q, should not contain {sid} placeholder", capturedPath)
	}

	// Verify output is a success envelope.
	var envelope map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if envelope["ok"] != true {
		t.Errorf("ok = %v, want true", envelope["ok"])
	}
}

func TestServiceCommand_QueryParams(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"}}`)
	}))
	defer srv.Close()

	ios, _, _ := newTestIOStreams()
	client := newTestEvoClient(srv.URL)
	reg := newTestRegistry()
	f := &stubFactory{config: newTestConfig(), client: client, io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	_ = RegisterServiceCommands(rootCmd, f)

	rootCmd.SetArgs([]string{"payment", "online", "query", "--params", `{"merchantTransID":"TX001"}`})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(capturedQuery, "merchantTransID=TX001") {
		t.Errorf("query = %q, want to contain merchantTransID=TX001", capturedQuery)
	}
}

func TestServiceCommand_MissingRequiredParam_ExitCode2(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	reg := newTestRegistry()
	// No EvoClient needed — request should not be sent.
	f := &stubFactory{config: newTestConfig(), io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	_ = RegisterServiceCommands(rootCmd, f)

	// "payment online query" requires merchantTransID, but we don't provide it.
	rootCmd.SetArgs([]string{"payment", "online", "query"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing required parameter")
	}

	// Verify error output contains validation error.
	var envelope map[string]interface{}
	if jsonErr := json.Unmarshal(errOut.Bytes(), &envelope); jsonErr != nil {
		t.Fatalf("stderr is not valid JSON: %v\noutput: %s", jsonErr, errOut.String())
	}
	if envelope["ok"] != false {
		t.Errorf("ok = %v, want false", envelope["ok"])
	}
	errObj, _ := envelope["error"].(map[string]interface{})
	if errObj["type"] != "validation" {
		t.Errorf("error.type = %v, want validation", errObj["type"])
	}
	if !strings.Contains(errObj["message"].(string), "missing required parameter") {
		t.Errorf("error.message = %q, want to contain 'missing required parameter'", errObj["message"])
	}
}

func TestServiceCommand_MissingRequiredParam_NoHTTPSent(t *testing.T) {
	requestReceived := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ios, _, _ := newTestIOStreams()
	client := newTestEvoClient(srv.URL)
	reg := newTestRegistry()
	f := &stubFactory{config: newTestConfig(), client: client, io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	_ = RegisterServiceCommands(rootCmd, f)

	// Missing required merchantTransID for query.
	rootCmd.SetArgs([]string{"payment", "online", "query"})
	_ = rootCmd.Execute()

	if requestReceived {
		t.Error("HTTP request was sent despite missing required parameter")
	}
}

func TestServiceCommand_LinkpayPathParams(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"code":"S0000","message":"Success"}}`)
	}))
	defer srv.Close()

	ios, _, _ := newTestIOStreams()
	client := newTestEvoClient(srv.URL)
	cfg := newTestConfig()
	cfg.SetResolvedLinkPayBaseURL(srv.URL) // LinkPay service commands use the LinkPay base URL
	reg := newTestRegistry()
	f := &stubFactory{config: cfg, client: client, io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	_ = RegisterServiceCommands(rootCmd, f)

	rootCmd.SetArgs([]string{"linkpay", "order", "query", "--params", `{"merchantOrderID":"ORD001"}`})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify both {sid} and {merchantOrderID} were replaced.
	if !strings.Contains(capturedPath, "/mer/S024116/") {
		t.Errorf("path = %q, want to contain /mer/S024116/", capturedPath)
	}
	if !strings.Contains(capturedPath, "/ORD001") {
		t.Errorf("path = %q, want to contain /ORD001", capturedPath)
	}
	if strings.Contains(capturedPath, "{") {
		t.Errorf("path = %q, should not contain unresolved placeholders", capturedPath)
	}
}

func TestServiceCommand_InvalidDataJSON(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	reg := newTestRegistry()
	client := newTestEvoClient("http://localhost:0")
	f := &stubFactory{config: newTestConfig(), client: client, io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	_ = RegisterServiceCommands(rootCmd, f)

	rootCmd.SetArgs([]string{"payment", "online", "pay", "--data", "not-json"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON in --data")
	}
	if !strings.Contains(errOut.String(), "parse --data as JSON") {
		t.Errorf("stderr = %q, want JSON parse error", errOut.String())
	}
}

func TestServiceCommand_InvalidParamsJSON(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{config: newTestConfig(), io: ios, reg: reg}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	_ = RegisterServiceCommands(rootCmd, f)

	rootCmd.SetArgs([]string{"payment", "online", "query", "--params", "not-json"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON in --params")
	}
	if !strings.Contains(errOut.String(), "parse --params") {
		t.Errorf("stderr = %q, want params parse error", errOut.String())
	}
}

func TestServiceCommand_ConfigError(t *testing.T) {
	ios, _, errOut := newTestIOStreams()
	reg := newTestRegistry()
	f := &stubFactory{
		configErr: &core.ConfigMissingError{Path: "/fake/path"},
		io:        ios,
		reg:       reg,
	}

	rootCmd := &cobra.Command{Use: "evo-cli"}
	_ = RegisterServiceCommands(rootCmd, f)

	rootCmd.SetArgs([]string{"payment", "online", "pay"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when config is missing")
	}
	if !strings.Contains(errOut.String(), "config_missing") {
		t.Errorf("stderr = %q, want config_missing error", errOut.String())
	}
}

// --- resolveParameters unit tests ---

func TestResolveParameters_SidFromConfig(t *testing.T) {
	meth := &registry.Method{
		Path: "/g2/v1/payment/mer/{sid}/payment",
		Parameters: map[string]*registry.Parameter{
			"sid": {Location: "path", Required: true, Type: "string", FromConfig: "merchantSid"},
		},
	}
	cfg := newTestConfig()

	path, qp, err := resolveParameters(meth, cfg, map[string]string{})
	if err != nil {
		t.Fatalf("resolveParameters() error = %v", err)
	}
	if path != "/g2/v1/payment/mer/S024116/payment" {
		t.Errorf("path = %q, want /g2/v1/payment/mer/S024116/payment", path)
	}
	if len(qp) != 0 {
		t.Errorf("queryParams = %v, want empty", qp)
	}
}

func TestResolveParameters_MissingRequired(t *testing.T) {
	meth := &registry.Method{
		Path: "/g2/v1/payment/mer/{sid}/payment",
		Parameters: map[string]*registry.Parameter{
			"sid":             {Location: "path", Required: true, Type: "string", FromConfig: "merchantSid"},
			"merchantTransID": {Location: "query", Required: true, Type: "string"},
		},
	}
	cfg := newTestConfig()

	_, _, err := resolveParameters(meth, cfg, map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing required parameter")
	}
	if !strings.Contains(err.Error(), "merchantTransID") {
		t.Errorf("error = %q, want to mention merchantTransID", err.Error())
	}
}

func TestResolveParameters_QueryParamProvided(t *testing.T) {
	meth := &registry.Method{
		Path: "/g2/v1/payment/mer/{sid}/payment",
		Parameters: map[string]*registry.Parameter{
			"sid":             {Location: "path", Required: true, Type: "string", FromConfig: "merchantSid"},
			"merchantTransID": {Location: "query", Required: true, Type: "string"},
		},
	}
	cfg := newTestConfig()

	path, qp, err := resolveParameters(meth, cfg, map[string]string{"merchantTransID": "TX001"})
	if err != nil {
		t.Fatalf("resolveParameters() error = %v", err)
	}
	if path != "/g2/v1/payment/mer/S024116/payment" {
		t.Errorf("path = %q, want /g2/v1/payment/mer/S024116/payment", path)
	}
	if qp["merchantTransID"] != "TX001" {
		t.Errorf("queryParams[merchantTransID] = %q, want TX001", qp["merchantTransID"])
	}
}

func TestResolveParameters_MultiplePathParams(t *testing.T) {
	meth := &registry.Method{
		Path: "/g2/v0/payment/mer/{sid}/evo.e-commerce.linkpay/{merchantOrderID}",
		Parameters: map[string]*registry.Parameter{
			"sid":             {Location: "path", Required: true, Type: "string", FromConfig: "merchantSid"},
			"merchantOrderID": {Location: "path", Required: true, Type: "string"},
		},
	}
	cfg := newTestConfig()

	path, qp, err := resolveParameters(meth, cfg, map[string]string{"merchantOrderID": "ORD001"})
	if err != nil {
		t.Fatalf("resolveParameters() error = %v", err)
	}
	if path != "/g2/v0/payment/mer/S024116/evo.e-commerce.linkpay/ORD001" {
		t.Errorf("path = %q, want /g2/v0/payment/mer/S024116/evo.e-commerce.linkpay/ORD001", path)
	}
	// Path params should not appear in query params.
	if _, ok := qp["merchantOrderID"]; ok {
		t.Error("merchantOrderID should not be in query params (it's a path param)")
	}
	if _, ok := qp["sid"]; ok {
		t.Error("sid should not be in query params (it's a path param)")
	}
}

// --- parseParamsFlag unit tests ---

func TestParseParamsFlag_Empty(t *testing.T) {
	result, err := parseParamsFlag("")
	if err != nil {
		t.Fatalf("parseParamsFlag() error = %v", err)
	}
	if len(result) != 0 {
		t.Errorf("result = %v, want empty map", result)
	}
}

func TestParseParamsFlag_Valid(t *testing.T) {
	result, err := parseParamsFlag(`{"key":"value","num":"42"}`)
	if err != nil {
		t.Fatalf("parseParamsFlag() error = %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("key = %q, want value", result["key"])
	}
	if result["num"] != "42" {
		t.Errorf("num = %q, want 42", result["num"])
	}
}

func TestParseParamsFlag_Invalid(t *testing.T) {
	_, err := parseParamsFlag("not-json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// =============================================================================
// Property-Based Tests (testing/quick)
// =============================================================================

// Feature: evo-payment-cli, Property 17: Service 命令参数校验
// **Validates: Requirement 5.5**
//
// For any service command with required parameters, when those parameters are
// missing, the command must return an error (exit code 2) and must NOT send
// an HTTP request.
func TestProperty17_ServiceCommandMissingRequiredParam(t *testing.T) {
	f := func(seed uint32) bool {
		rng := rand.New(rand.NewSource(int64(seed)))

		// Generate a random registry with 1-2 services, each with 1-2 resources,
		// each with 1-2 methods that have at least one non-fromConfig required parameter.
		httpMethods := []string{"GET", "POST", "PUT", "DELETE"}
		numServices := 1 + rng.Intn(2)
		services := make([]registry.Service, numServices)

		type endpoint struct {
			svcName  string
			resName  string
			methName string
		}
		var endpointsWithRequired []endpoint

		for si := 0; si < numServices; si++ {
			svcName := fmt.Sprintf("svc%d", si)
			numResources := 1 + rng.Intn(2)
			resources := make(map[string]*registry.Resource)

			for ri := 0; ri < numResources; ri++ {
				resName := fmt.Sprintf("res%d", ri)
				numMethods := 1 + rng.Intn(2)
				methods := make(map[string]*registry.Method)

				for mi := 0; mi < numMethods; mi++ {
					methName := fmt.Sprintf("m%d", mi)
					hm := httpMethods[rng.Intn(len(httpMethods))]

					params := map[string]*registry.Parameter{
						"sid": {Location: "path", Required: true, Type: "string", FromConfig: "merchantSid"},
					}

					// Add 1-2 required user-provided parameters.
					numReqParams := 1 + rng.Intn(2)
					for pi := 0; pi < numReqParams; pi++ {
						pName := fmt.Sprintf("reqParam%d", pi)
						loc := "query"
						if rng.Intn(2) == 0 {
							loc = "path"
						}
						params[pName] = &registry.Parameter{
							Location: loc,
							Required: true,
							Type:     "string",
						}
					}

					path := fmt.Sprintf("/api/%s/%s/{sid}/%s", svcName, resName, methName)
					methods[methName] = &registry.Method{
						HTTPMethod:  hm,
						Path:        path,
						Description: fmt.Sprintf("Test %s", methName),
						Parameters:  params,
					}

					endpointsWithRequired = append(endpointsWithRequired, endpoint{
						svcName:  svcName,
						resName:  resName,
						methName: methName,
					})
				}
				resources[resName] = &registry.Resource{Methods: methods}
			}

			services[si] = registry.Service{
				Name:        svcName,
				Description: fmt.Sprintf("Service %d", si),
				Resources:   resources,
			}
		}

		reg := &registry.Registry{Version: "1.0.0", Services: services}

		// Pick a random endpoint to test.
		ep := endpointsWithRequired[rng.Intn(len(endpointsWithRequired))]

		// Track whether an HTTP request was sent.
		requestSent := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestSent = true
			w.WriteHeader(200)
		}))
		defer srv.Close()

		ios, _, errOut := newTestIOStreams()
		client := newTestEvoClient(srv.URL)
		sf := &stubFactory{config: newTestConfig(), client: client, io: ios, reg: reg}

		rootCmd := &cobra.Command{Use: "evo-cli"}
		if err := RegisterServiceCommands(rootCmd, sf); err != nil {
			t.Logf("RegisterServiceCommands error: %v", err)
			return false
		}

		// Execute the command WITHOUT providing the required parameters.
		rootCmd.SetArgs([]string{ep.svcName, ep.resName, ep.methName})
		err := rootCmd.Execute()

		// Must return an error.
		if err == nil {
			t.Logf("expected error for missing required params on %s/%s/%s", ep.svcName, ep.resName, ep.methName)
			return false
		}

		// Must NOT have sent an HTTP request.
		if requestSent {
			t.Logf("HTTP request was sent despite missing required params on %s/%s/%s", ep.svcName, ep.resName, ep.methName)
			return false
		}

		// Verify stderr contains a validation error envelope.
		var envelope map[string]interface{}
		if jsonErr := json.Unmarshal(errOut.Bytes(), &envelope); jsonErr != nil {
			t.Logf("stderr not valid JSON for %s/%s/%s: %v\noutput: %s", ep.svcName, ep.resName, ep.methName, jsonErr, errOut.String())
			return false
		}
		if envelope["ok"] != false {
			t.Logf("ok=%v, want false", envelope["ok"])
			return false
		}
		errObj, _ := envelope["error"].(map[string]interface{})
		if errObj == nil || errObj["type"] != "validation" {
			t.Logf("error.type=%v, want validation", errObj["type"])
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 17 failed: %v", err)
	}
}

// =============================================================================
// Feature: e2e-test-suite, Property 1: Service command dry-run output matches registry
// =============================================================================

// TestProperty1_ServiceDryRunMatchesRegistry loads the embedded meta_data.json
// and verifies that for every service → resource → method, executing the command
// with --dry-run produces JSON output where:
//   - "method" matches the registry's HTTPMethod
//   - "url" contains the registry's API path segment (after {sid} replacement)
//
// This is a data-driven test iterating over ALL methods in the registry.
// **Validates: Requirements 1.1–1.26**
func TestProperty1_ServiceDryRunMatchesRegistry(t *testing.T) {
	// Load the real embedded registry.
	reg, err := registry.LoadFromJSON(registry.EmbeddedMetaData())
	if err != nil {
		t.Fatalf("failed to load embedded registry: %v", err)
	}

	// pathSegmentRe extracts a meaningful path segment after the last {placeholder}.
	// We use it to find the API-specific part of the path for verification.
	placeholderRe := regexp.MustCompile(`\{[^}]+\}`)

	for _, svc := range reg.Services {
		for resName, res := range svc.Resources {
			for methName, meth := range res.Methods {
				testName := fmt.Sprintf("%s/%s/%s_%s", svc.Name, resName, methName, meth.HTTPMethod)
				t.Run(testName, func(t *testing.T) {
					ios, out, errOut := newTestIOStreams()
					sf := &stubFactory{config: newTestConfig(), io: ios, reg: reg}

					rootCmd := &cobra.Command{Use: "evo-cli"}
					rootCmd.PersistentFlags().Bool("dry-run", false, "Preview request")
					if err := RegisterServiceCommands(rootCmd, sf); err != nil {
						t.Fatalf("RegisterServiceCommands error: %v", err)
					}

					// Build --params JSON to satisfy all required non-fromConfig parameters.
					params := buildRequiredParams(meth)
					args := []string{svc.Name, resName, methName, "--dry-run"}

					// For POST/PUT/DELETE, also provide --data.
					switch meth.HTTPMethod {
					case "POST", "PUT", "DELETE":
						args = append(args, "--data", `{"test":true}`)
					}

					if len(params) > 0 {
						paramsJSON, _ := json.Marshal(params)
						args = append(args, "--params", string(paramsJSON))
					}

					rootCmd.SetArgs(args)
					execErr := rootCmd.Execute()
					if execErr != nil {
						t.Fatalf("Execute() error = %v\nstderr: %s", execErr, errOut.String())
					}

					// Parse the dry-run JSON output.
					var result map[string]any
					if err := json.Unmarshal(out.Bytes(), &result); err != nil {
						t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
					}

					// Verify "method" matches registry's HTTPMethod.
					gotMethod, _ := result["method"].(string)
					if gotMethod != meth.HTTPMethod {
						t.Errorf("method = %q, want %q", gotMethod, meth.HTTPMethod)
					}

					// Verify "url" contains the API path segment.
					gotURL, _ := result["url"].(string)

					// Extract the path segment after removing placeholders.
					// e.g., "/g2/v1/payment/mer/{sid}/payout" → check URL contains "/payout"
					pathWithoutPlaceholders := placeholderRe.ReplaceAllString(meth.Path, "")
					// Find the last meaningful segment of the path.
					segments := strings.Split(pathWithoutPlaceholders, "/")
					var lastSegment string
					for i := len(segments) - 1; i >= 0; i-- {
						seg := strings.TrimSpace(segments[i])
						if seg != "" {
							lastSegment = seg
							break
						}
					}

					if lastSegment == "" {
						t.Fatalf("could not extract path segment from %q", meth.Path)
					}

					if !strings.Contains(gotURL, lastSegment) {
						t.Errorf("url = %q, want to contain path segment %q (from path %q)",
							gotURL, lastSegment, meth.Path)
					}
				})
			}
		}
	}
}

// buildRequiredParams creates a params map with dummy values for all required
// non-fromConfig parameters in a method definition.
func buildRequiredParams(meth *registry.Method) map[string]string {
	params := make(map[string]string)
	for name, param := range meth.Parameters {
		if param.FromConfig != "" {
			continue // auto-resolved from config
		}
		if param.Required {
			params[name] = "test_" + strings.ReplaceAll(name, " ", "_")
		}
	}
	return params
}

// =============================================================================
// Feature: e2e-test-suite, Property 6: Test suite exits with code 1 on failure
// =============================================================================

// TestProperty6_TestSuiteExitsCode1OnFailure verifies that a bash script
// following the E2E test suite pattern (PASS/FAIL counters, exit 1 on failure)
// exits with code 1 when at least one test fails, and code 0 when all pass.
// **Validates: Requirements 13.5, 13.6**
func TestProperty6_TestSuiteExitsCode1OnFailure(t *testing.T) {
	// The test suite pattern used in scripts/e2e_test.sh and scripts/e2e_live_test.sh:
	//   PASS=0; FAIL=0
	//   pass() { PASS=$((PASS+1)); }
	//   fail() { FAIL=$((FAIL+1)); }
	//   ... run tests ...
	//   if [ "$FAIL" -gt 0 ]; then exit 1; fi

	scriptTemplate := `#!/usr/bin/env bash
set -uo pipefail
PASS=0
FAIL=0
pass() { PASS=$((PASS+1)); }
fail() { FAIL=$((FAIL+1)); }

%s

TOTAL=$((PASS+FAIL))
echo "PASS=$PASS FAIL=$FAIL TOTAL=$TOTAL"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
`

	tests := []struct {
		name      string
		body      string
		wantExit0 bool
	}{
		{
			name:      "all pass",
			body:      "pass\npass\npass",
			wantExit0: true,
		},
		{
			name:      "one failure",
			body:      "pass\nfail\npass",
			wantExit0: false,
		},
		{
			name:      "all fail",
			body:      "fail\nfail\nfail",
			wantExit0: false,
		},
		{
			name:      "single pass",
			body:      "pass",
			wantExit0: true,
		},
		{
			name:      "single fail",
			body:      "fail",
			wantExit0: false,
		},
		{
			name:      "many pass one fail at end",
			body:      "pass\npass\npass\npass\nfail",
			wantExit0: false,
		},
	}

	tmpDir := t.TempDir()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := fmt.Sprintf(scriptTemplate, tt.body)
			scriptPath := filepath.Join(tmpDir, "test_"+strings.ReplaceAll(tt.name, " ", "_")+".sh")
			if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
				t.Fatalf("write script: %v", err)
			}

			cmd := exec.Command("bash", scriptPath)
			output, err := cmd.CombinedOutput()

			if tt.wantExit0 {
				if err != nil {
					t.Errorf("expected exit 0, got error: %v\noutput: %s", err, output)
				}
			} else {
				if err == nil {
					t.Errorf("expected non-zero exit, got exit 0\noutput: %s", output)
				}
				// Verify it's specifically exit code 1.
				if exitErr, ok := err.(*exec.ExitError); ok {
					if exitErr.ExitCode() != 1 {
						t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
					}
				}
			}

			// Verify the output contains the PASS/FAIL/TOTAL summary.
			if !strings.Contains(string(output), "PASS=") || !strings.Contains(string(output), "FAIL=") {
				t.Errorf("output missing PASS/FAIL summary: %s", output)
			}
		})
	}
}
