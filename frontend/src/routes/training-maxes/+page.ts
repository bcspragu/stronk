import apipath from '$lib/apipath';
import type { PageLoad } from './$types';
import type { TrainingMaxesResponse } from '$lib/api';

export const load: PageLoad = async ({ fetch }) => {
	const res = await fetch(apipath('/api/trainingMaxes'));
	const data: TrainingMaxesResponse = await res.json();
	return data;
};
