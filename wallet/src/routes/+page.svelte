<script>
	import { mint_urls, pending_mint_store, current_mint_index } from '$lib/stores.svelte';
	import { CashuMint, CashuWallet, decodeInvoice } from '@cashu/cashu-ts';

	let mint_wallet = $mint_urls[$current_mint_index];
	let cashu_mint = new CashuMint(mint_wallet);
	let wallet = new CashuWallet(cashu_mint);

	async function request_mint() {
		let req = await wallet.requestMint(10000);
		let new_pending_mint = new Map(pending_mint_store.pending_mint);

		let pending_mint_values = new_pending_mint.get(mint_wallet);

		if (pending_mint_values) {
			pending_mint_values.push(req);
			new_pending_mint.set(mint_wallet, pending_mint_values);
		} else {
			new_pending_mint.set(mint_wallet, [req]);
		}

		pending_mint_store.set(new_pending_mint);
	}
</script>

<h1>Welcome to SvelteKit</h1>
<button on:click={request_mint}>Request Mint</button>

<ul>
	{#each Array.from(pending_mint_store.pending_mint.values())?.[0] ?? [] as { pr, hash }, i}
		<li>
			<span>Hash: {hash}</span>
			<span>pr: {pr}</span>
		</li>
	{/each}
</ul>

<a href="/recieve">recieve</a>
