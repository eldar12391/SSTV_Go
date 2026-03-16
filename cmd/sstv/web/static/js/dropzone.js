/**
 * Attach drag-and-drop + click-to-select behaviour to an element.
 *
 * @param {HTMLElement} el
 * @param {string}      accept  - MIME type (e.g. "image/*")
 * @param {(File) => void} onFile
 */
export function makeDropzone(el, accept, onFile) {
  const input = el.querySelector('input[type="file"]');
  if (input) input.accept = accept;

  el.addEventListener('click', () => input?.click());
  el.addEventListener('keydown', e => { if (e.key === 'Enter' || e.key === ' ') input?.click(); });

  el.addEventListener('dragover', e => { e.preventDefault(); el.classList.add('--active'); });
  el.addEventListener('dragleave', ()  => el.classList.remove('--active'));
  el.addEventListener('drop', e => {
    e.preventDefault();
    el.classList.remove('--active');
    const f = e.dataTransfer.files[0];
    if (f) onFile(f);
  });

  input?.addEventListener('change', e => {
    const f = e.target.files[0];
    if (f) onFile(f);
  });
}
