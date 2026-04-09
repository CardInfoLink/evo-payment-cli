#!/usr/bin/env python3
"""Generate meta_data.json from Evo Payment swagger files.

Reads swagger-merchant-api.json and swagger-linkpay-api.json,
produces a unified meta_data.json for the CLI registry.
"""

import argparse
import json
import re
import sys


def parse_parameters(swagger_params):
    """Extract parameters from swagger parameter list."""
    params = {}
    if not swagger_params:
        return params
    for p in swagger_params:
        name = p.get("name", "")
        if not name or p.get("in") == "header":
            continue
        location = p.get("in", "query")
        param = {
            "location": location,
            "required": p.get("required", False),
            "type": p.get("schema", {}).get("type", "string"),
        }
        # Mark {sid} as fromConfig
        if name == "sid":
            param["fromConfig"] = "merchantSid"
        enum = p.get("schema", {}).get("enum")
        if enum:
            param["enum"] = enum
        params[name] = param
    return params


def parse_request_body(rb):
    """Extract top-level request body fields."""
    if not rb:
        return {}
    content = rb.get("content", {})
    json_content = content.get("application/json", {})
    schema = json_content.get("schema", {})
    props = schema.get("properties", {})
    required_fields = set(schema.get("required", []))

    body = {}
    for field_name, field_def in props.items():
        body[field_name] = {
            "type": field_def.get("type", "object"),
            "required": field_name in required_fields,
        }
    return body


def classify_path(path):
    """Classify a path into (service, resource, method_name, clean_path).
    
    Returns None if the path should be skipped (e.g., webhooks).
    """
    # Skip webhook/non-API paths
    if not path.startswith("/g2/"):
        return None

    # LinkPay paths
    if "evo.e-commerce.linkpay" in path:
        if "CancelorRefund" in path or "cancelorRefund" in path.lower():
            return ("linkpay", "order", None, path)  # method determined by HTTP method
        if "linkpayRefund" in path:
            return ("linkpay", "order", None, path)
        return ("linkpay", "order", None, path)

    # Merchant API paths: /g2/v1/payment/mer/{sid}/<resource>
    match = re.match(r"/g2/v\d+/payment/mer/\{?sid\}?/(\w+)", path)
    if match:
        resource_raw = match.group(1)
        resource_map = {
            "payment": "online",
            "capture": "online",
            "cancel": "online",
            "cancelOrRefund": "online",
            "refund": "online",
            "payout": "payout",
            "FXRateInquiry": "fxRate",
            "cryptogram": "cryptogram",
            "paymentMethod": "paymentMethod",
        }
        resource = resource_map.get(resource_raw, resource_raw)
        return ("payment", resource, None, path)

    return None


def method_name_from_path_and_verb(path, http_method, service):
    """Derive a method name from the path and HTTP verb."""
    http_method = http_method.upper()

    if service == "linkpay":
        if "CancelorRefund" in path or "cancelorRefund" in path.lower():
            return "cancelOrRefund"
        if "linkpayRefund" in path:
            if http_method == "GET":
                return "refundQuery"
            return "refund"
        # Base linkpay path
        if http_method == "POST":
            return "create"
        if http_method == "GET":
            return "query"

    # Merchant API
    segments = path.rstrip("/").split("/")
    last_segment = segments[-1] if segments else ""
    # Remove path params like {merchantOrderID}
    if last_segment.startswith("{"):
        last_segment = segments[-2] if len(segments) > 1 else ""

    resource_to_method = {
        "payment": {"POST": "pay", "GET": "query", "PUT": "submitAdditionalInfo"},
        "capture": {"POST": "capture", "GET": "captureQuery"},
        "cancel": {"POST": "cancel", "GET": "cancelQuery"},
        "cancelOrRefund": {"POST": "cancelOrRefund", "GET": "cancelOrRefundQuery"},
        "refund": {"POST": "refund", "GET": "refundQuery"},
        "payout": {"POST": "create", "GET": "query"},
        "FXRateInquiry": {"POST": "inquiry", "GET": "query"},
        "cryptogram": {"POST": "create", "GET": "query"},
        "paymentMethod": {"POST": "create", "GET": "list", "PUT": "update", "DELETE": "delete"},
    }

    if last_segment in resource_to_method:
        return resource_to_method[last_segment].get(http_method, http_method.lower())

    # For paths ending with a path param, look at the segment before
    for seg in reversed(segments):
        if not seg.startswith("{") and seg in resource_to_method:
            methods = resource_to_method[seg]
            return methods.get(http_method, http_method.lower())

    return http_method.lower()


def normalize_path(path):
    """Normalize swagger path: replace literal 'sid' with {sid}."""
    # Some swagger files use /mer/sid/ instead of /mer/{sid}/
    path = re.sub(r"/mer/sid/", "/mer/{sid}/", path)
    path = re.sub(r"/mer/sid$", "/mer/{sid}", path)
    # Clean up parameter names with spaces
    path = re.sub(r"\{[^}]*\}", lambda m: "{" + re.sub(r"\s+", "", m.group(0).strip("{}").split(" of ")[0]) + "}", path)
    return path


def process_swagger(filepath, existing_services):
    """Process a single swagger file and merge into existing_services dict."""
    with open(filepath) as f:
        swagger = json.load(f)

    paths = swagger.get("paths", {})

    for raw_path, methods in paths.items():
        clean_path = normalize_path(raw_path)
        classification = classify_path(clean_path)
        if classification is None:
            continue

        svc_name, res_name, _, _ = classification

        for http_verb, operation in methods.items():
            http_verb = http_verb.upper()
            if http_verb not in ("GET", "POST", "PUT", "DELETE"):
                continue

            meth_name = method_name_from_path_and_verb(clean_path, http_verb, svc_name)
            description = operation.get("summary", operation.get("description", meth_name))

            # Parse parameters (path + query, skip headers)
            params = parse_parameters(operation.get("parameters"))
            # Ensure {sid} param exists for all API paths
            if "{sid}" in clean_path and "sid" not in params:
                params["sid"] = {
                    "location": "path",
                    "required": True,
                    "type": "string",
                    "fromConfig": "merchantSid",
                }

            # Parse request body
            request_body = parse_request_body(operation.get("requestBody"))

            # Build method entry
            method_entry = {
                "httpMethod": http_verb,
                "path": clean_path,
                "description": description,
            }
            if params:
                method_entry["parameters"] = params
            if request_body:
                method_entry["requestBody"] = request_body

            # Merge into services structure
            if svc_name not in existing_services:
                svc_desc = "LinkPay hosted payment page" if svc_name == "linkpay" else "EC Payment APIs"
                existing_services[svc_name] = {
                    "name": svc_name,
                    "description": svc_desc,
                    "resources": {},
                }

            svc = existing_services[svc_name]
            if res_name not in svc["resources"]:
                svc["resources"][res_name] = {"methods": {}}

            # Avoid overwriting existing methods (first file wins for duplicates)
            res = svc["resources"][res_name]
            if meth_name not in res["methods"]:
                res["methods"][meth_name] = method_entry


def main():
    parser = argparse.ArgumentParser(description="Generate meta_data.json from swagger files")
    parser.add_argument("--merchant-input", required=True, help="Path to swagger-merchant-api.json")
    parser.add_argument("--linkpay-input", required=True, help="Path to swagger-linkpay-api.json")
    parser.add_argument("--output", required=True, help="Output path for meta_data.json")
    args = parser.parse_args()

    services = {}

    # Process merchant API first
    process_swagger(args.merchant_input, services)
    # Then LinkPay API
    process_swagger(args.linkpay_input, services)

    # Build final output
    output = {
        "version": "1.0.0",
        "services": list(services.values()),
    }

    with open(args.output, "w") as f:
        json.dump(output, f, indent=2, ensure_ascii=False)
        f.write("\n")

    # Summary
    total_methods = 0
    for svc in services.values():
        for res in svc["resources"].values():
            total_methods += len(res["methods"])
    print(f"Generated {args.output}: {len(services)} services, {total_methods} methods")


if __name__ == "__main__":
    main()
