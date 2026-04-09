package registry

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"testing/quick"
	"time"
)

func TestLoadFromJSON_ValidData(t *testing.T) {
	data := []byte(`{
		"version": "1.0.0",
		"services": [
			{
				"name": "payment",
				"description": "EC Payment APIs",
				"resources": {
					"online": {
						"methods": {
							"pay": {
								"httpMethod": "POST",
								"path": "/g2/v1/payment/mer/{sid}/payment",
								"description": "Create a payment"
							}
						}
					}
				}
			}
		]
	}`)

	reg, err := LoadFromJSON(data)
	if err != nil {
		t.Fatalf("LoadFromJSON returned error: %v", err)
	}
	if reg.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", reg.Version)
	}
	if len(reg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(reg.Services))
	}
	if reg.Services[0].Name != "payment" {
		t.Errorf("expected service name 'payment', got %s", reg.Services[0].Name)
	}
	res, ok := reg.Services[0].Resources["online"]
	if !ok {
		t.Fatal("expected 'online' resource")
	}
	m, ok := res.Methods["pay"]
	if !ok {
		t.Fatal("expected 'pay' method")
	}
	if m.HTTPMethod != "POST" {
		t.Errorf("expected POST, got %s", m.HTTPMethod)
	}
}

func TestLoadFromJSON_InvalidJSON(t *testing.T) {
	_, err := LoadFromJSON([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadFromJSON_EmptyServices(t *testing.T) {
	data := []byte(`{"version": "0.1.0", "services": []}`)
	reg, err := LoadFromJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reg.Services) != 0 {
		t.Errorf("expected 0 services, got %d", len(reg.Services))
	}
}

func TestLoadRegistry_EmbeddedData(t *testing.T) {
	// Ensure no cache interferes — use a temp HOME.
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	reg, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry returned error: %v", err)
	}
	if reg.Version == "" {
		t.Error("expected non-empty version from embedded data")
	}
	if len(reg.Services) == 0 {
		t.Error("expected at least one service from embedded data")
	}
}

func TestLoadRegistry_EmbeddedDataContainsAllServices(t *testing.T) {
	reg, err := LoadFromJSON(embeddedMetaData)
	if err != nil {
		t.Fatalf("failed to parse embedded data: %v", err)
	}

	// Verify we have both payment and linkpay services.
	serviceNames := make(map[string]bool)
	for _, svc := range reg.Services {
		serviceNames[svc.Name] = true
	}
	for _, expected := range []string{"payment", "linkpay"} {
		if !serviceNames[expected] {
			t.Errorf("expected service %q in embedded data", expected)
		}
	}
}

func TestLoadRegistry_EmbeddedPaymentResources(t *testing.T) {
	reg, err := LoadFromJSON(embeddedMetaData)
	if err != nil {
		t.Fatalf("failed to parse embedded data: %v", err)
	}

	var paymentSvc *Service
	for i := range reg.Services {
		if reg.Services[i].Name == "payment" {
			paymentSvc = &reg.Services[i]
			break
		}
	}
	if paymentSvc == nil {
		t.Fatal("payment service not found")
	}

	expectedResources := []string{"online", "payout", "fxRate", "cryptogram", "paymentMethod"}
	for _, name := range expectedResources {
		if _, ok := paymentSvc.Resources[name]; !ok {
			t.Errorf("expected resource %q in payment service", name)
		}
	}
}

func TestLoadRegistry_EmbeddedOnlineMethods(t *testing.T) {
	reg, err := LoadFromJSON(embeddedMetaData)
	if err != nil {
		t.Fatalf("failed to parse embedded data: %v", err)
	}

	var paymentSvc *Service
	for i := range reg.Services {
		if reg.Services[i].Name == "payment" {
			paymentSvc = &reg.Services[i]
			break
		}
	}
	if paymentSvc == nil {
		t.Fatal("payment service not found")
	}

	online := paymentSvc.Resources["online"]
	if online == nil {
		t.Fatal("online resource not found")
	}

	expectedMethods := []string{
		"pay", "query", "capture", "cancel", "refund",
		"refundQuery", "cancelOrRefund", "cancelOrRefundQuery", "submitAdditionalInfo",
	}
	for _, name := range expectedMethods {
		if _, ok := online.Methods[name]; !ok {
			t.Errorf("expected method %q in online resource", name)
		}
	}
}

func TestLoadRegistry_FreshCacheTakesPriority(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Write a custom cache file with a different version.
	cacheData := `{"version": "2.0.0-cached", "services": []}`
	cDir := filepath.Join(tmpDir, cacheDir)
	if err := os.MkdirAll(cDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(cDir, cacheFileName)
	if err := os.WriteFile(cachePath, []byte(cacheData), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry returned error: %v", err)
	}
	if reg.Version != "2.0.0-cached" {
		t.Errorf("expected version '2.0.0-cached' from cache, got %s", reg.Version)
	}
}

func TestLoadRegistry_StaleCacheFallsBackToEmbedded(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Write a stale cache file (modified time > 24h ago).
	cacheData := `{"version": "0.9.0-stale", "services": []}`
	cDir := filepath.Join(tmpDir, cacheDir)
	if err := os.MkdirAll(cDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cp := filepath.Join(cDir, cacheFileName)
	if err := os.WriteFile(cp, []byte(cacheData), 0o644); err != nil {
		t.Fatal(err)
	}
	// Set mod time to 25 hours ago.
	staleTime := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(cp, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	reg, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry returned error: %v", err)
	}
	// Should fall back to embedded data (version "1.0.0").
	if reg.Version != "1.0.0" {
		t.Errorf("expected embedded version '1.0.0', got %s", reg.Version)
	}
}

func TestLoadRegistry_CorruptCacheFallsBackToEmbedded(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Write corrupt cache.
	cDir := filepath.Join(tmpDir, cacheDir)
	if err := os.MkdirAll(cDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cp := filepath.Join(cDir, cacheFileName)
	if err := os.WriteFile(cp, []byte(`{corrupt json`), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry returned error: %v", err)
	}
	if reg.Version != "1.0.0" {
		t.Errorf("expected embedded version '1.0.0', got %s", reg.Version)
	}
}

func TestWriteCache(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	testData := []byte(`{"version":"test","services":[]}`)
	if err := writeCache(testData); err != nil {
		t.Fatalf("writeCache returned error: %v", err)
	}

	cp := filepath.Join(tmpDir, cacheDir, cacheFileName)
	got, err := os.ReadFile(cp)
	if err != nil {
		t.Fatalf("failed to read cache file: %v", err)
	}
	if string(got) != string(testData) {
		t.Errorf("cache content mismatch: got %s", string(got))
	}
}

func TestLoadFromJSON_ParameterParsing(t *testing.T) {
	data := []byte(`{
		"version": "1.0.0",
		"services": [{
			"name": "test",
			"description": "test service",
			"resources": {
				"res": {
					"methods": {
						"m": {
							"httpMethod": "GET",
							"path": "/test/{sid}",
							"description": "test method",
							"parameters": {
								"sid": {
									"location": "path",
									"required": true,
									"type": "string",
									"fromConfig": "merchantSid"
								},
								"status": {
									"location": "query",
									"required": false,
									"type": "string",
									"enum": ["active", "inactive"]
								}
							}
						}
					}
				}
			}
		}]
	}`)

	reg, err := LoadFromJSON(data)
	if err != nil {
		t.Fatalf("LoadFromJSON returned error: %v", err)
	}

	m := reg.Services[0].Resources["res"].Methods["m"]

	sidParam := m.Parameters["sid"]
	if sidParam == nil {
		t.Fatal("expected 'sid' parameter")
	}
	if sidParam.Location != "path" {
		t.Errorf("expected location 'path', got %s", sidParam.Location)
	}
	if !sidParam.Required {
		t.Error("expected sid to be required")
	}
	if sidParam.FromConfig != "merchantSid" {
		t.Errorf("expected fromConfig 'merchantSid', got %s", sidParam.FromConfig)
	}

	statusParam := m.Parameters["status"]
	if statusParam == nil {
		t.Fatal("expected 'status' parameter")
	}
	if statusParam.Required {
		t.Error("expected status to be optional")
	}
	if len(statusParam.Enum) != 2 || statusParam.Enum[0] != "active" {
		t.Errorf("unexpected enum values: %v", statusParam.Enum)
	}
}

func TestEmbeddedMetaDataIsValidJSON(t *testing.T) {
	var raw json.RawMessage
	if err := json.Unmarshal(embeddedMetaData, &raw); err != nil {
		t.Fatalf("embedded meta_data.json is not valid JSON: %v", err)
	}
}

// =============================================================================
// Property-Based Tests (testing/quick)
// =============================================================================

// Feature: evo-payment-cli, Property 16: Registry 命令生成完整性
// **Validates: Requirements 5.3, 5.4**
//
// For any random meta_data.json (with random services/resources/methods),
// after LoadFromJSON, every endpoint has a corresponding entry in the Registry
// structure, and parameter definitions match.
func TestProperty16_RegistryCommandCompleteness(t *testing.T) {
	f := func(seed uint32) bool {
		rng := rand.New(rand.NewSource(int64(seed)))

		// Generate a random Registry with 1-3 services.
		numServices := 1 + rng.Intn(3)
		services := make([]map[string]interface{}, numServices)

		// Track expected structure for verification.
		type expectedParam struct {
			Location   string
			Required   bool
			Type       string
			FromConfig string
		}
		type expectedMethod struct {
			HTTPMethod string
			Path       string
			Params     map[string]expectedParam
		}
		type expectedEndpoint struct {
			Service  string
			Resource string
			Method   string
			Detail   expectedMethod
		}
		var expected []expectedEndpoint

		locations := []string{"path", "query"}
		httpMethods := []string{"GET", "POST", "PUT", "DELETE"}
		paramTypes := []string{"string", "integer", "boolean"}

		for si := 0; si < numServices; si++ {
			svcName := fmt.Sprintf("svc%d", si)
			numResources := 1 + rng.Intn(3)
			resources := make(map[string]interface{})

			for ri := 0; ri < numResources; ri++ {
				resName := fmt.Sprintf("res%d", ri)
				numMethods := 1 + rng.Intn(3)
				methods := make(map[string]interface{})

				for mi := 0; mi < numMethods; mi++ {
					methName := fmt.Sprintf("meth%d", mi)
					httpMethod := httpMethods[rng.Intn(len(httpMethods))]
					path := fmt.Sprintf("/api/v1/%s/%s/%s", svcName, resName, methName)

					// Generate 0-3 parameters.
					numParams := rng.Intn(4)
					params := make(map[string]interface{})
					expParams := make(map[string]expectedParam)

					for pi := 0; pi < numParams; pi++ {
						pName := fmt.Sprintf("p%d", pi)
						loc := locations[rng.Intn(len(locations))]
						req := rng.Intn(2) == 1
						pType := paramTypes[rng.Intn(len(paramTypes))]
						fromCfg := ""
						if rng.Intn(3) == 0 {
							fromCfg = "merchantSid"
						}

						paramDef := map[string]interface{}{
							"location": loc,
							"required": req,
							"type":     pType,
						}
						if fromCfg != "" {
							paramDef["fromConfig"] = fromCfg
						}
						params[pName] = paramDef
						expParams[pName] = expectedParam{
							Location:   loc,
							Required:   req,
							Type:       pType,
							FromConfig: fromCfg,
						}
					}

					methDef := map[string]interface{}{
						"httpMethod":  httpMethod,
						"path":        path,
						"description": fmt.Sprintf("desc %s", methName),
					}
					if len(params) > 0 {
						methDef["parameters"] = params
					}
					methods[methName] = methDef

					expected = append(expected, expectedEndpoint{
						Service:  svcName,
						Resource: resName,
						Method:   methName,
						Detail: expectedMethod{
							HTTPMethod: httpMethod,
							Path:       path,
							Params:     expParams,
						},
					})
				}
				resources[resName] = map[string]interface{}{"methods": methods}
			}

			services[si] = map[string]interface{}{
				"name":        svcName,
				"description": fmt.Sprintf("Service %d", si),
				"resources":   resources,
			}
		}

		regJSON := map[string]interface{}{
			"version":  "1.0.0",
			"services": services,
		}
		data, err := json.Marshal(regJSON)
		if err != nil {
			t.Logf("json.Marshal failed: %v", err)
			return false
		}

		reg, err := LoadFromJSON(data)
		if err != nil {
			t.Logf("LoadFromJSON failed: %v", err)
			return false
		}

		// Verify every expected endpoint exists in the Registry.
		for _, ep := range expected {
			// Find service.
			var svc *Service
			for i := range reg.Services {
				if reg.Services[i].Name == ep.Service {
					svc = &reg.Services[i]
					break
				}
			}
			if svc == nil {
				t.Logf("service %q not found", ep.Service)
				return false
			}

			// Find resource.
			res, ok := svc.Resources[ep.Resource]
			if !ok {
				t.Logf("resource %q not found in service %q", ep.Resource, ep.Service)
				return false
			}

			// Find method.
			meth, ok := res.Methods[ep.Method]
			if !ok {
				t.Logf("method %q not found in resource %q", ep.Method, ep.Resource)
				return false
			}

			// Verify HTTP method and path.
			if meth.HTTPMethod != ep.Detail.HTTPMethod {
				t.Logf("method %q: httpMethod=%q, want %q", ep.Method, meth.HTTPMethod, ep.Detail.HTTPMethod)
				return false
			}
			if meth.Path != ep.Detail.Path {
				t.Logf("method %q: path=%q, want %q", ep.Method, meth.Path, ep.Detail.Path)
				return false
			}

			// Verify parameter count matches.
			if len(meth.Parameters) != len(ep.Detail.Params) {
				t.Logf("method %q: param count=%d, want %d", ep.Method, len(meth.Parameters), len(ep.Detail.Params))
				return false
			}

			// Verify each parameter definition.
			for pName, expP := range ep.Detail.Params {
				gotP, ok := meth.Parameters[pName]
				if !ok {
					t.Logf("method %q: param %q not found", ep.Method, pName)
					return false
				}
				if gotP.Location != expP.Location {
					t.Logf("param %q: location=%q, want %q", pName, gotP.Location, expP.Location)
					return false
				}
				if gotP.Required != expP.Required {
					t.Logf("param %q: required=%v, want %v", pName, gotP.Required, expP.Required)
					return false
				}
				if gotP.Type != expP.Type {
					t.Logf("param %q: type=%q, want %q", pName, gotP.Type, expP.Type)
					return false
				}
				if gotP.FromConfig != expP.FromConfig {
					t.Logf("param %q: fromConfig=%q, want %q", pName, gotP.FromConfig, expP.FromConfig)
					return false
				}
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property 16 failed: %v", err)
	}
}
