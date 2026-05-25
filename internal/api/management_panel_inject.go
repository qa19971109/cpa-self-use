package api

import "bytes"

var managementAuthFileTestScript = []byte(`<script id="cpa-auth-file-test-ui">
(function () {
  if (window.__cpaAuthFileTestUI) return;
  window.__cpaAuthFileTestUI = true;

  var state = { files: [], headers: {}, results: {} };
  var originalFetch = window.fetch;
  if (typeof originalFetch !== "function") return;

  function asHeaders(input, init) {
    try {
      if (init && init.headers) return new Headers(init.headers);
      if (input && input.headers) return new Headers(input.headers);
    } catch (_) {}
    return new Headers();
  }

  function captureHeaders(input, init) {
    var headers = asHeaders(input, init);
    ["authorization", "x-management-key"].forEach(function (name) {
      var value = headers.get(name);
      if (value) state.headers[name] = value;
    });
  }

  function requestURL(input) {
    if (typeof input === "string") return input;
    if (input && input.url) return input.url;
    return "";
  }

  function isAuthFilesList(url) {
    url = String(url || "");
    return /(?:^|\/)(?:v0\/management\/)?auth-files(?:\?|$)/.test(url);
  }

  function rememberAuthFiles(data, url) {
    if (data && Array.isArray(data.files)) {
      state.files = data.files;
      state.authFilesURL = String(url || "").split("#")[0].split("?")[0];
      setTimeout(scanRows, 50);
    }
  }

  window.fetch = function (input, init) {
    captureHeaders(input, init);
    var url = requestURL(input);
    var method = ((init && init.method) || (input && input.method) || "GET").toUpperCase();
    return originalFetch.apply(this, arguments).then(function (response) {
      if (method === "GET" && isAuthFilesList(url)) {
        response.clone().json().then(function (data) {
          rememberAuthFiles(data, url);
        }).catch(function () {});
      }
      return response;
    });
  };

  if (window.XMLHttpRequest) {
    var originalOpen = XMLHttpRequest.prototype.open;
    var originalSend = XMLHttpRequest.prototype.send;
    var originalSetRequestHeader = XMLHttpRequest.prototype.setRequestHeader;
    XMLHttpRequest.prototype.open = function (method, url) {
      this.__cpaAuthTestMethod = String(method || "GET").toUpperCase();
      this.__cpaAuthTestURL = String(url || "");
      this.__cpaAuthTestHeaders = {};
      return originalOpen.apply(this, arguments);
    };
    XMLHttpRequest.prototype.setRequestHeader = function (name, value) {
      var key = String(name || "").toLowerCase();
      if (key === "authorization" || key === "x-management-key") {
        state.headers[key] = String(value || "");
      }
      if (this.__cpaAuthTestHeaders) this.__cpaAuthTestHeaders[key] = String(value || "");
      return originalSetRequestHeader.apply(this, arguments);
    };
    XMLHttpRequest.prototype.send = function () {
      this.addEventListener("load", function () {
        if (this.__cpaAuthTestMethod === "GET" && isAuthFilesList(this.__cpaAuthTestURL)) {
          try { rememberAuthFiles(JSON.parse(this.responseText || "{}"), this.__cpaAuthTestURL); } catch (_) {}
        }
      });
      return originalSend.apply(this, arguments);
    };
  }

  function norm(value) {
    return String(value || "").replace(/\s+/g, " ").trim();
  }

  function fieldValues(file) {
    return [file.name, file.auth_index, file.email, file.account, file.id, file.label]
      .map(norm)
      .filter(function (value) { return value.length >= 3; });
  }

  function fileKey(file) {
    if (!file) return "";
    return norm(file.auth_index || file.authIndex || file.name || file.id);
  }

  function rowFile(row) {
    var text = norm(row && row.textContent);
    if (!text) return null;
    var best = null;
    var bestLen = 0;
    state.files.forEach(function (file) {
      fieldValues(file).forEach(function (value) {
        if (value.length > bestLen && text.indexOf(value) !== -1) {
          best = file;
          bestLen = value.length;
        }
      });
    });
    return best;
  }

  function isAuthFilesPage() {
    var route = String(window.location.pathname || "") + String(window.location.hash || "");
    if (/\/auth-files(?:[/?#]|$)/.test(route)) return true;
    return !!document.querySelector('[class*="AuthFilesPage-module__authFilesShell"],[class*="AuthFilesPage-module__authFilesHeader"]');
  }

  function authFilesScope() {
    return document.querySelector('[class*="AuthFilesPage-module__authFilesShell"]') ||
      document.querySelector('[class*="AuthFilesPage-module__page"]') ||
      document;
  }

  function cleanupMisplacedButtons() {
    document.querySelectorAll(".cpa-auth-test-btn").forEach(function (button) {
      var row = button.closest("tr,[role='row'],.ant-table-row,.el-table__row");
      button.remove();
      if (row) delete row.dataset.cpaAuthTestAttached;
    });
    document.querySelectorAll(".cpa-auth-page-test-btn").forEach(function (button) { button.remove(); });
  }

  function actionTarget(row) {
    var cardActions = row.querySelector('[class*="AuthFilesPage-module__cardActions"]');
    if (cardActions) return cardActions;
    return row.querySelector("td:last-child,[role='cell']:last-child") || row;
  }

  function authStatsTarget(card) {
    var nodes = card.querySelectorAll("div,span");
    var best = null;
    var bestLen = Infinity;
    nodes.forEach(function (node) {
      var text = norm(node.textContent);
      if (text.indexOf("\u6210\u529f") === -1 || text.indexOf("\u5931\u8d25") === -1) return;
      if (text.length < bestLen) {
        best = node;
        bestLen = text.length;
      }
    });
    return best || card;
  }

  function markAuthResult(card, file, result) {
    if (!card || !file || !result) return;
    var key = fileKey(file);
    if (key) state.results[key] = result;
    var badge = card.querySelector(".cpa-auth-validity-badge");
    if (!badge) {
      badge = document.createElement("span");
      badge.className = "cpa-auth-validity-badge";
      authStatsTarget(card).appendChild(badge);
    }
    var text = result.ok ? "\u8d26\u53f7\u6709\u6548" : "\u8d26\u53f7\u5df2\u5931\u6548";
    var title = result.ok ? "Last model test succeeded" : (result.error || "Last model test failed");
    var style = "display:inline-flex;align-items:center;margin-left:10px;padding:2px 9px;border-radius:999px;font-size:12px;font-weight:700;line-height:18px;color:" +
      (result.ok ? "#047857;background:#dcfce7;border:1px solid #86efac;" : "#b91c1c;background:#fee2e2;border:1px solid #fecaca;");
    var okValue = result.ok ? "1" : "0";
    if (badge.dataset.cpaOk === okValue && badge.textContent === text && badge.title === title) return;
    badge.dataset.cpaOk = okValue;
    badge.textContent = text;
    badge.title = title;
    if (badge.style.cssText !== style) badge.style.cssText = style;
  }

  function buildButton(file) {
    var button = document.createElement("button");
    button.type = "button";
    button.className = "cpa-auth-test-btn";
    button.textContent = "\u6d4b\u8bd5\u6a21\u578b";
    button.title = "Test this auth file with a minimal model call. Shift-click to choose model.";
    button.style.cssText = "margin-left:6px;padding:4px 8px;border:1px solid #2563eb;background:#2563eb;color:#fff;border-radius:6px;cursor:pointer;font-size:12px;line-height:18px;white-space:nowrap;";
    button.addEventListener("click", function (event) {
      event.preventDefault();
      event.stopPropagation();
      testAuthFile(file, button, event.shiftKey);
    });
    return button;
  }

  function buildPageTestButton() {
    var button = document.createElement("button");
    button.type = "button";
    button.className = "cpa-auth-page-test-btn";
    button.textContent = "\u6d4b\u8bd5\u672c\u9875";
    button.title = "Test all visible auth files on this page. Shift-click to choose model.";
    button.style.cssText = "padding:7px 12px;border:1px solid #2563eb;background:#2563eb;color:#fff;border-radius:8px;cursor:pointer;font-size:13px;font-weight:600;line-height:18px;white-space:nowrap;";
    button.addEventListener("click", function (event) {
      event.preventDefault();
      event.stopPropagation();
      testCurrentPage(button, event.shiftKey);
    });
    return button;
  }

  function ensurePageTestButton() {
    if (!isAuthFilesPage()) return;
    var scope = authFilesScope();
    var header = scope.querySelector('[class*="AuthFilesPage-module__authFilesHeader"] [class*="AuthFilesPage-module__headerActions"]') ||
      scope.querySelector('[class*="AuthFilesPage-module__headerActions"]');
    if (!header || header.querySelector(".cpa-auth-page-test-btn")) return;
    header.insertBefore(buildPageTestButton(), header.firstChild || null);
  }

  function visibleAuthCards() {
    if (!isAuthFilesPage()) return [];
    var scope = authFilesScope();
    var cards = Array.prototype.slice.call(scope.querySelectorAll('[class*="AuthFilesPage-module__fileCard"]'));
    return cards.map(function (card) {
      return { card: card, file: rowFile(card) };
    }).filter(function (item) {
      return item.file && fileKey(item.file);
    });
  }

  function scanRows() {
    if (!state.files.length) return;
    if (!isAuthFilesPage()) {
      cleanupMisplacedButtons();
      return;
    }
    ensurePageTestButton();
    var scope = authFilesScope();
    var rows = scope.querySelectorAll('tr,[role="row"],.ant-table-row,.el-table__row,[class*="AuthFilesPage-module__fileCard"]');
    rows.forEach(function (row) {
      if (!row) return;
      var file = rowFile(row);
      if (!file) return;
      var result = state.results[fileKey(file)];
      if (result) markAuthResult(row, file, result);
      if (row.dataset.cpaAuthTestAttached === "1" || row.querySelector(".cpa-auth-test-btn")) return;
      row.dataset.cpaAuthTestAttached = "1";
      actionTarget(row).appendChild(buildButton(file));
    });
  }

  function resultText(result) {
    if (!result) return "empty response";
    if (result.ok) {
      return (result.text || result.raw_response || "success") + "\n\nlatency: " + result.latency_ms + "ms";
    }
    return (result.error || "request failed") + "\n\nstatus: " + (result.status_code || "unknown") + "\nlatency: " + result.latency_ms + "ms";
  }

  function showModal(title, text, ok) {
    var cover = document.createElement("div");
    cover.style.cssText = "position:fixed;inset:0;background:rgba(15,23,42,.35);z-index:2147483647;display:flex;align-items:center;justify-content:center;padding:24px;";
    var panel = document.createElement("div");
    panel.style.cssText = "max-width:720px;width:min(720px,96vw);max-height:80vh;overflow:auto;background:#fff;color:#111827;border-radius:8px;box-shadow:0 20px 50px rgba(15,23,42,.25);padding:18px;";
    var header = document.createElement("div");
    header.style.cssText = "font-weight:600;margin-bottom:10px;color:" + (ok ? "#047857" : "#b91c1c") + ";";
    header.textContent = title;
    var pre = document.createElement("pre");
    pre.style.cssText = "white-space:pre-wrap;word-break:break-word;background:#f8fafc;border:1px solid #e5e7eb;border-radius:6px;padding:12px;font-size:12px;line-height:1.5;";
    pre.textContent = text;
    var close = document.createElement("button");
    close.type = "button";
    close.textContent = "Close";
    close.style.cssText = "margin-top:12px;padding:6px 12px;border:1px solid #d1d5db;background:#fff;border-radius:6px;cursor:pointer;";
    close.onclick = function () { cover.remove(); };
    panel.appendChild(header);
    panel.appendChild(pre);
    panel.appendChild(close);
    cover.appendChild(panel);
    cover.addEventListener("click", function (event) {
      if (event.target === cover) cover.remove();
    });
    document.body.appendChild(cover);
  }

  function fetchWithTimeout(url, options, timeoutMs) {
    timeoutMs = timeoutMs || 90000;
    var controller = new AbortController();
    var timer = setTimeout(function () { controller.abort(); }, timeoutMs);
    var nextOptions = Object.assign({}, options || {}, { signal: controller.signal });
    return originalFetch(url, nextOptions).finally(function () { clearTimeout(timer); });
  }

  function testAuthFile(file, button, chooseModel, card, silent) {
    var model = norm(file && file.__test_model) || "gpt-5.5";
    if (chooseModel) {
      model = window.prompt("Model", model) || model;
    }
    if (button) {
      button.disabled = true;
      button.textContent = "\u6d4b\u8bd5\u4e2d";
    }
    var headers = Object.assign({}, state.headers, { "content-type": "application/json" });
    var endpoint = state.authFilesURL ? state.authFilesURL + "/test" : "/v0/management/auth-files/test";
    return fetchWithTimeout(endpoint, {
      method: "POST",
      headers: headers,
      body: JSON.stringify({ auth_index: file.auth_index, name: file.name, model: model })
    }, 90000).then(function (response) {
      return response.json().catch(function () {
        return { ok: false, status_code: response.status, error: "invalid json response" };
      });
    }).then(function (result) {
      markAuthResult(card || authCardForFile(file), file, result);
      var title = (result.ok ? "Auth test succeeded: " : "Auth test failed: ") + (file.name || file.auth_index || "");
      if (!silent) showModal(title, resultText(result), !!result.ok);
      return result;
    }).catch(function (error) {
      var errorText = error && error.name === "AbortError" ? "request timed out after 90s" : String(error && error.message || error);
      var result = { ok: false, status_code: 0, error: errorText, latency_ms: 0 };
      markAuthResult(card || authCardForFile(file), file, result);
      if (!silent) showModal("Auth test failed", result.error, false);
      return result;
    }).finally(function () {
      if (button) {
        button.disabled = false;
        button.textContent = "\u6d4b\u8bd5\u6a21\u578b";
      }
    });
  }

  function authCardForFile(file) {
    var items = visibleAuthCards();
    var key = fileKey(file);
    for (var i = 0; i < items.length; i++) {
      if (fileKey(items[i].file) === key) return items[i].card;
    }
    return null;
  }

  function testCurrentPage(button, chooseModel) {
    var model = "gpt-5.5";
    if (chooseModel) {
      model = window.prompt("Model", model) || model;
    }
    var items = visibleAuthCards();
    var seen = {};
    items = items.filter(function (item) {
      var key = fileKey(item.file);
      if (!key || seen[key]) return false;
      seen[key] = true;
      return true;
    });
    if (!items.length) {
      showModal("\u6d4b\u8bd5\u672c\u9875", "\u5f53\u524d\u9875\u6ca1\u6709\u53ef\u6d4b\u8bd5\u7684\u8ba4\u8bc1\u6587\u4ef6", false);
      return;
    }
    button.disabled = true;
    var done = 0;
    var success = 0;
    var failed = 0;
    var failedLines = [];
    var runOne = function (item) {
      button.textContent = "\u6d4b\u8bd5 " + done + "/" + items.length;
      return testAuthFile(Object.assign({}, item.file, { __test_model: model }), null, false, item.card, true).then(function (result) {
        done++;
        if (result && result.ok) {
          success++;
        } else {
          failed++;
          failedLines.push((item.file.name || item.file.auth_index || "unknown") + ": " + ((result && result.error) || "failed"));
        }
        button.textContent = "\u6d4b\u8bd5 " + done + "/" + items.length;
      });
    };
    var chain = Promise.resolve();
    items.forEach(function (item) {
      chain = chain.then(function () { return runOne(item); });
    });
    chain.then(function () {
      var summary = "\u6210\u529f " + success + " \u4e2a\uff0c\u5931\u8d25 " + failed + " \u4e2a";
      if (failedLines.length) summary += "\n\n\u5931\u8d25\u8d26\u53f7:\n" + failedLines.join("\n");
      showModal("\u5f53\u524d\u9875\u6a21\u578b\u6d4b\u8bd5\u5b8c\u6210", summary, failed === 0);
    }).finally(function () {
      button.disabled = false;
      button.textContent = "\u6d4b\u8bd5\u672c\u9875";
    });
  }

  var scanTimer = 0;
  function scheduleScan() {
    if (scanTimer) return;
    scanTimer = setTimeout(function () {
      scanTimer = 0;
      scanRows();
    }, 250);
  }

  new MutationObserver(scheduleScan).observe(document.documentElement, { childList: true, subtree: true });
  setInterval(scanRows, 2000);
})();
</script>`)

func injectManagementAuthFileTestUI(html []byte) []byte {
	if len(html) == 0 || bytes.Contains(html, []byte("cpa-auth-file-test-ui")) {
		return html
	}
	lower := bytes.ToLower(html)
	idx := bytes.LastIndex(lower, []byte("</body>"))
	if idx < 0 {
		out := make([]byte, 0, len(html)+len(managementAuthFileTestScript))
		out = append(out, html...)
		out = append(out, managementAuthFileTestScript...)
		return out
	}
	out := make([]byte, 0, len(html)+len(managementAuthFileTestScript))
	out = append(out, html[:idx]...)
	out = append(out, managementAuthFileTestScript...)
	out = append(out, html[idx:]...)
	return out
}
