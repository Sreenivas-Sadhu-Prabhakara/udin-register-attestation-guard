/* The UDIN register interface. Talks only to /api on this same loopback
   address; there is no other host in this file and no outbound request. */
(function () {
  "use strict";

  var state = null;     // /api/state
  var board = null;     // /api/board
  var selected = null;  // the row being shown in the detail panel
  var filter = "needs-attention";
  var query = "";
  var checkTimer = null;
  var searchTimer = null;

  function el(id) { return document.getElementById(id); }

  function api(path, options) {
    return fetch(path, options || {}).then(function (response) {
      return response.json().catch(function () {
        throw new Error("The tool sent back something this page could not read.");
      }).then(function (body) {
        if (!response.ok) { throw new Error(body && body.error ? body.error : "Request failed."); }
        return body;
      });
    });
  }

  function send(path, method, payload) {
    return api(path, {
      method: method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    });
  }

  function text(node, value) { node.textContent = value == null ? "" : String(value); }

  function show(node, visible) {
    if (visible) { node.removeAttribute("hidden"); } else { node.setAttribute("hidden", ""); }
  }

  function say(node, message, ok) {
    text(node, message || "");
    node.classList.toggle("ok", !!ok);
  }

  function pretty(iso) {
    if (!iso) { return "—"; }
    var parts = String(iso).split("T");
    var day = parts[0].split("-");
    if (day.length !== 3) { return String(iso); }
    var months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
    var out = day[2] + " " + months[parseInt(day[1], 10) - 1] + " " + day[0];
    return parts[1] ? out + " " + parts[1] : out;
  }

  function dayCount(n) { return Math.abs(n) === 1 ? "1 day" : Math.abs(n) + " days"; }

  /* ---------- theme ---------- */

  function currentTheme() {
    var set = document.documentElement.getAttribute("data-theme");
    if (set) { return set; }
    return window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }

  function paintThemeButton(button) {
    var dark = currentTheme() === "dark";
    text(button, dark ? "☀ Light" : "☾ Dark");
    button.setAttribute("aria-label", dark ? "Switch to light theme" : "Switch to dark theme");
  }

  function wireTheme() {
    var button = document.querySelector("[data-themer]");
    if (!button) { return; }
    paintThemeButton(button);
    button.addEventListener("click", function () {
      var next = currentTheme() === "dark" ? "light" : "dark";
      document.documentElement.setAttribute("data-theme", next);
      try { localStorage.setItem("omsme-theme", next); } catch (e) { /* private window */ }
      paintThemeButton(button);
    });
  }

  /* ---------- summary tiles and filters ---------- */

  var TILES = [
    { filter: "no-udin", label: "No UDIN yet", pick: function (c) { return c.noUdin; }, sev: "warn" },
    { filter: "due-soon", label: "Deadline close", pick: function (c) { return c.dueSoon + c.dueToday; }, sev: "warn" },
    { filter: "overdue", label: "Window closed", pick: function (c) { return c.overdue; }, sev: "breach" },
    { filter: "in-circulation", label: "Revoked, still out", pick: function (c) { return c.revokedInCirculation; }, sev: "breach" },
    { filter: "integrity", label: "Record queries", pick: function (c) { return c.integrity; }, sev: "warn" },
    { filter: "recorded", label: "UDIN on record", pick: function (c) { return c.recorded; }, sev: "ok" },
    { filter: "all", label: "Documents in all", pick: function (c) { return c.total; }, sev: "" }
  ];

  var FILTER_LABELS = {
    "needs-attention": "Needs attention",
    "all": "Everything",
    "no-udin": "No UDIN",
    "due-soon": "Deadline close",
    "overdue": "Window closed",
    "recorded": "On record",
    "late": "Generated late",
    "revoked": "Revoked",
    "in-circulation": "Revoked, still out",
    "integrity": "Record queries"
  };

  function renderTiles(counts) {
    var host = el("tiles");
    host.innerHTML = "";
    TILES.forEach(function (tile) {
      var value = tile.pick(counts) || 0;
      var button = document.createElement("button");
      button.type = "button";
      button.className = "tile" + (value > 0 && tile.sev ? " sev-" + tile.sev : "");
      button.setAttribute("aria-pressed", filter === tile.filter ? "true" : "false");
      button.dataset.filter = tile.filter;
      var n = document.createElement("span");
      n.className = "n";
      text(n, value);
      var l = document.createElement("span");
      l.className = "l";
      text(l, tile.label);
      button.appendChild(n);
      button.appendChild(l);
      button.addEventListener("click", function () { setFilter(tile.filter); });
      host.appendChild(button);
    });
  }

  function renderFilters() {
    var host = el("filters");
    host.innerHTML = "";
    (state.filters || []).forEach(function (name) {
      var button = document.createElement("button");
      button.type = "button";
      text(button, FILTER_LABELS[name] || name);
      button.setAttribute("aria-pressed", filter === name ? "true" : "false");
      button.addEventListener("click", function () { setFilter(name); });
      host.appendChild(button);
    });
  }

  function setFilter(name) {
    filter = name;
    loadBoard();
  }

  /* ---------- the register ---------- */

  function renderRows() {
    var host = el("rows");
    host.innerHTML = "";
    if (!board.rows.length) {
      var empty = document.createElement("p");
      empty.className = "empty";
      text(empty, board.counts.total === 0
        ? "Nothing in the register yet. Record a signed document below, or paste a list of them."
        : "No document in the register matches this view.");
      host.appendChild(empty);
      return;
    }

    board.rows.forEach(function (row) {
      var entry = row.entry;
      var a = row.assessment;

      var card = document.createElement("button");
      card.type = "button";
      card.className = "rowcard sev-" + a.severity;
      card.setAttribute("aria-pressed", selected && selected.entry.id === entry.id ? "true" : "false");

      var top = document.createElement("div");
      top.className = "top";
      var client = document.createElement("span");
      client.className = "client";
      text(client, entry.clientName);
      var badge = document.createElement("span");
      badge.className = "badge sev-" + a.severity;
      text(badge, stageLabel(a.stage));
      top.appendChild(client);
      top.appendChild(badge);

      var meta = document.createElement("div");
      meta.className = "meta";
      text(meta, [entry.documentType, entry.financialYear ? "FY " + entry.financialYear : null,
        entry.partnerName || null].filter(Boolean).join(" · "));

      var when = document.createElement("div");
      when.className = "when";
      text(when, whenLine(entry, a));

      var udin = document.createElement("div");
      udin.className = "udin";
      text(udin, entry.udin ? entry.udin : "no UDIN recorded");

      card.appendChild(top);
      card.appendChild(meta);
      card.appendChild(when);
      card.appendChild(udin);
      card.addEventListener("click", function () { selectRow(row); });
      host.appendChild(card);
    });
  }

  function stageLabel(stage) {
    switch (stage) {
      case "awaiting": return "UDIN pending";
      case "overdue": return "Window closed";
      case "recorded": return "On record";
      case "late": return "Generated late";
      case "revoked": return "Revoked";
      case "not-required": return "Before the mandate";
      default: return stage;
    }
  }

  function whenLine(entry, a) {
    var parts = ["Signed " + pretty(entry.signedOn)];
    if (a.stage === "awaiting") {
      parts.push(a.daysLeft === 0 ? "last day is today" : dayCount(a.daysLeft) + " left (by " + pretty(a.deadline) + ")");
    } else if (a.stage === "overdue") {
      parts.push("deadline was " + pretty(a.deadline) + ", " + dayCount(a.daysLeft) + " ago");
    } else if (a.stage === "recorded" || a.stage === "late") {
      parts.push("UDIN on day " + a.daysUsed + " of " + a.windowDays);
    } else if (a.stage === "revoked") {
      parts.push("revoked " + pretty(entry.revocation ? entry.revocation.revokedAt : null));
    }
    return parts.join(" · ");
  }

  /* ---------- detail panel ---------- */

  function selectRow(row) {
    selected = row;
    fillForm(row.entry);
    renderDetail();
    renderRows();
    el("detail").scrollIntoView({ block: "nearest" });
  }

  function renderDetail() {
    var body = el("detail-body");
    if (!selected) {
      show(body, false);
      show(el("detail-empty"), true);
      return;
    }
    show(el("detail-empty"), false);
    show(body, true);

    var entry = selected.entry;
    var a = selected.assessment;

    text(el("detail-stage"), entry.id + " · " + stageLabel(a.stage));
    text(el("detail-days"), bigNumber(a));
    text(el("detail-headline"), a.headline);
    text(el("detail-next"), a.nextStep);

    var table = el("detail-table");
    table.innerHTML = "";
    var lines = [
      ["Client", entry.clientName + (entry.clientRef ? " (" + entry.clientRef + ")" : "")],
      ["Document", entry.documentType],
      ["Category", categoryLabel(entry.category)],
      ["Financial year", entry.financialYear ? entry.financialYear : "—"],
      ["Signed on", pretty(entry.signedOn)],
      ["Engagement partner", [entry.partnerName, entry.partnerMrn].filter(Boolean).join(", ") || "—"],
      ["UDIN", entry.udin || "not generated yet"],
      ["UDIN generated", entry.generatedAt ? pretty(entry.generatedAt) : "—"],
      ["Deadline", a.deadline ? pretty(a.deadline) + (a.deadlineRule ? " (" + a.deadlineRule + ")" : "") : "—"]
    ];
    if (a.stage === "recorded" || a.stage === "late") {
      lines.push(["Days used", a.daysUsed + " of " + a.windowDays + ", counting both ends"]);
    }
    if (entry.eFilingSubmittedOn) {
      lines.push(["Submitted on the e-filing portal", pretty(entry.eFilingSubmittedOn)]);
      lines.push(["E-filing update deadline", pretty(a.eFilingDeadline)]);
    }
    if (a.revokeBy) {
      lines.push(["Revocable on the portal", (a.revokeExact ? "until " : "until no later than ") + pretty(a.revokeBy) +
        (a.revokeWindow === "closed" ? " — shut" : a.revokeWindow === "closing-today" ? " — shuts today" : "")]);
    }
    if (entry.figures && entry.figures.length) {
      lines.push(["Figures", entry.figures.map(function (f) { return f.keyword + " = " + f.value; }).join("; ")]);
    }
    if (entry.description) { lines.push(["Description", entry.description]); }
    if (entry.revocation) {
      lines.push(["Revoked on", pretty(entry.revocation.revokedAt)]);
      lines.push(["Reason", entry.revocation.reason]);
      lines.push(["Replacement UDIN", entry.revocation.replacementUdin || "none recorded"]);
      lines.push(["Document recalled", entry.revocation.recalled ? "yes" : "no"]);
    }
    if (entry.notes) { lines.push(["Note", entry.notes]); }

    lines.forEach(function (pair) {
      var tr = document.createElement("tr");
      var th = document.createElement("th");
      text(th, pair[0]);
      var td = document.createElement("td");
      text(td, pair[1]);
      tr.appendChild(th);
      tr.appendChild(td);
      table.appendChild(tr);
    });

    var flags = el("detail-flags");
    flags.innerHTML = "";
    (a.flags || []).forEach(function (flag) {
      var li = document.createElement("li");
      li.className = "sev-" + flag.severity;
      text(li, flag.message);
      flags.appendChild(li);
    });

    var revokeBox = el("revoke-box");
    var canRevoke = !!entry.udin && !entry.revocation;
    show(revokeBox, canRevoke);
    if (canRevoke) {
      text(el("revoke-hint"), "The portal accepts a revocation only inside its window (" +
        state.settings.revocationWindowHours + " hours from generation). Recording it here keeps the firm's own record either way.");
      el("revokedOn").value = state.today;
      say(el("revoke-error"), "");
    }
    show(el("recall-box"), !!entry.revocation);
    if (entry.revocation) { el("recalled").checked = !!entry.revocation.recalled; }

    show(el("delete-entry"), !entry.udin && !entry.revocation);
  }

  function bigNumber(a) {
    switch (a.stage) {
      case "awaiting": return a.daysLeft === 0 ? "Today" : dayCount(a.daysLeft) + " left";
      case "overdue": return dayCount(a.daysLeft) + " over";
      case "recorded": return "Day " + a.daysUsed + " of " + a.windowDays;
      case "late": return "Day " + a.daysUsed + " of " + a.windowDays;
      case "revoked": return "Revoked";
      case "not-required": return "Not required";
      default: return "";
    }
  }

  function categoryLabel(key) {
    var found = (state.categories || []).filter(function (c) { return c.key === key; })[0];
    return found ? found.label : key;
  }

  /* ---------- the form ---------- */

  function figureRow(keyword, value) {
    var wrap = document.createElement("div");
    wrap.className = "figure-row";
    var k = document.createElement("input");
    k.type = "text";
    k.placeholder = "Keyword, e.g. Turnover";
    k.maxLength = 120;
    k.dataset.figureKeyword = "1";
    k.value = keyword || "";
    var v = document.createElement("input");
    v.type = "text";
    v.placeholder = "Figure as keyed";
    v.maxLength = 120;
    v.dataset.figureValue = "1";
    v.value = value || "";
    wrap.appendChild(k);
    wrap.appendChild(v);
    return wrap;
  }

  function setFigures(figures) {
    var host = el("figures");
    host.innerHTML = "";
    var list = (figures && figures.length) ? figures : [{}, {}];
    list.forEach(function (f) { host.appendChild(figureRow(f.keyword, f.value)); });
  }

  function readFigures() {
    var out = [];
    var rows = el("figures").querySelectorAll(".figure-row");
    Array.prototype.forEach.call(rows, function (row) {
      var keyword = row.querySelector("[data-figure-keyword]").value.trim();
      var value = row.querySelector("[data-figure-value]").value.trim();
      if (keyword || value) { out.push({ keyword: keyword, value: value }); }
    });
    return out;
  }

  function fillForm(entry) {
    el("entry-id").value = entry.id || "";
    el("category").value = entry.category || "";
    el("documentType").value = entry.documentType || "";
    el("clientName").value = entry.clientName || "";
    el("clientRef").value = entry.clientRef || "";
    el("financialYear").value = entry.financialYear || "";
    el("signedOn").value = entry.signedOn || "";
    el("partnerName").value = entry.partnerName || "";
    el("partnerMrn").value = entry.partnerMrn || "";
    el("udin").value = entry.udin || "";
    el("eFilingSubmittedOn").value = entry.eFilingSubmittedOn || "";
    el("description").value = entry.description || "";
    el("notes").value = entry.notes || "";
    el("inCirculation").checked = !!entry.inCirculation;

    var generated = entry.generatedAt ? String(entry.generatedAt).split("T") : [];
    el("generatedOn").value = generated[0] || "";
    el("generatedTime").value = generated[1] || "";

    setFigures(entry.figures);
    text(el("form-heading"), entry.id ? "Entry " + entry.id : "Record a signed document");
    say(el("form-error"), "");
    updateDescriptionCount();
    updateFinancialYearEcho();
    checkUDIN();
  }

  function clearForm() {
    selected = null;
    fillForm({ category: state.categories[0].key });
    renderDetail();
    renderRows();
  }

  function readForm() {
    var generatedOn = el("generatedOn").value;
    var generatedTime = el("generatedTime").value;
    return {
      id: el("entry-id").value,
      category: el("category").value,
      documentType: el("documentType").value,
      description: el("description").value,
      clientName: el("clientName").value,
      clientRef: el("clientRef").value,
      financialYear: el("financialYear").value,
      signedOn: el("signedOn").value,
      partnerName: el("partnerName").value,
      partnerMrn: el("partnerMrn").value,
      udin: el("udin").value,
      generatedAt: generatedOn ? (generatedTime ? generatedOn + "T" + generatedTime : generatedOn) : "",
      figures: readFigures(),
      eFilingSubmittedOn: el("eFilingSubmittedOn").value,
      inCirculation: el("inCirculation").checked,
      notes: el("notes").value
    };
  }

  function updateDescriptionCount() {
    var length = el("description").value.trim().length;
    var min = state.reference.descriptionMin;
    var max = state.reference.descriptionMax;
    if (length === 0) {
      text(el("description-count"), "The portal asks for " + min + " to " + max + " characters from the document.");
      return;
    }
    text(el("description-count"), length + " of " + max + " characters" + (length < min ? " — the portal asks for at least " + min + "." : "."));
  }

  function updateFinancialYearEcho() {
    var raw = el("financialYear").value.trim();
    var signed = el("signedOn").value;
    if (raw) {
      text(el("fy-echo"), "");
      return;
    }
    if (!signed) {
      text(el("fy-echo"), "");
      return;
    }
    var year = parseInt(signed.slice(0, 4), 10);
    var month = parseInt(signed.slice(5, 7), 10);
    var start = month >= 4 ? year : year - 1;
    var label = start + "-" + String((start + 1) % 100).padStart(2, "0");
    text(el("fy-echo"), "Signed in FY " + label + " — fill this in if the document is for another year.");
  }

  function checkUDIN() {
    var raw = el("udin").value.trim();
    var node = el("udin-check");
    node.className = "hint";
    if (!raw) {
      text(node, "");
      return;
    }
    var params = new URLSearchParams({ udin: raw });
    var signed = el("signedOn").value;
    if (signed) { params.set("signedOn", signed); }
    var mrn = el("partnerMrn").value.trim();
    if (mrn) { params.set("mrn", mrn); }

    api("/api/check?" + params.toString()).then(function (result) {
      if (!result.findings.length) {
        node.className = "hint check-ok";
        text(node, "Well formed: 20" + result.udin.yearDigits + ", membership number " +
          result.udin.membershipNumber + ". Only the portal can confirm it exists.");
        return;
      }
      var worst = result.findings[0];
      result.findings.forEach(function (f) {
        if (f.severity === "breach") { worst = f; }
      });
      node.className = "hint " + (worst.severity === "breach" ? "check-bad" : "check-warn");
      text(node, result.findings.map(function (f) { return f.message; }).join(" "));
    }).catch(function (err) {
      node.className = "hint check-bad";
      text(node, err.message);
    });
  }

  /* ---------- paste ---------- */

  function renderPaste(result) {
    var host = el("paste-result");
    host.innerHTML = "";

    var summary = document.createElement("p");
    summary.className = "assumptions";
    text(summary, result.committed
      ? result.imported + " of " + result.read + " documents went into the register."
      : result.read + " documents read. Nothing has been added yet.");
    host.appendChild(summary);

    if (result.entries && result.entries.length && !result.committed) {
      var wrap = document.createElement("div");
      wrap.className = "preview";
      var table = document.createElement("table");
      table.className = "breakdown";
      var head = document.createElement("tr");
      ["Client", "Document", "Signed", "UDIN"].forEach(function (heading) {
        var th = document.createElement("th");
        text(th, heading);
        head.appendChild(th);
      });
      table.appendChild(head);
      result.entries.slice(0, 30).forEach(function (entry) {
        var tr = document.createElement("tr");
        [entry.clientName, entry.documentType, pretty(entry.signedOn), entry.udin || "—"].forEach(function (cell) {
          var td = document.createElement("td");
          text(td, cell);
          tr.appendChild(td);
        });
        table.appendChild(tr);
      });
      wrap.appendChild(table);
      if (result.entries.length > 30) {
        var more = document.createElement("p");
        more.className = "assumptions";
        text(more, "and " + (result.entries.length - 30) + " more.");
        wrap.appendChild(more);
      }
      host.appendChild(wrap);
    }

    if (result.warnings && result.warnings.length) {
      var list = document.createElement("ul");
      list.className = "warnlist";
      result.warnings.forEach(function (warning) {
        var li = document.createElement("li");
        text(li, warning);
        list.appendChild(li);
      });
      host.appendChild(list);
    }

    show(el("paste-commit"), !result.committed && result.read > 0);
  }

  function pasteError(message) {
    var host = el("paste-result");
    host.innerHTML = "";
    var p = document.createElement("p");
    p.className = "form-error";
    text(p, message);
    host.appendChild(p);
    show(el("paste-commit"), false);
  }

  /* ---------- settings ---------- */

  function fillSettings(settings, firmName) {
    el("firmName").value = firmName || "";
    el("generationWindowDays").value = settings.generationWindowDays;
    el("eFilingWindowDays").value = settings.eFilingWindowDays;
    el("revocationWindowHours").value = settings.revocationWindowHours;
    el("warnDays").value = settings.warnDays;
  }

  function renderSources() {
    var r = state.reference;
    var list = el("sources-list");
    list.innerHTML = "";
    [
      ["Generation window", r.generationSource],
      ["How the days are counted", r.countingSource],
      ["E-filing update window", r.eFilingSource],
      ["Portal revocation window", r.revocationSource],
      ["When UDIN became mandatory", r.mandateSource],
      ["The shape of a UDIN", r.formatSource],
      ["The limit of the shape check", r.noCheckDigit],
      ["Why the firm keeps its own register", r.archiving],
      ["What is at stake", r.consequence]
    ].forEach(function (pair) {
      var li = document.createElement("li");
      var strong = document.createElement("strong");
      text(strong, pair[0] + ": ");
      li.appendChild(strong);
      li.appendChild(document.createTextNode(pair[1]));
      list.appendChild(li);
    });

    var footer = document.createElement("span");
    text(footer, "These figures are indicative. Confirm the current position at " + r.portalUrl + ".");
    el("foot-note").innerHTML = "";
    el("foot-note").appendChild(footer);
  }

  /* ---------- loading ---------- */

  function loadState() {
    return api("/api/state").then(function (result) {
      state = result;
      text(el("data-file"), result.dataFile);
      text(el("portal-url"), "Generate UDINs at " + result.reference.portalUrl + ". " + result.reference.archiving);
      text(el("pill-window"), result.settings.generationWindowDays + " days from signing");

      ["category", "paste-category"].forEach(function (id) {
        var select = el(id);
        select.innerHTML = "";
        result.categories.forEach(function (category) {
          var option = document.createElement("option");
          option.value = category.key;
          text(option, category.label + " (mandatory from " + pretty(category.mandatoryFrom) + ")");
          select.appendChild(option);
        });
      });

      var types = el("documentTypes");
      types.innerHTML = "";
      result.categories.forEach(function (category) {
        (category.documentTypes || []).forEach(function (name) {
          var option = document.createElement("option");
          option.value = name;
          types.appendChild(option);
        });
      });

      fillList("clients", result.clients);
      fillList("partners", (result.partners || []).map(function (p) { return p.name; }));
      fillSettings(result.settings, result.firmName);
      renderSources();
      renderFilters();
    });
  }

  function fillList(id, values) {
    var list = el(id);
    list.innerHTML = "";
    (values || []).forEach(function (value) {
      if (!value) { return; }
      var option = document.createElement("option");
      option.value = value;
      list.appendChild(option);
    });
  }

  function loadBoard() {
    var params = new URLSearchParams({ filter: filter });
    if (query) { params.set("q", query); }
    return api("/api/board?" + params.toString()).then(function (result) {
      board = result;
      if (selected) {
        var again = result.rows.filter(function (row) { return row.entry.id === selected.entry.id; })[0];
        if (again) { selected = again; }
      }
      renderTiles(result.counts);
      renderFilters();
      renderRows();
      renderDetail();
      text(el("board-summary"), result.shown + " of " + result.counts.total +
        " documents shown · window " + result.settings.generationWindowDays + " days · " +
        result.counts.noUdin + " without a UDIN · " + result.counts.overdue + " past the deadline");
    });
  }

  /* ---------- wiring ---------- */

  function wire() {
    el("entry-form").addEventListener("submit", function (event) {
      event.preventDefault();
      say(el("form-error"), "");
      send("/api/entry", "PUT", readForm()).then(function (row) {
        selected = row;
        return loadState().then(loadBoard).then(function () {
          fillForm(row.entry);
          say(el("form-error"), "Saved as " + row.entry.id + ".", true);
        });
      }).catch(function (err) { say(el("form-error"), err.message); });
    });

    el("clear-form").addEventListener("click", clearForm);

    el("delete-entry").addEventListener("click", function () {
      var id = el("entry-id").value;
      if (!id) { return; }
      api("/api/entry?id=" + encodeURIComponent(id), { method: "DELETE" }).then(function () {
        clearForm();
        return loadState().then(loadBoard);
      }).catch(function (err) { say(el("form-error"), err.message); });
    });

    el("add-figure").addEventListener("click", function () {
      el("figures").appendChild(figureRow("", ""));
    });

    el("description").addEventListener("input", updateDescriptionCount);
    el("financialYear").addEventListener("input", updateFinancialYearEcho);
    el("signedOn").addEventListener("change", function () {
      updateFinancialYearEcho();
      checkUDIN();
    });
    el("partnerMrn").addEventListener("change", checkUDIN);
    el("udin").addEventListener("input", function () {
      window.clearTimeout(checkTimer);
      checkTimer = window.setTimeout(checkUDIN, 350);
    });

    el("search").addEventListener("input", function () {
      query = el("search").value;
      window.clearTimeout(searchTimer);
      searchTimer = window.setTimeout(loadBoard, 250);
    });

    el("revoke").addEventListener("click", function () {
      if (!selected) { return; }
      say(el("revoke-error"), "");
      send("/api/revoke", "POST", {
        id: selected.entry.id,
        revokedAt: el("revokedOn").value,
        reason: el("revokeReason").value,
        replacementUdin: el("replacementUdin").value,
        recalled: false
      }).then(function (row) {
        selected = row;
        el("revokeReason").value = "";
        el("replacementUdin").value = "";
        return loadBoard();
      }).catch(function (err) { say(el("revoke-error"), err.message); });
    });

    el("recalled").addEventListener("change", function () {
      if (!selected) { return; }
      send("/api/recall", "POST", { id: selected.entry.id, recalled: el("recalled").checked })
        .then(function (row) {
          selected = row;
          return loadBoard();
        })
        .catch(function (err) { say(el("revoke-error"), err.message); });
    });

    el("settings-form").addEventListener("submit", function (event) {
      event.preventDefault();
      say(el("settings-error"), "");
      send("/api/settings", "PUT", {
        firmName: el("firmName").value,
        settings: {
          generationWindowDays: parseInt(el("generationWindowDays").value, 10),
          eFilingWindowDays: parseInt(el("eFilingWindowDays").value, 10),
          revocationWindowHours: parseInt(el("revocationWindowHours").value, 10),
          warnDays: parseInt(el("warnDays").value, 10)
        }
      }).then(function (result) {
        state = result;
        fillSettings(result.settings, result.firmName);
        text(el("pill-window"), result.settings.generationWindowDays + " days from signing");
        say(el("settings-error"), "Saved.", true);
        return loadBoard();
      }).catch(function (err) { say(el("settings-error"), err.message); });
    });

    el("reset-settings").addEventListener("click", function () {
      var r = state.reference;
      fillSettings({
        generationWindowDays: r.generationWindowDays,
        eFilingWindowDays: r.eFilingWindowDays,
        revocationWindowHours: r.revocationWindowHours,
        warnDays: r.warnDays
      }, el("firmName").value);
      say(el("settings-error"), "The reference values are in the boxes. Save to apply them.", true);
    });

    el("paste-preview").addEventListener("click", function () {
      send("/api/paste", "POST", {
        text: el("paste-text").value,
        category: el("paste-category").value,
        commit: false
      }).then(renderPaste).catch(function (err) { pasteError(err.message); });
    });

    el("paste-commit").addEventListener("click", function () {
      send("/api/paste", "POST", {
        text: el("paste-text").value,
        category: el("paste-category").value,
        commit: true
      }).then(function (result) {
        renderPaste(result);
        el("paste-text").value = "";
        return loadState().then(loadBoard);
      }).catch(function (err) { pasteError(err.message); });
    });
  }

  wireTheme();
  loadState()
    .then(function () {
      wire();
      fillForm({ category: state.categories[0].key });
      return loadBoard();
    })
    .catch(function (err) {
      var host = el("rows");
      host.innerHTML = "";
      var p = document.createElement("p");
      p.className = "form-error";
      text(p, err.message);
      host.appendChild(p);
    });
})();
