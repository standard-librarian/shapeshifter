const STORAGE_KEY = "shapeshifter.examples.v1";
const TABS = ["Overview", "Request Preview", "Response Preview", "Try It Out", "Compare Versions", "Examples"];

const state = {
  config: { preview_api_base: "/_shapeshifter/api", try_it_out_enabled: false, try_it_out_base: "/" },
  spec: null,
  endpointKey: "",
  contractID: "",
  tab: "overview",
  search: "",
  tag: "",
  bodies: { request: "{}", response: "{}" },
  formData: { request: {}, response: {} },
  preview: { request: null, response: null },
  busy: false,
  error: "",
  pathParams: {},
  headers: [],
  tryResult: null,
  tryBusy: false,
  compare: { base: "", target: "", direction: "request" },
  localExamples: loadLocalExamples(),
};

const root = document.getElementById("root");

boot();

async function boot() {
  render();
  try {
    state.config = { ...state.config, ...(await fetchJSON("./config.json")) };
  } catch {
    state.config.preview_api_base = inferAPIBase();
  }
  try {
    state.spec = await fetchJSON(`${trimRight(state.config.preview_api_base, "/")}/spec`);
    applyInitialSelection();
    syncFromQuery();
    seedPhase("request");
    seedPhase("response");
    initTryHeaders();
  } catch (err) {
    state.error = err.message;
  }
  render();
}

async function fetchJSON(url, options) {
  const res = await fetch(url, options);
  const text = await res.text();
  let body = null;
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = text;
    }
  }
  if (!res.ok) {
    const message = body?.error || body?.message || `${url} returned ${res.status}`;
    const err = new Error(message);
    err.status = res.status;
    err.body = body;
    throw err;
  }
  return body;
}

function render() {
  const endpoint = currentEndpoint();
  const contract = currentContract();
  root.innerHTML = `
    <div class="shell">
      <header class="topbar">
        <div class="brand">
          <span class="mark">SS</span>
          <div>
            <h1>ShapeShifter</h1>
            <p>${escapeHTML(state.spec?.title || "Contract Portal")}</p>
          </div>
        </div>
        <div class="top-meta">
          <span>${state.spec ? `${state.spec.endpoints?.length || 0} endpoint(s)` : "Loading"}</span>
          <span class="${state.config.try_it_out_enabled ? "ok" : "muted"}">Try-it-out ${state.config.try_it_out_enabled ? "enabled" : "disabled"}</span>
        </div>
      </header>
      <div class="workspace">
        ${renderSidebar()}
        <main class="main">
          ${state.error ? `<div class="banner error">${escapeHTML(state.error)}</div>` : ""}
          ${endpoint && contract ? renderContent(endpoint, contract) : renderEmpty()}
        </main>
      </div>
    </div>
  `;
  bindEvents();
}

function renderSidebar() {
  const endpoints = filteredEndpoints();
  const groups = groupEndpoints(endpoints);
  return `
    <aside class="sidebar">
      <div class="field compact">
        <label for="endpoint-search">Search</label>
        <input id="endpoint-search" data-action="search" value="${escapeHTML(state.search)}" placeholder="Path, summary, tag" />
      </div>
      <div class="field compact">
        <label for="tag-filter">Tag</label>
        <select id="tag-filter" data-action="tag">
          <option value="">All tags</option>
          ${allTags().map((tag) => `<option value="${escapeHTML(tag)}" ${tag === state.tag ? "selected" : ""}>${escapeHTML(tag)}</option>`).join("")}
        </select>
      </div>
      <nav class="endpoint-list" aria-label="Endpoints">
        ${groups.map(([tag, items]) => `
          <section>
            <h2>${escapeHTML(tag)}</h2>
            ${items.map((endpoint) => {
              const key = endpointKey(endpoint);
              const deprecated = endpoint.contracts?.some((contract) => contract.deprecated);
              return `
                <button type="button" data-endpoint="${escapeHTML(key)}" class="endpoint ${key === state.endpointKey ? "active" : ""}">
                  <span class="method ${methodClass(endpoint.route.method)}">${escapeHTML(endpoint.route.method)}</span>
                  <span class="path">${escapeHTML(endpoint.route.path)}</span>
                  <span class="count">${endpoint.contracts?.length || 0}</span>
                  ${deprecated ? `<span class="deprecated">deprecated</span>` : ""}
                </button>
              `;
            }).join("")}
          </section>
        `).join("")}
      </nav>
    </aside>
  `;
}

function renderContent(endpoint, contract) {
  const visibleTabs = state.config.try_it_out_enabled ? TABS : TABS.filter((tab) => tab !== "Try It Out");
  if (!state.config.try_it_out_enabled && state.tab === "try") state.tab = "overview";
  return `
    <section class="endpoint-header">
      <div>
        <div class="route-line"><span class="method ${methodClass(endpoint.route.method)}">${escapeHTML(endpoint.route.method)}</span><strong>${escapeHTML(endpoint.route.path)}</strong></div>
        <h2>${escapeHTML(endpoint.summary || endpoint.route.path)}</h2>
        ${endpoint.description ? `<p>${escapeHTML(endpoint.description)}</p>` : ""}
      </div>
      <div class="selectors">
        <label>Contract</label>
        <select data-action="contract">
          ${(endpoint.contracts || []).map((item) => `<option value="${escapeHTML(item.id)}" ${item.id === contract.id ? "selected" : ""}>${escapeHTML(item.id)}${item.deprecated ? " (deprecated)" : ""}</option>`).join("")}
        </select>
      </div>
    </section>
    <div class="tabs">
      ${visibleTabs.map((tab) => `<button type="button" data-tab="${tabID(tab)}" class="${state.tab === tabID(tab) ? "active" : ""}">${escapeHTML(tab)}</button>`).join("")}
    </div>
    ${renderActiveTab(endpoint, contract)}
  `;
}

function renderActiveTab(endpoint, contract) {
  if (state.tab === "request") return renderPreviewPanel(endpoint, contract, "request");
  if (state.tab === "response") return renderPreviewPanel(endpoint, contract, "response");
  if (state.tab === "try") return renderTryItOut(endpoint, contract);
  if (state.tab === "compare") return renderCompare(endpoint, contract);
  if (state.tab === "examples") return renderExamples(endpoint, contract);
  return renderOverview(endpoint, contract);
}

function renderOverview(endpoint, contract) {
  return `
    <div class="content-grid">
      <section class="surface">
        <h3>Overview</h3>
        <dl class="facts">
          <dt>Default contract</dt><dd>${escapeHTML(endpoint.default_contract || "None")}</dd>
          <dt>Tags</dt><dd>${renderTags(endpoint.tags)}</dd>
          <dt>Request limit</dt><dd>${formatBytes(limitValue(endpoint.limits, "request"))}</dd>
          <dt>Response limit</dt><dd>${formatBytes(limitValue(endpoint.limits, "response"))}</dd>
        </dl>
        <h3>Contracts</h3>
        <div class="contract-cards">
          ${(endpoint.contracts || []).map((item) => `
            <article class="mini-card ${item.id === contract.id ? "selected" : ""}">
              <strong>${escapeHTML(item.id)}${item.deprecated ? " · deprecated" : ""}</strong>
              ${item.summary ? `<p>${escapeHTML(item.summary)}</p>` : ""}
              <small>${item.has_request ? "request" : ""}${item.has_request && item.has_response ? " + " : ""}${item.has_response ? "response" : ""}</small>
            </article>
          `).join("")}
        </div>
      </section>
      <section class="surface">
        <h3>Selected Contract</h3>
        ${contract.description ? `<p>${escapeHTML(contract.description)}</p>` : ""}
        <dl class="facts">
          <dt>Request shape</dt><dd>${escapeHTML(contract.request?.shape || "None")}</dd>
          <dt>Internal request shape</dt><dd>${escapeHTML(contract.request?.target_shape || "Not validated")}</dd>
          <dt>Internal response shape</dt><dd>${escapeHTML(contract.response?.source_shape || "Not validated")}</dd>
          <dt>External response shape</dt><dd>${escapeHTML(contract.response?.shape || "None")}</dd>
        </dl>
      </section>
    </div>
    <section class="surface">
      <h3>Transform Mappings</h3>
      ${renderMappingTable(contract)}
    </section>
  `;
}

function renderMappingTable(contract) {
  const rows = [];
  for (const phase of ["request", "response"]) {
    const side = contract[phase];
    if (!side) continue;
    const coercions = new Map((side.transform?.coerce || []).map((item) => [item.field, item.type]));
    const validations = (side.transform?.validate || []).map((item) => `${item.field}: ${item.error || item.rule}`).join("; ");
    if (side.transform?.passthrough) {
      rows.push([phase, phase === "request" ? ".external" : ".internal", phase === "request" ? ".internal" : ".external", "true", "", validations]);
    }
    for (const field of side.transform?.fields || []) {
      rows.push([phase, phase === "request" ? `.external${field.from.slice(1)}` : `.internal${field.from.slice(1)}`, phase === "request" ? `.internal${field.to.slice(1)}` : `.external${field.to.slice(1)}`, String(field.required), coercions.get(field.to) || "", validations]);
    }
  }
  if (!rows.length) return `<p class="muted">No mappings.</p>`;
  return `
    <div class="table-wrap">
      <table>
        <thead><tr><th>Direction</th><th>From</th><th>To</th><th>Required</th><th>Coerce</th><th>Validation</th></tr></thead>
        <tbody>${rows.map((row) => `<tr>${row.map((cell) => `<td>${escapeHTML(cell)}</td>`).join("")}</tr>`).join("")}</tbody>
      </table>
    </div>
  `;
}

function renderPreviewPanel(endpoint, contract, phase) {
  const side = contract[phase];
  if (!side) return `<section class="surface"><h3>${titleCase(phase)} Preview</h3><p class="muted">This contract has no ${phase} side.</p></section>`;
  const schema = inputSchemaFor(contract, phase);
  const result = state.preview[phase];
  return `
    <div class="preview-layout">
      <section class="surface">
        <div class="section-title">
          <div>
            <h3>${phase === "request" ? "Request Preview" : "Response Preview"}</h3>
            <p>${phase === "request" ? "external client request -> internal controller request" : "internal controller response -> external client response"}</p>
          </div>
          <button type="button" data-copy="${phase}-input">Copy input JSON</button>
        </div>
        ${renderExamplePicker(endpoint, contract, phase, "preview")}
        ${renderSchemaForm(schema, state.formData[phase], phase)}
        <div class="field">
          <label for="${phase}-raw">Raw JSON</label>
          <textarea id="${phase}-raw" data-raw="${phase}" spellcheck="false">${escapeHTML(state.bodies[phase])}</textarea>
        </div>
        <div class="actions">
          <button type="button" data-preview="${phase}" ${state.busy ? "disabled" : ""}>${state.busy ? "Running" : "Run Preview"}</button>
          <button type="button" data-seed="${phase}">Generated example</button>
          <button type="button" data-save-current="${phase}">Save local example</button>
        </div>
      </section>
      <aside class="surface output">
        <div class="section-title">
          <h3>Output</h3>
          <button type="button" data-copy="${phase}-output" ${result ? "" : "disabled"}>Copy output JSON</button>
        </div>
        ${renderPreviewResult(result)}
      </aside>
    </div>
  `;
}

function renderExamplePicker(endpoint, contract, phase, context) {
  const entries = exampleEntries(endpoint, contract, phase);
  return `
    <div class="picker-row">
      <div class="field">
        <label>${titleCase(phase)} examples</label>
        <select data-example-picker="${phase}">
          ${entries.map((entry) => `<option value="${escapeHTML(entry.id)}">${escapeHTML(entry.label)}</option>`).join("")}
        </select>
      </div>
      <button type="button" data-load-example="${phase}" data-context="${context}">Load</button>
    </div>
  `;
}

function renderSchemaForm(schema, data, phase) {
  if (!schema || typeof schema !== "object") return `<p class="muted">No schema available. Use raw JSON.</p>`;
  if (!schema.properties || hasUnsupportedFormFeatures(schema)) {
    return `<p class="notice">This schema uses features outside the basic form renderer. Raw JSON editing is available.</p>`;
  }
  return `<div class="schema-form">${renderSchemaFields(schema, data || {}, [], phase)}</div>`;
}

function renderSchemaFields(schema, data, path, phase) {
  const required = new Set(schema.required || []);
  return Object.entries(schema.properties || {}).map(([key, child]) => {
    const childPath = [...path, key];
    if (child.type === "object" || child.properties) {
      return `
        <fieldset>
          <legend>${escapeHTML(key)} ${required.has(key) ? "" : "<span>optional</span>"}</legend>
          ${child.description ? `<p>${escapeHTML(child.description)}</p>` : ""}
          ${renderSchemaFields(child, data?.[key] || {}, childPath, phase)}
        </fieldset>
      `;
    }
    return renderPrimitiveField(key, child, data?.[key], childPath, phase, required.has(key));
  }).join("");
}

function renderPrimitiveField(key, schema, value, path, phase, required) {
  const pathData = escapeHTML(JSON.stringify(path));
  const description = schema.description ? `<small>${escapeHTML(schema.description)}</small>` : "";
  const optional = required ? "" : `<span class="optional">optional</span>`;
  if (Array.isArray(schema.enum)) {
    return `
      <div class="form-group">
        <label>${escapeHTML(key)} ${optional}</label>
        <select data-form-phase="${phase}" data-path="${pathData}" data-schema-type="${escapeHTML(schema.type || "string")}">
          ${schema.enum.map((item) => `<option value="${escapeHTML(String(item))}" ${item === value ? "selected" : ""}>${escapeHTML(String(item))}</option>`).join("")}
        </select>
        ${description}
      </div>
    `;
  }
  const type = schema.type || "string";
  const inputType = type === "integer" || type === "number" ? "number" : type === "boolean" ? "checkbox" : schema.format === "email" ? "email" : "text";
  const checked = type === "boolean" && value ? "checked" : "";
  const inputValue = type === "boolean" ? "" : `value="${escapeHTML(value ?? "")}"`;
  return `
    <div class="form-group">
      <label>${escapeHTML(key)} ${optional}</label>
      <input type="${inputType}" data-form-phase="${phase}" data-path="${pathData}" data-schema-type="${escapeHTML(type)}" ${inputValue} ${checked} />
      ${description}
    </div>
  `;
}

function renderPreviewResult(result) {
  if (!result) return `<pre class="empty"></pre>`;
  if (result.error) {
    return `
      <div class="banner error">${escapeHTML(result.error)}</div>
      ${result.phase || result.stage ? `<p class="muted">Phase: ${escapeHTML(result.phase || "")} · Stage: ${escapeHTML(result.stage || "")}</p>` : ""}
      ${renderErrorDetails(result)}
      <pre>${escapeHTML(JSON.stringify(result, null, 2))}</pre>
    `;
  }
  return `
    <pre>${escapeHTML(JSON.stringify(result.payload ?? result, null, 2))}</pre>
    ${result.skipped_handlers?.length ? `<div class="notice">Skipped unsafe handlers: ${escapeHTML(result.skipped_handlers.join(", "))}</div>` : ""}
  `;
}

function renderTryItOut(endpoint, contract) {
  const pathParams = extractPathParams(endpoint.route.path);
  const url = resolvedTryURL(endpoint.route.path);
  const curl = buildCurl(endpoint, contract, url);
  return `
    <div class="preview-layout">
      <section class="surface">
        <div class="section-title">
          <div>
            <h3>Try It Out</h3>
            <p>Real same-origin request. This calls the application handler.</p>
          </div>
          <span class="real-call">Real request</span>
        </div>
        <dl class="facts compact-facts">
          <dt>Method</dt><dd>${escapeHTML(endpoint.route.method)}</dd>
          <dt>URL</dt><dd><code>${escapeHTML(url)}</code></dd>
          <dt>Contract header</dt><dd><code>${escapeHTML(state.spec.header)}: ${escapeHTML(contract.id)}</code></dd>
        </dl>
        ${pathParams.map((name) => `
          <div class="field">
            <label>Path parameter: ${escapeHTML(name)}</label>
            <input data-path-param="${escapeHTML(name)}" value="${escapeHTML(state.pathParams[name] || "123")}" />
          </div>
        `).join("")}
        <h4>Headers</h4>
        <div class="header-editor">
          ${state.headers.map((row, index) => `
            <input data-header-key="${index}" value="${escapeHTML(row.key)}" placeholder="Header" />
            <input data-header-value="${index}" value="${escapeHTML(row.value)}" placeholder="Value" />
            <button type="button" data-remove-header="${index}" ${row.locked ? "disabled" : ""}>Remove</button>
          `).join("")}
        </div>
        <button type="button" data-add-header>Add header</button>
        ${renderSchemaForm(inputSchemaFor(contract, "request"), state.formData.request, "request")}
        <div class="field">
          <label>Request body</label>
          <textarea data-raw="request" spellcheck="false">${escapeHTML(state.bodies.request)}</textarea>
        </div>
        <div class="actions">
          <button type="button" data-send-request ${state.tryBusy ? "disabled" : ""}>${state.tryBusy ? "Sending" : "Send Request"}</button>
          <button type="button" data-copy="curl">Copy curl</button>
          <button type="button" data-save-current="request">Save local example</button>
        </div>
        <pre class="curl">${escapeHTML(curl)}</pre>
      </section>
      <aside class="surface output">
        <div class="section-title">
          <h3>Response</h3>
          <button type="button" data-copy="try-response" ${state.tryResult ? "" : "disabled"}>Copy response</button>
        </div>
        ${renderTryResult()}
      </aside>
    </div>
  `;
}

function renderTryResult() {
  if (!state.tryResult) return `<pre class="empty"></pre>`;
  const body = state.tryResult.body;
  return `
    <dl class="facts compact-facts">
      <dt>Status</dt><dd>${state.tryResult.status}</dd>
      <dt>Duration</dt><dd>${state.tryResult.duration} ms</dd>
    </dl>
    ${body?.error ? renderErrorDetails(body) : ""}
    <h4>Headers</h4>
    <pre>${escapeHTML(JSON.stringify(state.tryResult.headers, null, 2))}</pre>
    <h4>Body</h4>
    <pre>${escapeHTML(typeof body === "string" ? body : JSON.stringify(body, null, 2))}</pre>
  `;
}

function renderCompare(endpoint, contract) {
  const contracts = endpoint.contracts || [];
  if (!state.compare.base) state.compare.base = contract.id;
  if (!state.compare.target) state.compare.target = contracts.find((item) => item.id !== state.compare.base)?.id || contract.id;
  const base = contracts.find((item) => item.id === state.compare.base) || contract;
  const target = contracts.find((item) => item.id === state.compare.target) || contract;
  const comparison = compareContracts(base, target, state.compare.direction);
  return `
    <section class="surface">
      <h3>Compare Versions</h3>
      <div class="compare-controls">
        ${selectHTML("Base", "compare-base", base.id, contracts.map((item) => [item.id, item.id]))}
        ${selectHTML("Compare", "compare-target", target.id, contracts.map((item) => [item.id, item.id]))}
        ${selectHTML("Direction", "compare-direction", state.compare.direction, [["request", "request"], ["response", "response"]])}
      </div>
      <div class="diff-grid">
        ${renderDiffSection("Shape fields", comparison.fields)}
        ${renderDiffSection("Mapping changes", comparison.mappings)}
        ${renderDiffSection("Validation and coercion", comparison.rules)}
      </div>
    </section>
  `;
}

function renderDiffSection(title, rows) {
  return `
    <section class="diff-section">
      <h4>${escapeHTML(title)}</h4>
      ${rows.length ? `<ul>${rows.map((row) => `<li><strong>${escapeHTML(row.kind)}</strong> ${escapeHTML(row.text)}</li>`).join("")}</ul>` : `<p class="muted">No differences.</p>`}
    </section>
  `;
}

function renderExamples(endpoint, contract) {
  const local = scopedLocalExamples(endpoint, contract);
  const curated = curatedExamples(contract);
  return `
    <div class="content-grid">
      <section class="surface">
        <h3>Curated Examples</h3>
        ${curated.length ? curated.map((item) => renderExampleCard(item, true)).join("") : `<p class="muted">No curated examples in the spec.</p>`}
      </section>
      <section class="surface">
        <div class="section-title">
          <h3>Local Examples</h3>
          <button type="button" data-save-current="request">Save request input</button>
        </div>
        ${local.length ? local.map((item) => renderExampleCard(item, false)).join("") : `<p class="muted">No browser-saved examples for this contract.</p>`}
        <h4>Import / Export</h4>
        <div class="actions">
          <button type="button" data-export-examples>Export all local examples</button>
          <button type="button" data-import-examples>Import JSON</button>
        </div>
        <textarea id="examples-io" spellcheck="false" placeholder="Import or export JSON appears here."></textarea>
      </section>
    </div>
  `;
}

function renderExampleCard(item, curated) {
  return `
    <article class="example-card">
      <div>
        <strong>${escapeHTML(item.name)}</strong>
        <span>${escapeHTML(item.phase)}</span>
      </div>
      ${item.description ? `<p>${escapeHTML(item.description)}</p>` : ""}
      <pre>${escapeHTML(JSON.stringify(item.body, null, 2))}</pre>
      <div class="actions">
        <button type="button" data-load-stored="${escapeHTML(item.id)}">Load</button>
        <button type="button" data-copy-example="${escapeHTML(item.id)}">Copy JSON</button>
        ${curated ? "" : `<button type="button" data-rename-example="${escapeHTML(item.id)}">Rename</button><button type="button" data-delete-example="${escapeHTML(item.id)}">Delete</button>`}
      </div>
    </article>
  `;
}

function bindEvents() {
  root.querySelector("[data-action='search']")?.addEventListener("input", (event) => {
    state.search = event.target.value;
    render();
  });
  root.querySelector("[data-action='tag']")?.addEventListener("change", (event) => {
    state.tag = event.target.value;
    render();
  });
  for (const button of root.querySelectorAll("[data-endpoint]")) {
    button.addEventListener("click", () => {
      state.endpointKey = button.dataset.endpoint;
      state.contractID = currentEndpoint()?.contracts?.[0]?.id || "";
      state.tab = "overview";
      initAfterSelection();
    });
  }
  root.querySelector("[data-action='contract']")?.addEventListener("change", (event) => {
    state.contractID = event.target.value;
    initAfterSelection();
  });
  for (const button of root.querySelectorAll("[data-tab]")) {
    button.addEventListener("click", () => {
      state.tab = button.dataset.tab;
      syncToQuery();
      render();
    });
  }
  bindFormInputs();
  bindRawEditors();
  bindPreviewActions();
  bindTryActions();
  bindCompareActions();
  bindExampleActions();
}

function bindFormInputs() {
  for (const input of root.querySelectorAll("[data-form-phase]")) {
    input.addEventListener("input", () => updateFormInput(input));
    input.addEventListener("change", () => updateFormInput(input));
  }
}

function updateFormInput(input) {
  const phase = input.dataset.formPhase;
  const path = JSON.parse(input.dataset.path);
  setByPath(state.formData[phase], path, parseInputValue(input));
  state.bodies[phase] = JSON.stringify(state.formData[phase], null, 2);
  for (const raw of root.querySelectorAll(`[data-raw="${phase}"]`)) raw.value = state.bodies[phase];
}

function bindRawEditors() {
  for (const raw of root.querySelectorAll("[data-raw]")) {
    raw.addEventListener("input", () => {
      const phase = raw.dataset.raw;
      state.bodies[phase] = raw.value;
      try {
        state.formData[phase] = JSON.parse(raw.value);
        state.error = "";
      } catch {
        state.error = "Body is not valid JSON";
      }
    });
  }
}

function bindPreviewActions() {
  for (const button of root.querySelectorAll("[data-preview]")) button.addEventListener("click", () => runPreview(button.dataset.preview));
  for (const button of root.querySelectorAll("[data-seed]")) button.addEventListener("click", () => {
    seedPhase(button.dataset.seed);
    render();
  });
  for (const button of root.querySelectorAll("[data-copy]")) button.addEventListener("click", () => copyByKind(button.dataset.copy));
  for (const button of root.querySelectorAll("[data-load-example]")) button.addEventListener("click", () => {
    const phase = button.dataset.loadExample;
    const picker = root.querySelector(`[data-example-picker="${phase}"]`);
    loadExampleByID(picker?.value);
  });
}

async function runPreview(phase) {
  state.busy = true;
  state.preview[phase] = null;
  state.error = "";
  render();
  let body;
  try {
    body = JSON.parse(state.bodies[phase]);
  } catch {
    state.error = "Body is not valid JSON";
    state.busy = false;
    render();
    return;
  }
  try {
    const endpoint = currentEndpoint();
    const contract = currentContract();
    const res = await fetch(`${trimRight(state.config.preview_api_base, "/")}/process/${phase}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ route: endpoint.route, contract: contract.id, body }),
    });
    state.preview[phase] = await res.json();
    if (!res.ok) state.error = state.preview[phase].error || `preview ${res.status}`;
  } catch (err) {
    state.error = err.message;
  }
  state.busy = false;
  render();
}

function bindTryActions() {
  for (const input of root.querySelectorAll("[data-path-param]")) {
    input.addEventListener("input", () => {
      state.pathParams[input.dataset.pathParam] = input.value;
      render();
    });
  }
  for (const input of root.querySelectorAll("[data-header-key]")) {
    input.addEventListener("input", () => { state.headers[Number(input.dataset.headerKey)].key = input.value; });
  }
  for (const input of root.querySelectorAll("[data-header-value]")) {
    input.addEventListener("input", () => { state.headers[Number(input.dataset.headerValue)].value = input.value; });
  }
  root.querySelector("[data-add-header]")?.addEventListener("click", () => {
    state.headers.push({ key: "", value: "", locked: false });
    render();
  });
  for (const button of root.querySelectorAll("[data-remove-header]")) {
    button.addEventListener("click", () => {
      state.headers.splice(Number(button.dataset.removeHeader), 1);
      render();
    });
  }
  root.querySelector("[data-send-request]")?.addEventListener("click", sendRealRequest);
}

async function sendRealRequest() {
  const endpoint = currentEndpoint();
  if (["POST", "PUT", "PATCH", "DELETE"].includes(endpoint.route.method) && !state.confirmedMutation) {
    if (!window.confirm("This sends a real request to the application and may mutate data. Continue?")) return;
    state.confirmedMutation = true;
  }
  let parsedBody = null;
  if (endpoint.route.method !== "GET" && endpoint.route.method !== "HEAD") {
    try {
      parsedBody = JSON.parse(state.bodies.request);
    } catch {
      state.error = "Body is not valid JSON";
      render();
      return;
    }
  }
  state.tryBusy = true;
  state.tryResult = null;
  render();
  const started = performance.now();
  try {
    const headers = Object.fromEntries(state.headers.filter((row) => row.key).map((row) => [row.key, row.value]));
    if (parsedBody !== null) headers["Content-Type"] = headers["Content-Type"] || "application/json";
    const res = await fetch(resolvedTryURL(endpoint.route.path), {
      method: endpoint.route.method,
      headers,
      credentials: "same-origin",
      body: parsedBody === null ? undefined : JSON.stringify(parsedBody),
    });
    const text = await res.text();
    let body = text;
    try {
      body = text ? JSON.parse(text) : null;
    } catch {}
    state.tryResult = {
      status: res.status,
      duration: Math.round(performance.now() - started),
      headers: Object.fromEntries(res.headers.entries()),
      body,
    };
  } catch (err) {
    state.tryResult = { status: "network error", duration: Math.round(performance.now() - started), headers: {}, body: err.message };
  }
  state.tryBusy = false;
  render();
}

function bindCompareActions() {
  root.querySelector("[data-select='compare-base']")?.addEventListener("change", (event) => {
    state.compare.base = event.target.value;
    render();
  });
  root.querySelector("[data-select='compare-target']")?.addEventListener("change", (event) => {
    state.compare.target = event.target.value;
    render();
  });
  root.querySelector("[data-select='compare-direction']")?.addEventListener("change", (event) => {
    state.compare.direction = event.target.value;
    render();
  });
}

function bindExampleActions() {
  for (const button of root.querySelectorAll("[data-save-current]")) {
    button.addEventListener("click", () => saveCurrentExample(button.dataset.saveCurrent));
  }
  for (const button of root.querySelectorAll("[data-load-stored]")) button.addEventListener("click", () => loadExampleByID(button.dataset.loadStored));
  for (const button of root.querySelectorAll("[data-copy-example]")) button.addEventListener("click", () => {
    const item = findExample(button.dataset.copyExample);
    if (item) copyText(JSON.stringify(item.body, null, 2));
  });
  for (const button of root.querySelectorAll("[data-delete-example]")) button.addEventListener("click", () => {
    state.localExamples.examples = state.localExamples.examples.filter((item) => item.id !== button.dataset.deleteExample);
    saveLocalExamples();
    render();
  });
  for (const button of root.querySelectorAll("[data-rename-example]")) button.addEventListener("click", () => {
    const item = findLocalExample(button.dataset.renameExample);
    const name = item && window.prompt("Example name", item.name);
    if (name) {
      item.name = name;
      item.updated_at = new Date().toISOString();
      saveLocalExamples();
      render();
    }
  });
  root.querySelector("[data-export-examples]")?.addEventListener("click", () => {
    root.querySelector("#examples-io").value = JSON.stringify(state.localExamples, null, 2);
  });
  root.querySelector("[data-import-examples]")?.addEventListener("click", () => {
    const text = root.querySelector("#examples-io").value;
    try {
      const imported = JSON.parse(text);
      if (!Array.isArray(imported.examples)) throw new Error("missing examples array");
      state.localExamples = { version: 1, examples: imported.examples };
      saveLocalExamples();
      render();
    } catch (err) {
      state.error = `Import failed: ${err.message}`;
      render();
    }
  });
}

function saveCurrentExample(phase) {
  let body;
  try {
    body = JSON.parse(state.bodies[phase]);
  } catch {
    state.error = "Body is not valid JSON";
    render();
    return;
  }
  const name = window.prompt("Example name", `${currentContract().id} ${phase} example`);
  if (!name) return;
  const now = new Date().toISOString();
  state.localExamples.examples.push({
    id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
    route: currentEndpoint().route,
    contract: currentContract().id,
    phase,
    name,
    body,
    created_at: now,
    updated_at: now,
  });
  saveLocalExamples();
  render();
}

function loadExampleByID(id) {
  if (id === "generated-request") return seedAndRender("request");
  if (id === "generated-response") return seedAndRender("response");
  const item = findExample(id);
  if (!item) return;
  state.formData[item.phase] = clone(item.body);
  state.bodies[item.phase] = JSON.stringify(item.body, null, 2);
  state.tab = item.phase === "request" ? "request" : "response";
  render();
}

function seedAndRender(phase) {
  seedPhase(phase);
  render();
}

function copyByKind(kind) {
  if (kind === "request-input") return copyText(state.bodies.request);
  if (kind === "response-input") return copyText(state.bodies.response);
  if (kind === "request-output") return copyText(JSON.stringify(state.preview.request?.payload ?? state.preview.request ?? {}, null, 2));
  if (kind === "response-output") return copyText(JSON.stringify(state.preview.response?.payload ?? state.preview.response ?? {}, null, 2));
  if (kind === "try-response") return copyText(typeof state.tryResult?.body === "string" ? state.tryResult.body : JSON.stringify(state.tryResult?.body ?? {}, null, 2));
  if (kind === "curl") return copyText(buildCurl(currentEndpoint(), currentContract(), resolvedTryURL(currentEndpoint().route.path)));
}

async function copyText(text) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.style.position = "fixed";
  textarea.style.left = "-1000px";
  document.body.append(textarea);
  textarea.select();
  document.execCommand("copy");
  textarea.remove();
}

function applyInitialSelection() {
  const endpoint = state.spec?.endpoints?.[0];
  state.endpointKey = endpoint ? endpointKey(endpoint) : "";
  state.contractID = endpoint?.default_contract || endpoint?.contracts?.[0]?.id || "";
}

function initAfterSelection() {
  seedPhase("request");
  seedPhase("response");
  initTryHeaders();
  state.preview = { request: null, response: null };
  state.tryResult = null;
  state.compare = { base: currentContract()?.id || "", target: "", direction: "request" };
  syncToQuery();
  render();
}

function initTryHeaders() {
  const header = state.spec?.header || "X-ShapeShifter-Contract";
  state.headers = [
    { key: header, value: currentContract()?.id || "", locked: true },
    { key: "Content-Type", value: "application/json", locked: true },
  ];
}

function seedPhase(phase) {
  const schema = inputSchemaFor(currentContract(), phase);
  const body = seedForSchema(schema, "");
  state.formData[phase] = body;
  state.bodies[phase] = JSON.stringify(body, null, 2);
}

function inputSchemaFor(contract, phase) {
  const side = contract?.[phase];
  if (!side) return null;
  const name = phase === "request" ? side.shape : side.source_shape || side.shape;
  return state.spec?.shape_schemas?.[name] || null;
}

function seedForSchema(schema, key) {
  if (!schema || typeof schema !== "object") return {};
  if (schema.default !== undefined) return clone(schema.default);
  if (Array.isArray(schema.enum) && schema.enum.length) return schema.enum[0];
  const type = Array.isArray(schema.type) ? schema.type[0] : schema.type;
  if (type === "object" || schema.properties) {
    const out = {};
    for (const [childKey, child] of Object.entries(schema.properties || {})) out[childKey] = seedForSchema(child, childKey);
    return out;
  }
  if (type === "array") return [];
  if (type === "integer") return 1;
  if (type === "number") return 1.5;
  if (type === "boolean") return false;
  if (schema.format === "email" || key.toLowerCase().includes("email")) return "alice@example.com";
  if (key === "id" || key.endsWith("_id")) return "123";
  if (key.includes("name")) return "Alice";
  if (key.includes("phone")) return "+15551234567";
  return "string";
}

function exampleEntries(endpoint, contract, phase) {
  const entries = [{ id: `generated-${phase}`, label: "Generated from schema" }];
  const side = contract[phase];
  if (phase === "request") {
    for (const example of side?.examples || []) entries.push({ id: `curated:${phase}:${example.name}`, label: `Spec: ${example.name}` });
  }
  for (const example of scopedLocalExamples(endpoint, contract).filter((item) => item.phase === phase)) {
    entries.push({ id: example.id, label: `Local: ${example.name}` });
  }
  return entries;
}

function curatedExamples(contract) {
  const out = [];
  for (const phase of ["request", "response"]) {
    for (const example of contract[phase]?.examples || []) {
      out.push({ ...example, id: `curated:${phase}:${example.name}`, phase });
    }
  }
  return out;
}

function scopedLocalExamples(endpoint, contract) {
  return state.localExamples.examples.filter((item) =>
    item.route?.method === endpoint.route.method &&
    item.route?.path === endpoint.route.path &&
    item.contract === contract.id
  );
}

function loadLocalExamples() {
  try {
    const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) || "");
    if (Array.isArray(parsed.examples)) return parsed;
  } catch {}
  return { version: 1, examples: [] };
}

function saveLocalExamples() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state.localExamples));
}

function findExample(id) {
  if (!id) return null;
  if (id.startsWith("curated:")) {
    const [, phase, name] = id.split(":");
    return curatedExamples(currentContract()).find((item) => item.phase === phase && item.name === name) || null;
  }
  return findLocalExample(id);
}

function findLocalExample(id) {
  return state.localExamples.examples.find((item) => item.id === id) || null;
}

function compareContracts(base, target, direction) {
  const baseSide = base?.[direction];
  const targetSide = target?.[direction];
  const baseSchema = state.spec?.shape_schemas?.[baseSide?.shape] || {};
  const targetSchema = state.spec?.shape_schemas?.[targetSide?.shape] || {};
  return {
    fields: compareFields(flattenSchema(baseSchema), flattenSchema(targetSchema)),
    mappings: compareMappings(baseSide?.transform?.fields || [], targetSide?.transform?.fields || []),
    rules: [
      ...compareRules("validation", baseSide?.transform?.validate || [], targetSide?.transform?.validate || [], (item) => `${item.field}|${item.rule}|${item.required}`),
      ...compareRules("coercion", baseSide?.transform?.coerce || [], targetSide?.transform?.coerce || [], (item) => `${item.field}|${item.type}`),
    ],
  };
}

function flattenSchema(schema, prefix = ".") {
  const required = new Set(schema.required || []);
  const fields = new Map();
  for (const [key, child] of Object.entries(schema.properties || {})) {
    const path = prefix === "." ? `.${key}` : `${prefix}.${key}`;
    fields.set(path, { path, type: child.type || "object", required: required.has(key), enum: child.enum || [] });
    if (child.properties) {
      for (const [childPath, childInfo] of flattenSchema(child, path)) fields.set(childPath, childInfo);
    }
  }
  return fields;
}

function compareFields(baseFields, targetFields) {
  const rows = [];
  for (const [path, field] of targetFields) {
    const before = baseFields.get(path);
    if (!before) rows.push({ kind: "Added", text: `${path} (${field.type})` });
    else {
      if (before.type !== field.type) rows.push({ kind: "Type changed", text: `${path}: ${before.type} -> ${field.type}` });
      if (before.required !== field.required) rows.push({ kind: "Required changed", text: `${path}: ${before.required ? "required" : "optional"} -> ${field.required ? "required" : "optional"}` });
    }
  }
  for (const [path] of baseFields) if (!targetFields.has(path)) rows.push({ kind: "Removed", text: path });
  return rows;
}

function compareMappings(baseFields, targetFields) {
  const rows = [];
  const key = (field) => field.to || field.from;
  const base = new Map(baseFields.map((field) => [key(field), field]));
  const target = new Map(targetFields.map((field) => [key(field), field]));
  for (const [path, field] of target) {
    const before = base.get(path);
    if (!before) rows.push({ kind: "Added", text: `${field.from} -> ${field.to}` });
    else {
      if (before.from !== field.from || before.to !== field.to) rows.push({ kind: "Changed", text: `${before.from} -> ${before.to} became ${field.from} -> ${field.to}` });
      if (before.required !== field.required) rows.push({ kind: "Required changed", text: `${field.to}` });
    }
  }
  for (const [path, field] of base) if (!target.has(path)) rows.push({ kind: "Removed", text: `${field.from} -> ${field.to}` });
  return rows;
}

function compareRules(label, baseItems, targetItems, key) {
  const rows = [];
  const base = new Set(baseItems.map(key));
  const target = new Set(targetItems.map(key));
  for (const item of target) if (!base.has(item)) rows.push({ kind: "Added", text: `${label}: ${item}` });
  for (const item of base) if (!target.has(item)) rows.push({ kind: "Removed", text: `${label}: ${item}` });
  return rows;
}

function resolvedTryURL(path) {
  let resolved = path;
  for (const name of extractPathParams(path)) {
    const value = encodeURIComponent(state.pathParams[name] || "123");
    resolved = resolved.replace(`:${name}`, value).replace(`{${name}}`, value);
  }
  const base = state.config.try_it_out_base || "/";
  return `${trimRight(base, "/")}/${resolved.replace(/^\/+/, "")}`;
}

function buildCurl(endpoint, contract, url) {
  const origin = window.location.origin || "";
  const headers = state.headers.filter((row) => row.key).map((row) => `  -H '${shellQuote(row.key)}: ${shellQuote(row.value)}'`).join(" \\\n");
  const body = endpoint.route.method === "GET" || endpoint.route.method === "HEAD" ? "" : ` \\\n  -d '${shellQuote(compactJSON(state.bodies.request))}'`;
  return `curl -X ${endpoint.route.method} ${origin}${url.startsWith("/") ? url : `/${url}`} \\\n${headers}${body}`;
}

function compactJSON(text) {
  try {
    return JSON.stringify(JSON.parse(text));
  } catch {
    return text;
  }
}

function extractPathParams(path) {
  const params = new Set();
  for (const match of path.matchAll(/:([A-Za-z_][A-Za-z0-9_]*)/g)) params.add(match[1]);
  for (const match of path.matchAll(/\{([A-Za-z_][A-Za-z0-9_]*)\}/g)) params.add(match[1]);
  return [...params];
}

function renderErrorDetails(body) {
  if (!body?.details?.length) return "";
  return `
    <div class="table-wrap">
      <table>
        <thead><tr><th>Field</th><th>Code</th><th>Message</th></tr></thead>
        <tbody>${body.details.map((item) => `<tr><td>${escapeHTML(item.field || "")}</td><td>${escapeHTML(item.code || "")}</td><td>${escapeHTML(item.message || "")}</td></tr>`).join("")}</tbody>
      </table>
    </div>
  `;
}

function selectHTML(label, name, value, options) {
  return `
    <div class="field compact">
      <label>${escapeHTML(label)}</label>
      <select data-select="${escapeHTML(name)}">
        ${options.map(([optionValue, text]) => `<option value="${escapeHTML(optionValue)}" ${optionValue === value ? "selected" : ""}>${escapeHTML(text)}</option>`).join("")}
      </select>
    </div>
  `;
}

function filteredEndpoints() {
  const q = state.search.trim().toLowerCase();
  return (state.spec?.endpoints || []).filter((endpoint) => {
    const tags = endpoint.tags || [];
    if (state.tag && !tags.includes(state.tag)) return false;
    if (!q) return true;
    return [endpoint.route.path, endpoint.summary, endpoint.description, ...tags].join(" ").toLowerCase().includes(q);
  });
}

function groupEndpoints(endpoints) {
  const groups = new Map();
  for (const endpoint of endpoints) {
    const tags = endpoint.tags?.length ? endpoint.tags : ["untagged"];
    for (const tag of tags) {
      if (!groups.has(tag)) groups.set(tag, []);
      groups.get(tag).push(endpoint);
    }
  }
  return [...groups.entries()].sort(([a], [b]) => a.localeCompare(b));
}

function allTags() {
  return [...new Set((state.spec?.endpoints || []).flatMap((endpoint) => endpoint.tags || []))].sort();
}

function currentEndpoint() {
  return state.spec?.endpoints?.find((endpoint) => endpointKey(endpoint) === state.endpointKey) || null;
}

function currentContract() {
  const endpoint = currentEndpoint();
  return endpoint?.contracts?.find((contract) => contract.id === state.contractID) || endpoint?.contracts?.[0] || null;
}

function endpointKey(endpoint) {
  return `${endpoint.route.method} ${endpoint.route.path}`;
}

function tabID(label) {
  if (label === "Overview") return "overview";
  if (label === "Request Preview") return "request";
  if (label === "Response Preview") return "response";
  if (label === "Try It Out") return "try";
  if (label === "Compare Versions") return "compare";
  if (label === "Examples") return "examples";
  return label.toLowerCase().replaceAll(" ", "-").replace("-preview", "");
}

function syncFromQuery() {
  const query = new URLSearchParams(window.location.search);
  if (query.get("_endpoint")) state.endpointKey = query.get("_endpoint");
  if (query.get("_contract")) state.contractID = query.get("_contract");
  if (query.get("_tab")) state.tab = query.get("_tab");
}

function syncToQuery() {
  const query = new URLSearchParams();
  query.set("_endpoint", state.endpointKey);
  query.set("_contract", state.contractID);
  query.set("_tab", state.tab);
  window.history.replaceState(null, "", `${window.location.pathname}?${query}`);
}

function hasUnsupportedFormFeatures(schema) {
  if (!schema || typeof schema !== "object") return false;
  if (schema.oneOf || schema.anyOf || schema.allOf || schema.if || schema.$ref) return true;
  return Object.values(schema.properties || {}).some(hasUnsupportedFormFeatures);
}

function parseInputValue(input) {
  const type = input.dataset.schemaType;
  if (type === "boolean") return input.checked;
  if (type === "integer") return Number.parseInt(input.value || "0", 10);
  if (type === "number") return Number.parseFloat(input.value || "0");
  return input.value;
}

function setByPath(target, path, value) {
  let current = target;
  for (const segment of path.slice(0, -1)) {
    if (!current[segment] || typeof current[segment] !== "object") current[segment] = {};
    current = current[segment];
  }
  current[path[path.length - 1]] = value;
}

function limitValue(limits, phase) {
  return phase === "request" ? limits?.RequestBodyBytes ?? limits?.request_body_bytes : limits?.ResponseBodyBytes ?? limits?.response_body_bytes;
}

function renderTags(tags = []) {
  return tags.length ? tags.map((tag) => `<span class="tag">${escapeHTML(tag)}</span>`).join(" ") : "None";
}

function renderEmpty() {
  return `<section class="surface"><h2>No endpoints</h2><p class="muted">The sanitized spec did not include any endpoints.</p></section>`;
}

function methodClass(method) {
  return `method-${String(method).toLowerCase()}`;
}

function titleCase(value) {
  return value.slice(0, 1).toUpperCase() + value.slice(1);
}

function formatBytes(value) {
  if (!value) return "Not configured";
  if (value >= 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MiB`;
  if (value >= 1024) return `${(value / 1024).toFixed(1)} KiB`;
  return `${value} B`;
}

function inferAPIBase() {
  const marker = "/ui";
  const path = window.location.pathname;
  const index = path.indexOf(marker);
  if (index >= 0) return `${path.slice(0, index)}/api`;
  return "/_shapeshifter/api";
}

function trimRight(value, char) {
  while (value.length > 1 && value.endsWith(char)) value = value.slice(0, -1);
  return value;
}

function shellQuote(value) {
  return String(value).replaceAll("'", "'\\''");
}

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}
