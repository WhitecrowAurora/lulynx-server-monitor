(function () {
  document.addEventListener("DOMContentLoaded", () => {
    if (window.TZ && TZ.admin && typeof TZ.admin.init === "function") {
      TZ.admin.init();
    }
  });
})();

