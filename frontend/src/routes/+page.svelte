<script lang="ts">
	import type { PageData } from './$types';
	import type {
		RecordLiftRequest,
		Movement,
		Set,
		NextLiftResponse,
		SkipOptionalWeekRequest,
		RecordLiftResponse,
		Lift
	} from '$lib/api';
	import apipath from '$lib/apipath';
	import Modal from '$lib/Modal.svelte';

	export let data: PageData;

	// Standard lift note
	let note = '';
	let showNote = false;

	// When we're actively doing some request
	let updating = false;

	// Note specifically when skipping a deload week
	let skipNote = '';

	// Show editing reps on non-failure sets.
	let showEditReps = false;

	let editingLift: Lift | undefined = undefined;
	let editReps = 0;
	let editNote = '';

	let liftInfo: NextLiftResponse = data;

	$: curMvmt = liftInfo.Workout[liftInfo.NextMovementIndex];
	$: curSet = curMvmt.Sets[liftInfo.NextSetIndex];
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

	const decEditReps = () => {
		editReps -= 1;
	};
	const incEditReps = () => {
		editReps += 1;
	};

	const liftInfoStr1 = (mvmt: Movement): string => {
		return `[${mvmt.SetType}] ${mvmt.Exercise}`;
	};

	const liftInfoStr2 = (set: Set): string => {
		const suffix = set.ToFailure ? '+' : '';
		return `${set.WeightTarget.Value / 10} lbs for ${set.RepTarget}${suffix}`;
	};

	const setString = (set: Set): string => {
		const suffix = set.ToFailure ? '+' : '';
		return `[${set.TrainingMaxPercentage}%] ${set.WeightTarget.Value / 10} x ${
			set.RepTarget
		}${suffix}`;
	};

	const record = (numReps: number) => {
		var req: RecordLiftRequest = {
			Exercise: curMvmt.Exercise,
			SetType: curMvmt.SetType,
			Weight: (curSet.WeightTarget.Value / 10).toString(),
			Set: liftInfo.NextSetIndex,
			Reps: numReps,
			Note: note,
			Day: liftInfo.DayNumber,
			Week: liftInfo.WeekNumber,
			Iteration: liftInfo.IterationNumber,
			ToFailure: curSet.ToFailure
		};

		updating = true;
		fetch(apipath('/api/recordLift'), { method: 'POST', body: JSON.stringify(req) })
			.then((resp) => resp.json() as Promise<RecordLiftResponse>)
			.then((dat) => {
				liftInfo = dat.NextLift;
			})
			.finally(() => {
				skipNote = '';
				note = '';
				showNote = false;
				showEditReps = false;
				updating = false;
			});
	};

	const recordLift = () => record(reps);
	const recordSkip = () => record(0);

	const skipOptionalWeek = () => {
		var req = {
			Week: liftInfo.WeekNumber,
			Iteration: liftInfo.IterationNumber,
			Note: skipNote
		} as SkipOptionalWeekRequest;

		updating = true;
		fetch(apipath('/api/skipOptionalWeek'), { method: 'POST', body: JSON.stringify(req) })
			.then((resp) => resp.json() as Promise<NextLiftResponse>)
			.then((dat) => {
				liftInfo = dat;
			})
			.finally(() => {
				skipNote = '';
				note = '';
				showNote = false;
				showEditReps = false;
				updating = false;
			});
	};

	const setEditingLift = async (liftID?: number) => {
		if (!liftID) {
			return;
		}
		const params = { id: liftID.toString() };
		const res = await fetch(apipath('/api/lift', params));
		const lift: Lift = await res.json();
		editReps = lift.Reps;
		editNote = lift.Note;
		editingLift = lift;
	};

	const clearEditingLift = () => {
		editingLift = undefined;
	};

	const editExistingLift = () => {
		if (!editingLift) {
			return;
		}
		const req = {
			id: editingLift.ID,
			note: editNote,
			reps: editReps
		};
		updating = true;
		fetch(apipath('/api/editLift'), { method: 'POST', body: JSON.stringify(req) })
			.then(clearEditingLift)
			.finally(() => (updating = false));
	};
</script>

<div class="lifts-page">
	<Modal active={!!editingLift} on:close={clearEditingLift}>
		{#if editingLift}
			<h1>Edit Lift ID #{editingLift.ID}</h1>
		{:else}
			<h1>Edit Lift</h1>
		{/if}
		<div class="lift-input-row">
			<button class="weight-adj-button" on:click={decEditReps}>-</button>
			<input class="lift-input" type="number" name="Lift Input" bind:value={editReps} />
			<button class="weight-adj-button" on:click={incEditReps}>+</button>
		</div>
		<textarea class="note" bind:value={editNote} rows="3" />
		<button class="edit-button" on:click={editExistingLift} on:keypress={editExistingLift}
			>Edit</button
		>
	</Modal>

	{#if liftInfo.OptionalWeek}
		<h1 class="header">Skip optional<br />{liftInfo.WeekName}?</h1>
		<button class="dont-skip-button" on:click={() => (liftInfo.OptionalWeek = false)}
			>Do the week</button
		>
		<button class="skip-button" on:click={skipOptionalWeek}>Skip it</button>
		<textarea class="note" bind:value={skipNote} rows="3" />
	{:else}
		<h1 class="header">{liftInfo.WeekName} - {liftInfo.DayName}</h1>

		<ul class="lift-list">
			{#each liftInfo.Workout as mvmt, i}
				<li>
					{mvmt.Exercise} - {mvmt.SetType}
					<ul>
						{#each mvmt.Sets as set, j}
							<li
								class:current-lift={i === liftInfo.NextMovementIndex && j === liftInfo.NextSetIndex}
							>
								<button
									class="lift-button"
									class:completed={i < liftInfo.NextMovementIndex ||
										(i == liftInfo.NextMovementIndex && j < liftInfo.NextSetIndex)}
									on:click={() => setEditingLift(set.AssociatedLiftID)}
								>
									{setString(set)}
								</button>
							</li>
						{/each}
					</ul>
				</li>
			{/each}
		</ul>

		<hr class="spacer" />

		<div class="lift-entry">
			<div class="lift-info">
				{liftInfoStr1(curMvmt)}
				<br />
				<strong>{liftInfoStr2(curSet)}</strong>
			</div>

			{#if curSet.ToFailure || showEditReps}
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
					{#if curSet.FailureComparables?.ClosestWeight}
						<div>
							Closest Comparison: {curSet.FailureComparables.ClosestWeight.Weight.Value / 10} x {curSet
								.FailureComparables.ClosestWeight.Reps}
						</div>
					{/if}
					{#if curSet.FailureComparables?.PersonalRecord}
						<div>
							Lift PR: {curSet.FailureComparables.PersonalRecord.Weight.Value / 10} x {curSet
								.FailureComparables.PersonalRecord.Reps} &thickapprox; {curSet.FailureComparables.PREquivalentReps.toFixed(
								1
							)} reps @ {curSet.WeightTarget.Value / 10} lbs
						</div>
					{/if}
				</div>
				<div />
			{/if}

			<div class="lift-bottom-row">
				<button class="record-button" on:click={recordLift} disabled={updating}>Record</button>
				{#if showNote}
					<textarea class="note" bind:value={note} rows="3" />
				{:else}
					<button class="add-note-button" on:click={() => (showNote = true)}>Add Note</button>
				{/if}
				{#if !showEditReps && !curSet.ToFailure}
					<button class="edit-button" on:click={() => (showEditReps = true)}>Edit Reps</button>
				{/if}
				<button class="skip-button" on:click={recordSkip} disabled={updating}>Skip</button>
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

	.record-button,
	.skip-button,
	.add-note-button,
	.edit-button {
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

	.dont-skip-button,
	.skip-button {
		display: block;
		width: 50vw;
		height: 30px;
		margin: 15px auto;
	}

	.lift-button {
		background: none;
		border: none;
		color: inherit;
		font: inherit;
		padding: 0;
		margin: 0;
		cursor: pointer;
		text-align: left;
	}
</style>
