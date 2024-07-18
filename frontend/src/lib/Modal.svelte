<script lang="ts">
	import { createEventDispatcher } from 'svelte';

	interface Emit {
		close: void;
	}
	const dispatch = createEventDispatcher<Emit>();
	export let active = false;

	const close = () => {
		dispatch('close');
	};
</script>

<div class="modal hidden" class:hidden={!active}>
	<div class="modal-content">
		<button class="close" on:click={close} on:keypress={close}>&times;</button>
		<slot />
	</div>
</div>

<style>
	.hidden {
		display: none;
	}

	.modal {
		position: fixed;
		z-index: 1;
		left: 0;
		top: 0;
		width: 100%;
		height: 100%;
		overflow: auto;
		background-color: rgba(0, 0, 0, 0.4);
	}

	.modal-content {
		background-color: #fefefe;
		margin: 15% auto;
		padding: 20px;
		border: 1px solid #888;
		width: 80%;
	}

	/* The Close Button */
	.close {
		float: right;

		font: inherit;
		color: #aaa;
		font-size: 28px;
		font-weight: bold;

		background: none;
		border: none;
		padding: 0;
		margin: 0;
		cursor: pointer;
	}

	.close:hover,
	.close:focus {
		color: black;
		text-decoration: none;
		cursor: pointer;
	}
</style>
