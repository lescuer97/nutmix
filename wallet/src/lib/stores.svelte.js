import { browser } from '$app/environment';
import { writable } from 'svelte/store';

/**
 * @function setUpLocalStorageWritableStore
 * @description A function to set up a writable store that is persisted to local storage
 * @param {string} key
 * @param {any} defaultValue
 * @returns {import("svelte/store").Writable<any>}
 */
function setUpLocalStorageWritableStore(key, defaultValue) {
	let storedValue = browser ? window.localStorage.getItem(key) : null;

	if (storedValue) {
		storedValue = JSON.parse(storedValue);

		if (defaultValue instanceof Map) {
			storedValue = new Map(storedValue);
		}
	}

	const initialValue = storedValue ? storedValue : defaultValue;
	const store = writable(initialValue);

	store.subscribe((value) => {
		let valueToStore = value;
		if (browser) {
			if (value instanceof Map) {
				valueToStore = Array.from(valueToStore);
			}
			window.localStorage.setItem(key, JSON.stringify(valueToStore));
		}
	});
	return store;
}

/**
 * @function setUpLocalStorageWritableStore
 * @description A function to set up a writable store that is persisted to local storage
 * @param {string} key
 * @param {any} defaultValue
 */
function setupLocalStorageRune(key, defaultValue) {
	let storedValue = browser ? window.localStorage.getItem(key) : null;

	if (storedValue) {
		storedValue = JSON.parse(storedValue);

		if (defaultValue instanceof Map) {
			storedValue = new Map(storedValue);
		}
	}

	const initialValue = storedValue ? storedValue : defaultValue;
	let rune = $state(initialValue);

	return {
		get [key]() {
			return rune;
		},

		set: (value) => {
			let valueToStore = value;
			if (browser) {
				if (value instanceof Map) {
					valueToStore = Array.from(valueToStore);
				}
				window.localStorage.setItem(key, JSON.stringify(valueToStore));
			}
			rune = value;
		}
	};
}

/** @type {import("svelte/store").Writable<string[]>} */
export const mint_urls = setUpLocalStorageWritableStore('mint_urls', ['http://127.0.0.1:3338']);

/** @type {import("svelte/store").Writable<number>} */
export const current_mint_index = setUpLocalStorageWritableStore('current_mint_index', 0);


export const proofs_store = setupLocalStorageRune('proofs', new Map());

export const pending_mint_store = setupLocalStorageRune('pending_mint', new Map());

export const keysets_store = setupLocalStorageRune('keysets', []);
