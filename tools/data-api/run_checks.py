#!/usr/bin/env python3
"""Execute this project's DATA-API checks, including its external S3 photo step."""
from __future__ import annotations

import argparse
import ipaddress
import json
import re
import sys
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode, urlsplit
from urllib.request import HTTPRedirectHandler, Request, build_opener

import yaml
from jsonschema import Draft202012Validator, FormatChecker
from validate_data_api import semantic_checks

ROOT = Path(__file__).resolve().parents[2]
VARIABLE = re.compile(r"\$\{([A-Za-z_][A-Za-z0-9_.-]*)\}")
ROLES = {"resident": "RESIDENT", "neighbor": "RESIDENT", "chairman": "CHAIRMAN"}


class CheckError(Exception):
    pass


class NoRedirects(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def lookup(value, path):
    """JSONPath subset used by the manifest: $.field, $.items[0].id."""
    path = path.removeprefix("$.")
    if not re.fullmatch(r"\w+(?:\[\d+\])?(?:\.\w+(?:\[\d+\])?)*", path):
        raise CheckError("Unsupported JSON path")
    try:
        for name, index in re.findall(r"(\w+)|\[(\d+)\]", path):
            value = value[name] if name else value[int(index)]
    except (KeyError, IndexError, TypeError):
        raise CheckError(f"Missing response field: {path}") from None
    return value


def expand(value, variables):
    if isinstance(value, str):
        def replace(match):
            if match[1] not in variables:
                raise CheckError(f"Missing variable: {match[1]}")
            return str(variables[match[1]])
        return VARIABLE.sub(replace, value)
    if isinstance(value, list):
        return [expand(v, variables) for v in value]
    if isinstance(value, dict):
        return {k: expand(v, variables) for k, v in value.items()}
    return value


def origin(url):
    p = urlsplit(url)
    if p.username or p.password or not p.hostname:
        raise CheckError("Invalid URL authority")
    return f"{p.scheme}://{p.netloc}"


def validate_base(url, allow_http):
    p = urlsplit(url)
    origin(url)
    if p.query or p.fragment or p.path not in ("", "/"):
        raise CheckError("API base must be an origin without a path or query")
    if p.scheme == "https":
        return
    try:
        local = ipaddress.ip_address(p.hostname).is_loopback
    except ValueError:
        local = p.hostname == "localhost"
    if not (allow_http and p.scheme == "http" and local):
        raise CheckError("HTTPS required; test HTTP is restricted to loopback")


def validate_document(doc):
    schema = json.loads((Path(__file__).parent / "DATA-API.schema.json").read_text(encoding="utf8"))
    if list(Draft202012Validator(schema, format_checker=FormatChecker()).iter_errors(doc)):
        raise CheckError("DATA-API does not match the organizer schema")
    errors, warnings = semantic_checks(doc)
    if errors or warnings:
        raise CheckError("DATA-API semantic validation failed; run validate_data_api.py for details")


def validate_response(expected, status, content_type, data):
    if status not in expected["statusCodes"]:
        raise CheckError(f"HTTP {status}; expected {expected['statusCodes']}")
    if expected.get("contentType") and content_type.split(";", 1)[0] != expected["contentType"]:
        raise CheckError("Unexpected response Content-Type")
    for field in expected.get("requiredFields", []):
        lookup(data, field)
    errors = list(Draft202012Validator(expected.get("bodySchema", {}), format_checker=FormatChecker()).iter_errors(data))
    if errors:
        # Validation messages can contain signed URLs; report only the field path.
        raise CheckError("Response schema mismatch at " + ".".join(map(str, errors[0].absolute_path)))


class Runner:
    def __init__(self, doc, base, app_origin, sessions, house, s3_origins, allow_http=False):
        validate_base(base, allow_http)
        self.doc, self.base, self.app_origin = doc, base.rstrip("/"), app_origin
        self.sessions, self.house = sessions, house
        self.s3_origins, self.allow_http = set(s3_origins), allow_http
        self.variables, self.completed, self.results = {}, set(), []
        self.client = build_opener(NoRedirects())
        self.photo = (ROOT / "scripts/demo-content/images/stair-light.png").read_bytes()
        upload = next(c for c in doc["checks"] if c["id"] == "photo-upload")
        if upload["request"]["body"]["size_bytes"] != len(self.photo):
            raise CheckError("Photo size differs from photo-upload request")

    def request(self, method, url, headers, body=None, timeout=10):
        req = Request(url, data=body, method=method, headers=headers)
        try:
            response = self.client.open(req, timeout=timeout)
        except HTTPError as error:
            response = error
        except (URLError, TimeoutError, OSError):
            raise CheckError("Network/TLS error (URL and credentials omitted)") from None
        with response:
            data = response.read(21 * 1024 * 1024)
            if len(data) > 20 * 1024 * 1024:
                raise CheckError("Response exceeds 20 MiB")
            return response.status, response.headers.get("Content-Type", ""), data

    def api(self, role, method, path, body=None, headers=None, timeout=10):
        if not path.startswith("/api/v1/") or path.startswith("//"):
            raise CheckError("Only Gateway /api/v1 paths are supported")
        h = dict(self.doc["api"].get("defaultHeaders", {}))
        h.update(headers or {})
        h["Origin"] = self.app_origin
        if role != "public":
            token = self.sessions.get(role)
            if not isinstance(token, str) or not re.fullmatch(r"[A-Za-z0-9_-]+", token):
                raise CheckError(f"Missing/invalid sq_session value for {role}")
            h["Cookie"] = "sq_session=" + token
        raw = None if body is None else json.dumps(body, ensure_ascii=False).encode()
        status, content_type, data = self.request(method, self.base + path, h, raw, timeout)
        try:
            return status, content_type, json.loads(data)
        except (ValueError, UnicodeError):
            raise CheckError(f"Gateway returned non-JSON HTTP {status}") from None

    def context(self, role):
        status, _, data = self.api(role, "GET", "/api/v1/me")
        if status != 200:
            raise CheckError(f"Session for {role}: HTTP {status}; log in again through MAX")
        uid = data["user"]["id"]
        member = any(m["house_id"] == self.house and m["user_id"] == uid
                     and m["status"] == "ACTIVE" and m["role"] == ROLES[role]
                     for m in data["memberships"])
        if data["active_house_id"] != self.house or not member:
            raise CheckError(f"{role}: wrong active house or membership; no role/house changes made")
        return uid

    def preflight(self):
        users = [self.context(role) for role in ROLES]
        if len(set(users)) != 3:
            raise CheckError("resident, neighbor and chairman must be three different users")

    def storage(self, method, url, headers=None, body=None):
        if origin(url) not in self.s3_origins:
            raise CheckError("S3 origin is not allowed; add its exact origin with --s3-origin")
        if urlsplit(url).scheme != "https" and not self.allow_http:
            raise CheckError("S3 HTTPS is required")
        # Never forward the Gateway cookie, Origin or authentication to S3.
        headers = headers or {}
        if any(k.lower() in ("cookie", "authorization", "proxy-authorization", "host") for k in headers):
            raise CheckError("Unexpected sensitive header in S3 required_headers")
        status, _, data = self.request(method, url, headers, body, 60)
        if status != 200:
            raise CheckError(f"External S3 {method} returned HTTP {status}")
        return data

    def step(self, step, cleanup=False):
        if not cleanup and not set(step.get("dependsOn", [])).issubset(self.completed):
            raise CheckError("A required prior check did not pass")
        req = expand(step.get("request", {}), self.variables)
        path = step["path"]
        for key, value in req.get("path", {}).items():
            path = path.replace("{"+key+"}", quote(str(value), safe=""))
        if req.get("query"):
            path += "?" + urlencode(req["query"], doseq=True)
        if step["method"] != "GET":
            self.context(step["role"])
        status, content_type, data = self.api(step["role"], step["method"], path,
                                            req.get("body"), req.get("headers"), step.get("timeoutMs", 10000)/1000)
        if 200 <= status < 300:
            for key, value in step.get("extract", {}).items():
                self.variables[key] = lookup(data, value)
        validate_response(step.get("expected", {"statusCodes": [200]}), status, content_type, data)
        if step["id"] == "photo-upload":
            self.storage("PUT", data["presigned_url"], data["required_headers"], self.photo)
        if step["id"] == "photo-download":
            if self.storage("GET", data["url"]) != self.photo:
                raise CheckError("Downloaded photo differs from the uploaded fixture")
        self.completed.add(step["id"])
        self.results.append({"id": step["id"], "cleanup": cleanup, "status": status})
        print(f"PASS {'cleanup ' if cleanup else ''}{step['id']} ({status})", flush=True)

    def run(self):
        self.preflight()
        failure = None
        try:
            for step in self.doc["checks"]:
                try:
                    self.step(step)
                except CheckError as error:
                    raise CheckError(f"{step['id']}: {error}") from None
        except CheckError as error:
            failure = error
        finally:
            for step in self.doc.get("cleanup", []):
                required = VARIABLE.findall(json.dumps(step.get("request", {})))
                if any(key not in self.variables for key in required):
                    continue
                try:
                    self.step(step, cleanup=True)
                except CheckError as error:
                    print(f"FAIL cleanup {step['id']}: {error}", file=sys.stderr)
                    failure = failure or CheckError("Cleanup incomplete")
            ids = {key: value for key, value in self.variables.items() if key.endswith("Id")}
            print("Created resource IDs: " + json.dumps(ids, ensure_ascii=False))
            print("Issues, announcements and closed polls/initiatives remain as review history; no DELETE API exists.")
        if failure:
            raise failure
        return len(self.doc["checks"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, default=ROOT / "DATA-API.yaml")
    parser.add_argument("--execute", action="store_true", help="Create review data; otherwise print the plan only")
    parser.add_argument("--base-url")
    parser.add_argument("--origin")
    parser.add_argument("--house", help="UUID of the dedicated review house, already active for all three users")
    group = parser.add_mutually_exclusive_group()
    group.add_argument("--sessions", type=Path, help="Private JSON file containing sq_session values by role")
    group.add_argument("--sessions-stdin", action="store_true")
    parser.add_argument("--s3-origin", action="append", default=[], help="Allowed exact S3 origin; repeat for additional origins")
    parser.add_argument("--allow-http-for-tests", action="store_true", help="Loopback Gateway only, isolated integration tests")
    args = parser.parse_args()
    doc = yaml.safe_load(args.manifest.read_text(encoding="utf8"))
    validate_document(doc)
    if not args.execute:
        for c in doc["checks"]:
            print(f"{c['id']}: {c['role']} {c['method']} {c['path']}")
        print("Plan only. No network requests. Use --execute with a dedicated house and private sessions.")
        return
    if not args.house or not (args.sessions or args.sessions_stdin) or not args.s3_origin:
        parser.error("--execute requires --house, --sessions/--sessions-stdin and --s3-origin")
    sessions = json.loads(sys.stdin.read() if args.sessions_stdin else args.sessions.read_text(encoding="utf8"))
    base = args.base_url or doc["api"]["baseUrl"]
    runner = Runner(doc, base, args.origin or origin(base), sessions, args.house, args.s3_origin, args.allow_http_for_tests)
    print(f"PASS: {runner.run()} API checks and external S3 upload/download")


if __name__ == "__main__":
    try:
        main()
    except (CheckError, ValueError, KeyError, OSError) as error:
        if isinstance(error, CheckError):
            print(f"FAIL: {error}", file=sys.stderr)
        else:
            print("FAIL: invalid input or unavailable local file; sensitive values omitted", file=sys.stderr)
        sys.exit(1)
