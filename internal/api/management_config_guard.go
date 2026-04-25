package api

import (
	"bytes"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

var managementConfigVersionGuardScript = []byte(`<script id="cliproxy-config-version-guard">
(function () {
  if (window.__cliproxyConfigVersionGuard) return;
  window.__cliproxyConfigVersionGuard = true;
  var latestConfigVersion = "";
  var writeQueue = Promise.resolve();
  var conflictAlertVisible = false;
  var originalFetch = window.fetch ? window.fetch.bind(window) : null;
  if (!originalFetch) return;

  function requestPath(input) {
    try {
      var raw = typeof input === "string" ? input : input && input.url;
      return new URL(raw || "", window.location.href).pathname;
    } catch (_) {
      return "";
    }
  }

  function requestMethod(input, init) {
    return String((init && init.method) || (input && input.method) || "GET").toUpperCase();
  }

  function shouldGuard(method, path) {
    if (method === "GET" || method === "HEAD") return false;
    return path.indexOf("/v0/management/") === 0;
  }

  function observeResponseVersion(response) {
    var nextVersion = response && response.headers && response.headers.get("X-Config-Version");
    if (nextVersion) latestConfigVersion = nextVersion;
  }

  function showConflictAlertOnce() {
    if (conflictAlertVisible) return;
    conflictAlertVisible = true;
    window.alert("配置文件已被其他操作修改，请刷新页面后再保存，避免覆盖最新配置。");
    window.setTimeout(function () { conflictAlertVisible = false; }, 1000);
  }

  function handleConflictResponse(response) {
    if (!response || response.status !== 409) return;
    response.clone().json().then(function (body) {
      if (body && body["current-version"]) latestConfigVersion = body["current-version"];
      if (body && body.error === "config_conflict") showConflictAlertOnce();
    }).catch(function () {});
  }

  function enqueueWrite(task) {
    var previous = writeQueue.catch(function () {});
    var next = previous.then(task, task);
    writeQueue = next.catch(function () {});
    return next;
  }

  window.fetch = async function (input, init) {
    init = init || {};
    var method = requestMethod(input, init);
    var path = requestPath(input);
    var guarded = shouldGuard(method, path);

    var send = async function () {
      var headers = new Headers(init.headers || (input && input.headers) || {});
      if (latestConfigVersion && guarded && !headers.has("If-Match") && !headers.has("X-Config-Version")) {
        headers.set("If-Match", '"' + latestConfigVersion + '"');
        init = Object.assign({}, init, { headers: headers });
      }
      var response = await originalFetch(input, init);
      observeResponseVersion(response);
      handleConflictResponse(response);
      return response;
    };

    if (guarded) {
      return enqueueWrite(send);
    }
    return send();
  };

  if (window.XMLHttpRequest) {
    var originalOpen = window.XMLHttpRequest.prototype.open;
    var originalSend = window.XMLHttpRequest.prototype.send;
    var originalSetRequestHeader = window.XMLHttpRequest.prototype.setRequestHeader;

    window.XMLHttpRequest.prototype.open = function (method, url, async) {
      this.__cliproxyMethod = String(method || "GET").toUpperCase();
      this.__cliproxyPath = requestPath(url || "");
      this.__cliproxyAsync = async !== false;
      this.__cliproxyHeaders = {};
      return originalOpen.apply(this, arguments);
    };

    window.XMLHttpRequest.prototype.setRequestHeader = function (name, value) {
      if (name) this.__cliproxyHeaders[String(name).toLowerCase()] = true;
      return originalSetRequestHeader.apply(this, arguments);
    };

    function observeXHR(xhr, done) {
      xhr.addEventListener("loadend", function () {
        var nextVersion = "";
        try { nextVersion = xhr.getResponseHeader("X-Config-Version") || ""; } catch (_) {}
        if (nextVersion) latestConfigVersion = nextVersion;
        if (xhr.status === 409) {
          try {
            var body = JSON.parse(xhr.responseText || "{}");
            if (body && body["current-version"]) latestConfigVersion = body["current-version"];
            if (body && body.error === "config_conflict") {
              showConflictAlertOnce();
            }
          } catch (_) {}
        }
        if (done) done();
      });
    }

    function attachXHRVersionHeader(xhr) {
      var headers = xhr.__cliproxyHeaders || {};
      if (latestConfigVersion && !headers["if-match"] && !headers["x-config-version"]) {
        originalSetRequestHeader.call(xhr, "If-Match", '"' + latestConfigVersion + '"');
      }
    }

    window.XMLHttpRequest.prototype.send = function () {
      var xhr = this;
      var args = arguments;
      var guarded = shouldGuard(xhr.__cliproxyMethod || "GET", xhr.__cliproxyPath || "");
      if (!guarded || xhr.__cliproxyAsync === false) {
        if (guarded) attachXHRVersionHeader(xhr);
        observeXHR(xhr);
        return originalSend.apply(xhr, args);
      }

      enqueueWrite(function () {
        return new Promise(function (resolve) {
          attachXHRVersionHeader(xhr);
          observeXHR(xhr, resolve);
          originalSend.apply(xhr, args);
        });
      });
      return undefined;
    };
  }
})();
</script>`)

func injectManagementConfigVersionGuard(data []byte) []byte {
	data = removeManagementConfigVersionGuard(data)
	lower := bytes.ToLower(data)
	if idx := bytes.LastIndex(lower, []byte("</body>")); idx >= 0 {
		out := make([]byte, 0, len(data)+len(managementConfigVersionGuardScript))
		out = append(out, data[:idx]...)
		out = append(out, managementConfigVersionGuardScript...)
		out = append(out, data[idx:]...)
		return out
	}
	out := make([]byte, 0, len(data)+len(managementConfigVersionGuardScript))
	out = append(out, data...)
	out = append(out, managementConfigVersionGuardScript...)
	return out
}

func removeManagementConfigVersionGuard(data []byte) []byte {
	start := bytes.Index(data, []byte(`<script id="cliproxy-config-version-guard">`))
	if start < 0 {
		return data
	}
	remaining := data[start:]
	endRel := bytes.Index(bytes.ToLower(remaining), []byte("</script>"))
	if endRel < 0 {
		return data
	}
	end := start + endRel + len("</script>")
	out := make([]byte, 0, len(data)-(end-start))
	out = append(out, data[:start]...)
	out = append(out, data[end:]...)
	return out
}

func serveManagementControlPanelFile(c *gin.Context, filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		c.File(filePath)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", injectManagementConfigVersionGuard(data))
}
