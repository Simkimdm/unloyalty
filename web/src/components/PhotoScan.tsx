import { useState } from 'preact/hooks';
import { FORMATS } from '../formats/formats';
import type { DecodeMapping } from '../formats/fromDecoder';
import type { PendingBarcode } from '../pending';
import { decodeImage } from '../scan/decoder';

interface Props {
  onDetected: (barcode: PendingBarcode) => void;
}

/** Longest side of a photo handed to the decoder; larger images only cost memory, not accuracy. */
const MAX_PHOTO_SIDE = 2048;

/** Decode a picked photo, downscaled so a huge image can't exhaust the phone's memory in the WASM decoder. */
async function decodePhoto(file: Blob): Promise<DecodeMapping[]> {
  const bmp = await createImageBitmap(file);
  try {
    const scale = Math.min(1, MAX_PHOTO_SIDE / Math.max(bmp.width, bmp.height));
    const canvas = document.createElement('canvas');
    canvas.width = Math.max(1, Math.round(bmp.width * scale));
    canvas.height = Math.max(1, Math.round(bmp.height * scale));
    const ctx = canvas.getContext('2d');
    if (!ctx) return decodeImage(file);
    ctx.drawImage(bmp, 0, 0, canvas.width, canvas.height);
    return decodeImage(ctx.getImageData(0, 0, canvas.width, canvas.height));
  } finally {
    bmp.close();
  }
}

/** "Scan from photo": decode a picked image; works without camera permission and for screenshots. */
export function PhotoScan({ onDetected }: Props) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [choices, setChoices] = useState<PendingBarcode[]>([]);

  async function onPhoto(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) return;
    setBusy(true);
    setError(null);
    setChoices([]);
    let results: DecodeMapping[];
    try {
      results = await decodePhoto(file);
    } catch (err) {
      setBusy(false);
      setError(`Couldn’t read that image (${err instanceof Error ? err.message : String(err)})`);
      return;
    }
    setBusy(false);
    const found = results.flatMap((r) => (r.ok ? [{ format: r.format, value: r.value }] : []));
    if (found.length === 1) return onDetected(found[0]!);
    if (found.length > 1) return setChoices(found);
    const unsupported = results.find((r) => !r.ok);
    setError(unsupported && !unsupported.ok ? unsupported.error : 'No barcode found. Try a sharper, closer photo, or type the number in.');
  }

  return (
    <>
      <label class={`choice ${busy ? 'busy' : ''}`}>
        <strong>{busy ? 'Reading photo…' : 'Scan from photo'}</strong>
        <span class="muted">Pick a photo or screenshot of the barcode</span>
        <input type="file" accept="image/*" class="visually-hidden" onChange={onPhoto} disabled={busy} />
      </label>

      {error && (
        <p class="form-error" role="alert">
          {error}
        </p>
      )}

      {choices.length > 0 && (
        <section>
          <h2>Several barcodes found</h2>
          <ul class="list">
            {choices.map((c) => (
              <li key={`${c.format}:${c.value}`}>
                <button type="button" class="list-button" onClick={() => onDetected(c)}>
                  <span>{FORMATS[c.format].label}</span>
                  <span class="mono">{c.value}</span>
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
    </>
  );
}
