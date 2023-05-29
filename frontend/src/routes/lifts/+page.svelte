<script lang="ts">
	import type { PageData } from './$types';
	import type { Movement, Set } from './+page.server';
	import { PUBLIC_API_BASE_URL } from '$env/static/public';
	import { browser } from '$app/environment';

	export let data: PageData;

	let note = '';

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

	const liftInfoStr = (mvmt: Movement, set: Set): string => {
		let info =
			mvmt.SetType +
			': ' +
			mvmt.Exercise +
			' ' +
			set.WeightTarget.Value / 10 +
			' for ' +
			set.RepTarget;
		if (set.ToFailure) {
			info += '+';
		}
		return info;
	};

	const setString = (set: Set): string => {
		let setStr = set.RepTarget.toString();
		if (set.ToFailure) {
			setStr += '+';
		}
		setStr += ' @ ' + set.TrainingMaxPercentage + '%';
		return setStr;
	};

	const record = (numReps: number) => {
		var req = {
			exercise: curMvmt.Exercise,
			set_type: curMvmt.SetType,
			weight: (curSet.WeightTarget.Value / 10).toString(),
			set: data.NextSetIndex,
			reps: numReps,
			note,
			day: data.DayNumber,
			week: data.WeekNumber,
			iteration: data.IterationNumber
		};

		const baseURL = browser ? '' : PUBLIC_API_BASE_URL;
		fetch(`${baseURL}/api/recordLift`, { method: 'POST', body: JSON.stringify(req) })
			.then((resp) => resp.json())
			.then((dat) => (data = dat))
			.finally(() => (note = ''));
	};

	const recordLift = () => record(reps);
	const recordSkip = () => record(0);
</script>

<div class="lifts-page">
	<h1>{data.WeekName} - {data.DayName}</h1>

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

	<div class="lift-entry">
		<div>{liftInfoStr(curMvmt, curSet)}</div>

		{#if curSet.ToFailure}
			<div class="lift-input-row">
				<button on:mousedown={decReps}>-</button>
				<input
					class="lift-input"
					type="number"
					name="Lift Input"
					value={reps}
					on:input={updateReps}
				/>
				<button on:mousedown={incReps}>+</button>
			</div>
		{/if}

		<div class="lift-bottom-row">
			<textarea bind:value={note} cols="10" rows="1" />
			<button on:mousedown={recordLift}>Record</button>
			<button on:mousedown={recordSkip}>Skip</button>
		</div>
	</div>
</div>

<style>
	.lifts-page {
		display: flex;
		flex-direction: column;
		min-height: 100%;
	}

	.current-lift {
		font-weight: bold;
	}

	.lift-list {
		flex: 0 1;
		max-height: 80vh;
		overflow-y: scroll;
	}

	.lift-entry {
		border-top: 1px solid black;
		flex: 1 0;
		height: 80%;
	}

	.lift-input {
		width: 30px;
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
</style>
