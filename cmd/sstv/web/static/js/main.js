import { makeDropzone } from './dropzone.js';
import { drawWaveform }  from './waveform.js';
import { encodeImage, decodeAudio } from './api.js';

// ── Element refs ──────────────────────────────────────────────────────────────
const $ = id => document.getElementById(id);

const txDrop    = $('tx-drop');
const txPreview = $('tx-preview');
const txStatus  = $('tx-status');
const txProgress= $('tx-progress');
const txWave    = $('tx-wave');
const txAudioFormat = $('tx-audio-format');
const txSampleRate = $('tx-sample-rate');
const txMp3BitrateWrap = $('tx-mp3-bitrate-wrap');
const txMp3Bitrate = $('tx-mp3-bitrate');
const btnEncode = $('btn-encode');

const rxDrop    = $('rx-drop');
const rxStatus  = $('rx-status');
const rxProgress= $('rx-progress');
const rxCanvas  = $('rx-canvas');
const rxMeta    = $('rx-meta');
const rxPlaceholder = $('rx-placeholder');
const rxOutWidth = $('rx-out-width');
const rxOutHeight = $('rx-out-height');
const rxRenderSpeed = $('rx-render-speed');
const btnDecode = $('btn-decode');
const btnDlImg  = $('btn-dl-img');

// ── State ─────────────────────────────────────────────────────────────────────
let txFile  = null;   // File (image)
let rxFile  = null;   // File (wav)
let rxBlob  = null;   // PNG Blob from server
const rxCtx = rxCanvas.getContext('2d');

function updateTxAudioControls() {
  const isMP3 = (txAudioFormat.value || 'wav').toLowerCase() === 'mp3';
  txMp3Bitrate.disabled = !isMP3;
  txMp3BitrateWrap.style.opacity = isMP3 ? '1' : '.5';
}

updateTxAudioControls();
txAudioFormat.addEventListener('change', updateTxAudioControls);

// ── Helpers ───────────────────────────────────────────────────────────────────
function setStatus(el, text, type = 'idle') {
  el.textContent = text;
  el.dataset.type = type;
}

function setProgress(el, pct) {
  el.style.setProperty('--pct', `${Math.round(pct)}%`);
}

function download(blob, name) {
  const url = URL.createObjectURL(blob);
  Object.assign(document.createElement('a'), { href: url, download: name }).click();
  URL.revokeObjectURL(url);
}

function resetRxCanvas() {
  rxCanvas.width = 320;
  rxCanvas.height = 256;
  rxCtx.clearRect(0, 0, rxCanvas.width, rxCanvas.height);
  rxCanvas.classList.remove('--visible');
}

async function animateRowsFromBlob(blob, rowsPerFrame, onRow) {
  const bitmap = await createImageBitmap(blob);
  const src = document.createElement('canvas');
  src.width = bitmap.width;
  src.height = bitmap.height;
  const srcCtx = src.getContext('2d');
  srcCtx.drawImage(bitmap, 0, 0);
  bitmap.close();

  rxCanvas.width = src.width;
  rxCanvas.height = src.height;
  rxCtx.clearRect(0, 0, src.width, src.height);

  let y = 0;
  const rpf = Math.max(1, rowsPerFrame || 6);

  return new Promise(resolve => {
    const step = () => {
      const rows = Math.min(rpf, src.height - y);
      if (rows <= 0) {
        resolve();
        return;
      }

      rxCtx.drawImage(src, 0, y, src.width, rows, 0, y, src.width, rows);
      y += rows;
      onRow?.(y, src.height);
      requestAnimationFrame(step);
    };
    requestAnimationFrame(step);
  });
}

// ── TX — image drop ───────────────────────────────────────────────────────────
makeDropzone(txDrop, 'image/*', file => {
  txFile = file;
  btnEncode.disabled = false;
  setStatus(txStatus, file.name, 'done');
  setProgress(txProgress, 0);

  // Show preview
  const img = new Image();
  img.onload = () => {
    img.style.cssText = 'display:block;width:100%;max-height:220px;object-fit:contain;border:1px solid #242424;border-radius:2px';
    txPreview.innerHTML = '';
    txPreview.appendChild(img);
  };
  img.src = URL.createObjectURL(file);
});

// ── TX — encode ───────────────────────────────────────────────────────────────
btnEncode.addEventListener('click', async () => {
  if (!txFile) return;

  btnEncode.disabled = true;
  setStatus(txStatus, 'загрузка…', 'working');
  setProgress(txProgress, 0);

  try {
    const sampleRate = Number(txSampleRate.value) || 44100;
    const audioFormat = (txAudioFormat.value || 'wav').toLowerCase();
    const mp3Bitrate = Number(txMp3Bitrate.value) || 192;

    const audioBlob = await encodeImage(txFile, { sampleRate, audioFormat, mp3Bitrate }, (loaded, total) => {
      setProgress(txProgress, (loaded / total) * 50); // upload = first 50 %
      setStatus(txStatus, `загрузка ${Math.round(loaded/total*100)} %`, 'working');
    });

    setProgress(txProgress, 60);
    setStatus(txStatus, 'кодирование на сервере…', 'working');

    const arrayBuf = await audioBlob.arrayBuffer();
    let duration = null;
    try {
      const audioCtx = new AudioContext();
      const audioBuf = await audioCtx.decodeAudioData(arrayBuf.slice(0));
      drawWaveform(txWave, audioBuf);
      duration = audioBuf.duration;
      await audioCtx.close();
    } catch (_) {}

    setProgress(txProgress, 100);
    const kb = (audioBlob.size / 1024).toFixed(0);
    const durationText = duration ? `${duration.toFixed(1)} c` : 'аудио готово';
    const extra = audioFormat === 'mp3' ? ` · ${mp3Bitrate} kbps` : '';
    setStatus(txStatus, `${durationText} · ${kb} KB · ${sampleRate} Hz${extra} — скачивание`, 'done');

    const outName = audioFormat === 'mp3' ? 'sstv_signal.mp3' : 'sstv_signal.wav';
    download(audioBlob, outName);
  } catch (err) {
    setStatus(txStatus, `ошибка: ${err.message}`, 'error');
  } finally {
    btnEncode.disabled = false;
  }
});

// ── RX — audio drop ───────────────────────────────────────────────────────────
makeDropzone(rxDrop, 'audio/wav,audio/*', file => {
  rxFile = file;
  btnDecode.disabled = false;
  rxBlob = null;
  setStatus(rxStatus, file.name, 'done');
  setProgress(rxProgress, 0);
  rxMeta.textContent = '';
  rxPlaceholder.style.display = 'block';
  resetRxCanvas();
  btnDlImg.disabled = true;
});

// ── RX — decode ───────────────────────────────────────────────────────────────
btnDecode.addEventListener('click', async () => {
  if (!rxFile) return;

  btnDecode.disabled = true;
  btnDlImg.disabled  = true;
  setStatus(rxStatus, 'загрузка…', 'working');
  setProgress(rxProgress, 0);
  rxPlaceholder.style.display = 'none';
  resetRxCanvas();

  try {
    const outWidth = Number(rxOutWidth.value) || 320;
    const outHeight = Number(rxOutHeight.value) || 256;
    const renderRows = Number(rxRenderSpeed.value) || 6;

    rxBlob = await decodeAudio(rxFile, { outWidth, outHeight }, {
      onUploadProgress: (loaded, total) => {
        const ratio = total > 0 ? loaded / total : 0;
        setProgress(rxProgress, ratio * 55);
        setStatus(rxStatus, `загрузка ${Math.round(ratio*100)} %`, 'working');
      },
      onDownloadProgress: (loaded, total) => {
        const ratio = total > 0 ? loaded / total : 0;
        setProgress(rxProgress, 55 + ratio * 25);
        setStatus(rxStatus, 'декодирование на сервере…', 'working');
      }
    });

    setProgress(rxProgress, 82);
    setStatus(rxStatus, 'отрисовка строк…', 'working');
    rxCanvas.classList.add('--visible');

    await animateRowsFromBlob(rxBlob, renderRows, (row, totalRows) => {
      const ratio = totalRows > 0 ? row / totalRows : 1;
      setProgress(rxProgress, 82 + ratio * 18);
      setStatus(rxStatus, `отрисовка строк ${row}/${totalRows}`, 'working');
    });

    setProgress(rxProgress, 100);
    setStatus(rxStatus, `${rxCanvas.width} × ${rxCanvas.height} — готово`, 'done');
    rxMeta.textContent = `${rxCanvas.width} × ${rxCanvas.height} px · Martin M1`;
    btnDlImg.disabled = false;
  } catch (err) {
    rxPlaceholder.style.display = 'block';
    resetRxCanvas();
    setStatus(rxStatus, `ошибка: ${err.message}`, 'error');
  } finally {
    btnDecode.disabled = false;
  }
});

btnDlImg.addEventListener('click', () => {
  if (rxBlob) download(rxBlob, 'sstv_decoded.png');
});
