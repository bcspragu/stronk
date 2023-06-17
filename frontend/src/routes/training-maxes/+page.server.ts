import apipath from '$lib/apipath'
import type { PageServerLoad } from './$types'
import type { TrainingMaxesResponse } from '$lib/api'

export const load: PageServerLoad = async ({ fetch }) => {
	const res = await fetch(apipath('/api/trainingMaxes'));
	const data: TrainingMaxesResponse = await res.json();
	return data;
};
