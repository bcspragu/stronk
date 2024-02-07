<script lang="ts">
	import type { PageData } from './$types';
	import { goto } from '$app/navigation';
	import apipath from '$lib/apipath';
	import type { Exercise, SetTrainingMaxesRequest } from '$lib/api';

	export let data: PageData;

	const getTM = (ex: Exercise): number | undefined => {
		for (const tm of data.TrainingMaxes) {
			if (tm.Exercise === ex) {
				return tm.Max.Value / 10;
			}
		}
		return undefined;
	};

	let press = getTM('OVERHEAD_PRESS');
	let squat = getTM('SQUAT');
	let bench = getTM('BENCH_PRESS');
	let deadlift = getTM('DEADLIFT');
	let latestFailureSets = data.LatestFailureSets ?? [];

	// The smallest denominator is the minimal delta between two loads that you
	// can do with your equipment, the smallest increment of change. E.g what's the
	// smallest amount of weight you can put on a bar over 100 pounds?
	// - If you only have 5 lb plates, it's 110.
	// - 2.5 lb plates? 105.
	// - 1.25 lb plates? 102.5
	//
	// We don't allow lower than that because it doesn't play nice with our silly
	// decipound system, and at that point we're getting into rounding noise anyway,
	// e.g. the barbell collars would probably throw off your .625 increment.
	let smallestDenom = data.SmallestDenom ? data.SmallestDenom : undefined;

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
			SmallestDenom: smallestDenom || ''
		} as SetTrainingMaxesRequest;

		fetch(apipath('/api/setTrainingMaxes'), {
			method: 'POST',
			body: JSON.stringify(req)
		}).then(() => {
			goto('/');
		});
	};
</script>

<h1>Enter Training Maxes</h1>

<p>Your one rep max can be calculated as: Weight + Weight * Num reps * 0.0333333</p>
<p>Your training max should be 90% of your one rep max.</p>

<div>
	<label for="Press">Press</label>
	<input type="number" bind:value={press} placeholder="Press Max" name="Press" />
</div>

<div>
	<label for="Squat">Squat</label>
	<input type="number" bind:value={squat} placeholder="Squat Max" name="Squat" />
</div>

<div>
	<label for="Bench">Bench</label>
	<input type="number" bind:value={bench} placeholder="Bench Max" name="Bench" />
</div>

<div>
	<label for="Deadlift">Deadlift</label>
	<input type="number" bind:value={deadlift} placeholder="Deadlift Max" name="Deadlift" />
</div>

<br />
<label for="smallest-plate-input">Smallest Plate</label>
<select bind:value={smallestDenom} name="Smallest Plate">
	<option value={undefined} selected>Please choose</option>
	<option value={'1.25'}>1.25</option>
	<option value={'2.5'}>2.5</option>
	<option value={'5'}>5</option>
</select>
<br />
<button on:click={setTrainingMaxes} disabled={!canSubmit}>Enter</button>
<hr>
<h2>Previous Cycle</h2>
{#each latestFailureSets as week, i}
	<h3>Week {i+1}</h3>
	<ul>
	{#each week as lift, j}
			<li>{lift.Exercise}: {lift.Weight.Value / 10} for {lift.Reps} reps {#if lift.Note}{lift.Note}{/if}</li>
	{/each}
	</ul>
{/each}
