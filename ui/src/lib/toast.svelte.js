// One short message at a time at the bottom of the screen ("Copié", "Déconnecté"…).
export const toast = $state({ msg: '' });
let timer;

export function say(msg) {
  clearTimeout(timer);
  toast.msg = msg;
  timer = setTimeout(() => { toast.msg = ''; }, 2600);
}
