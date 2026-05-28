import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import Form from "https://esm.sh/@rjsf/core@5.24.10?external=react";
import validator from "https://esm.sh/@rjsf/validator-ajv8@5.24.10?external=react";

const apiBase = inferAPIBase();

function inferAPIBase() {
  const marker = "/ui";
  const path = window.location.pathname;
  const index = path.indexOf(marker);
  if (index >= 0) {
    return `${path.slice(0, index)}/api`;
  }
  return "/_shapeshifter/api";
}

function App() {
  const [spec, setSpec] = useState(null);
  const [error, setError] = useState("");
  const [routeKey, setRouteKey] = useState("");
  const [contractID, setContractID] = useState("");
  const [phase, setPhase] = useState("request");
  const [formData, setFormData] = useState({});
  const [rawBody, setRawBody] = useState("{}");
  const [result, setResult] = useState(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    fetch(`${apiBase}/spec`)
      .then((res) => {
        if (!res.ok) throw new Error(`spec ${res.status}`);
        return res.json();
      })
      .then((nextSpec) => {
        setSpec(nextSpec);
        const firstEndpoint = nextSpec.endpoints?.[0];
        const firstContract = firstEndpoint?.contracts?.[0];
        if (firstEndpoint) setRouteKey(routeValue(firstEndpoint.route));
        if (firstContract) setContractID(firstContract.id);
      })
      .catch((err) => setError(err.message));
  }, []);

  const endpoint = useMemo(() => {
    return spec?.endpoints?.find((item) => routeValue(item.route) === routeKey) || null;
  }, [spec, routeKey]);

  const contract = useMemo(() => {
    return endpoint?.contracts?.find((item) => item.id === contractID) || endpoint?.contracts?.[0] || null;
  }, [endpoint, contractID]);

  useEffect(() => {
    if (endpoint?.contracts?.length && !endpoint.contracts.some((item) => item.id === contractID)) {
      setContractID(endpoint.contracts[0].id);
    }
  }, [endpoint, contractID]);

  const side = phase === "request" ? contract?.request : contract?.response;
  const schemaName = phase === "request" ? side?.shape : side?.source_shape || side?.shape;
  const schema = schemaName ? spec?.shape_schemas?.[schemaName] : null;

  useEffect(() => {
    if (!schema) {
      setFormData({});
      setRawBody("{}");
      return;
    }
    const seeded = seedForSchema(schema);
    setFormData(seeded);
    setRawBody(JSON.stringify(seeded, null, 2));
    setResult(null);
  }, [schemaName]);

  function updateFormData(nextData) {
    setFormData(nextData || {});
    setRawBody(JSON.stringify(nextData || {}, null, 2));
  }

  function updateRawBody(value) {
    setRawBody(value);
    try {
      setFormData(JSON.parse(value));
      setError("");
    } catch {
      setError("Body is not valid JSON");
    }
  }

  async function runPreview() {
    setBusy(true);
    setError("");
    setResult(null);
    let body;
    try {
      body = JSON.parse(rawBody);
    } catch {
      setError("Body is not valid JSON");
      setBusy(false);
      return;
    }
    try {
      const route = endpoint.route;
      const res = await fetch(`${apiBase}/process/${phase}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ route, contract: contract.id, body }),
      });
      const payload = await res.json();
      setResult(payload);
      if (!res.ok) setError(payload.error || `preview ${res.status}`);
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    React.createElement("div", { className: "shell" },
      React.createElement("header", { className: "topbar" },
        React.createElement("div", { className: "brand" },
          React.createElement("div", { className: "mark" }, "SS"),
          React.createElement("h1", null, "ShapeShifter")
        ),
        React.createElement("div", { className: "status" }, spec ? `${spec.endpoints?.length || 0} endpoint(s)` : "Loading")
      ),
      React.createElement("div", { className: "workspace" },
        React.createElement("aside", { className: "sidebar" },
          React.createElement(SelectField, {
            label: "Endpoint",
            value: routeKey,
            onChange: setRouteKey,
            options: (spec?.endpoints || []).map((item) => [routeValue(item.route), `${item.route.method} ${item.route.path}`]),
          }),
          React.createElement(SelectField, {
            label: "Contract",
            value: contractID,
            onChange: setContractID,
            options: (endpoint?.contracts || []).map((item) => [item.id, item.id]),
          }),
          React.createElement("div", { className: "tabs" },
            React.createElement("button", { className: phase === "request" ? "active" : "", onClick: () => setPhase("request") }, "Request"),
            React.createElement("button", { className: phase === "response" ? "active" : "", onClick: () => setPhase("response") }, "Response")
          ),
          React.createElement("div", { className: "field" },
            React.createElement("label", null, "Shape"),
            React.createElement("select", { value: schemaName || "", disabled: true },
              React.createElement("option", null, schemaName || "Unavailable")
            )
          ),
          error ? React.createElement("div", { className: "error" }, error) : null
        ),
        React.createElement("section", { className: "panel" },
          React.createElement("div", { className: "grid" },
            React.createElement("div", { className: "surface" },
              React.createElement("h2", null, "Input"),
              schema ? React.createElement("div", { className: "rjsf" },
                React.createElement(Form, {
                  schema,
                  validator,
                  formData,
                  onChange: (event) => updateFormData(event.formData),
                  liveValidate: false,
                  omitExtraData: false,
                  showErrorList: false,
                })
              ) : null,
              React.createElement("div", { className: "field" },
                React.createElement("label", null, "JSON"),
                React.createElement("textarea", { value: rawBody, onChange: (event) => updateRawBody(event.target.value), spellCheck: "false" })
              ),
              React.createElement("div", { className: "actions" },
                React.createElement("button", { onClick: runPreview, disabled: busy || !contract || !side }, busy ? "Running" : "Run Preview")
              )
            ),
            React.createElement("div", { className: "surface result" },
              React.createElement("h2", null, "Output"),
              React.createElement("pre", null, result ? JSON.stringify(result.payload || result, null, 2) : ""),
              result?.skipped_handlers?.length ? React.createElement("div", { className: "notice" }, `Skipped handlers: ${result.skipped_handlers.join(", ")}`) : null
            )
          )
        )
      )
    )
  );
}

function SelectField({ label, value, onChange, options }) {
  return React.createElement("div", { className: "field" },
    React.createElement("label", null, label),
    React.createElement("select", { value, onChange: (event) => onChange(event.target.value) },
      options.map(([optionValue, text]) => React.createElement("option", { key: optionValue, value: optionValue }, text))
    )
  );
}

function routeValue(route) {
  return `${route.method} ${route.path}`;
}

function seedForSchema(schema) {
  if (!schema || typeof schema !== "object") return {};
  if (schema.default !== undefined) return schema.default;
  if (schema.type === "object" || schema.properties) {
    const out = {};
    for (const [key, child] of Object.entries(schema.properties || {})) {
      out[key] = seedForSchema(child);
    }
    return out;
  }
  if (schema.type === "array") return [];
  if (schema.type === "integer" || schema.type === "number") return 0;
  if (schema.type === "boolean") return false;
  return "";
}

createRoot(document.getElementById("root")).render(React.createElement(App));
