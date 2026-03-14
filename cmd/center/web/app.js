(function () {
  document.addEventListener("DOMContentLoaded", () => {
    if (window.TZ && TZ.dashboard && typeof TZ.dashboard.init === "function") {
      TZ.dashboard.init();
    }
  });
})();

