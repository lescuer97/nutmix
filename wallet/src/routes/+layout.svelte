<script>
	import {
		mint_urls,
		pending_mint_store,
		proofs_store,
		current_mint_index
	} from '$lib/stores.svelte';

	// import {  mint_urls, pending_mint, proofs, update_requested_mints, current_mint_index, update_requested_proofs } from '$lib/stores';
	import { CashuMint, CashuWallet, decodeInvoice } from '@cashu/cashu-ts';
	import { onMount } from 'svelte';

	let mint_wallet = $mint_urls[$current_mint_index];
	let cashu_mint = new CashuMint(mint_wallet);
	let wallet = new CashuWallet(cashu_mint);

	onMount(() => {
		const keys_of_proofs = Array.from(pending_mint_store.pending_mint.keys());

		/** @type {Map<String, import('@cashu/cashu-ts').Proof[]>} */
		const final_proof_map = proofs_store.proofs;

		// look at every available mint and check if there are any pending proofs to mint
		for (let i = 0; i < keys_of_proofs.length; i++) {
			const mint = keys_of_proofs[i];

			let minting_to_check = pending_mint_store.pending_mint.get(mint);

			/** @type {import('@cashu/cashu-ts').Proof[][]} */
			const new_proofs = [];

			/** @type {number[]} */
			const indexes_to_remove = [];

			if (minting_to_check) {
				const pending_mint_request = minting_to_check.map((request) => {
					let decodedInvoice = decodeInvoice(request.pr);

					return wallet.requestTokens(decodedInvoice.amountInSats, request.hash);
				});

				Promise.allSettled(pending_mint_request)
					.then((res) => {
						for (let i = 0; i < res.length; i++) {
							const proof_result = res[i];

							if (proof_result.status == 'rejected') {
								if (proof_result.reason.message == 'quote already issued') {
									indexes_to_remove.push(i);
								}
							}
							if (proof_result.status == 'fulfilled') {
								new_proofs.push(proof_result?.value?.proofs);
								indexes_to_remove.push(i);
							}
						}

						const mint_to_modify = pending_mint_store.pending_mint;

						const newArr =
							minting_to_check?.filter((_, index) => !indexes_to_remove.includes(index)) ?? [];
						let new_mint_to_check = mint_to_modify.set(mint, newArr);
						pending_mint_store.set(new_mint_to_check);

						let new_set_proofs = final_proof_map.get(mint) ?? [];
						new_set_proofs.push(...new_proofs.flat());
						final_proof_map.set(mint, new_set_proofs);

						proofs_store.set(final_proof_map);
					})
					.catch((e) => {
						console.log({ e });
					});
			}
		}
	});
</script>

<slot />
