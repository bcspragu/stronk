<script lang="ts">
	import { goto } from '$app/navigation';
	import apipath from '$lib/apipath'
	import type { SetTrainingMaxesRequest } from '$lib/api';

	let press: number | undefined;
	let squat: number | undefined;
	let bench: number | undefined;
	let deadlift: number | undefined;
	let smallestDenom: number | undefined;

	$: canSubmit =
		press !== undefined &&
		squat !== undefined &&
		bench !== undefined &&
		deadlift !== undefined &&
		smallestDenom !== undefined;

	const setTrainingMaxes = () => {
		if (!canSubmit) {
			return;
		}

		var req = {
			OverheadPress: press?.toString(),
			Squat: squat?.toString(),
			BenchPress: bench?.toString(),
			Deadlift: deadlift?.toString(),
			SmallestDenom: smallestDenom?.toString()
		} as SetTrainingMaxesRequest;

		fetch(apipath('/api/setTrainingMaxes'), {
			method: 'POST',
			body: JSON.stringify(req)
		}).then(() => {
			goto('/lifts');
		});
	};
</script>

<h1>Enter Training Maxes</h1>

<p>Your one rep max can be calculated as: Weight + Weight * Num reps * 0.0333333</p>
<p>Your training max should be 90% of your one rep max.</p>

<input type="number" bind:value={press} placeholder="Press Max" name="Press" />
<input type="number" bind:value={squat} placeholder="Squat Max" name="Squat" />
<input type="number" bind:value={bench} placeholder="Bench Max" name="Bench" />
<input type="number" bind:value={deadlift} placeholder="Deadlift Max" name="Deadlift" />
<label for="smallest-plate-input">Smallest Plate</label>
<select bind:value={smallestDenom} name="Smallest Plate">
	<option value={undefined} selected>Please choose</option>
	<option value={1.25}>1.25</option>
	<option value={2.5}>2.5</option>
	<option value={5}>5</option>
</select>
<button on:click={setTrainingMaxes} disabled={!canSubmit}>Enter</button>
