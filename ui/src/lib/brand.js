// Color and logo of the selected server: --accent (and its soft and text colors) follow its accent, the logo
// replaces the initials. Both are set by the owner (Plus › Personnalisation).
export const DEFAULT_ACCENT = '#F5A524';

export const SWATCHES = [
  ['#F5A524', 'Ambre'], ['#FF5A4F', 'Rouge'], ['#3E8BFF', 'Bleu'], ['#3DD68C', 'Vert'], ['#B98CFF', 'Violet'], ['#2EC5CE', 'Turquoise']
];

const rgb = hex => [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16));

/** Text on the accent: dark on a light color, white on a dark one. */
function onAccent(hex) {
  const [r, g, b] = rgb(hex).map(v => {
    const c = v / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b > 0.25 ? '#11141A' : '#FFFFFF';
}

export function applyAccent(hex) {
  const c = /^#[0-9a-fA-F]{6}$/.test(hex ?? '') ? hex : DEFAULT_ACCENT;
  const [r, g, b] = rgb(c);
  const s = document.documentElement.style;
  s.setProperty('--accent', c);
  s.setProperty('--accent-soft', `rgba(${r}, ${g}, ${b}, .15)`);
  s.setProperty('--on-accent', onAccent(c));
}

export const logoUrl = server => (server?.logo_ts ? `/api/servers/${server.id}/logo.png?v=${server.logo_ts}` : '');

/** Any picture (PNG, JPEG, SVG…) → square PNG data URL of `size` px, cropped to the center. */
export function toSquarePng(file, size = 256) {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      const side = Math.min(img.naturalWidth || size, img.naturalHeight || size);
      const sx = ((img.naturalWidth || size) - side) / 2, sy = ((img.naturalHeight || size) - side) / 2;
      const cv = document.createElement('canvas');
      cv.width = cv.height = size;
      cv.getContext('2d').drawImage(img, sx, sy, side, side, 0, 0, size, size);
      URL.revokeObjectURL(url);
      resolve(cv.toDataURL('image/png'));
    };
    img.onerror = () => { URL.revokeObjectURL(url); reject(new Error('image illisible')); };
    img.src = url;
  });
}
