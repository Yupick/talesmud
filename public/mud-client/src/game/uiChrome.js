import { writable } from 'svelte/store';

export const accountMenuOpen = writable(false);
export const auditOpen = writable(false);
export const battleDockOpen = writable(false);
export const cheatSheetOpen = writable(false);
export const clanPanelOpen = writable(false);
export const classesEditorOpen = writable(false);
export const dmsPanelOpen = writable(false);
export const layoutDialogOpen = writable(false);
export const dungeonBuilderOpen = writable(false);
export const npcFactoryOpen = writable(false);
export const pickupPanelOpen = writable(false);
export const spellbookOpen = writable(false);

let closeLayoutDialog = () => {};

export function setLayoutDialogCloser(fn) {
  closeLayoutDialog = typeof fn === 'function' ? fn : () => {};
}

export function requestCloseLayoutDialog() {
  closeLayoutDialog();
  layoutDialogOpen.set(false);
}
