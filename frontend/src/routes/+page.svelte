<script lang="ts">
	import type { PageData } from './$types';
	import type { RecordLiftRequest, Movement, Set, NextLiftResponse, SkipOptionalWeekRequest} from '$lib/api';
	import apipath from '$lib/apipath'

	export let data: PageData;

	let note = '';
	let showNote = false;

	$: curMvmt = data.Workout[data.NextMovementIndex];
	$: curSet = curMvmt.Sets[data.NextSetIndex];
	$: reps = curSet.RepTarget;

	const decReps = () => {
		reps -= 1;
	};
	const incReps = () => {
		reps += 1;
	};
	const updateReps = (e: Event) => {
		reps = Number.parseInt((e.target as HTMLInputElement).value);
	};

	const liftInfoStr1 = (mvmt: Movement): string => {
		return `[${mvmt.SetType}] ${mvmt.Exercise}`
	};

	const liftInfoStr2 = (set: Set): string => {
		const suffix = set.ToFailure ? '+' : ''
		return  `${set.WeightTarget.Value / 10} lbs for ${set.RepTarget}${suffix}`
	};

	const setString = (set: Set): string => {
		const suffix = set.ToFailure ? '+' : ''
		return `[${set.TrainingMaxPercentage}%] ${set.WeightTarget.Value / 10} x ${set.RepTarget}${suffix}`
	};

	const record = (numReps: number) => {
		var req = {
			Exercise: curMvmt.Exercise,
			SetType: curMvmt.SetType,
			Weight: (curSet.WeightTarget.Value / 10).toString(),
			Set: data.NextSetIndex,
			Reps: numReps,
			Note: note,
			Day: data.DayNumber,
			Week: data.WeekNumber,
			Iteration: data.IterationNumber
		} as RecordLiftRequest;

		fetch(apipath('/api/recordLift'), { method: 'POST', body: JSON.stringify(req) })
			.then((resp) => resp.json() as Promise<NextLiftResponse>)
			.then((dat) => {
				data = dat
			})
			.finally(() => {
				note = ''
				showNote = false
			});
	};

	const recordLift = () => record(reps);
	const recordSkip = () => record(0);

	const skipOptionalWeek = () => {
		var req = {
			Week: data.WeekNumber,
			Iteration: data.IterationNumber
		} as SkipOptionalWeekRequest;

		fetch(apipath('/api/skipOptionalWeek'), { method: 'POST', body: JSON.stringify(req) })
			.then((resp) => resp.json() as Promise<NextLiftResponse>)
			.then((dat) => {
				data = dat
			})
			.finally(() => {
				note = ''
				showNote = false
			});
	};
</script>

<div class="lifts-page">
	{#if data.OptionalWeek}
			<h1 class="header">Skip optional<br>{data.WeekName}?</h1>
			<button class="dont-skip-button" on:click={() => data.OptionalWeek = false}>Do the week</button>
			<button class="skip-button" on:click={skipOptionalWeek}>Skip it</button>
	{:else}
	<h1 class="header">{data.WeekName} - {data.DayName}</h1>

	<ul class="lift-list">
		{#each data.Workout as mvmt, i}
			<li>
				{mvmt.Exercise} - {mvmt.SetType}
				<ul>
					{#each mvmt.Sets as set, j}
						<li
							class:current-lift={i === data.NextMovementIndex && j === data.NextSetIndex}
							class:completed={i < data.NextMovementIndex ||
								(i == data.NextMovementIndex && j < data.NextSetIndex)}
						>
							{setString(set)}
						</li>
					{/each}
				</ul>
			</li>
		{/each}
	</ul>

	<hr class="spacer">

	<div class="lift-entry">
		<div class="lift-info">
			{liftInfoStr1(curMvmt)}
			<br>
			<strong>{liftInfoStr2(curSet)}</strong>
		</div>

		{#if curSet.ToFailure}
			<div class="lift-input-row">
				<button class="weight-adj-button" on:click={decReps}>-</button>
				<input
					class="lift-input"
					type="number"
					name="Lift Input"
					value={reps}
					on:input={updateReps}
				/>
				<button class="weight-adj-button" on:click={incReps}>+</button>
			</div>
			{#if data.FailureComparables.ClosestWeight}
				<div>Closest Comparison: {data.FailureComparables.ClosestWeight.Weight.Value / 10} x {data.FailureComparables.ClosestWeight.Reps}</div>
			{/if}
			{#if data.FailureComparables.PersonalRecord}
				<div>Lift PR: {data.FailureComparables.PersonalRecord.Weight.Value / 10} x {data.FailureComparables.PersonalRecord.Reps} &thickapprox; {data.FailureComparables.PREquivalentReps} reps</div>
			{/if}
			<div>

			</div>
		{/if}

		<div class="lift-bottom-row">
			<button class="record-button" on:click={recordLift}>Record</button>
			<button class="skip-button" on:click={recordSkip}>Skip</button>
			{#if showNote}
				<textarea class="note" bind:value={note} rows="3" />
			{:else}
				<button class="add-note-button" on:click={() => showNote = true}>Add Note</button>
			{/if}
			<button class="back-button" on:click={() => alert('todo')}>Back</button>
		</div>
	</div>
	{/if}
</div>

<style>
	:global(html) {
		margin: 0;
		height: 100%;
	}

	:global(body) {
		margin: 0;
		height: 100%;
	}

	.header {
		text-align: center;
		margin-bottom: 0;
	}

	.lifts-page {
		display: flex;
		flex-direction: column;
	}

	.current-lift {
		font-weight: bold;
	}

	.lift-list {
		max-height: 60vh;
		overflow-y: scroll;
	}

	.spacer {
		border: none;
		height: 1px;
		background-color: black;
		width: 75%;
		margin-bottom: 20px;
	}

	.lift-entry {
		flex: 1 0;
		height: 80%;
	}

	.lift-input {
		text-align: center;
		font-size: 18px;
		font-weight: bold;
		width: 30px;
		height: 30px;
		margin: 0 15px;
	}

	.completed {
		text-decoration: line-through;
	}

	/* Hide the input buttons and use our custom ones */
	input[type='number'] {
		-webkit-appearance: textfield;
		-moz-appearance: textfield;
		appearance: textfield;
	}
	input[type='number']::-webkit-inner-spin-button,
	input[type='number']::-webkit-outer-spin-button {
		-webkit-appearance: none;
	}

	.record-button, .skip-button, .add-note-button, .back-button {
		display: block;
		width: 50vw;
		height: 30px;
		margin: 15px auto;
	}

	.note {
		display: block;
		width: 50vw;
		margin: 10px auto;
	}

	.lift-info {
		text-align: center;
	}

	.weight-adj-button {
		height: 30px;
		width: 30px;
		font-size: 20px;
	}

	.lift-input-row {
		margin-top: 10px;
		text-align: center;
	}

	.dont-skip-button, .skip-button {
		display: block;
		width: 50vw;
		height: 30px;
		margin: 15px auto;
	}
</style>
