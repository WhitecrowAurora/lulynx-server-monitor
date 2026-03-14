(function () {
  window.TZ = window.TZ || {};
  const { clamp01 } = TZ.util || {};

  function drawSpark(canvas, vals, color) {
    const ctx = canvas.getContext("2d");
    const w = canvas.width;
    const h = canvas.height;
    ctx.clearRect(0, 0, w, h);

    // grid
    ctx.strokeStyle = "rgba(0,0,0,0.10)";
    ctx.lineWidth = 1;
    const cols = 10;
    const rows = 3;
    for (let i = 1; i < cols; i++) {
      const x = Math.round((w * i) / cols) + 0.5;
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, h);
      ctx.stroke();
    }
    for (let i = 1; i < rows; i++) {
      const y = Math.round((h * i) / rows) + 0.5;
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(w, y);
      ctx.stroke();
    }

    if (!vals || vals.length < 2) return;
    let min = Infinity,
      max = -Infinity;
    for (const v of vals) {
      if (!isFinite(v)) continue;
      min = Math.min(min, v);
      max = Math.max(max, v);
    }
    if (!isFinite(min) || !isFinite(max)) return;
    if (min === max) {
      min -= 1;
      max += 1;
    }

    ctx.strokeStyle = color;
    ctx.lineWidth = 2;
    ctx.beginPath();
    for (let i = 0; i < vals.length; i++) {
      const x = (i / (vals.length - 1)) * (w - 6) + 3;
      const t = (vals[i] - min) / (max - min);
      const y = (1 - clamp01(t)) * (h - 6) + 3;
      if (i === 0) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    }
    ctx.stroke();
  }

  TZ.spark = { drawSpark };
})();

