/**
 * Draw a static min/max waveform for an AudioBuffer.
 *
 * @param {HTMLCanvasElement} canvas
 * @param {AudioBuffer}       buffer
 */
export function drawWaveform(canvas, buffer) {
  const W = canvas.width  = canvas.offsetWidth  || 480;
  const H = canvas.height = canvas.offsetHeight || 52;
  const ctx = canvas.getContext('2d');
  const samples = buffer.getChannelData(0);

  ctx.clearRect(0, 0, W, H);

  const step = Math.max(1, Math.floor(samples.length / W));
  ctx.beginPath();
  ctx.strokeStyle = getComputedStyle(canvas).color || '#555';
  ctx.lineWidth = 1;

  for (let x = 0; x < W; x++) {
    let lo = 1, hi = -1;
    const base = x * step;
    for (let i = 0; i < step; i++) {
      const s = samples[base + i] ?? 0;
      if (s < lo) lo = s;
      if (s > hi) hi = s;
    }
    const y0 = H / 2 - hi * (H / 2 - 2);
    const y1 = H / 2 - lo * (H / 2 - 2);
    ctx.moveTo(x + 0.5, y0);
    ctx.lineTo(x + 0.5, y1 + 1);
  }

  ctx.stroke();
}
