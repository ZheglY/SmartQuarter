import contextlib
import io
import json
import unittest
from unittest.mock import Mock

import yaml
from jsonschema import Draft202012Validator, FormatChecker

from run_checks import ROOT, CheckError, Runner, expand, lookup, validate_base, validate_document, validate_response


class SubmissionChecks(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.doc = yaml.safe_load((ROOT / "DATA-API.yaml").read_text(encoding="utf8"))

    def test_organizer_schema_and_semantics(self):
        validate_document(self.doc)

    def test_requests_and_statuses_match_openapi(self):
        spec = yaml.safe_load((ROOT / "contracts/openapi/openapi.yaml").read_text(encoding="utf8"))
        variables = {key: "11111111-1111-4111-8111-111111111111"
                     for step in self.doc["checks"] for key in step.get("extract", {})}
        for step in self.doc["checks"] + self.doc["cleanup"]:
            with self.subTest(step=step["id"]):
                operation = spec["paths"][step["path"]][step["method"].lower()]
                for status in step["expected"]["statusCodes"]:
                    self.assertIn(str(status), operation["responses"])
                request = expand(step.get("request", {}), variables)
                if "body" in request:
                    schema = operation["requestBody"]["content"]["application/json"]["schema"]
                    validator = Draft202012Validator(dict(schema, components=spec["components"]), format_checker=FormatChecker())
                    validator.validate(request["body"])
                for parameter in operation.get("parameters", []):
                    if parameter.get("required"):
                        values = self.doc["api"]["defaultHeaders"] if parameter["in"] == "header" else request.get(parameter["in"], {})
                        self.assertIn(parameter["name"], values)

    def test_jsonpath_and_substitution(self):
        self.assertEqual(lookup({"options": [{"id": "option"}]}, "$.options[0].id"), "option")
        self.assertEqual(expand({"ids": ["${id}"]}, {"id": "value"}), {"ids": ["value"]})
        with self.assertRaises(CheckError):
            expand("${missing}", {})

    def test_error_response_does_not_leak_values(self):
        with self.assertRaises(CheckError) as raised:
            validate_response({"statusCodes": [200], "bodySchema": {"properties": {"url": {"type": "integer"}}}},
                              200, "application/json", {"url": "secret-signed-url"})
        self.assertNotIn("secret-signed-url", str(raised.exception))

    def test_https_and_test_host_restrictions(self):
        validate_base("https://maxhack.ru", False)
        validate_base("http://127.0.0.1:1234", True)
        for url in ("http://maxhack.ru", "https://user:password@example.com", "https://example.com/api"):
            with self.assertRaises(CheckError):
                validate_base(url, True)

    def runner(self):
        return Runner(self.doc, "https://maxhack.ru", "https://maxhack.ru",
                      dict(resident="a", neighbor="b", chairman="c"), "review-house", ["https://storage.example.com"])

    def test_wrong_house_stops_before_mutation(self):
        runner = self.runner()
        runner.api = Mock(return_value=(200, "application/json", {
            "user": {"id": "user"}, "active_house_id": "wrong-house", "memberships": []}))
        with self.assertRaises(CheckError):
            runner.preflight()
        self.assertEqual(runner.api.call_args.args, ("resident", "GET", "/api/v1/me"))

    def test_three_distinct_accounts_required(self):
        runner = self.runner()
        runner.context = Mock(return_value="same-user")
        with self.assertRaises(CheckError):
            runner.preflight()

    def test_s3_never_receives_application_cookie(self):
        runner = self.runner()
        runner.request = Mock(return_value=(200, "image/png", b"photo"))
        runner.storage("PUT", "https://storage.example.com/object?signature=secret", {"Content-Type": "image/png"}, b"photo")
        self.assertEqual(runner.request.call_args.args[2], {"Content-Type": "image/png"})
        for url, headers in [("https://other.example.com/object", {}),
                             ("https://storage.example.com/object", {"Cookie": "secret"})]:
            with self.assertRaises(CheckError):
                runner.storage("PUT", url, headers, b"photo")

    def test_cleanup_after_failure_uses_only_created_ids(self):
        runner = self.runner()
        runner.preflight = Mock()
        runner.doc = {"checks": [{"id": "failure"}], "cleanup": self.doc["cleanup"]}
        runner.variables = {"eventId": "created-event"}
        calls = []

        def step(value, cleanup=False):
            calls.append((value["id"], cleanup))
            if not cleanup:
                raise CheckError("test failure")

        runner.step = step
        with contextlib.redirect_stdout(io.StringIO()), self.assertRaises(CheckError):
            runner.run()
        self.assertEqual(calls, [("failure", False), ("delete-event", True)])


if __name__ == "__main__":
    unittest.main()
