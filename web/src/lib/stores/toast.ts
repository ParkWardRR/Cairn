import { writable } from 'svelte/store';

interface ToastMessage {
  id: number;
  text: string;
}

let nextId = 0;

function createToastStore() {
  const { subscribe, update } = writable<ToastMessage[]>([]);

  return {
    subscribe,
    show(text: string, duration = 3000) {
      const id = nextId++;
      update(all => [...all, { id, text }]);
      setTimeout(() => {
        update(all => all.filter(t => t.id !== id));
      }, duration);
    }
  };
}

export const toasts = createToastStore();
