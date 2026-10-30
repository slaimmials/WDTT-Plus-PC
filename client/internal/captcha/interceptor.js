(function () {
  if (window.__wdttCaptcha) return;
  window.__wdttCaptcha = true;

  function report(data) {
    var token = data && data.response && data.response.success_token;
    if (token) window.wdttCaptchaSolved(token);
  }

  function isCheck(url) {
    return String(url || '').indexOf('captchaNotRobot.check') !== -1;
  }

  var originalFetch = window.fetch;
  window.fetch = function (input, init) {
    var url = typeof input === 'string' ? input : (input && input.url);
    var request = originalFetch.apply(this, arguments);
    if (isCheck(url)) {
      request.then(function (response) {
        response.clone().json().then(report).catch(function () {});
      }).catch(function () {});
    }
    return request;
  };

  var originalOpen = XMLHttpRequest.prototype.open;
  var originalSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function (method, url) {
    this.__wdttUrl = url;
    return originalOpen.apply(this, arguments);
  };
  XMLHttpRequest.prototype.send = function () {
    if (isCheck(this.__wdttUrl)) {
      this.addEventListener('load', function () {
        try { report(JSON.parse(this.responseText)); } catch (e) {}
      });
    }
    return originalSend.apply(this, arguments);
  };
})();
