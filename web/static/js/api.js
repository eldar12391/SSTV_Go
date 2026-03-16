/**
 * POST an image file to /api/encode and return a WAV Blob.
 *
 * @param {File}     imageFile
 * @param {Object}   options
 * @param {number}   options.sampleRate
 * @param {string}   options.audioFormat
 * @param {number}   options.mp3Bitrate
 * @param {Function} onProgress - called with (loaded, total) during upload
 * @returns {Promise<Blob>}
 */
export function encodeImage(imageFile, options = {}, onProgress) {
  const { sampleRate, audioFormat, mp3Bitrate } = options;

  return new Promise((resolve, reject) => {
    const fd = new FormData();
    fd.append('image', imageFile);
    if (sampleRate) fd.append('sample_rate', String(sampleRate));
    if (audioFormat) fd.append('audio_format', String(audioFormat));
    if (mp3Bitrate) fd.append('mp3_bitrate', String(mp3Bitrate));

    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/api/encode');
    xhr.responseType = 'blob';

    xhr.upload.onprogress = e => {
      if (e.lengthComputable) onProgress?.(e.loaded, e.total);
    };

    xhr.onload = () => {
      if (xhr.status === 200) {
        resolve(xhr.response);
      } else {
        xhr.response.text().then(t => reject(new Error(t || xhr.statusText)));
      }
    };

    xhr.onerror = () => reject(new Error('network error'));
    xhr.send(fd);
  });
}

/**
 * POST a WAV file to /api/decode and return a PNG Blob.
 *
 * @param {File}   wavFile
 * @param {Object} settings
 * @param {number} settings.outWidth
 * @param {number} settings.outHeight
 * @param {Object} callbacks
 * @param {Function} callbacks.onUploadProgress - (loaded, total)
 * @param {Function} callbacks.onDownloadProgress - (loaded, total)
 * @returns {Promise<Blob>}
 */
export function decodeAudio(wavFile, settings = {}, callbacks = {}) {
  const { outWidth, outHeight } = settings;
  const { onUploadProgress, onDownloadProgress } = callbacks;

  return new Promise((resolve, reject) => {
    const fd = new FormData();
    fd.append('audio', wavFile);
    if (outWidth) fd.append('out_width', String(outWidth));
    if (outHeight) fd.append('out_height', String(outHeight));

    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/api/decode');
    xhr.responseType = 'blob';

    xhr.upload.onprogress = e => {
      if (e.lengthComputable) onUploadProgress?.(e.loaded, e.total);
    };

    xhr.onprogress = e => {
      onDownloadProgress?.(e.loaded, e.total);
    };

    xhr.onload = () => {
      if (xhr.status === 200) {
        resolve(xhr.response);
      } else {
        xhr.response.text().then(t => reject(new Error(t || xhr.statusText)));
      }
    };

    xhr.onerror = () => reject(new Error('network error'));
    xhr.send(fd);
  });
}
