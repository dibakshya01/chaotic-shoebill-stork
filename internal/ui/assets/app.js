"use strict";
// chaotic-shoebill-stork dashboard — dependency-free. Drives six screens off the API.

const FAMILIES = [
  ["mesh", "Mesh"], ["gateway", "Gateway"], ["identity", "Identity"], ["network", "Network"],
  ["data", "Data"], ["supplychain", "Supply"], ["dr", "DR"], ["detection", "Detect"], ["ai", "AI"],
];
const FAMILY_LABEL = Object.fromEntries(FAMILIES);
const GLYPH = { pass: "●", slow: "◐", untested: "○", fail: "▲", fail_open: "✖" };

async function api(path, opts) {
  const r = await fetch(path, opts);
  if (!r.ok && r.status >= 500) throw new Error(path + " -> " + r.status);
  return r.json();
}
function h(tag, attrs, ...kids) {
  const e = document.createElement(tag);
  for (const k in (attrs || {})) {
    if (k === "class") e.className = attrs[k];
    else if (k === "html") e.innerHTML = attrs[k];
    else if (k.startsWith("on")) e.addEventListener(k.slice(2), attrs[k]);
    else if (attrs[k] != null) e.setAttribute(k, attrs[k]);
  }
  for (const kid of kids.flat()) {
    if (kid == null) continue;
    e.appendChild(typeof kid === "string" ? document.createTextNode(kid) : kid);
  }
  return e;
}
const main = () => document.getElementById("main");
function clear(n) { while (n.firstChild) n.removeChild(n.firstChild); }
function scoreClass(v) { return v >= 0.8 ? "good" : v >= 0.5 ? "mid" : "bad"; }
function pct(v) { return Math.round((v || 0) * 100) + "%"; }
function fmtTime(s) { try { return new Date(s).toLocaleString(); } catch { return s; } }

// ---------- router ----------
const views = {};
function setView(name) {
  for (const b of document.querySelectorAll("nav.tabs button"))
    b.classList.toggle("active", b.dataset.view === name);
  const m = main(); clear(m);
  m.appendChild(h("div", { class: "spin" }, "Loading…"));
  (views[name] || views.scorecard)().then((node) => { clear(m); m.appendChild(node); })
    .catch((err) => { clear(m); m.appendChild(h("div", { class: "empty" }, "Error: " + err.message)); });
  if (location.hash !== "#" + name) history.replaceState(null, "", "#" + name);
}

// ---------- scorecard ----------
views.scorecard = async () => {
  const sc = await api("/api/v1/scores");
  const s = sc.summary || {};
  const wrap = h("div", { class: "view" });
  wrap.appendChild(h("div", { class: "view-head" },
    h("h2", {}, "Security Control Posture"),
    h("p", {}, "Every control proven — or visibly not. Untested is as loud as failing.")));

  const tiles = h("div", { class: "tiles" });
  tiles.appendChild(tile("Efficacy", pct(s.efficacy), "mean service score", s.efficacy));
  tiles.appendChild(tile("Fail-open", String(s.fail_open_open || 0) + " open",
    "permissive control failures", null, (s.fail_open_open || 0) > 0));
  tiles.appendChild(tile("Coverage", (s.coverage_tested || 0) + "/" + (s.coverage_total || 0),
    "controls tested", (s.coverage_total ? s.coverage_tested / s.coverage_total : 0)));
  tiles.appendChild(tile("Freshness", pct(s.freshness), "within decay window", s.freshness));
  wrap.appendChild(tiles);

  const services = sc.services || [];
  const present = new Set();
  services.forEach((sv) => Object.keys(sv.cells || {}).forEach((f) => present.add(f)));
  const cols = FAMILIES.filter(([k]) => present.has(k));

  const card = h("div", { class: "card" });
  const table = h("table", { class: "matrix" });
  const thead = h("tr", {}, h("th", { class: "svc" }, "Service"));
  cols.forEach(([, lbl]) => thead.appendChild(h("th", {}, lbl)));
  thead.appendChild(h("th", {}, "Score"));
  table.appendChild(h("thead", {}, thead));
  const tbody = h("tbody");
  services.forEach((sv) => {
    const tr = h("tr", {});
    tr.appendChild(h("td", { class: "svc" },
      sv.service.name, h("span", { class: "tier" }, "T" + sv.service.tier)));
    cols.forEach(([k]) => {
      const cell = (sv.cells || {})[k];
      const td = h("td", {});
      if (cell) {
        const g = h("span", { class: "cell " + cell.state, title: cellTitle(cell) }, GLYPH[cell.state] || "·");
        if (cell.state === "fail_open") g.addEventListener("click", () => setView("failopen"));
        td.appendChild(g);
      } else td.appendChild(document.createTextNode(""));
      tr.appendChild(td);
    });
    tr.appendChild(h("td", {}, h("span", { class: "score " + scoreClass(sv.score) }, sv.score.toFixed(2))));
    tbody.appendChild(tr);
  });
  table.appendChild(tbody);
  card.appendChild(table);
  card.appendChild(h("div", { class: "legend" },
    ...[["pass", "pass"], ["slow", "slow"], ["untested", "untested"], ["fail", "fail"], ["fail_open", "fail-open"]]
      .map(([st, lbl]) => h("span", {}, h("b", { class: "cell " + st, style: "width:auto;height:auto;box-shadow:none" }, GLYPH[st] + " "), lbl))));
  wrap.appendChild(card);
  return wrap;
};
function cellTitle(c) {
  if (!c.tested) return "untested — scored 0.0";
  return c.verdict + " · raw " + c.raw.toFixed(2) + " · score " + c.score.toFixed(2);
}
function tile(label, value, foot, bar, alarm) {
  const t = h("div", { class: "tile" + (alarm ? " alarm" : "") },
    h("div", { class: "label" }, label),
    h("div", { class: "value" }, value),
    h("div", { class: "foot" }, foot));
  if (bar != null) {
    const b = h("div", { class: "bar" }); const i = h("i");
    b.appendChild(i); t.appendChild(b);
    requestAnimationFrame(() => { i.style.width = Math.round(bar * 100) + "%"; });
  }
  return t;
}

// ---------- fail-open board ----------
views.failopen = async () => {
  const list = await api("/api/v1/findings/failopen");
  const wrap = h("div", { class: "view" });
  wrap.appendChild(h("div", { class: "view-head" },
    h("h2", {}, "✖ Fail-Open Board"),
    h("p", {}, "A control degraded permissively instead of denying — the highest-severity class.")));
  if (!list.length) { wrap.appendChild(h("div", { class: "empty" }, "No open fail-open findings. 🎉")); return wrap; }
  list.forEach((f) => {
    const card = h("div", { class: "finding " + (f.severity === "critical" ? "critical" : "") });
    card.appendChild(h("div", { class: "top" },
      h("span", { class: "sev " + f.severity }, f.severity.toUpperCase()),
      h("span", { class: "muted" }, f.service_name + " · " + FAMILY_LABEL[f.family] + " · T" + f.tier),
      h("span", { class: "faint" }, "· " + fmtTime(f.opened_at))));
    card.appendChild(h("h3", {}, f.title));
    card.appendChild(h("div", { class: "detail" }, f.detail || ""));
    const chips = h("div", { class: "chips" });
    (f.attack_ids || []).concat(f.owasp_ids || [], f.control_families || [])
      .forEach((id) => chips.appendChild(h("span", { class: "chip" }, id)));
    card.appendChild(chips);
    wrap.appendChild(card);
  });
  return wrap;
};

// ---------- run console ----------
views.runs = async () => {
  const runs = await api("/api/v1/runs");
  const wrap = h("div", { class: "view" });
  wrap.appendChild(h("div", { class: "view-head" },
    h("h2", {}, "Run Console"),
    h("p", {}, "Every run, its verdict, and the control-response ladder it produced.")));
  if (!runs.length) { wrap.appendChild(h("div", { class: "empty" }, "No runs yet — run `shoebill demo`.")); return wrap; }
  const detail = h("div", { id: "run-detail" });
  const card = h("div", { class: "card" });
  const t = h("table", { class: "list" });
  t.appendChild(h("thead", {}, h("tr", {},
    h("th", {}, "Experiment"), h("th", {}, "Service"), h("th", {}, "Env"),
    h("th", {}, "Verdict"), h("th", {}, "When"))));
  const tb = h("tbody");
  runs.forEach((r) => {
    const tr = h("tr", { class: "click", onclick: () => renderRun(detail, r) },
      h("td", {}, r.title || r.experiment),
      h("td", {}, (r.service && r.service.name) || "—"),
      h("td", {}, r.env + (r.ring ? "/" + r.ring : "")),
      h("td", {}, r.refused ? h("span", { class: "pill INCONCLUSIVE" }, "REFUSED")
        : h("span", { class: "pill " + r.verdict }, r.verdict)),
      h("td", { class: "faint" }, fmtTime(r.at)));
    tb.appendChild(tr);
  });
  t.appendChild(tb); card.appendChild(t);
  wrap.appendChild(card);
  wrap.appendChild(detail);
  renderRun(detail, runs[0]);
  return wrap;
};
function renderRun(host, r) {
  clear(host);
  const card = h("div", { class: "card", style: "margin-top:16px" });
  card.appendChild(h("h3", { style: "margin:0 0 4px" }, (r.title || r.experiment)));
  card.appendChild(h("div", { class: "muted", style: "font-size:12.5px;margin-bottom:6px" },
    (r.service ? r.service.name : "") + " · " + r.env + " · " +
    (r.refused ? "REFUSED" : r.verdict) + (r.receipt_id ? " · receipt " + r.receipt_id : "")));
  if (r.refused) {
    (r.refusals || []).forEach((x) => card.appendChild(h("div", { class: "rung FAIL" }, h("span", { class: "ic" }, "⛔"), h("span", { class: "d" }, x))));
  } else {
    const ladder = h("div", { class: "ladder" });
    (r.ladder || []).forEach((st) => {
      const ic = st.outcome === "PASS" ? "✓" : st.outcome === "SLOW" ? "◐" : st.outcome === "SKIP" ? "·" : "✗";
      ladder.appendChild(h("div", { class: "rung " + st.outcome },
        h("span", { class: "ic" }, ic),
        h("span", { class: "st" }, st.stage),
        h("span", { class: "d" }, st.detail || st.outcome + (st.fail_open ? " — FAIL-OPEN" : "")),
        h("span", { class: "lat" }, st.latency_ms ? (Math.round(st.latency_ms / 100) / 10) + "s" : "")));
    });
    card.appendChild(ladder);
  }
  host.appendChild(card);
}

// ---------- safety preview ----------
views.preview = async () => {
  const exps = await api("/api/v1/experiments");
  const wrap = h("div", { class: "view" });
  wrap.appendChild(h("div", { class: "view-head" },
    h("h2", {}, "Safety Preview"),
    h("p", {}, "Run the Safety Governor without detonating. All ten gates, evaluated at once.")));

  const sel = h("select", { id: "p-exp" }, ...exps.map((e) => h("option", { value: e.name }, e.name + " — " + (e.title || ""))));
  const env = h("select", { id: "p-env" }, ...["playground", "stage", "preprod", "prod-canary", "prod-ring"].map((v) => h("option", { value: v }, v)));
  env.value = "prod-canary";
  const tier = h("select", { id: "p-tier" }, ...[1, 2, 3].map((v) => h("option", { value: v }, "Tier " + v)));
  const pods = h("input", { type: "number", id: "p-pods", value: "1", min: "0" });
  const dur = h("input", { type: "number", id: "p-dur", value: "90", min: "0" });
  const traffic = h("input", { type: "number", id: "p-traffic", value: "1", min: "0" });
  const appr = h("input", { type: "number", id: "p-appr", value: "1", min: "0", max: "2" });
  const pay = h("input", { type: "checkbox", id: "p-pay" });
  const capProd = h("input", { type: "checkbox", id: "p-cap-prod", checked: "checked" });

  const form = h("div", { class: "card" },
    h("div", { class: "form-grid" },
      field("Experiment", sel), field("Environment", env), field("Service tier", tier),
      field("Pods", pods), field("Duration (s)", dur), field("Traffic %", traffic),
      field("Distinct-team approvals", appr)),
    h("div", { class: "checks", style: "margin-top:14px" },
      h("label", {}, pay, "on payments path"),
      h("label", {}, capProd, "granted: prod-injection")),
    h("div", { style: "margin-top:16px" }, h("button", { class: "primary", onclick: runPreview }, "Evaluate gates →")));
  wrap.appendChild(form);
  wrap.appendChild(h("div", { id: "p-out" }));
  return wrap;
};
function field(label, input) { return h("div", {}, h("label", { class: "f" }, label), input); }
async function runPreview() {
  const approvalsN = parseInt(document.getElementById("p-appr").value || "0", 10);
  const approvals = []; const teams = ["service-owner", "appsec"];
  for (let i = 0; i < approvalsN; i++) approvals.push(teams[i] || ("team-" + i));
  const caps = []; if (document.getElementById("p-cap-prod").checked) caps.push("prod-injection");
  const body = {
    experiment: document.getElementById("p-exp").value,
    serviceId: "preview-svc", serviceName: "preview-svc",
    tier: parseInt(document.getElementById("p-tier").value, 10),
    team: "service-owner", onPaymentsPath: document.getElementById("p-pay").checked,
    namespace: "preview", env: document.getElementById("p-env").value, ring: "canary",
    scope: {
      podCount: num("p-pods"), pctOfReplicas: 5, durationSeconds: num("p-dur"), pctTraffic: num("p-traffic"),
    },
    approvals, grantedCapabilities: caps,
  };
  const out = document.getElementById("p-out"); clear(out);
  out.appendChild(h("div", { class: "spin" }, "Evaluating…"));
  try {
    const dec = await api("/api/v1/runs:plan", {
      method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body),
    });
    clear(out);
    out.appendChild(h("div", { class: "banner " + (dec.allowed ? "allow" : "refuse") },
      dec.allowed ? "✓ ALLOWED — capability would be minted" : "⛔ REFUSED — " + dec.gates.filter((g) => !g.ok).length + " gate(s) failed"));
    const gates = h("div", { class: "gates" });
    (dec.gates || []).forEach((g) => {
      gates.appendChild(h("div", { class: "gate " + (g.ok ? "ok" : "bad") },
        h("span", { class: "g-ic" }, g.ok ? "✓" : "✗"),
        h("span", { class: "g-name" }, g.gate),
        h("span", { class: "g-why" }, g.ok ? h("span", { class: "muted" }, "passed")
          : [h("div", {}, g.reason), h("div", { class: "remedy" }, "→ " + g.remedy)])));
    });
    out.appendChild(gates);
    if (dec.constraints) out.appendChild(h("div", { class: "faint", style: "margin-top:10px;font-size:12px" },
      "Policy " + dec.policyVersion + " · envelope: " + dec.constraints.max_pods + " pod(s), " +
      dec.constraints.max_duration_seconds + "s, ≤" + dec.constraints.max_traffic_pct + "% traffic"));
  } catch (e) { clear(out); out.appendChild(h("div", { class: "empty" }, "Error: " + e.message)); }
}
function num(id) { return parseInt(document.getElementById(id).value || "0", 10); }

// ---------- experiments ----------
views.experiments = async () => {
  const exps = await api("/api/v1/experiments");
  const wrap = h("div", { class: "view" });
  wrap.appendChild(h("div", { class: "view-head" },
    h("h2", {}, "Experiment Catalog"),
    h("p", {}, exps.length + " experiments, each mapped to MITRE ATT&CK, OWASP, and compliance control families.")));
  const card = h("div", { class: "card" });
  const t = h("table", { class: "list" });
  t.appendChild(h("thead", {}, h("tr", {},
    h("th", {}, "ID"), h("th", {}, "Family"), h("th", {}, "Title"), h("th", {}, "Mode"), h("th", {}, "Standards"))));
  const tb = h("tbody");
  exps.forEach((e) => {
    const chips = h("div", { class: "chips" });
    (e.attackIds || []).concat(e.owaspIds || []).slice(0, 3).forEach((id) => chips.appendChild(h("span", { class: "chip" }, id)));
    tb.appendChild(h("tr", {},
      h("td", { class: "faint" }, e.name),
      h("td", {}, e.familyTitle),
      h("td", {}, e.title),
      h("td", {}, h("span", { class: "muted" }, e.mode)),
      h("td", {}, chips)));
  });
  t.appendChild(tb); card.appendChild(t);
  wrap.appendChild(card);
  return wrap;
};

// ---------- game day ----------
views.gameday = async () => {
  const gd = await api("/api/v1/gamedays");
  const wrap = h("div", { class: "view" });
  wrap.appendChild(h("div", { class: "view-head" },
    h("h2", {}, "⛨ Game Day — " + (gd.title || "")),
    h("p", {}, "A pre-event security readiness run (read-only view of a grouped sequence of runs).")));
  const tiles = h("div", { class: "tiles" });
  tiles.appendChild(tile("Readiness", pct(gd.readiness), "mean efficacy", gd.readiness));
  tiles.appendChild(tile("Fail-open", String(gd.failOpen || 0), "observed", null, (gd.failOpen || 0) > 0));
  tiles.appendChild(tile("SLO breaches", String(gd.sloBreaches || 0), "slow detections", null));
  tiles.appendChild(tile("Aborts", String(gd.aborts || 0), "safety aborts", null));
  wrap.appendChild(tiles);
  const card = h("div", { class: "card" });
  (gd.steps || []).forEach((s) => {
    const ic = s.verdict === "PASS" ? "✓" : s.verdict === "FAIL_SLOW" ? "◐" : "✗";
    card.appendChild(h("div", { class: "gd-step" },
      h("span", { class: "n" }, String(s.n)),
      h("span", { class: "ic rung " + (s.verdict === "PASS" ? "PASS" : "FAIL") }, ic),
      h("span", { style: "flex:1" }, s.experiment + " ", h("span", { class: "faint" }, "· " + s.service + " · " + s.family)),
      h("span", { class: "pill " + s.verdict }, s.verdict)));
  });
  if (!(gd.steps || []).length) card.appendChild(h("div", { class: "empty" }, "No runs to assemble a game day — run `shoebill demo`."));
  wrap.appendChild(card);
  return wrap;
};

// ---------- boot ----------
document.getElementById("tabs").addEventListener("click", (e) => {
  if (e.target.dataset && e.target.dataset.view) setView(e.target.dataset.view);
});
window.addEventListener("hashchange", () => setView((location.hash || "#scorecard").slice(1)));
api("/api/v1/version").then((v) => { document.getElementById("ver").textContent = (v.version || "dev"); }).catch(() => {});
setView((location.hash || "#scorecard").slice(1));
